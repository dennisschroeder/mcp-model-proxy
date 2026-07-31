package main

import (
	"context"
	"log"
)

func main() {
	checker := NewToolChecker()

	// Create server without validating all dependencies upfront
	// Validation happens lazily when a model is invoked
	log.Println("Starting MCP server...")
	log.Println("(Dependencies will be validated when models are used)")

	server := NewMCPServer(checker)
	if err := server.Run(context.Background()); err != nil {
		log.Fatalf("Server error: %v", err)
	}
}
