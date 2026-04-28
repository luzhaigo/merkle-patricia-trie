package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"portless-go/cli"
	"portless-go/proxy"
	"portless-go/spawner"
	"strconv"
	"syscall"
	"time"
)

func resolveAdminPortFromProxyPort(port int) int {
	adminPort := port + 1
	if envAdminPort, err := strconv.Atoi(os.Getenv("ADMIN_PORT")); err == nil {
		adminPort = envAdminPort
	}

	return adminPort
}

func startProxy() {
	log.Println("start proxy")

	port := resolveProxyPort()
	adminPort := resolveAdminPortFromProxyPort(port)

	impl := os.Getenv("IMPL")
	maxHops := os.Getenv("MAX_HOPS")
	maxHopsInt := proxy.MaxHops
	if envMaxHops, err := strconv.Atoi(maxHops); err == nil {
		maxHopsInt = envMaxHops
	}

	config := proxy.Config{
		Port:      port,
		Impl:      impl,
		MaxHops:   maxHopsInt,
		AdminPort: adminPort,
	}

	rt := proxy.NewRouteTable(proxy.GetRoutesFilePath())

	srv, adminSrv, err := proxy.StartServer(config, rt)
	if err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	go func() {
		log.Printf("Admin server listening on %s", adminSrv.Addr)
		if err := adminSrv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("ListenAndServe: %v", err)
		}
	}()

	go func() {
		log.Printf("Server listening on %s (host-based routing)", srv.Addr)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("ListenAndServe: %v", err)
		}
	}()

	log.Println("waiting for signal")
	<-ctx.Done()
	log.Println("shutting down servers")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := adminSrv.Shutdown(shutdownCtx); err != nil {
		log.Fatalf("Failed to shutdown admin server: %v", err)
	}
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Fatalf("Failed to shutdown server: %v", err)
	}
	log.Println("servers shut down")
}

func resolveProxyPort() int {
	if v, err := strconv.Atoi(os.Getenv("PORT")); err == nil {
		return v
	}
	return proxy.DefaultPort
}

func resolveAdminAddr() string {
	adminPort := resolveAdminPortFromProxyPort(resolveProxyPort())
	return fmt.Sprintf("localhost:%d", adminPort)
}

func runApp(name string, cmdArgs []string) (int, error) {
	port, err := spawner.FindFreePort(4000, 4999)
	if err != nil {
		return 0, fmt.Errorf("find free port: %w", err)
	}

	hostname := name + ".localhost"
	backendURL := fmt.Sprintf("http://localhost:%d", port)
	log.Printf("%s -> %s (PORT=%d)", hostname, backendURL, port)
	adminAddr := resolveAdminAddr()
	if err := proxy.RegisterRoute(adminAddr, hostname, backendURL); err != nil {
		return 0, fmt.Errorf("register route: %w", err)
	}

	signalCtx, signalCancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer signalCancel()

	result, err := spawner.SpawnCommand(signalCtx, cmdArgs, []string{fmt.Sprintf("PORT=%d", port)}, os.Stdout, os.Stderr)
	if err != nil {
		return 0, fmt.Errorf("spawn: %w", err)
	}
	log.Printf("Spawned command: %d", result.PID)

	waitErr := result.Wait()
	if err := proxy.DeregisterRoute(adminAddr, hostname); err != nil {
		log.Printf("remove route: %v", err)
	}
	exitCode := result.ExitCode()
	if waitErr != nil {
		if exitErr, ok := waitErr.(*exec.ExitError); ok {
			return exitErr.ExitCode(), nil
		}
		return 0, fmt.Errorf("wait: %w", waitErr)
	}

	log.Printf("Command exited with code: %d", exitCode)
	return exitCode, nil
}

func main() {
	exitCode, err := cli.Parse(cli.ParseOptions{
		OnDefault: startProxy,
		OnList: func() {
			log.Println("list (not wired to admin API yet)")
		},
		OnRun: runApp,
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
	}
	os.Exit(exitCode)
}
