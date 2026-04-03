package main

import (
	"log"
	"os"
	"portless-go/src"
	"strconv"
)

func main() {
	// src.Cli()

	port := src.DefaultPort
	if envPort, err := strconv.Atoi(os.Getenv("PORT")); err == nil {
		port = envPort
	}

	impl := os.Getenv("IMPL")
	maxHops := os.Getenv("MAX_HOPS")
	maxHopsInt := src.MaxHops
	if envMaxHops, err := strconv.Atoi(maxHops); err == nil {
		maxHopsInt = envMaxHops
	}
	config := src.Config{
		Port:    port,
		Impl:    impl,
		MaxHops: maxHopsInt,
	}
	if err := src.StartServer(config); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
