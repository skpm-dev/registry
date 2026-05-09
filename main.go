package main

import (
	"fmt"
	"log"
	"os"

	"github.com/skpm-dev/registry/internal/db"
	"github.com/skpm-dev/registry/internal/server"
)

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	dbPath := os.Getenv("DB_PATH")
	if dbPath == "" {
		dbPath = "skpm.db"
	}

	database, err := db.Open(dbPath)
	if err != nil {
		log.Fatalf("could not open database: %v", err)
	}
	defer database.Close()

	srv := server.New(database, port)

	fmt.Printf("skpm registry running on port %s\n", port)
	if err := srv.ListenAndServe(); err != nil {
		log.Fatalf("server error: %v", err)
	}
}
