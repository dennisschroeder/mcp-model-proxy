package main

import (
	"context"
	"log"
)

func main() {
	checker := NewToolChecker()

	if err := checker.ValidateAll(); err != nil {
		log.Fatalf("Dependency check failed:\n%v", err)
	}

	log.Println("✓ All dependencies validated")
	log.Println("Starting MCP server...")

	server := NewMCPServer(checker)
	if err := server.Run(context.Background()); err != nil {
		log.Fatalf("Server error: %v", err)
	}
}
