package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gotunnel/internal/config"
)

func TestRunInitCreatesDefaultAgentConfig(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	var stdout bytes.Buffer
	if err := run([]string{"init"}, &stdout, &stdout); err != nil {
		t.Fatalf("run init: %v", err)
	}

	path := filepath.Join(home, ".gotunnel", "agent.json")
	raw := readFile(t, path)

	var cfg config.AgentConfig
	if err := json.Unmarshal(raw, &cfg); err != nil {
		t.Fatalf("decode init config: %v", err)
	}

	if len(cfg.Targets) != 0 {
		t.Fatalf("unexpected targets after init: %+v", cfg.Targets)
	}
	if !strings.Contains(stdout.String(), path) {
		t.Fatalf("expected output to mention config path, got %q", stdout.String())
	}
}

func TestRunSetRelayAuthTargetAndShowUseDefaultConfig(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	var stdout bytes.Buffer
	for _, args := range [][]string{
		{"init"},
		{"set", "relay", "--url", "ws://127.0.0.1:18443/connect", "--allow-insecure"},
		{"set", "auth", "--agent-id", "home-mac", "--auth-token", "secret"},
		{"target", "add", "--name", "ssh", "--local-addr", "127.0.0.1:22"},
	} {
		stdout.Reset()
		if err := run(args, &stdout, &stdout); err != nil {
			t.Fatalf("run %v: %v", args, err)
		}
	}

	cfg, err := config.LoadDefaultAgentConfig()
	if err != nil {
		t.Fatalf("load default agent config: %v", err)
	}

	if cfg.RelayURL != "ws://127.0.0.1:18443/connect" {
		t.Fatalf("unexpected relay url: %q", cfg.RelayURL)
	}
	if !cfg.AllowInsecure {
		t.Fatal("expected allow_insecure to be set")
	}
	if cfg.AgentID != "home-mac" {
		t.Fatalf("unexpected agent id: %q", cfg.AgentID)
	}
	if len(cfg.Targets) != 1 || cfg.Targets[0].Name != "ssh" {
		t.Fatalf("unexpected targets: %+v", cfg.Targets)
	}

	stdout.Reset()
	if err := run([]string{"show"}, &stdout, &stdout); err != nil {
		t.Fatalf("run show: %v", err)
	}

	text := stdout.String()
	for _, want := range []string{"home-mac", "ws://127.0.0.1:18443/connect", "ssh -> 127.0.0.1:22"} {
		if !strings.Contains(text, want) {
			t.Fatalf("expected %q in show output: %s", want, text)
		}
	}
}

func TestRunTargetAddRejectsInvalidLocalAddr(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	var stdout bytes.Buffer
	if err := run([]string{"init"}, &stdout, &stdout); err != nil {
		t.Fatalf("run init: %v", err)
	}

	err := run([]string{"target", "add", "--name", "ssh", "--local-addr", "not-a-tcp-address"}, &stdout, &stdout)
	if err == nil {
		t.Fatal("expected invalid local address to fail")
	}
	if !strings.Contains(err.Error(), "invalid local address") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRunSetRelayRejectsPlainWSWithoutAllowInsecure(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	var stdout bytes.Buffer
	if err := run([]string{"init"}, &stdout, &stdout); err != nil {
		t.Fatalf("run init: %v", err)
	}

	err := run([]string{"set", "relay", "--url", "ws://127.0.0.1:18443/connect"}, &stdout, &stdout)
	if err == nil {
		t.Fatal("expected plain ws relay url to fail without --allow-insecure")
	}
	if !strings.Contains(err.Error(), "allow_insecure") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRunSetRelayRejectsUnsupportedRelayScheme(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	var stdout bytes.Buffer
	if err := run([]string{"init"}, &stdout, &stdout); err != nil {
		t.Fatalf("run init: %v", err)
	}

	err := run([]string{"set", "relay", "--url", "http://127.0.0.1:18443/connect"}, &stdout, &stdout)
	if err == nil {
		t.Fatal("expected unsupported relay scheme to fail")
	}
	if !strings.Contains(err.Error(), "unsupported relay URL scheme") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRunLegacyConfigFlagKeepsCompatibilityPath(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "missing-agent.json")

	var stdout bytes.Buffer
	err := run([]string{"-config", missing}, &stdout, &stdout)
	if err == nil {
		t.Fatal("expected missing explicit config to fail")
	}
	if !strings.Contains(err.Error(), "load agent config") || !strings.Contains(err.Error(), missing) {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRunSubcommandsSupportExplicitConfigPath(t *testing.T) {
	path := filepath.Join(t.TempDir(), "custom-agent.json")

	var stdout bytes.Buffer
	for _, args := range [][]string{
		{"init", "--config", path},
		{"set", "relay", "--config", path, "--url", "ws://127.0.0.1:18443/connect", "--allow-insecure"},
		{"set", "auth", "--config", path, "--agent-id", "office-mac", "--auth-token", "secret"},
		{"target", "add", "--config", path, "--name", "ssh", "--local-addr", "127.0.0.1:22"},
	} {
		stdout.Reset()
		if err := run(args, &stdout, &stdout); err != nil {
			t.Fatalf("run %v: %v", args, err)
		}
	}

	cfg, err := config.LoadAgentConfig(path)
	if err != nil {
		t.Fatalf("load explicit config: %v", err)
	}
	if cfg.AgentID != "office-mac" {
		t.Fatalf("unexpected agent id: %q", cfg.AgentID)
	}

	stdout.Reset()
	if err := run([]string{"show", "--config", path}, &stdout, &stdout); err != nil {
		t.Fatalf("run show with explicit config: %v", err)
	}
	if !strings.Contains(stdout.String(), path) {
		t.Fatalf("expected show output to mention explicit path, got %q", stdout.String())
	}
}

func readFile(t *testing.T, path string) []byte {
	t.Helper()

	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read file %s: %v", path, err)
	}
	return raw
}
