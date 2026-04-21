package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

func LoadRelayConfig(path string) (RelayConfig, error) {
	var cfg RelayConfig
	if err := loadJSON(path, &cfg); err != nil {
		return RelayConfig{}, err
	}
	if err := cfg.Validate(); err != nil {
		return RelayConfig{}, err
	}
	return cfg, nil
}

func LoadRelayStatusConfig(path string) (RelayConfig, error) {
	var cfg RelayConfig
	if err := loadJSON(path, &cfg); err != nil {
		return RelayConfig{}, err
	}
	if err := cfg.ValidateStatusMode(); err != nil {
		return RelayConfig{}, err
	}
	return cfg, nil
}

func LoadAgentConfig(path string) (AgentConfig, error) {
	var cfg AgentConfig
	if err := loadJSON(path, &cfg); err != nil {
		return AgentConfig{}, err
	}
	if err := cfg.Validate(); err != nil {
		return AgentConfig{}, err
	}
	return cfg, nil
}

func LoadAgentDraftConfig(path string) (AgentConfig, error) {
	var cfg AgentConfig
	if err := loadJSON(path, &cfg); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return AgentConfig{}, nil
		}
		return AgentConfig{}, err
	}
	return cfg, nil
}

func DefaultAgentConfigPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve user home: %w", err)
	}
	return filepath.Join(home, ".gotunnel", "agent.json"), nil
}

func LoadDefaultAgentConfig() (AgentConfig, error) {
	path, err := DefaultAgentConfigPath()
	if err != nil {
		return AgentConfig{}, err
	}
	return LoadAgentConfig(path)
}

func SaveDefaultAgentConfig(cfg AgentConfig) (string, error) {
	path, err := DefaultAgentConfigPath()
	if err != nil {
		return "", err
	}
	if err := SaveAgentConfig(path, cfg); err != nil {
		return "", err
	}
	return path, nil
}

func SaveAgentConfig(path string, cfg AgentConfig) error {
	raw, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("encode config %s: %w", path, err)
	}
	raw = append(raw, '\n')

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create config dir %s: %w", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, raw, 0o600); err != nil {
		return fmt.Errorf("write config %s: %w", path, err)
	}
	return nil
}

func loadJSON(path string, dst any) error {
	raw, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read config %s: %w", path, err)
	}
	if err := json.Unmarshal(raw, dst); err != nil {
		return fmt.Errorf("decode config %s: %w", path, err)
	}
	return nil
}
