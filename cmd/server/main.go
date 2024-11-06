package main

import (
	"era/internal/handlers"
	"era/internal/parser"
	"era/internal/storage"
	"log"
	"net/http"
	"os"
)

func main() {
	// Get environment variables with defaults
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	
	dataDir := os.Getenv("DATA_DIR")
	if dataDir == "" {
		dataDir = "./pb_data"
	}

	// Ensure data directory exists
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		log.Fatal("Failed to create data directory:", err)
	}

	// Initialize PocketBase store with data directory
	store, err := storage.NewPocketBaseStore(dataDir)
	if err != nil {
		log.Fatal("Failed to initialize storage:", err)
	}

	// Initialize parser manager
	manager, err := parser.NewParserManager(store.GetPocketBase())
	if err != nil {
		log.Fatal("Failed to initialize parser manager:", err)
	}
	defer manager.Cleanup()

	// Initialize handler
	countyHandler := handlers.NewCountyHandler(store, manager)

	// Create mux router
	mux := http.NewServeMux()

	// Register routes
	mux.HandleFunc("POST /api/county-links", countyHandler.HandleSaveCountyLink)
	mux.HandleFunc("POST /api/county-links/bulk", countyHandler.HandleBulkSaveCountyLinks)
	mux.HandleFunc("GET /api/county-links/{id...}", countyHandler.HandleGetCountyLink)
	mux.HandleFunc("GET /api/county-links", countyHandler.HandleGetCountyLink)
	mux.HandleFunc("PUT /api/county-links/{id}", countyHandler.HandleUpdateCountyLink)
	mux.HandleFunc("DELETE /api/county-links/{id}", countyHandler.HandleDeleteCountyLink)
	mux.HandleFunc("POST /api/county-links/{id}/parse", countyHandler.HandleParseCountyLink)
	mux.HandleFunc("POST /api/bulk-parse/{method}", countyHandler.HandleBulkParseByMethod)
	mux.HandleFunc("POST /api/cleanup", countyHandler.HandleCleanupCollections)
	mux.HandleFunc("GET /api/county-results/{id}", countyHandler.HandleGetCountyResults)
	mux.HandleFunc("GET /api/county-measures/{id}", countyHandler.HandleGetMeasuresHTML)
	mux.HandleFunc("GET /api/county-candidates/{id}", countyHandler.HandleGetCandidatesHTML)

	// Add health check endpoint
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	// Start server
	log.Printf("Server starting on :%s...", port)
	if err := http.ListenAndServe(":"+port, mux); err != nil {
		log.Fatal(err)
	}
} 