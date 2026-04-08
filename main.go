package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"portless-go/proxy"
	"strconv"
	"syscall"
	"time"
)

func main() {
	// src.Cli()

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
