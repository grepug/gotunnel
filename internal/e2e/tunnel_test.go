package e2e_test

import (
	"context"
	"io"
	"net"
	"testing"
	"time"

	"gotunnel/internal/agent"
	"gotunnel/internal/config"
	"gotunnel/internal/relay"
)

func TestTunnelForwardsTCPTraffic(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	targetAddr := startEchoServer(t)

	relayCfg := config.RelayConfig{
		ControlAddr:   "127.0.0.1:0",
		AuthTokens:    []string{"secret"},
		AllowInsecure: true,
		Ports: []config.PortMapping{
			{
				Name:       "ssh",
				ListenAddr: "127.0.0.1:0",
			},
		},
	}

	relayServer, err := relay.NewServer(relayCfg)
	if err != nil {
		t.Fatalf("new relay server: %v", err)
	}

	go func() {
		if err := relayServer.Start(ctx); err != nil && ctx.Err() == nil {
			t.Errorf("relay start: %v", err)
		}
	}()

	agentCfg := config.AgentConfig{
		RelayURL:      relayServer.ControlURL(),
		AuthToken:     "secret",
		AllowInsecure: true,
		Targets: []config.TargetMapping{
			{
				Name:      "ssh",
				LocalAddr: targetAddr,
			},
		},
	}

	agentClient, err := agent.NewClient(agentCfg)
	if err != nil {
		t.Fatalf("new agent client: %v", err)
	}

	go func() {
		if err := agentClient.Start(ctx); err != nil && ctx.Err() == nil {
			t.Errorf("agent start: %v", err)
		}
	}()

	publicAddr := waitForPublicAddr(t, relayServer)

	conn, err := net.DialTimeout("tcp", publicAddr, 2*time.Second)
	if err != nil {
		t.Fatalf("dial public addr: %v", err)
	}
	defer conn.Close()

	if _, err := conn.Write([]byte("ping")); err != nil {
		t.Fatalf("write public conn: %v", err)
	}

	reply := make([]byte, 4)
	if _, err := io.ReadFull(conn, reply); err != nil {
		t.Fatalf("read public conn: %v", err)
	}

	if string(reply) != "ping" {
		t.Fatalf("unexpected reply: got %q want %q", string(reply), "ping")
	}
}

func startEchoServer(t *testing.T) string {
	t.Helper()

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen echo: %v", err)
	}

	t.Cleanup(func() {
		_ = ln.Close()
	})

	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}

			go func(c net.Conn) {
				defer c.Close()
				_, _ = io.Copy(c, c)
			}(conn)
		}
	}()

	return ln.Addr().String()
}

func waitForPublicAddr(t *testing.T, relayServer *relay.Server) string {
	t.Helper()

	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if addr := relayServer.ListenerAddress("ssh"); addr != "" {
			return addr
		}
		time.Sleep(25 * time.Millisecond)
	}

	t.Fatal("relay did not expose public address in time")
	return ""
}
