package spawner

import (
	"net"
	"testing"
)

func TestFindFreePort(t *testing.T) {
	t.Parallel()
	t.Run("invalid range", func(t *testing.T) {
		t.Parallel()
		_, err := FindFreePort(4999, 4000)
		if err == nil {
			t.Fatal("expected error, got nil")
		}
	})

	t.Run("free port is found", func(t *testing.T) {
		t.Parallel()
		port, err := FindFreePort(4000, 4999)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if port < 4000 || port > 4999 {
			t.Fatalf("expected port to be in range 4000-4999, got %d", port)
		}
	})

	t.Run("free port is found in range 0-0", func(t *testing.T) {
		t.Parallel()
		port, err := FindFreePort(0, 0)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if port < 4000 || port > 4999 {
			t.Fatalf("expected port to be in range 4000-4999, got %d", port)
		}
	})

	t.Run("bind on port 4000", func(t *testing.T) {
		t.Parallel()
		_, err := FindFreePort(4000, 0)
		if err == nil {
			t.Fatalf("expected error, got nil")
		}
	})

	t.Run("free port is found in range 2000-2500", func(t *testing.T) {
		t.Parallel()
		port, err := FindFreePort(2000, 2500)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if port < 2000 || port > 2500 {
			t.Fatalf("expected port to be in range 2000-2500, got %d", port)
		}
	})

	t.Run("error when single port in range is busy", func(t *testing.T) {
		t.Parallel()

		// Bind the same way FindFreePort does (all interfaces on a port), using an
		// ephemeral port so we don't collide with fixed ports or other tests.
		ln, err := net.Listen("tcp", ":0")
		if err != nil {
			t.Fatalf("listen: %v", err)
		}
		defer ln.Close()

		port := ln.Addr().(*net.TCPAddr).Port
		_, err = FindFreePort(port, port)
		if err == nil {
			t.Fatalf("expected error when the only candidate port is already bound, got nil")
		}
	})
}
