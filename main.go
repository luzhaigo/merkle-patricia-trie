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
	if err := src.StartServer(port, impl); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
