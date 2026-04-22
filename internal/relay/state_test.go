package relay

import (
	"strings"
	"testing"

	"gotunnel/internal/config"
)

func TestCreateLoadAndRemoveRouteRegistrations(t *testing.T) {
	statePath := t.TempDir() + "/relay-state.json"
	agents := []config.AgentAuth{
		{AgentID: "home-mac", AuthToken: "secret"},
	}

	route := RouteRegistration{
		RouteName:  "ssh",
		ListenAddr: "127.0.0.1:2222",
		AgentID:    "home-mac",
		TargetName: "ssh",
	}

	if err := CreateRouteRegistration(statePath, agents, nil, route); err != nil {
		t.Fatalf("create route registration: %v", err)
	}

	routes, err := LoadRouteRegistrations(statePath)
	if err != nil {
		t.Fatalf("load route registrations: %v", err)
	}
	if len(routes) != 1 {
		t.Fatalf("expected one route registration, got %d", len(routes))
	}
	if routes[0] != route {
		t.Fatalf("unexpected route registration: %+v", routes[0])
	}

	removed, err := RemoveRouteRegistration(statePath, "ssh")
	if err != nil {
		t.Fatalf("remove route registration: %v", err)
	}
	if !removed {
		t.Fatal("expected route registration to be removed")
	}

	routes, err = LoadRouteRegistrations(statePath)
	if err != nil {
		t.Fatalf("load route registrations after remove: %v", err)
	}
	if len(routes) != 0 {
		t.Fatalf("expected no route registrations after remove, got %d", len(routes))
	}
}

func TestCreateRouteRegistrationRejectsEffectiveListenConflict(t *testing.T) {
	statePath := t.TempDir() + "/relay-state.json"
	agents := []config.AgentAuth{
		{AgentID: "home-mac", AuthToken: "secret"},
	}
	staticPorts := []config.PortMapping{
		{
			Name:       "web",
			ListenAddr: "127.0.0.1:28080",
			AgentID:    "home-mac",
			TargetName: "web",
		},
	}

	err := CreateRouteRegistration(statePath, agents, staticPorts, RouteRegistration{
		RouteName:  "desktop",
		ListenAddr: "127.0.0.1:28080",
		AgentID:    "home-mac",
		TargetName: "desktop",
	})
	if err == nil {
		t.Fatal("expected conflicting route registration to fail")
	}
	if !strings.Contains(err.Error(), "listen_addr") {
		t.Fatalf("expected listen_addr conflict error, got %v", err)
	}
}

func TestResolvePublicRoutesPrefersPersistedRoutesByName(t *testing.T) {
	cfg := config.RelayConfig{
		Ports: []config.PortMapping{
			{
				Name:       "ssh",
				ListenAddr: "127.0.0.1:2222",
				AgentID:    "home-mac",
				TargetName: "ssh",
			},
			{
				Name:       "web",
				ListenAddr: "127.0.0.1:28080",
				AgentID:    "home-mac",
				TargetName: "web",
			},
		},
	}

	routes, err := resolvePublicRoutes(cfg, []RouteRegistration{
		{
			RouteName:  "ssh",
			ListenAddr: "127.0.0.1:2200",
			AgentID:    "office-pc",
			TargetName: "ssh",
		},
	})
	if err != nil {
		t.Fatalf("resolve public routes: %v", err)
	}
	if len(routes) != 2 {
		t.Fatalf("expected 2 effective routes, got %d", len(routes))
	}
	if routes[0].Name != "ssh" || routes[0].ListenAddr != "127.0.0.1:2200" || routes[0].AgentID != "office-pc" {
		t.Fatalf("expected persisted ssh route to win, got %+v", routes[0])
	}
	if routes[1].Name != "web" || routes[1].ListenAddr != "127.0.0.1:28080" {
		t.Fatalf("expected static web route to remain, got %+v", routes[1])
	}
}
