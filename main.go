package main

import (
	"log"
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
	if err := proxy.StartServer(config); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
