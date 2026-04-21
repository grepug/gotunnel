package e2e_test

import (
	"context"
	"errors"
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

	publicAddr := waitForPublicAddr(t, relayServer, "ssh")

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

func TestTunnelReconnectsAfterRelayRestart(t *testing.T) {
	t.Parallel()

	parentCtx, cancel := context.WithCancel(context.Background())
	defer cancel()

	controlAddr := reserveTCPAddress(t)
	publicAddr := reserveTCPAddress(t)
	targetAddr := startEchoServer(t)

	relayCfg := config.RelayConfig{
		ControlAddr:   controlAddr,
		AuthTokens:    []string{"secret"},
		AllowInsecure: true,
		Ports: []config.PortMapping{
			{
				Name:       "ssh",
				ListenAddr: publicAddr,
			},
		},
	}

	relayServer, err := relay.NewServer(relayCfg)
	if err != nil {
		t.Fatalf("new relay server: %v", err)
	}

	relayCtx, stopRelay := context.WithCancel(parentCtx)
	go func() {
		if err := relayServer.Start(relayCtx); err != nil && relayCtx.Err() == nil {
			t.Errorf("relay start: %v", err)
		}
	}()

	agentCfg := config.AgentConfig{
		RelayURL:      relayServer.ControlURL(),
		AuthToken:     "secret",
		AllowInsecure: true,
		Targets: []config.TargetMapping{
			{Name: "ssh", LocalAddr: targetAddr},
		},
	}

	agentClient, err := agent.NewClient(agentCfg)
	if err != nil {
		t.Fatalf("new agent client: %v", err)
	}

	go func() {
		if err := agentClient.Start(parentCtx); err != nil && parentCtx.Err() == nil {
			t.Errorf("agent start: %v", err)
		}
	}()

	waitForForwarding(t, publicAddr)
	assertTunnelEcho(t, publicAddr, "one")

	stopRelay()
	time.Sleep(200 * time.Millisecond)

	relayServer, err = relay.NewServer(relayCfg)
	if err != nil {
		t.Fatalf("restart relay server: %v", err)
	}

	relayCtx, stopRelay = context.WithCancel(parentCtx)
	defer stopRelay()

	go func() {
		if err := relayServer.Start(relayCtx); err != nil && relayCtx.Err() == nil {
			t.Errorf("relay restart: %v", err)
		}
	}()

	waitForForwarding(t, publicAddr)
	assertTunnelEcho(t, publicAddr, "two")
}

func TestTunnelFailsFastWhenTargetIsUnavailable(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

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
			{Name: "ssh", LocalAddr: reserveTCPAddress(t)},
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

	publicAddr := waitForPublicAddr(t, relayServer, "ssh")
	conn, err := net.DialTimeout("tcp", publicAddr, 2*time.Second)
	if err != nil {
		t.Fatalf("dial public addr: %v", err)
	}
	defer conn.Close()

	_ = conn.SetDeadline(time.Now().Add(2 * time.Second))
	if _, err := conn.Write([]byte("ping")); err != nil && !isConnClosed(err) {
		t.Fatalf("write public conn: %v", err)
	}

	buf := make([]byte, 1)
	if _, err := conn.Read(buf); err == nil {
		t.Fatal("expected read to fail when target is unavailable")
	}
}

func TestTunnelForwardsMultiplePublicPorts(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sshTarget := startEchoServer(t)
	webTarget := startEchoServer(t)

	relayCfg := config.RelayConfig{
		ControlAddr:   "127.0.0.1:0",
		AuthTokens:    []string{"secret"},
		AllowInsecure: true,
		Ports: []config.PortMapping{
			{Name: "ssh", ListenAddr: "127.0.0.1:0"},
			{Name: "web", ListenAddr: "127.0.0.1:0"},
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
			{Name: "ssh", LocalAddr: sshTarget},
			{Name: "web", LocalAddr: webTarget},
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

	assertTunnelEcho(t, waitForPublicAddr(t, relayServer, "ssh"), "ssh")
	assertTunnelEcho(t, waitForPublicAddr(t, relayServer, "web"), "web")
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

func reserveTCPAddress(t *testing.T) string {
	t.Helper()

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("reserve tcp addr: %v", err)
	}
	addr := ln.Addr().String()
	_ = ln.Close()
	return addr
}

func assertTunnelEcho(t *testing.T, publicAddr, payload string) {
	t.Helper()

	conn, err := net.DialTimeout("tcp", publicAddr, 2*time.Second)
	if err != nil {
		t.Fatalf("dial public addr: %v", err)
	}
	defer conn.Close()

	if _, err := conn.Write([]byte(payload)); err != nil {
		t.Fatalf("write public conn: %v", err)
	}

	reply := make([]byte, len(payload))
	if _, err := io.ReadFull(conn, reply); err != nil {
		t.Fatalf("read public conn: %v", err)
	}

	if string(reply) != payload {
		t.Fatalf("unexpected reply: got %q want %q", string(reply), payload)
	}
}

func waitForForwarding(t *testing.T, publicAddr string) {
	t.Helper()

	deadline := time.Now().Add(8 * time.Second)
	for time.Now().Before(deadline) {
		conn, err := net.DialTimeout("tcp", publicAddr, 200*time.Millisecond)
		if err == nil {
			_ = conn.Close()
			return
		}
		time.Sleep(50 * time.Millisecond)
	}

	t.Fatalf("public address %s did not accept connections in time", publicAddr)
}

func waitForPublicAddr(t *testing.T, relayServer *relay.Server, name string) string {
	t.Helper()

	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if addr := relayServer.ListenerAddress(name); addr != "" {
			return addr
		}
		time.Sleep(25 * time.Millisecond)
	}

	t.Fatal("relay did not expose public address in time")
	return ""
}

func isConnClosed(err error) bool {
	return errors.Is(err, io.EOF) || errors.Is(err, net.ErrClosed)
}
