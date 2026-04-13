package spawner

import (
	"bytes"
	"context"
	"net"
	"os"
	"os/exec"
	"runtime"
	"strings"
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

	t.Run("min equals max returns that port when free", func(t *testing.T) {
		t.Parallel()
		ln, err := net.Listen("tcp", ":0")
		if err != nil {
			t.Fatalf("listen: %v", err)
		}
		port := ln.Addr().(*net.TCPAddr).Port
		if err := ln.Close(); err != nil {
			t.Fatalf("close: %v", err)
		}
		got, err := FindFreePort(port, port)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if got != port {
			t.Fatalf("expected port %d, got %d", port, got)
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

	t.Run("capture output", func(t *testing.T) {
		if runtime.GOOS == "windows" {
			t.Skip("echo is not a standalone binary; output line endings differ from Unix")
		}
		t.Parallel()
		buf := bytes.NewBuffer(nil)
		result, err := SpawnCommand(context.Background(), []string{"echo", "hello"}, []string{}, buf, os.Stderr)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		err = result.Wait()
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		if got := strings.TrimSpace(buf.String()); got != "hello" {
			t.Fatalf("expected output hello, got %q", buf.String())
		}

		if result.ExitCode() != 0 {
			t.Fatalf("expected exit code to be 0, got %d", result.ExitCode())
		}
	})

	t.Run("check exit code", func(t *testing.T) {
		// `exit` is a shell builtin, not a binary on PATH — os/exec needs a real file.
		// Run the builtin via sh (same idea as `sh -c 'exit 1'` in a terminal).
		if runtime.GOOS == "windows" {
			t.Skip("use cmd semantics on Windows if you extend this test")
		}
		t.Parallel()
		result, err := SpawnCommand(context.Background(), []string{"sh", "-c", "exit 1"}, []string{}, os.Stdout, os.Stderr)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		err = result.Wait()
		if err == nil {
			t.Fatalf("expected error, got nil")
		}
		if result.ExitCode() != 1 {
			t.Fatalf("expected exit code to be 1, got %d", result.ExitCode())
		}
	})

	t.Run("check context cancel", func(t *testing.T) {
		if runtime.GOOS == "windows" {
			t.Skip("sleep and signal cancellation differ on Windows")
		}
		t.Parallel()
		sleepPath, err := exec.LookPath("sleep")
		if err != nil {
			t.Skip("sleep not in PATH:", err)
		}
		ctx, cancel := context.WithCancel(context.Background())
		result, err := SpawnCommand(ctx, []string{sleepPath, "30"}, []string{}, os.Stdout, os.Stderr)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		cancel()

		waitDone := make(chan error, 1)
		go func() { waitDone <- result.Wait() }()

		select {
		case err := <-waitDone:
			if err == nil {
				t.Fatalf("expected error, got nil")
			}
			if result.ExitCode() != -1 {
				t.Fatalf("expected exit code to be -1, got %d", result.ExitCode())
			}
		case <-time.After(2 * time.Second):
			t.Fatal("Wait did not return within 2s after context cancel (sleep would take 30s)")
		}
	})

	t.Run("no arguments provided", func(t *testing.T) {
		t.Parallel()
		_, err := SpawnCommand(context.Background(), []string{}, []string{}, os.Stdout, os.Stderr)
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
		result, err := SpawnCommand(context.Background(), []string{exePath}, []string{}, os.Stdout, os.Stderr)
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
		result, err := SpawnCommand(context.Background(), []string{exePath}, []string{}, os.Stdout, os.Stderr)
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
}
