package main

import (
	"era/internal/handlers"
	"era/internal/parser"
	"era/internal/storage"
	"log"
	"net/http"
)

func main() {
	// Initialize PocketBase store
	store, err := storage.NewPocketBaseStore()
	if err != nil {
		log.Fatal("Failed to initialize storage:", err)
	}

	// Initialize parser manager with PocketBase instance
	manager, err := parser.NewParserManager(store.GetPocketBase())
	if err != nil {
		log.Fatal("Failed to initialize parser manager:", err)
	}
	defer manager.Cleanup()

	// Initialize handler with both store and parser manager
	countyHandler := handlers.NewCountyHandler(store, manager)

	// Create new mux router (Go 1.22+)
	mux := http.NewServeMux()

	// Register all routes
	mux.HandleFunc("POST /api/county-links", countyHandler.HandleSaveCountyLink)
	mux.HandleFunc("POST /api/county-links/bulk", countyHandler.HandleBulkSaveCountyLinks)
	mux.HandleFunc("GET /api/county-links/{id...}", countyHandler.HandleGetCountyLink)
	mux.HandleFunc("GET /api/county-links", countyHandler.HandleGetCountyLink)
	mux.HandleFunc("PUT /api/county-links/{id}", countyHandler.HandleUpdateCountyLink)
	mux.HandleFunc("DELETE /api/county-links/{id}", countyHandler.HandleDeleteCountyLink)
	mux.HandleFunc("POST /api/county-links/{id}/parse", countyHandler.HandleParseCountyLink)
	mux.HandleFunc("POST /api/bulk-parse/{method}", countyHandler.HandleBulkParseByMethod)
	mux.HandleFunc("POST /api/cleanup", countyHandler.HandleCleanupCollections)

	// Start server
	log.Println("Server starting on :8080...")
	if err := http.ListenAndServe(":8080", mux); err != nil {
		log.Fatal(err)
	}
} 