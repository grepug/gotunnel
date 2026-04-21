package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"gotunnel/internal/config"
)

func TestDefaultAgentConfigPathUsesHomeGotunnelDirectory(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	path, err := config.DefaultAgentConfigPath()
	if err != nil {
		t.Fatalf("default agent config path: %v", err)
	}

	want := filepath.Join(home, ".gotunnel", "agent.json")
	if path != want {
		t.Fatalf("unexpected default agent config path: got %q want %q", path, want)
	}
}

func TestSaveAndLoadDefaultAgentConfigRoundTrips(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	cfg := config.AgentConfig{
		RelayURL:      "ws://127.0.0.1:18443/connect",
		AgentID:       "home-mac",
		AuthToken:     "secret",
		AllowInsecure: true,
		Targets: []config.TargetMapping{
			{Name: "ssh", LocalAddr: "127.0.0.1:22"},
		},
	}

	path, err := config.SaveDefaultAgentConfig(cfg)
	if err != nil {
		t.Fatalf("save default agent config: %v", err)
	}

	if _, err := os.Stat(filepath.Dir(path)); err != nil {
		t.Fatalf("stat config dir: %v", err)
	}

	loaded, err := config.LoadDefaultAgentConfig()
	if err != nil {
		t.Fatalf("load default agent config: %v", err)
	}

	if loaded.AgentID != cfg.AgentID {
		t.Fatalf("unexpected agent id: got %q want %q", loaded.AgentID, cfg.AgentID)
	}
	if loaded.RelayURL != cfg.RelayURL {
		t.Fatalf("unexpected relay url: got %q want %q", loaded.RelayURL, cfg.RelayURL)
	}
	if len(loaded.Targets) != 1 || loaded.Targets[0].Name != "ssh" {
		t.Fatalf("unexpected targets: %+v", loaded.Targets)
	}
}
