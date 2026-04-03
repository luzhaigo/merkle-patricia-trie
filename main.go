package main

import (
	"log"
	"net/http"
	"os"
	"portless-go/proxy"
	"strconv"
)

func main() {
	// src.Cli()

	port := proxy.DefaultPort
	if envPort, err := strconv.Atoi(os.Getenv("PORT")); err == nil {
		port = envPort
	}

	impl := os.Getenv("IMPL")
	maxHops := os.Getenv("MAX_HOPS")
	maxHopsInt := proxy.MaxHops
	if envMaxHops, err := strconv.Atoi(maxHops); err == nil {
		maxHopsInt = envMaxHops
	}
	backend := os.Getenv("BACKEND")
	if backend == "" {
		backend = proxy.DefaultBackendURL
	}

	config := proxy.Config{
		Port:    port,
		Impl:    impl,
		MaxHops: maxHopsInt,
		Backend: backend,
	}

	srv, err := proxy.StartServer(config)
	if err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}

	log.Printf("Server listening on %s, forwarding to %s", srv.Addr, config.Backend)

	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("ListenAndServe: %v", err)
	}
}
