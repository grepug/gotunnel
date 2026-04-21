package e2e_test

import (
	"context"
	"errors"
	"io"
	"net"
	"strings"
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
		Agents:        []config.AgentAuth{{AgentID: "mac-mini", AuthToken: "secret"}},
		AllowInsecure: true,
		Ports: []config.PortMapping{
			{
				Name:       "ssh",
				ListenAddr: "127.0.0.1:0",
				AgentID:    "mac-mini",
				TargetName: "ssh",
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
		AgentID:       "mac-mini",
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
		Agents:        []config.AgentAuth{{AgentID: "mac-mini", AuthToken: "secret"}},
		AllowInsecure: true,
		Ports: []config.PortMapping{
			{
				Name:       "ssh",
				ListenAddr: publicAddr,
				AgentID:    "mac-mini",
				TargetName: "ssh",
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
		AgentID:       "mac-mini",
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
		Agents:        []config.AgentAuth{{AgentID: "mac-mini", AuthToken: "secret"}},
		AllowInsecure: true,
		Ports: []config.PortMapping{
			{
				Name:       "ssh",
				ListenAddr: "127.0.0.1:0",
				AgentID:    "mac-mini",
				TargetName: "ssh",
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
		AgentID:       "mac-mini",
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
		Agents:        []config.AgentAuth{{AgentID: "mac-mini", AuthToken: "secret"}},
		AllowInsecure: true,
		Ports: []config.PortMapping{
			{Name: "ssh", ListenAddr: "127.0.0.1:0", AgentID: "mac-mini", TargetName: "ssh"},
			{Name: "web", ListenAddr: "127.0.0.1:0", AgentID: "mac-mini", TargetName: "web"},
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
		AgentID:       "mac-mini",
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

func TestTunnelRoutesOverlappingTargetNamesToTheConfiguredAgent(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	relayCfg := config.RelayConfig{
		ControlAddr: "127.0.0.1:0",
		Agents: []config.AgentAuth{
			{AgentID: "mac-mini", AuthToken: "secret"},
			{AgentID: "office-pc", AuthToken: "office-secret"},
		},
		AllowInsecure: true,
		Ports: []config.PortMapping{
			{Name: "mac-ssh", ListenAddr: "127.0.0.1:0", AgentID: "mac-mini", TargetName: "ssh"},
			{Name: "office-ssh", ListenAddr: "127.0.0.1:0", AgentID: "office-pc", TargetName: "ssh"},
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

	macClient, err := agent.NewClient(config.AgentConfig{
		RelayURL:      relayServer.ControlURL(),
		AgentID:       "mac-mini",
		AuthToken:     "secret",
		AllowInsecure: true,
		Targets: []config.TargetMapping{
			{Name: "ssh", LocalAddr: startTaggedEchoServer(t, "mac:")},
		},
	})
	if err != nil {
		t.Fatalf("new mac agent client: %v", err)
	}

	officeClient, err := agent.NewClient(config.AgentConfig{
		RelayURL:      relayServer.ControlURL(),
		AgentID:       "office-pc",
		AuthToken:     "office-secret",
		AllowInsecure: true,
		Targets: []config.TargetMapping{
			{Name: "ssh", LocalAddr: startTaggedEchoServer(t, "office:")},
		},
	})
	if err != nil {
		t.Fatalf("new office agent client: %v", err)
	}

	go func() {
		if err := macClient.Start(ctx); err != nil && ctx.Err() == nil {
			t.Errorf("mac agent start: %v", err)
		}
	}()
	go func() {
		if err := officeClient.Start(ctx); err != nil && ctx.Err() == nil {
			t.Errorf("office agent start: %v", err)
		}
	}()

	assertTunnelReply(t, waitForPublicAddr(t, relayServer, "mac-ssh"), "ping", "mac:ping")
	assertTunnelReply(t, waitForPublicAddr(t, relayServer, "office-ssh"), "ping", "office:ping")
}

func TestTunnelRejectsDuplicateActiveAgentID(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	relayCfg := config.RelayConfig{
		ControlAddr:   "127.0.0.1:0",
		Agents:        []config.AgentAuth{{AgentID: "mac-mini", AuthToken: "secret"}},
		AllowInsecure: true,
		Ports: []config.PortMapping{
			{Name: "ssh", ListenAddr: "127.0.0.1:0", AgentID: "mac-mini", TargetName: "ssh"},
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

	firstClient, err := agent.NewClient(config.AgentConfig{
		RelayURL:      relayServer.ControlURL(),
		AgentID:       "mac-mini",
		AuthToken:     "secret",
		AllowInsecure: true,
		Targets: []config.TargetMapping{
			{Name: "ssh", LocalAddr: startTaggedEchoServer(t, "first:")},
		},
	})
	if err != nil {
		t.Fatalf("new first agent client: %v", err)
	}

	go func() {
		if err := firstClient.Start(ctx); err != nil && ctx.Err() == nil {
			t.Errorf("first agent start: %v", err)
		}
	}()

	publicAddr := waitForPublicAddr(t, relayServer, "ssh")
	assertTunnelReply(t, publicAddr, "ping", "first:ping")

	duplicateCtx, stopDuplicate := context.WithCancel(ctx)
	defer stopDuplicate()

	duplicateClient, err := agent.NewClient(config.AgentConfig{
		RelayURL:      relayServer.ControlURL(),
		AgentID:       "mac-mini",
		AuthToken:     "secret",
		AllowInsecure: true,
		Targets: []config.TargetMapping{
			{Name: "ssh", LocalAddr: startTaggedEchoServer(t, "second:")},
		},
	})
	if err != nil {
		t.Fatalf("new duplicate agent client: %v", err)
	}

	duplicateErrCh := make(chan error, 1)
	go func() {
		duplicateErrCh <- duplicateClient.Start(duplicateCtx)
	}()

	select {
	case err := <-duplicateErrCh:
		if err == nil || !strings.Contains(err.Error(), "duplicate active agent id") {
			t.Fatalf("expected duplicate agent rejection, got %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("duplicate agent was not rejected in time")
	}

	assertTunnelReply(t, publicAddr, "ping", "first:ping")
}

func TestTunnelRejectsWrongCredentialForNamedAgent(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	relayCfg := config.RelayConfig{
		ControlAddr: "127.0.0.1:0",
		Agents: []config.AgentAuth{
			{AgentID: "mac-mini", AuthToken: "secret"},
		},
		AllowInsecure: true,
		Ports: []config.PortMapping{
			{Name: "ssh", ListenAddr: "127.0.0.1:0", AgentID: "mac-mini", TargetName: "ssh"},
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

	badClient, err := agent.NewClient(config.AgentConfig{
		RelayURL:      relayServer.ControlURL(),
		AgentID:       "mac-mini",
		AuthToken:     "wrong-secret",
		AllowInsecure: true,
		Targets: []config.TargetMapping{
			{Name: "ssh", LocalAddr: startTaggedEchoServer(t, "bad:")},
		},
	})
	if err != nil {
		t.Fatalf("new bad agent client: %v", err)
	}

	errCh := make(chan error, 1)
	go func() {
		errCh <- badClient.Start(ctx)
	}()

	select {
	case err := <-errCh:
		if err == nil || !strings.Contains(err.Error(), "unauthorized") {
			t.Fatalf("expected unauthorized error, got %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("wrong-credential agent was not rejected in time")
	}
}

func TestTunnelRejectsUnknownNamedAgent(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	relayCfg := config.RelayConfig{
		ControlAddr: "127.0.0.1:0",
		Agents: []config.AgentAuth{
			{AgentID: "mac-mini", AuthToken: "secret"},
		},
		AllowInsecure: true,
		Ports: []config.PortMapping{
			{Name: "ssh", ListenAddr: "127.0.0.1:0", AgentID: "mac-mini", TargetName: "ssh"},
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

	unknownClient, err := agent.NewClient(config.AgentConfig{
		RelayURL:      relayServer.ControlURL(),
		AgentID:       "unknown-agent",
		AuthToken:     "secret",
		AllowInsecure: true,
		Targets: []config.TargetMapping{
			{Name: "ssh", LocalAddr: startTaggedEchoServer(t, "bad:")},
		},
	})
	if err != nil {
		t.Fatalf("new unknown agent client: %v", err)
	}

	errCh := make(chan error, 1)
	go func() {
		errCh <- unknownClient.Start(ctx)
	}()

	select {
	case err := <-errCh:
		if err == nil || !strings.Contains(err.Error(), "unauthorized") {
			t.Fatalf("expected unauthorized error, got %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("unknown agent was not rejected in time")
	}
}

func TestTunnelRejectsCredentialForDifferentNamedAgent(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	relayCfg := config.RelayConfig{
		ControlAddr: "127.0.0.1:0",
		Agents: []config.AgentAuth{
			{AgentID: "mac-mini", AuthToken: "secret"},
			{AgentID: "office-pc", AuthToken: "office-secret"},
		},
		AllowInsecure: true,
		Ports: []config.PortMapping{
			{Name: "ssh", ListenAddr: "127.0.0.1:0", AgentID: "mac-mini", TargetName: "ssh"},
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

	wrongAgentClient, err := agent.NewClient(config.AgentConfig{
		RelayURL:      relayServer.ControlURL(),
		AgentID:       "mac-mini",
		AuthToken:     "office-secret",
		AllowInsecure: true,
		Targets: []config.TargetMapping{
			{Name: "ssh", LocalAddr: startTaggedEchoServer(t, "bad:")},
		},
	})
	if err != nil {
		t.Fatalf("new wrong-agent client: %v", err)
	}

	errCh := make(chan error, 1)
	go func() {
		errCh <- wrongAgentClient.Start(ctx)
	}()

	select {
	case err := <-errCh:
		if err == nil || !strings.Contains(err.Error(), "unauthorized") {
			t.Fatalf("expected unauthorized error, got %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("wrong-agent credential was not rejected in time")
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

func startTaggedEchoServer(t *testing.T, prefix string) string {
	t.Helper()

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen tagged echo: %v", err)
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

				buf := make([]byte, 1024)
				n, err := c.Read(buf)
				if n > 0 {
					_, _ = c.Write([]byte(prefix + string(buf[:n])))
				}
				if err != nil {
					return
				}
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

	assertTunnelReply(t, publicAddr, payload, payload)
}

func assertTunnelReply(t *testing.T, publicAddr, payload, want string) {
	t.Helper()

	conn, err := net.DialTimeout("tcp", publicAddr, 2*time.Second)
	if err != nil {
		t.Fatalf("dial public addr: %v", err)
	}
	defer conn.Close()

	if _, err := conn.Write([]byte(payload)); err != nil {
		t.Fatalf("write public conn: %v", err)
	}

	reply := make([]byte, len(want))
	if _, err := io.ReadFull(conn, reply); err != nil {
		t.Fatalf("read public conn: %v", err)
	}

	if string(reply) != want {
		t.Fatalf("unexpected reply: got %q want %q", string(reply), want)
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
