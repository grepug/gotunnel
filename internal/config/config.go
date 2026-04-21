package config

import (
	"errors"
	"fmt"
	"net"
	"net/url"
)

type RelayConfig struct {
	ControlAddr   string        `json:"control_addr"`
	AuthTokens    []string      `json:"auth_tokens"`
	TLSCertFile   string        `json:"tls_cert_file"`
	TLSKeyFile    string        `json:"tls_key_file"`
	AllowInsecure bool          `json:"allow_insecure"`
	Ports         []PortMapping `json:"ports"`
}

type PortMapping struct {
	Name       string `json:"name"`
	ListenAddr string `json:"listen_addr"`
	AgentID    string `json:"agent_id"`
	TargetName string `json:"target_name"`
}

type AgentConfig struct {
	RelayURL      string          `json:"relay_url"`
	AgentID       string          `json:"agent_id"`
	AuthToken     string          `json:"auth_token"`
	AllowInsecure bool            `json:"allow_insecure"`
	Targets       []TargetMapping `json:"targets"`
}

type TargetMapping struct {
	Name      string `json:"name"`
	LocalAddr string `json:"local_addr"`
}

func (c RelayConfig) Validate() error {
	if c.ControlAddr == "" {
		return errors.New("control address is required")
	}
	if _, err := net.ResolveTCPAddr("tcp", c.ControlAddr); err != nil {
		return fmt.Errorf("invalid control address: %w", err)
	}
	if len(c.AuthTokens) == 0 {
		return errors.New("at least one auth token is required")
	}
	if (c.TLSCertFile == "") != (c.TLSKeyFile == "") {
		return errors.New("tls_cert_file and tls_key_file must be set together")
	}
	if !c.AllowInsecure && c.TLSCertFile == "" {
		return errors.New("tls_cert_file and tls_key_file are required unless allow_insecure is true")
	}
	if len(c.Ports) == 0 {
		return errors.New("at least one port mapping is required")
	}

	seen := make(map[string]struct{}, len(c.Ports))
	for _, port := range c.Ports {
		if port.Name == "" {
			return errors.New("port mapping name is required")
		}
		if _, exists := seen[port.Name]; exists {
			return fmt.Errorf("duplicate port mapping name: %s", port.Name)
		}
		seen[port.Name] = struct{}{}

		if port.ListenAddr == "" {
			return fmt.Errorf("listen address is required for port mapping %s", port.Name)
		}
		if _, err := net.ResolveTCPAddr("tcp", port.ListenAddr); err != nil {
			return fmt.Errorf("invalid listen address for %s: %w", port.Name, err)
		}
		if port.AgentID == "" {
			return fmt.Errorf("agent_id is required for port mapping %s", port.Name)
		}
		if port.TargetName == "" {
			return fmt.Errorf("target_name is required for port mapping %s", port.Name)
		}
	}

	return nil
}

func (c AgentConfig) Validate() error {
	if c.RelayURL == "" {
		return errors.New("relay URL is required")
	}
	parsed, err := url.Parse(c.RelayURL)
	if err != nil {
		return fmt.Errorf("invalid relay URL: %w", err)
	}
	if parsed.Scheme != "ws" && parsed.Scheme != "wss" {
		return fmt.Errorf("unsupported relay URL scheme: %s", parsed.Scheme)
	}
	if parsed.Scheme == "ws" && !c.AllowInsecure {
		return errors.New("plain ws relay URL requires allow_insecure to be true")
	}
	if c.AgentID == "" {
		return errors.New("agent_id is required")
	}
	if c.AuthToken == "" {
		return errors.New("auth token is required")
	}
	if len(c.Targets) == 0 {
		return errors.New("at least one target mapping is required")
	}

	seen := make(map[string]struct{}, len(c.Targets))
	for _, target := range c.Targets {
		if target.Name == "" {
			return errors.New("target name is required")
		}
		if _, exists := seen[target.Name]; exists {
			return fmt.Errorf("duplicate target name: %s", target.Name)
		}
		seen[target.Name] = struct{}{}

		if target.LocalAddr == "" {
			return fmt.Errorf("local address is required for target %s", target.Name)
		}
		if _, err := net.ResolveTCPAddr("tcp", target.LocalAddr); err != nil {
			return fmt.Errorf("invalid local address for %s: %w", target.Name, err)
		}
	}

	return nil
}
