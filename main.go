package main

import (
	"fmt"
	"log"
	"os"

	"github.com/skpm-dev/registry/internal/server"
	"github.com/skpm-dev/registry/internal/store"
)

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	if os.Getenv("REGISTRY_GITHUB_TOKEN") == "" {
		log.Fatal("REGISTRY_GITHUB_TOKEN is not set")
	}

	store.InitDB()

	srv := server.New(port)

	fmt.Printf("skpm registry running on port %s\n", port)
	if err := srv.ListenAndServe(); err != nil {
		log.Fatalf("server error: %v", err)
	}
}
