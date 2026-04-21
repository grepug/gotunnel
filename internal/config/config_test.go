package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"gotunnel/internal/config"
)

func TestRelayConfigValidateAcceptsMinimalValidConfig(t *testing.T) {
	cfg := config.RelayConfig{
		ControlAddr:   "127.0.0.1:0",
		AuthTokens:    []string{"secret"},
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

	if err := cfg.Validate(); err != nil {
		t.Fatalf("expected valid relay config, got error: %v", err)
	}
}

func TestRelayConfigValidateRejectsDuplicatePortNames(t *testing.T) {
	cfg := config.RelayConfig{
		ControlAddr:   "127.0.0.1:0",
		AuthTokens:    []string{"secret"},
		AllowInsecure: true,
		Ports: []config.PortMapping{
			{Name: "ssh", ListenAddr: "127.0.0.1:0", AgentID: "mac-mini", TargetName: "ssh"},
			{Name: "ssh", ListenAddr: "127.0.0.1:0", AgentID: "office-pc", TargetName: "ssh"},
		},
	}

	if err := cfg.Validate(); err == nil {
		t.Fatal("expected duplicate port names to fail validation")
	}
}

func TestAgentConfigValidateRejectsUnknownTargetNames(t *testing.T) {
	cfg := config.AgentConfig{
		RelayURL:      "ws://127.0.0.1:443/connect",
		AgentID:       "mac-mini",
		AuthToken:     "secret",
		AllowInsecure: true,
		Targets: []config.TargetMapping{
			{Name: "ssh", LocalAddr: "127.0.0.1:22"},
			{Name: "", LocalAddr: "127.0.0.1:3000"},
		},
	}

	if err := cfg.Validate(); err == nil {
		t.Fatal("expected unnamed target to fail validation")
	}
}

func TestAgentConfigValidateRejectsPlainWSByDefault(t *testing.T) {
	cfg := config.AgentConfig{
		RelayURL:  "ws://127.0.0.1:443/connect",
		AgentID:   "mac-mini",
		AuthToken: "secret",
		Targets: []config.TargetMapping{
			{Name: "ssh", LocalAddr: "127.0.0.1:22"},
		},
	}

	if err := cfg.Validate(); err == nil {
		t.Fatal("expected plain ws relay URL to fail without allow_insecure")
	}
}

func TestRelayConfigValidateRejectsPartialTLSConfig(t *testing.T) {
	cfg := config.RelayConfig{
		ControlAddr: "127.0.0.1:0",
		AuthTokens:  []string{"secret"},
		TLSCertFile: "/tmp/cert.pem",
		Ports: []config.PortMapping{
			{Name: "ssh", ListenAddr: "127.0.0.1:0", AgentID: "mac-mini", TargetName: "ssh"},
		},
	}

	if err := cfg.Validate(); err == nil {
		t.Fatal("expected partial TLS config to fail validation")
	}
}

func TestLoadRelayConfigFromJSONFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "relay.json")
	content := `{
		"control_addr": "127.0.0.1:0",
		"auth_tokens": ["secret"],
		"allow_insecure": true,
		"ports": [
			{"name": "ssh", "listen_addr": "127.0.0.1:0", "agent_id": "mac-mini", "target_name": "ssh"}
		]
	}`

	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write relay config: %v", err)
	}

	cfg, err := config.LoadRelayConfig(path)
	if err != nil {
		t.Fatalf("load relay config: %v", err)
	}

	if cfg.ControlAddr != "127.0.0.1:0" {
		t.Fatalf("unexpected control addr: %s", cfg.ControlAddr)
	}
	if len(cfg.Ports) != 1 || cfg.Ports[0].Name != "ssh" {
		t.Fatalf("unexpected ports: %+v", cfg.Ports)
	}
}

func TestLoadAgentConfigFromJSONFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "agent.json")
	content := `{
		"relay_url": "ws://127.0.0.1:8080/connect",
		"agent_id": "mac-mini",
		"auth_token": "secret",
		"allow_insecure": true,
		"targets": [
			{"name": "ssh", "local_addr": "127.0.0.1:22"}
		]
	}`

	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write agent config: %v", err)
	}

	cfg, err := config.LoadAgentConfig(path)
	if err != nil {
		t.Fatalf("load agent config: %v", err)
	}

	if cfg.RelayURL != "ws://127.0.0.1:8080/connect" {
		t.Fatalf("unexpected relay url: %s", cfg.RelayURL)
	}
	if cfg.AgentID != "mac-mini" {
		t.Fatalf("unexpected agent id: %s", cfg.AgentID)
	}
	if len(cfg.Targets) != 1 || cfg.Targets[0].Name != "ssh" {
		t.Fatalf("unexpected targets: %+v", cfg.Targets)
	}
}

func TestAgentConfigValidateRejectsMissingAgentID(t *testing.T) {
	cfg := config.AgentConfig{
		RelayURL:      "ws://127.0.0.1:443/connect",
		AuthToken:     "secret",
		AllowInsecure: true,
		Targets: []config.TargetMapping{
			{Name: "ssh", LocalAddr: "127.0.0.1:22"},
		},
	}

	if err := cfg.Validate(); err == nil {
		t.Fatal("expected missing agent id to fail validation")
	}
}

func TestRelayConfigValidateRejectsMissingPortRouteFields(t *testing.T) {
	cfg := config.RelayConfig{
		ControlAddr:   "127.0.0.1:0",
		AuthTokens:    []string{"secret"},
		AllowInsecure: true,
		Ports: []config.PortMapping{
			{Name: "ssh", ListenAddr: "127.0.0.1:0"},
		},
	}

	if err := cfg.Validate(); err == nil {
		t.Fatal("expected missing port route fields to fail validation")
	}
}
