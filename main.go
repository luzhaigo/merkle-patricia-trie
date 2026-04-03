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
	config := proxy.Config{
		Port:    port,
		Impl:    impl,
		MaxHops: maxHopsInt,
	}

	srv, err := proxy.StartServer(config)
	if err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}

	backend := config.Backend
	if backend == "" {
		backend = proxy.DefaultBackendURL
	}
	log.Printf("Server listening on %s, forwarding to %s", srv.Addr, backend)

	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("ListenAndServe: %v", err)
	}
}
