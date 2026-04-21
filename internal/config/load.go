package config

import (
	"encoding/json"
	"fmt"
	"os"
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
