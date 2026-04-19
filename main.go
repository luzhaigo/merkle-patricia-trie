package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"portless-go/cli"
	"portless-go/proxy"
	"portless-go/spawner"
	"strconv"
	"syscall"
	"time"
)

func startProxy() {
	log.Println("start proxy")

	port := proxy.DefaultPort
	if envPort, err := strconv.Atoi(os.Getenv("PORT")); err == nil {
		port = envPort
	}
	adminPort := port + 1
	if envAdminPort, err := strconv.Atoi(os.Getenv("ADMIN_PORT")); err == nil {
		adminPort = envAdminPort
	}

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
		if err := adminSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("ListenAndServe: %v", err)
		}
	}()

	go func() {
		log.Printf("Server listening on %s (host-based routing)", srv.Addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
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

func runSpawn(name string, cmdArgs []string) error {
	if len(cmdArgs) == 0 {
		return fmt.Errorf("missing command after %s", name)
	}

	port, err := spawner.FindFreePort(4000, 4999)
	if err != nil {
		return fmt.Errorf("find free port: %w", err)
	}
	log.Printf("Found free port: %d", port)

	hostname := name + ".localhost"
	backendURL := fmt.Sprintf("http://localhost:%d", port)
	log.Printf("Adding route: %s -> %s", hostname, backendURL)
	rt := proxy.NewRouteTable(proxy.GetRoutesFilePath())
	if err := rt.AddRoute(hostname, backendURL, false); err != nil {
		return fmt.Errorf("add route: %w", err)
	}

	signalCtx, signalCancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer signalCancel()

	result, err := spawner.SpawnCommand(signalCtx, cmdArgs, []string{fmt.Sprintf("PORT=%d", port)}, os.Stdout, os.Stderr)
	if err != nil {
		return fmt.Errorf("spawn: %w", err)
	}
	log.Printf("Spawned command: %d", result.PID)

	waitErr := result.Wait()
	if err := rt.RemoveRoute(hostname); err != nil {
		log.Printf("remove route: %v", err)
	}
	if waitErr != nil {
		return fmt.Errorf("wait: %w", waitErr)
	}
	log.Printf("Command exited with code: %d", result.ExitCode())
	return nil
}

func main() {
	err := cli.Parse(cli.ParseOptions{
		OnDefault: startProxy,
		OnList: func() {
			log.Println("list (not wired to admin API yet)")
		},
		OnRun: runSpawn,
	})
	if err != nil {
		log.Fatalf("%v", err)
	}
}
