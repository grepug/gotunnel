package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"gotunnel/internal/config"
	"gotunnel/internal/relay"
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("gotunneld", flag.ContinueOnError)
	flags.SetOutput(stderr)

	configPath := flags.String("config", "", "Path to relay JSON config")
	statusMode := flags.Bool("status", false, "Print persisted relay registration status and exit")
	if err := flags.Parse(args); err != nil {
		return 2
	}

	if *configPath == "" {
		fmt.Fprintln(stderr, "-config is required")
		return 2
	}

	remaining := flags.Args()
	if *statusMode && len(remaining) > 0 {
		fmt.Fprintln(stderr, "-status cannot be combined with subcommands")
		return 2
	}
	if len(remaining) > 0 {
		return runSubcommand(*configPath, remaining, stdout, stderr)
	}

	cfg, err := loadRelayConfig(*configPath, *statusMode)
	if err != nil {
		fmt.Fprintf(stderr, "load relay config: %v\n", err)
		return 1
	}

	if *statusMode {
		if err := renderStatusReport(stdout, cfg); err != nil {
			fmt.Fprintf(stderr, "render relay status: %v\n", err)
			return 1
		}
		return 0
	}

	server, err := relay.NewServer(cfg)
	if err != nil {
		fmt.Fprintf(stderr, "create relay server: %v\n", err)
		return 1
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := server.Start(ctx); err != nil {
		fmt.Fprintf(stderr, "run relay server: %v\n", err)
		return 1
	}
	return 0
}

func runSubcommand(configPath string, args []string, stdout, stderr io.Writer) int {
	switch args[0] {
	case "routes":
		return runRoutesCommand(configPath, args[1:], stdout, stderr)
	default:
		fmt.Fprintf(stderr, "unknown subcommand: %s\n", args[0])
		return 2
	}
}

func runRoutesCommand(configPath string, args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, "routes subcommand is required: create, list, or remove")
		return 2
	}

	cfg, err := loadRouteAdminConfig(configPath)
	if err != nil {
		fmt.Fprintf(stderr, "load relay config: %v\n", err)
		return 1
	}

	switch args[0] {
	case "list":
		if err := renderRouteRegistrations(stdout, cfg); err != nil {
			fmt.Fprintf(stderr, "list routes: %v\n", err)
			return 1
		}
		return 0
	case "create":
		return runRoutesCreate(cfg, args[1:], stdout, stderr)
	case "remove":
		return runRoutesRemove(cfg, args[1:], stdout, stderr)
	default:
		fmt.Fprintf(stderr, "unknown routes subcommand: %s\n", args[0])
		return 2
	}
}

func runRoutesCreate(cfg config.RelayConfig, args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("gotunneld routes create", flag.ContinueOnError)
	flags.SetOutput(stderr)

	name := flags.String("name", "", "Unique public route name")
	listenAddr := flags.String("listen", "", "Public listen address")
	agentID := flags.String("agent", "", "Destination agent ID")
	targetName := flags.String("target", "", "Destination target name")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	if len(flags.Args()) != 0 {
		fmt.Fprintf(stderr, "unexpected arguments for routes create: %s\n", strings.Join(flags.Args(), " "))
		return 2
	}

	err := relay.CreateRouteRegistration(cfg.StateFile, cfg.Agents, cfg.Ports, relay.RouteRegistration{
		RouteName:  *name,
		ListenAddr: *listenAddr,
		AgentID:    *agentID,
		TargetName: *targetName,
	})
	if err != nil {
		fmt.Fprintf(stderr, "create route: %v\n", err)
		return 1
	}

	fmt.Fprintf(stdout, "created route %s\n", *name)
	return 0
}

func runRoutesRemove(cfg config.RelayConfig, args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("gotunneld routes remove", flag.ContinueOnError)
	flags.SetOutput(stderr)

	name := flags.String("name", "", "Public route name")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	if len(flags.Args()) != 0 {
		fmt.Fprintf(stderr, "unexpected arguments for routes remove: %s\n", strings.Join(flags.Args(), " "))
		return 2
	}
	if *name == "" {
		fmt.Fprintln(stderr, "remove route: name is required")
		return 2
	}

	removed, err := relay.RemoveRouteRegistration(cfg.StateFile, *name)
	if err != nil {
		fmt.Fprintf(stderr, "remove route: %v\n", err)
		return 1
	}
	if !removed {
		fmt.Fprintf(stderr, "remove route: route %s does not exist\n", *name)
		return 1
	}

	fmt.Fprintf(stdout, "removed route %s\n", *name)
	return 0
}

func loadRelayConfig(path string, statusMode bool) (config.RelayConfig, error) {
	if statusMode {
		return config.LoadRelayStatusConfig(path)
	}
	return config.LoadRelayConfig(path)
}

func loadRouteAdminConfig(path string) (config.RelayConfig, error) {
	cfg, err := config.LoadRelayStatusConfig(path)
	if err != nil {
		return config.RelayConfig{}, err
	}
	if cfg.StateFile == "" {
		return config.RelayConfig{}, errors.New("state_file is required for routes commands")
	}
	return cfg, nil
}

func renderStatusReport(w io.Writer, cfg config.RelayConfig) error {
	statuses, err := relay.LoadAgentStatuses(cfg.StateFile, cfg.Agents)
	if err != nil {
		return err
	}

	for _, status := range statuses {
		targets := "-"
		if len(status.LastKnownTargets) > 0 {
			targets = strings.Join(status.LastKnownTargets, ",")
		}
		lastConnected := "-"
		if !status.LastConnectedAt.IsZero() {
			lastConnected = status.LastConnectedAt.Format(time.RFC3339)
		}
		lastDisconnected := "-"
		if !status.LastDisconnectedAt.IsZero() {
			lastDisconnected = status.LastDisconnectedAt.Format(time.RFC3339)
		}
		if _, err := fmt.Fprintf(
			w,
			"%s\t%s\ttargets=%s\tlast_connected=%s\tlast_disconnected=%s\n",
			status.AgentID,
			status.Status,
			targets,
			lastConnected,
			lastDisconnected,
		); err != nil {
			return err
		}
	}
	return nil
}

func renderRouteRegistrations(w io.Writer, cfg config.RelayConfig) error {
	routes, err := relay.LoadRouteRegistrations(cfg.StateFile)
	if err != nil {
		return err
	}

	for _, route := range routes {
		if _, err := fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", route.RouteName, route.ListenAddr, route.AgentID, route.TargetName); err != nil {
			return err
		}
	}
	return nil
}
