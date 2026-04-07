package proxy

import (
	"os"
	"runtime"
	"testing"
)

func mockRoutesFile(t *testing.T, filePath string) *RouteTable {
	t.Helper()
	err := os.WriteFile(filePath, []byte(`[{"hostname": "myapp.localhost", "backend": "http://localhost:3000", "pid": 999999}]`), 0644)
	if err != nil {
		t.Fatalf("Failed to write routes file: %v", err)
	}
	return NewRouteTable(filePath)
}

func TestPruneStalePID(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows does not support pruning stale PIDs")
	}
	t.Parallel()

	filePath := t.TempDir() + "/routes.json"
	rt := mockRoutesFile(t, filePath)
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
