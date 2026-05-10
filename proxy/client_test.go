package proxy

import (
	"net/http/httptest"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func newTestAdminServer(t *testing.T) (*httptest.Server, *RouteTable) {
	t.Helper()
	rt := NewRouteTable(filepath.Join(t.TempDir(), "routes.json"))
	srv := httptest.NewServer(AdminHandler(rt))
	t.Cleanup(srv.Close)
	return srv, rt
}

func TestRegisterRoute(t *testing.T) {
	srv, rt := newTestAdminServer(t)

	if err := RegisterRoute(srv.URL, "test.localhost", "http://localhost:3000"); err != nil {
		t.Fatalf("RegisterRoute: %v", err)
	}

	route, ok := rt.Lookup("test.localhost")
	if !ok {
		t.Fatalf("expected route to be created")
	}
	if route.Backend != "http://localhost:3000" {
		t.Errorf("backend = %q, want %q", route.Backend, "http://localhost:3000")
	}
}

func TestDeregisterRoute(t *testing.T) {
	srv, rt := newTestAdminServer(t)

	if err := RegisterRoute(srv.URL, "test.localhost", "http://localhost:3000"); err != nil {
		t.Fatalf("RegisterRoute: %v", err)
	}
	if err := DeregisterRoute(srv.URL, "test.localhost"); err != nil {
		t.Fatalf("DeregisterRoute: %v", err)
	}

	if _, ok := rt.Lookup("test.localhost"); ok {
		t.Fatalf("expected route to be removed")
	}
}

func TestRegisterRouteConnectionRefused(t *testing.T) {
	// Bind a port then close it so we know nothing is listening there.
	srv := httptest.NewServer(nil)
	addr := srv.URL
	srv.Close()

	err := RegisterRoute(addr, "test.localhost", "http://localhost:3000")
	if err == nil {
		t.Fatal("expected connection error, got nil")
	}
}

// TestRegisterRouteConflict verifies that re-registering a hostname owned by
// another live PID returns a 409 conflict. The conflict path treats same-PID
// re-registrations as a refresh, so we spawn a real helper process and inject
// its PID as the route owner.
func TestRegisterRouteConflict(t *testing.T) {
	sleeper := exec.Command("sleep", "30")
	if err := sleeper.Start(); err != nil {
		t.Skipf("could not start sleep helper: %v", err)
	}
	t.Cleanup(func() {
		_ = sleeper.Process.Kill()
		_ = sleeper.Wait()
	})

	srv, rt := newTestAdminServer(t)

	rt.mu.Lock()
	rt.routes["test.localhost"] = Route{
		Hostname: "test.localhost",
		Backend:  "http://localhost:3000",
		PID:      sleeper.Process.Pid,
	}
	rt.mu.Unlock()

	err := RegisterRoute(srv.URL, "test.localhost", "http://localhost:4000")
	if err == nil {
		t.Fatal("expected conflict error, got nil")
	}
	if !strings.Contains(err.Error(), "409") {
		t.Errorf("err = %v, want it to mention 409", err)
	}
}
