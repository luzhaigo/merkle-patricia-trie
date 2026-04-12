package spawner

import (
	"context"
	"fmt"
	"math/rand/v2"
	"net"
	"os"
	"os/exec"
	"sync"
	"syscall"
	"time"
)

var (
	DefaultMinPort = 4000
	DefaultMaxPort = 4999
)

func FindFreePort(min, max int) (port int, outErr error) {
	if min == 0 && max == 0 {
		min = DefaultMinPort
		max = DefaultMaxPort
	}

	if min > max {
		return 0, fmt.Errorf("min port is greater than max port")
	}

	port = rand.IntN(max-min+1) + min

	ln, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err == nil {
		ln.Close()
		return port, nil
	} else {
		outErr = err
	}

	for p := min; p < max+1; p++ {
		if p == port {
			continue
		}
		ln, err = net.Listen("tcp", fmt.Sprintf(":%d", p))
		if err == nil {
			ln.Close()
			return p, nil
		} else {
			outErr = err
		}
	}

	return 0, fmt.Errorf("no free port in range %d-%d: %w", min, max, outErr)
}

type SpawnResult struct {
	PID  int
	Wait func() error
}

func SpawnCommand(ctx context.Context, args []string, extraEnv []string) (*SpawnResult, error) {
	if len(args) == 0 {
		return nil, fmt.Errorf("no arguments provided")
	}

	cmd := exec.CommandContext(ctx, args[0], args[1:]...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = append(os.Environ(), extraEnv...)
	cmd.Cancel = func() error {
		return cmd.Process.Signal(syscall.SIGTERM)
	}
	cmd.WaitDelay = 10 * time.Second
	if err := cmd.Start(); err != nil {
		return nil, err
	}

	var once sync.Once
	var waitErr error
	wait := func() error {
		once.Do(func() {
			waitErr = cmd.Wait()
		})
		return waitErr
	}

	return &SpawnResult{PID: cmd.Process.Pid, Wait: wait}, nil
}
