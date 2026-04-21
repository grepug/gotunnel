package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"net"
	"net/url"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"gotunnel/internal/agent"
	"gotunnel/internal/config"
)

func main() {
	if err := run(os.Args[1:], os.Stdout, os.Stderr); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(args []string, stdout, stderr io.Writer) error {
	if len(args) == 0 {
		return fmt.Errorf("expected a command such as init, set, target, show, or run")
	}

	if strings.HasPrefix(args[0], "-") {
		return runLegacy(args)
	}

	switch args[0] {
	case "init":
		return runInit(args[1:], stdout)
	case "set":
		return runSet(args[1:], stdout)
	case "target":
		return runTarget(args[1:], stdout)
	case "show":
		return runShow(args[1:], stdout)
	case "run":
		return runAgentCommand(args[1:])
	default:
		return fmt.Errorf("unknown command %q", args[0])
	}
}

func runLegacy(args []string) error {
	fs := flag.NewFlagSet("gotunnel", flag.ContinueOnError)
	configPath := fs.String("config", "", "Path to agent JSON config")
	fs.SetOutput(io.Discard)
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *configPath == "" {
		return fmt.Errorf("-config is required")
	}
	return runAgentWithPath(*configPath)
}

func runInit(args []string, stdout io.Writer) error {
	fs := flag.NewFlagSet("init", flag.ContinueOnError)
	configPath := fs.String("config", "", "Path to agent JSON config")
	fs.SetOutput(io.Discard)
	if err := fs.Parse(args); err != nil {
		return err
	}

	path, err := resolveAgentConfigPath(*configPath)
	if err != nil {
		return err
	}
	if _, err := os.Stat(path); err == nil {
		fmt.Fprintf(stdout, "agent config already exists at %s\n", path)
		return nil
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("stat config %s: %w", path, err)
	}

	cfg := config.AgentConfig{
		Targets: []config.TargetMapping{},
	}
	if err := config.SaveAgentConfig(path, cfg); err != nil {
		return err
	}

	fmt.Fprintf(stdout, "initialized agent config at %s\n", path)
	return nil
}

func runSet(args []string, stdout io.Writer) error {
	if len(args) == 0 {
		return fmt.Errorf("expected a set subcommand")
	}

	switch args[0] {
	case "relay":
		return runSetRelay(args[1:], stdout)
	case "auth":
		return runSetAuth(args[1:], stdout)
	default:
		return fmt.Errorf("unknown set subcommand %q", args[0])
	}
}

func runSetRelay(args []string, stdout io.Writer) error {
	fs := flag.NewFlagSet("set relay", flag.ContinueOnError)
	configPath := fs.String("config", "", "Path to agent JSON config")
	relayURL := fs.String("url", "", "Relay URL")
	allowInsecure := fs.Bool("allow-insecure", false, "Allow plain ws relay URLs")
	fs.SetOutput(io.Discard)
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *relayURL == "" {
		return fmt.Errorf("--url is required")
	}

	path, cfg, err := loadDraftConfig(*configPath)
	if err != nil {
		return err
	}
	if err := validateRelayURL(*relayURL, *allowInsecure || cfg.AllowInsecure); err != nil {
		return err
	}
	cfg.RelayURL = *relayURL
	cfg.AllowInsecure = *allowInsecure
	if cfg.Targets == nil {
		cfg.Targets = []config.TargetMapping{}
	}

	if err := config.SaveAgentConfig(path, cfg); err != nil {
		return err
	}
	fmt.Fprintf(stdout, "updated relay settings in %s\n", path)
	return nil
}

func runSetAuth(args []string, stdout io.Writer) error {
	fs := flag.NewFlagSet("set auth", flag.ContinueOnError)
	configPath := fs.String("config", "", "Path to agent JSON config")
	agentID := fs.String("agent-id", "", "Agent ID")
	authToken := fs.String("auth-token", "", "Auth token")
	fs.SetOutput(io.Discard)
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *agentID == "" {
		return fmt.Errorf("--agent-id is required")
	}
	if *authToken == "" {
		return fmt.Errorf("--auth-token is required")
	}

	path, cfg, err := loadDraftConfig(*configPath)
	if err != nil {
		return err
	}
	cfg.AgentID = *agentID
	cfg.AuthToken = *authToken
	if cfg.Targets == nil {
		cfg.Targets = []config.TargetMapping{}
	}

	if err := config.SaveAgentConfig(path, cfg); err != nil {
		return err
	}
	fmt.Fprintf(stdout, "updated auth settings in %s\n", path)
	return nil
}

func runTarget(args []string, stdout io.Writer) error {
	if len(args) == 0 {
		return fmt.Errorf("expected a target subcommand")
	}

	switch args[0] {
	case "add":
		return runTargetAdd(args[1:], stdout)
	default:
		return fmt.Errorf("unknown target subcommand %q", args[0])
	}
}

func runTargetAdd(args []string, stdout io.Writer) error {
	fs := flag.NewFlagSet("target add", flag.ContinueOnError)
	configPath := fs.String("config", "", "Path to agent JSON config")
	name := fs.String("name", "", "Target name")
	localAddr := fs.String("local-addr", "", "Local address")
	fs.SetOutput(io.Discard)
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *name == "" {
		return fmt.Errorf("--name is required")
	}
	if *localAddr == "" {
		return fmt.Errorf("--local-addr is required")
	}
	if _, err := net.ResolveTCPAddr("tcp", *localAddr); err != nil {
		return fmt.Errorf("invalid local address: %w", err)
	}

	path, cfg, err := loadDraftConfig(*configPath)
	if err != nil {
		return err
	}
	if cfg.Targets == nil {
		cfg.Targets = []config.TargetMapping{}
	}

	replaced := false
	for i := range cfg.Targets {
		if cfg.Targets[i].Name == *name {
			cfg.Targets[i].LocalAddr = *localAddr
			replaced = true
			break
		}
	}
	if !replaced {
		cfg.Targets = append(cfg.Targets, config.TargetMapping{Name: *name, LocalAddr: *localAddr})
	}

	if err := config.SaveAgentConfig(path, cfg); err != nil {
		return err
	}
	fmt.Fprintf(stdout, "updated targets in %s\n", path)
	return nil
}

func runShow(args []string, stdout io.Writer) error {
	fs := flag.NewFlagSet("show", flag.ContinueOnError)
	configPath := fs.String("config", "", "Path to agent JSON config")
	fs.SetOutput(io.Discard)
	if err := fs.Parse(args); err != nil {
		return err
	}

	path, cfg, err := loadDraftConfig(*configPath)
	if err != nil {
		return err
	}

	fmt.Fprintf(stdout, "config: %s\n", path)
	fmt.Fprintf(stdout, "relay_url: %s\n", cfg.RelayURL)
	fmt.Fprintf(stdout, "agent_id: %s\n", cfg.AgentID)
	if len(cfg.Targets) == 0 {
		fmt.Fprintln(stdout, "targets: -")
		return nil
	}

	fmt.Fprintln(stdout, "targets:")
	for _, target := range cfg.Targets {
		fmt.Fprintf(stdout, "%s -> %s\n", target.Name, target.LocalAddr)
	}
	return nil
}

func runAgentCommand(args []string) error {
	fs := flag.NewFlagSet("run", flag.ContinueOnError)
	configPath := fs.String("config", "", "Path to agent JSON config")
	fs.SetOutput(io.Discard)
	if err := fs.Parse(args); err != nil {
		return err
	}

	path, err := resolveAgentConfigPath(*configPath)
	if err != nil {
		return err
	}
	return runAgentWithPath(path)
}

func runAgentWithPath(path string) error {
	cfg, err := config.LoadAgentConfig(path)
	if err != nil {
		return fmt.Errorf("load agent config: %w", err)
	}

	client, err := agent.NewClient(cfg)
	if err != nil {
		return fmt.Errorf("create agent client: %w", err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := client.Start(ctx); err != nil {
		return fmt.Errorf("run agent client: %w", err)
	}
	return nil
}

func loadDraftConfig(explicitPath string) (string, config.AgentConfig, error) {
	path, err := resolveAgentConfigPath(explicitPath)
	if err != nil {
		return "", config.AgentConfig{}, err
	}

	cfg, err := config.LoadAgentDraftConfig(path)
	if err != nil {
		return "", config.AgentConfig{}, fmt.Errorf("load agent config: %w", err)
	}
	return path, cfg, nil
}

func resolveAgentConfigPath(explicitPath string) (string, error) {
	if explicitPath != "" {
		return explicitPath, nil
	}
	return config.DefaultAgentConfigPath()
}

func validateRelayURL(rawURL string, allowInsecure bool) error {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("invalid relay URL: %w", err)
	}
	if parsed.Scheme != "ws" && parsed.Scheme != "wss" {
		return fmt.Errorf("unsupported relay URL scheme: %s", parsed.Scheme)
	}
	if parsed.Scheme == "ws" && !allowInsecure {
		return fmt.Errorf("plain ws relay URL requires allow_insecure to be true")
	}
	return nil
}
