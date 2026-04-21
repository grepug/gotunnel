package relay

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"gotunnel/internal/config"
)

const (
	registrationStateVersion   = 1
	registrationStatusNever    = "never_connected"
	registrationStatusActive   = "active"
	registrationStatusInactive = "inactive"
)

type registrationStore struct {
	path string

	mu    sync.Mutex
	state persistedRelayState
}

type persistedRelayState struct {
	Version int                             `json:"version"`
	Agents  map[string]persistedAgentRecord `json:"agents"`
}

type persistedAgentRecord struct {
	AgentID            string    `json:"agent_id"`
	Status             string    `json:"status"`
	LastKnownTargets   []string  `json:"last_known_targets"`
	LastConnectedAt    time.Time `json:"last_connected_at,omitempty"`
	LastDisconnectedAt time.Time `json:"last_disconnected_at,omitempty"`
	UpdatedAt          time.Time `json:"updated_at"`
}

func newRegistrationStore(path string, agents []config.AgentAuth) (*registrationStore, error) {
	store := &registrationStore{
		path: path,
		state: persistedRelayState{
			Version: registrationStateVersion,
			Agents:  make(map[string]persistedAgentRecord),
		},
	}

	if path != "" {
		raw, err := os.ReadFile(path)
		if err == nil {
			if err := json.Unmarshal(raw, &store.state); err != nil {
				return nil, err
			}
			if store.state.Agents == nil {
				store.state.Agents = make(map[string]persistedAgentRecord)
			}
		} else if !errors.Is(err, os.ErrNotExist) {
			return nil, err
		}
	}

	now := time.Now().UTC()
	changed := false
	for _, agent := range agents {
		record, ok := store.state.Agents[agent.AgentID]
		if !ok {
			store.state.Agents[agent.AgentID] = persistedAgentRecord{
				AgentID:   agent.AgentID,
				Status:    registrationStatusNever,
				UpdatedAt: now,
			}
			changed = true
			continue
		}
		if record.Status == registrationStatusActive {
			record.Status = registrationStatusInactive
			record.LastDisconnectedAt = now
			record.UpdatedAt = now
			store.state.Agents[agent.AgentID] = record
			changed = true
		}
	}

	if changed {
		if err := store.saveLocked(); err != nil {
			return nil, err
		}
	}

	return store, nil
}

func (s *registrationStore) markActive(agentID string, targets map[string]struct{}) error {
	if s == nil {
		return nil
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now().UTC()
	record := s.state.Agents[agentID]
	record.AgentID = agentID
	record.Status = registrationStatusActive
	record.LastKnownTargets = sortedTargets(targets)
	record.LastConnectedAt = now
	record.UpdatedAt = now
	s.state.Agents[agentID] = record
	return s.saveLocked()
}

func (s *registrationStore) markInactive(agentID string) error {
	if s == nil {
		return nil
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	record, ok := s.state.Agents[agentID]
	if !ok {
		return nil
	}

	now := time.Now().UTC()
	record.Status = registrationStatusInactive
	record.LastDisconnectedAt = now
	record.UpdatedAt = now
	s.state.Agents[agentID] = record
	return s.saveLocked()
}

func (s *registrationStore) saveLocked() error {
	if s.path == "" {
		return nil
	}

	s.state.Version = registrationStateVersion
	raw, err := json.MarshalIndent(s.state, "", "  ")
	if err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return err
	}

	tmpFile, err := os.CreateTemp(filepath.Dir(s.path), "gotunnel-state-*.json")
	if err != nil {
		return err
	}
	tmpName := tmpFile.Name()
	if _, err := tmpFile.Write(raw); err != nil {
		_ = tmpFile.Close()
		_ = os.Remove(tmpName)
		return err
	}
	if err := tmpFile.Close(); err != nil {
		_ = os.Remove(tmpName)
		return err
	}
	if err := os.Rename(tmpName, s.path); err != nil {
		_ = os.Remove(tmpName)
		return err
	}
	return nil
}

func sortedTargets(targets map[string]struct{}) []string {
	names := make([]string, 0, len(targets))
	for name := range targets {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}
