package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"gotunnel/internal/config"
)

func TestRenderStatusReportShowsNeverConnectedAgentWhenStateFileIsMissing(t *testing.T) {
	cfg := config.RelayConfig{
		Agents: []config.AgentAuth{
			{AgentID: "mac-mini", AuthToken: "secret"},
		},
		StateFile: filepath.Join(t.TempDir(), "missing-state.json"),
	}

	var out bytes.Buffer
	if err := renderStatusReport(&out, cfg); err != nil {
		t.Fatalf("render status report: %v", err)
	}

	text := out.String()
	if !strings.Contains(text, "mac-mini") || !strings.Contains(text, "never_connected") {
		t.Fatalf("unexpected status output: %s", text)
	}
}

func TestRenderStatusReportNormalizesOfflineActiveAgentWithoutMutatingState(t *testing.T) {
	statePath := filepath.Join(t.TempDir(), "relay-state.json")
	now := time.Date(2026, 4, 21, 13, 0, 0, 0, time.UTC)
	state := map[string]any{
		"version": 1,
		"agents": map[string]any{
			"mac-mini": map[string]any{
				"agent_id":           "mac-mini",
				"status":             "active",
				"last_known_targets": []string{"ssh", "web"},
				"last_connected_at":  now.Format(time.RFC3339),
				"updated_at":         now.Format(time.RFC3339),
			},
			"office-pc": map[string]any{
				"agent_id":             "office-pc",
				"status":               "inactive",
				"last_known_targets":   []string{"rdp"},
				"last_connected_at":    now.Add(-time.Hour).Format(time.RFC3339),
				"last_disconnected_at": now.Format(time.RFC3339),
				"updated_at":           now.Format(time.RFC3339),
			},
		},
	}
	raw, err := json.Marshal(state)
	if err != nil {
		t.Fatalf("marshal state: %v", err)
	}
	original := append([]byte(nil), raw...)
	if err := os.WriteFile(statePath, raw, 0o600); err != nil {
		t.Fatalf("write state file: %v", err)
	}

	cfg := config.RelayConfig{
		Agents: []config.AgentAuth{
			{AgentID: "mac-mini", AuthToken: "secret"},
			{AgentID: "office-pc", AuthToken: "office-secret"},
		},
		StateFile: statePath,
	}

	var out bytes.Buffer
	if err := renderStatusReport(&out, cfg); err != nil {
		t.Fatalf("render status report: %v", err)
	}

	text := out.String()
	for _, want := range []string{"mac-mini", "inactive", "ssh,web", "office-pc", "rdp"} {
		if !strings.Contains(text, want) {
			t.Fatalf("expected %q in status output: %s", want, text)
		}
	}
	if strings.Contains(text, "\tactive\t") {
		t.Fatalf("expected offline status to avoid active records: %s", text)
	}

	persistedAfter, err := os.ReadFile(statePath)
	if err != nil {
		t.Fatalf("read state file after status render: %v", err)
	}
	if !bytes.Equal(persistedAfter, original) {
		t.Fatalf("status render mutated state file: before=%s after=%s", string(original), string(persistedAfter))
	}
}

func TestLoadRelayConfigUsesRelaxedValidationForStatusMode(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "relay.json")
	content := `{
		"agents": [
			{"agent_id": "mac-mini", "auth_token": "secret"}
		],
		"state_file": "/tmp/relay-state.json"
	}`
	if err := os.WriteFile(configPath, []byte(content), 0o600); err != nil {
		t.Fatalf("write relay config: %v", err)
	}

	cfg, err := loadRelayConfig(configPath, true)
	if err != nil {
		t.Fatalf("load relay config for status mode: %v", err)
	}
	if cfg.StateFile != "/tmp/relay-state.json" {
		t.Fatalf("unexpected state file: %s", cfg.StateFile)
	}
	if len(cfg.Agents) != 1 || cfg.Agents[0].AgentID != "mac-mini" {
		t.Fatalf("unexpected agents: %+v", cfg.Agents)
	}
}

func TestLoadRelayConfigRequiresFullValidationOutsideStatusMode(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "relay.json")
	content := `{
		"agents": [
			{"agent_id": "mac-mini", "auth_token": "secret"}
		],
		"state_file": "/tmp/relay-state.json"
	}`
	if err := os.WriteFile(configPath, []byte(content), 0o600); err != nil {
		t.Fatalf("write relay config: %v", err)
	}

	if _, err := loadRelayConfig(configPath, false); err == nil {
		t.Fatal("expected full relay validation to fail without runtime fields")
	}
}

func TestRunRoutesLifecycle(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "relay.json")
	statePath := filepath.Join(t.TempDir(), "relay-state.json")
	content := `{
		"agents": [
			{"agent_id": "home-mac", "auth_token": "secret"}
		],
		"state_file": "` + statePath + `"
	}`
	if err := os.WriteFile(configPath, []byte(content), 0o600); err != nil {
		t.Fatalf("write relay config: %v", err)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	exitCode := run([]string{
		"-config", configPath,
		"routes", "create",
		"--name", "ssh",
		"--listen", "127.0.0.1:2222",
		"--agent", "home-mac",
		"--target", "ssh",
	}, &stdout, &stderr)
	if exitCode != 0 {
		t.Fatalf("create route exit code = %d, stderr=%s", exitCode, stderr.String())
	}
	if !strings.Contains(stdout.String(), "created route ssh") {
		t.Fatalf("unexpected create output: %s", stdout.String())
	}

	stdout.Reset()
	stderr.Reset()
	exitCode = run([]string{
		"-config", configPath,
		"routes", "list",
	}, &stdout, &stderr)
	if exitCode != 0 {
		t.Fatalf("list routes exit code = %d, stderr=%s", exitCode, stderr.String())
	}
	if !strings.Contains(stdout.String(), "ssh\t127.0.0.1:2222\thome-mac\tssh") {
		t.Fatalf("unexpected list output: %s", stdout.String())
	}

	stdout.Reset()
	stderr.Reset()
	exitCode = run([]string{
		"-config", configPath,
		"routes", "remove",
		"--name", "ssh",
	}, &stdout, &stderr)
	if exitCode != 0 {
		t.Fatalf("remove route exit code = %d, stderr=%s", exitCode, stderr.String())
	}
	if !strings.Contains(stdout.String(), "removed route ssh") {
		t.Fatalf("unexpected remove output: %s", stdout.String())
	}
}

func TestRunRoutesRequiresStateFile(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "relay.json")
	content := `{
		"agents": [
			{"agent_id": "home-mac", "auth_token": "secret"}
		]
	}`
	if err := os.WriteFile(configPath, []byte(content), 0o600); err != nil {
		t.Fatalf("write relay config: %v", err)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	exitCode := run([]string{
		"-config", configPath,
		"routes", "list",
	}, &stdout, &stderr)
	if exitCode != 1 {
		t.Fatalf("expected missing state_file to fail, exit code=%d stderr=%s", exitCode, stderr.String())
	}
	if !strings.Contains(stderr.String(), "state_file is required for routes commands") {
		t.Fatalf("unexpected stderr: %s", stderr.String())
	}
}
