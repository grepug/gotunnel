package relay

import (
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"gotunnel/internal/config"
)

const (
	registrationStateVersion   = 2
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
	Routes  map[string]persistedRouteRecord `json:"routes,omitempty"`
}

type persistedAgentRecord struct {
	AgentID            string    `json:"agent_id"`
	Status             string    `json:"status"`
	LastKnownTargets   []string  `json:"last_known_targets"`
	LastConnectedAt    time.Time `json:"last_connected_at,omitempty"`
	LastDisconnectedAt time.Time `json:"last_disconnected_at,omitempty"`
	UpdatedAt          time.Time `json:"updated_at"`
}

type persistedRouteRecord struct {
	RouteName  string `json:"route_name"`
	ListenAddr string `json:"listen_addr"`
	AgentID    string `json:"agent_id"`
	TargetName string `json:"target_name"`
}

type AgentStatus struct {
	AgentID            string
	Status             string
	LastKnownTargets   []string
	LastConnectedAt    time.Time
	LastDisconnectedAt time.Time
	UpdatedAt          time.Time
}

type RouteRegistration struct {
	RouteName  string
	ListenAddr string
	AgentID    string
	TargetName string
}

func LoadAgentStatuses(path string, agents []config.AgentAuth) ([]AgentStatus, error) {
	state, err := loadPersistedRelayState(path)
	if err != nil {
		return nil, err
	}

	now := time.Now().UTC()
	if state.Agents == nil {
		state.Agents = make(map[string]persistedAgentRecord)
	}
	for _, agent := range agents {
		if _, ok := state.Agents[agent.AgentID]; !ok {
			state.Agents[agent.AgentID] = persistedAgentRecord{
				AgentID:   agent.AgentID,
				Status:    registrationStatusNever,
				UpdatedAt: now,
			}
		}
	}

	ids := make([]string, 0, len(state.Agents))
	for agentID := range state.Agents {
		ids = append(ids, agentID)
	}
	sort.Strings(ids)

	statuses := make([]AgentStatus, 0, len(ids))
	for _, agentID := range ids {
		record := state.Agents[agentID]
		status := record.Status
		if status == registrationStatusActive {
			status = registrationStatusInactive
		}
		if record.AgentID == "" {
			record.AgentID = agentID
		}
		statuses = append(statuses, AgentStatus{
			AgentID:            record.AgentID,
			Status:             status,
			LastKnownTargets:   append([]string(nil), record.LastKnownTargets...),
			LastConnectedAt:    record.LastConnectedAt,
			LastDisconnectedAt: record.LastDisconnectedAt,
			UpdatedAt:          record.UpdatedAt,
		})
	}
	return statuses, nil
}

func newRegistrationStore(path string, agents []config.AgentAuth) (*registrationStore, error) {
	store := &registrationStore{
		path: path,
		state: persistedRelayState{
			Version: registrationStateVersion,
			Agents:  make(map[string]persistedAgentRecord),
			Routes:  make(map[string]persistedRouteRecord),
		},
	}

	if path != "" {
		state, err := loadPersistedRelayState(path)
		if err != nil {
			return nil, err
		}
		store.state = state
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

func loadPersistedRelayState(path string) (persistedRelayState, error) {
	state := persistedRelayState{
		Version: registrationStateVersion,
		Agents:  make(map[string]persistedAgentRecord),
		Routes:  make(map[string]persistedRouteRecord),
	}
	if path == "" {
		return state, nil
	}

	raw, err := os.ReadFile(path)
	if err == nil {
		if err := json.Unmarshal(raw, &state); err != nil {
			return persistedRelayState{}, err
		}
		if state.Agents == nil {
			state.Agents = make(map[string]persistedAgentRecord)
		}
		if state.Routes == nil {
			state.Routes = make(map[string]persistedRouteRecord)
		}
		return state, nil
	}
	if errors.Is(err, os.ErrNotExist) {
		return state, nil
	}
	return persistedRelayState{}, err
}

func LoadRouteRegistrations(path string) ([]RouteRegistration, error) {
	state, err := loadPersistedRelayState(path)
	if err != nil {
		return nil, err
	}

	names := make([]string, 0, len(state.Routes))
	for routeName := range state.Routes {
		names = append(names, routeName)
	}
	sort.Strings(names)

	routes := make([]RouteRegistration, 0, len(names))
	for _, routeName := range names {
		record := state.Routes[routeName]
		if record.RouteName == "" {
			record.RouteName = routeName
		}
		routes = append(routes, RouteRegistration{
			RouteName:  record.RouteName,
			ListenAddr: record.ListenAddr,
			AgentID:    record.AgentID,
			TargetName: record.TargetName,
		})
	}

	return routes, nil
}

func CreateRouteRegistration(path string, agents []config.AgentAuth, staticPorts []config.PortMapping, route RouteRegistration) error {
	state, err := loadPersistedRelayState(path)
	if err != nil {
		return err
	}

	if err := validateRouteRegistration(route, agents); err != nil {
		return err
	}

	if _, ok := state.Routes[route.RouteName]; ok {
		return fmt.Errorf("route_name already exists: %s", route.RouteName)
	}

	routes := routeRegistrationsFromState(state)
	routes = append(routes, route)
	if _, err := resolvePublicRoutes(config.RelayConfig{Ports: staticPorts}, routes); err != nil {
		return err
	}

	state.Routes[route.RouteName] = persistedRouteRecord{
		RouteName:  route.RouteName,
		ListenAddr: route.ListenAddr,
		AgentID:    route.AgentID,
		TargetName: route.TargetName,
	}
	return savePersistedRelayState(path, state)
}

func RemoveRouteRegistration(path, routeName string) (bool, error) {
	state, err := loadPersistedRelayState(path)
	if err != nil {
		return false, err
	}
	if _, ok := state.Routes[routeName]; !ok {
		return false, nil
	}
	delete(state.Routes, routeName)
	if err := savePersistedRelayState(path, state); err != nil {
		return false, err
	}
	return true, nil
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
	return savePersistedRelayState(s.path, s.state)
}

func savePersistedRelayState(path string, state persistedRelayState) error {
	if path == "" {
		return nil
	}

	state.Version = registrationStateVersion
	raw, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}

	tmpFile, err := os.CreateTemp(filepath.Dir(path), "gotunnel-state-*.json")
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
	if err := os.Rename(tmpName, path); err != nil {
		_ = os.Remove(tmpName)
		return err
	}
	return nil
}

func routeRegistrationsFromState(state persistedRelayState) []RouteRegistration {
	names := make([]string, 0, len(state.Routes))
	for routeName := range state.Routes {
		names = append(names, routeName)
	}
	sort.Strings(names)

	routes := make([]RouteRegistration, 0, len(names))
	for _, routeName := range names {
		record := state.Routes[routeName]
		if record.RouteName == "" {
			record.RouteName = routeName
		}
		routes = append(routes, RouteRegistration{
			RouteName:  record.RouteName,
			ListenAddr: record.ListenAddr,
			AgentID:    record.AgentID,
			TargetName: record.TargetName,
		})
	}
	return routes
}

func validateRouteRegistration(route RouteRegistration, agents []config.AgentAuth) error {
	if route.RouteName == "" {
		return errors.New("route_name is required")
	}
	if route.ListenAddr == "" {
		return fmt.Errorf("listen_addr is required for route %s", route.RouteName)
	}
	if _, err := net.ResolveTCPAddr("tcp", route.ListenAddr); err != nil {
		return fmt.Errorf("invalid listen_addr for route %s: %w", route.RouteName, err)
	}
	if route.AgentID == "" {
		return fmt.Errorf("agent_id is required for route %s", route.RouteName)
	}
	knownAgents := make(map[string]struct{}, len(agents))
	for _, agent := range agents {
		knownAgents[agent.AgentID] = struct{}{}
	}
	if _, ok := knownAgents[route.AgentID]; !ok {
		return fmt.Errorf("unknown agent_id %s for route %s", route.AgentID, route.RouteName)
	}
	if route.TargetName == "" {
		return fmt.Errorf("target_name is required for route %s", route.RouteName)
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
