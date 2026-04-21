package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"gotunnel/internal/config"
	"gotunnel/internal/relay"
)

func main() {
	configPath := flag.String("config", "", "Path to relay JSON config")
	flag.Parse()

	if *configPath == "" {
		fmt.Fprintln(os.Stderr, "-config is required")
		os.Exit(2)
	}

	cfg, err := config.LoadRelayConfig(*configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "load relay config: %v\n", err)
		os.Exit(1)
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
