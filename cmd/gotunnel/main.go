package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"gotunnel/internal/agent"
	"gotunnel/internal/config"
)

func main() {
	configPath := flag.String("config", "", "Path to agent JSON config")
	flag.Parse()

	if *configPath == "" {
		fmt.Fprintln(os.Stderr, "-config is required")
		os.Exit(2)
	}

	cfg, err := config.LoadAgentConfig(*configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "load agent config: %v\n", err)
		os.Exit(1)
	}

	client, err := agent.NewClient(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "create agent client: %v\n", err)
		os.Exit(1)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := client.Start(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "run agent client: %v\n", err)
		os.Exit(1)
	}
}
