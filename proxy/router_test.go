package proxy

import (
	"os"
	"runtime"
	"testing"
)

func newTestRouteTable(t *testing.T) (*RouteTable, string) {
	t.Helper()

	filePath := t.TempDir() + "/routes.json"

	return NewRouteTable(filePath), filePath
}

func mockStalePIDRoutesFile(t *testing.T, filePath string) {
	t.Helper()
	err := os.WriteFile(filePath, []byte(`[{"hostname": "myapp.localhost", "backend": "http://localhost:3000", "pid": 999999}]`), 0644)
	if err != nil {
		t.Fatalf("Failed to write routes file: %v", err)
	}
}

func TestAddAndLookup(t *testing.T) {
	t.Parallel()

	rt, _ := newTestRouteTable(t)
	tests := []struct {
		hostname string
		backend  string
	}{
		{"myapp.localhost", "http://localhost:3000"},
		{"api.localhost", "http://localhost:4000"},
	}
	for _, tt := range tests {
		if err := rt.AddRoute(tt.hostname, tt.backend, false); err != nil {
			t.Fatalf("AddRoute: %v", err)
		}

		route, ok := rt.Lookup(tt.hostname)
		if !ok {
			t.Fatalf("Lookup: %v", tt.hostname)
		}
		backend := route.Backend
		if backend != tt.backend {
			t.Fatalf("Lookup: got %q, want %q", backend, tt.backend)
		}
	}
}

func TestLookupUnknown(t *testing.T) {
	t.Parallel()

	rt, _ := newTestRouteTable(t)
	route, ok := rt.Lookup("unknown.localhost")
	if ok {
		t.Fatalf("Lookup: got %q, want false", route.Backend)
	}
	backend := route.Backend
	if backend != "" {
		t.Fatalf("Lookup: got %q, want empty string", backend)
	}
}

func TestRemoveRoute(t *testing.T) {
	t.Parallel()

	rt, _ := newTestRouteTable(t)
	if err := rt.AddRoute("myapp.localhost", "http://localhost:3000", false); err != nil {
		t.Fatalf("AddRoute: %v", err)
	}
	rt.RemoveRoute("myapp.localhost")

	route, ok := rt.Lookup("myapp.localhost")
	if ok {
		t.Fatalf("Lookup: got %q, want false", route.Backend)
	}
	backend := route.Backend
	if backend != "" {
		t.Fatalf("Lookup: got %q, want empty string", backend)
	}
}

func TestOverwriteRoute(t *testing.T) {
	t.Parallel()

	rt, _ := newTestRouteTable(t)
	if err := rt.AddRoute("myapp.localhost", "http://localhost:3000", false); err != nil {
		t.Fatalf("AddRoute: %v", err)
	}
	// Same process owns the route: update backend without force (upstream same-owner rule).
	if err := rt.AddRoute("myapp.localhost", "http://localhost:4000", false); err != nil {
		t.Fatalf("AddRoute: %v", err)
	}

	route, ok := rt.Lookup("myapp.localhost")
	if !ok {
		t.Fatalf("Lookup: got %q, want true", route.Backend)
	}
	backend := route.Backend
	if backend != "http://localhost:4000" {
		t.Fatalf("Lookup: got %q, want http://localhost:4000", backend)
	}
}

func TestPersistence(t *testing.T) {
	t.Parallel()

	rt, filePath := newTestRouteTable(t)
	if err := rt.AddRoute("myapp.localhost", "http://localhost:3000", false); err != nil {
		t.Fatalf("AddRoute: %v", err)
	}

	rt1 := NewRouteTable(filePath)
	if err := rt1.Load(); err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(rt1.routes) != 1 {
		t.Fatalf("Expected 1 route, got %d", len(rt1.routes))
	}
	if _, ok := rt1.routes["myapp.localhost"]; !ok {
		t.Fatalf("Expected route for myapp.localhost, got none")
	}
}

func TestLoadMissingFile(t *testing.T) {
	t.Parallel()

	rt, _ := newTestRouteTable(t)
	if err := rt.Load(); err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(rt.routes) != 0 {
		t.Fatalf("Expected 0 route, got %d", len(rt.routes))
	}
}

func TestPruneStalePID(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows does not support pruning stale PIDs")
	}
	t.Parallel()

	rt, _ := newTestRouteTable(t)
	mockStalePIDRoutesFile(t, rt.filePath)
	err := rt.Load()
	if err != nil {
		t.Fatalf("Failed to load routes file: %v", err)
	}

	if len(rt.routes) != 0 {
		t.Fatalf("Expected 0 route, got %d", len(rt.routes))
	}

	if err := rt.Load(); err != nil {
		t.Fatalf("second Load: %v", err)
	}
	if len(rt.routes) != 0 {
		t.Fatalf("after second Load: expected 0 route, got %d", len(rt.routes))
	}
}
