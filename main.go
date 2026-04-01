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

	if err := src.StartServer(port); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
