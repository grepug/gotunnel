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

func TestRenderStatusReportShowsActiveAndInactiveAgents(t *testing.T) {
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
	for _, want := range []string{"mac-mini", "active", "ssh,web", "office-pc", "inactive", "rdp"} {
		if !strings.Contains(text, want) {
			t.Fatalf("expected %q in status output: %s", want, text)
		}
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
