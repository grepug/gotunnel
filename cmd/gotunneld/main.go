package main

import (
	"context"
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
	configPath := flag.String("config", "", "Path to relay JSON config")
	statusMode := flag.Bool("status", false, "Print persisted relay registration status and exit")
	flag.Parse()

	if *configPath == "" {
		fmt.Fprintln(os.Stderr, "-config is required")
		os.Exit(2)
	}

	cfg, err := loadRelayConfig(*configPath, *statusMode)
	if err != nil {
		fmt.Fprintf(os.Stderr, "load relay config: %v\n", err)
		os.Exit(1)
	}

	if *statusMode {
		if err := renderStatusReport(os.Stdout, cfg); err != nil {
			fmt.Fprintf(os.Stderr, "render relay status: %v\n", err)
			os.Exit(1)
		}
		return
	}

	server, err := relay.NewServer(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "create relay server: %v\n", err)
		os.Exit(1)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := server.Start(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "run relay server: %v\n", err)
		os.Exit(1)
	}
}

func loadRelayConfig(path string, statusMode bool) (config.RelayConfig, error) {
	if statusMode {
		return config.LoadRelayStatusConfig(path)
	}
	return config.LoadRelayConfig(path)
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
