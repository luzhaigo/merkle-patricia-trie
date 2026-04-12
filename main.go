package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"portless-go/proxy"
	"portless-go/spawner"
	"strconv"
	"syscall"
	"time"
)

func main() {
	// src.Cli()

	if len(os.Args) <= 1 {
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
	} else {
		port, err := spawner.FindFreePort(4000, 4999)
		if err != nil {
			log.Fatalf("Failed to find free port: %v", err)
		}
		log.Printf("Found free port: %d", port)

		hostname := os.Args[1]
		if hostname == "" {
			log.Fatalf("Hostname is required")
		}
		hostname = hostname + ".localhost"
		backendURL := fmt.Sprintf("http://localhost:%d", port)
		log.Printf("Adding route: %s -> %s", hostname, backendURL)
		rt := proxy.NewRouteTable(proxy.GetRoutesFilePath())
		err = rt.AddRoute(hostname, backendURL, false)
		if err != nil {
			log.Fatalf("Failed to add route: %v", err)
		}

		signalCtx, signalCancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
		defer signalCancel()

		result, err := spawner.SpawnCommand(signalCtx, os.Args[2:], []string{fmt.Sprintf("PORT=%d", port)})
		if err != nil {
			log.Fatalf("Failed to spawn command: %v", err)
		}
		log.Printf("Spawned command: %d", result.PID)

		err = result.Wait()
		if err := rt.RemoveRoute(hostname); err != nil {
			log.Fatalf("Failed to remove route: %v", err)
		}
		if err != nil {
			log.Fatalf("Failed to wait for command: %v", err)
		}
		log.Printf("Command exited with code: %d", result.ExitCode())
	}

}
