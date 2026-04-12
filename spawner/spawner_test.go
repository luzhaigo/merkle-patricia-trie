package spawner

import (
	"context"
	"net"
	"os/exec"
	"runtime"
	"sync"
	"testing"
	"time"
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

func TestSpawnCommand(t *testing.T) {
	t.Parallel()
	t.Run("no arguments provided", func(t *testing.T) {
		t.Parallel()
		_, err := SpawnCommand(context.Background(), []string{}, []string{})
		if err == nil {
			t.Fatalf("expected error, got nil")
		}
	})

	t.Run("spawn command and wait for it to exit", func(t *testing.T) {
		if runtime.GOOS == "windows" {
			t.Skip("true is not available on Windows")
		}
		t.Parallel()
		exePath, err := exec.LookPath("true")
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		result, err := SpawnCommand(context.Background(), []string{exePath}, []string{})
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		err = result.Wait()
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
	})

	t.Run("concurrent Wait is safe", func(t *testing.T) {
		if runtime.GOOS == "windows" {
			t.Skip("true is not available on Windows")
		}
		t.Parallel()
		exePath, err := exec.LookPath("true")
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		result, err := SpawnCommand(context.Background(), []string{exePath}, []string{})
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		var wg sync.WaitGroup
		var err0, err1 error
		wg.Add(2)
		go func() {
			err0 = result.Wait()
			wg.Done()
		}()
		go func() {
			err1 = result.Wait()
			wg.Done()
		}()

		wg.Wait()

		if err0 != nil {
			t.Fatalf("expected no error from first Wait, got %v", err0)
		}
		if err1 != nil {
			t.Fatalf("expected no error from second Wait, got %v", err1)
		}

		// Both Wait calls must return the same error value from the shared once.Do
		// closure (including both nil on success).
		if err0 != err1 {
			t.Fatalf("expected errors to be identical, got %v and %v", err0, err1)
		}
	})

	t.Run("context cancel ends long sleep", func(t *testing.T) {
		if runtime.GOOS == "windows" {
			t.Skip("exec cancellation signals differ on Windows")
		}
		t.Parallel()
		sleepPath, err := exec.LookPath("sleep")
		if err != nil {
			t.Skip("sleep not in PATH:", err)
		}

		ctx, cancel := context.WithCancel(context.Background())
		result, err := SpawnCommand(ctx, []string{sleepPath, "60"}, nil)
		if err != nil {
			t.Fatalf("SpawnCommand: %v", err)
		}
		cancel()

		waitDone := make(chan error, 1)
		go func() { waitDone <- result.Wait() }()

		select {
		case <-waitDone:
			// Exit status and wrapping depend on OS/Go (SIGTERM, context, etc.).
			// The contract we assert is: Wait returns without waiting full sleep.
		case <-time.After(5 * time.Second):
			t.Fatal("Wait did not return within 5s after context cancel (sleep would take 60s)")
		}
	})

}
