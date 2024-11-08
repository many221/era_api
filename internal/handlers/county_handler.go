package handlers

import (
	"context"
	"encoding/json"
	"era/internal/models"
	"era/internal/parser"
	"era/internal/storage"
	"fmt"
	"github.com/pocketbase/dbx"
	pb "github.com/pocketbase/pocketbase/models"
	"html/template"
	"log"
	"net/http"
	"strings"
)

// Type definitions
type MeasureGroup struct {
	Title    string
	Measures []Measure
}

type Measure struct {
	Name        string
	Description string
	YesVotes    string
	NoVotes     string
}

type Race struct {
	Title      string
	Candidates []Candidate
}

type Candidate struct {
	Name     string
	Position string
	Votes    string
	Percentage string
}

type Result struct {
	CountyName string `json:"county_name"`
	Success    bool   `json:"success"`
	Error      string `json:"error,omitempty"`
}

type ParseRequest struct {
	CountyName  string `json:"county_name"`
	Link        string `json:"link"`
	ParseMethod string `json:"parse_method"`
	ResultType  string `json:"result_type"` // "measures" or "candidates"
}

type BulkParseRequest struct {
	Links []ParseRequest `json:"links"`
}

// CountyHandler definition
type CountyHandler struct {
	store   *storage.PocketBaseStore
	manager *parser.ParserManager
}

// Helper functions
func NewCountyHandler(store *storage.PocketBaseStore, manager *parser.ParserManager) *CountyHandler {
	return &CountyHandler{
		store:   store,
		manager: manager,
	}
}

func formatVotes(votes int) string {
	if votes == 0 {
		return "NA"
	}
	return fmt.Sprintf("%d", votes)
}

func filter(results []Result, fn func(Result) bool) []Result {
	filtered := make([]Result, 0)
	for _, result := range results {
		if fn(result) {
			filtered = append(filtered, result)
		}
	}
	return filtered
}

// Add this helper function at the top with other helper functions
func enableCORS(w http.ResponseWriter, r *http.Request) {
	// Allow both localhost and deployed frontend
	origins := []string{
		"http://localhost:5173",
		"https://era-fe-sparkling-sun-7787.fly.dev",
	}
	
	// Get the origin from the request header
	origin := r.Header.Get("Origin")
	
	// Check if the origin is allowed
	for _, allowedOrigin := range origins {
		if origin == allowedOrigin {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			break
		}
	}
	
	w.Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS, PUT, DELETE")
	w.Header().Set("Access-Control-Allow-Headers", "Accept, Content-Type, Content-Length, Accept-Encoding, Authorization")
}

// County Link Management Handlers
func (h *CountyHandler) HandleSaveCountyLink(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var countyLink models.CountyLink
	if err := json.NewDecoder(r.Body).Decode(&countyLink); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	if err := countyLink.Validate(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if err := h.store.SaveCountyLink(&countyLink); err != nil {
		http.Error(w, "Error saving county link", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]string{
		"message": "County link saved successfully",
	})
}

func (h *CountyHandler) HandleGetCountyLink(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	id := r.PathValue("id")
	if id == "" {
		links, err := h.store.GetAllCountyLinks()
		if err != nil {
			http.Error(w, "Error fetching county links", http.StatusInternalServerError)
			return
		}
		json.NewEncoder(w).Encode(links)
		return
	}

	link, err := h.store.GetCountyLink(id)
	if err != nil {
		http.Error(w, "County link not found", http.StatusNotFound)
		return
	}

	json.NewEncoder(w).Encode(link)
}

func (h *CountyHandler) HandleUpdateCountyLink(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	id := r.PathValue("id")
	if id == "" {
		http.Error(w, "ID is required", http.StatusBadRequest)
		return
	}

	var countyLink models.CountyLink
	if err := json.NewDecoder(r.Body).Decode(&countyLink); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	if err := countyLink.Validate(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if err := h.store.UpdateCountyLink(id, &countyLink); err != nil {
		http.Error(w, "Error updating county link", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{
		"message": "County link updated successfully",
	})
}

func (h *CountyHandler) HandleDeleteCountyLink(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	id := r.PathValue("id")
	if id == "" {
		http.Error(w, "ID is required", http.StatusBadRequest)
		return
	}

	if err := h.store.DeleteCountyLink(id); err != nil {
		http.Error(w, "Error deleting county link", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{
		"message": "County link deleted successfully",
	})
}

func (h *CountyHandler) HandleBulkSaveCountyLinks(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var countyLinks []models.CountyLink
	if err := json.NewDecoder(r.Body).Decode(&countyLinks); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	// Validate all links before saving
	for i, link := range countyLinks {
		if err := link.Validate(); err != nil {
			http.Error(w, fmt.Sprintf("Invalid link at index %d: %s", i, err.Error()), http.StatusBadRequest)
			return
		}
	}

	// Save all links
	var savedCount int
	var errors []string
	for i, link := range countyLinks {
		if err := h.store.SaveCountyLink(&link); err != nil {
			errors = append(errors, fmt.Sprintf("Failed to save link at index %d: %s", i, err.Error()))
			continue
		}
		savedCount++
	}

	// Prepare response
	response := map[string]interface{}{
		"total_submitted": len(countyLinks),
		"saved_count":    savedCount,
	}
	if len(errors) > 0 {
		response["errors"] = errors
	}

	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(response)
}

// Parsing Handlers
func (h *CountyHandler) HandleParseCountyLink(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Get ID from path
	id := r.PathValue("id")
	if id == "" {
		http.Error(w, "ID is required", http.StatusBadRequest)
		return
	}

	// Get the county link
	countyLink, err := h.store.GetCountyLink(id)
	if err != nil {
		http.Error(w, "County link not found", http.StatusNotFound)
		return
	}

	// Get the parser
	p, err := h.manager.GetParser(string(countyLink.ParseMethod))
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to get parser: %v", err), http.StatusInternalServerError)
		return
	}

	// Set county name for the parser
	p.SetCountyName(countyLink.CountyName)

	// Parse the URL
	ctx := r.Context()
	if err := p.Parse(ctx, countyLink.Link); err != nil {
		http.Error(w, fmt.Sprintf("Failed to parse data: %v", err), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{
		"message": fmt.Sprintf("Successfully parsed data for county: %s", countyLink.CountyName),
	})
}

func (h *CountyHandler) HandleBulkParseByMethod(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	parseMethod := r.PathValue("method")
	log.Printf("Starting bulk parse for method: %s", parseMethod)

	links, err := h.store.GetAllCountyLinks()
	if err != nil {
		log.Printf("Error fetching county links: %v", err)
		http.Error(w, "Error fetching county links", http.StatusInternalServerError)
		return
	}
	log.Printf("Found %d total county links", len(links))

	if len(links) == 0 {
		log.Printf("No county links found")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"message": "No county links found to process",
		})
		return
	}

	var results struct {
		TotalCounties int      `json:"total_counties"`
		Processed     int      `json:"processed"`
		Successful    int      `json:"successful"`
		Failed        []string `json:"failed,omitempty"`
	}

	// Process each matching county
	for _, link := range links {
		log.Printf("Examining county: %s (Method: %s)", link.CountyName, link.ParseMethod)
		
		if string(link.ParseMethod) != parseMethod {
			log.Printf("Skipping county %s - different parse method (%s != %s)", 
				link.CountyName, link.ParseMethod, parseMethod)
			continue
		}
		results.TotalCounties++

		log.Printf("Processing county: %s with URL: %s", link.CountyName, link.Link)
		
		p, err := h.manager.GetParser(parseMethod)
		if err != nil {
			errMsg := fmt.Sprintf("County %s: Failed to get parser: %v", link.CountyName, err)
			log.Printf("Error: %s", errMsg)
			results.Failed = append(results.Failed, errMsg)
			continue
		}

		log.Printf("Setting county name for parser: %s", link.CountyName)
		p.SetCountyName(link.CountyName)

		log.Printf("Starting parse for county %s", link.CountyName)
		ctx := r.Context()
		if err := p.Parse(ctx, link.Link); err != nil {
			errMsg := fmt.Sprintf("County %s: %v", link.CountyName, err)
			log.Printf("Error: %s", errMsg)
			results.Failed = append(results.Failed, errMsg)
			continue
		}

		results.Successful++
		results.Processed++
		log.Printf("Successfully processed county: %s", link.CountyName)
	}

	log.Printf("Bulk parse completed. Total: %d, Processed: %d, Successful: %d, Failed: %d",
		results.TotalCounties, results.Processed, results.Successful, len(results.Failed))

	// Always return a response, even if no counties were processed
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(results)
}

func (h *CountyHandler) HandleDirectParse(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req ParseRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	// Validate required fields (ResultType is optional for direct parse)
	if req.CountyName == "" || req.Link == "" || req.ParseMethod == "" {
		http.Error(w, "Missing required fields", http.StatusBadRequest)
		return
	}

	// Get the parser
	p, err := h.manager.GetParser(req.ParseMethod)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to get parser: %v", err), http.StatusInternalServerError)
		return
	}

	// Set county name for the parser
	p.SetCountyName(req.CountyName)

	// Parse the URL
	ctx := r.Context()
	if err := p.Parse(ctx, req.Link); err != nil {
		http.Error(w, fmt.Sprintf("Failed to parse data: %v", err), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{
		"message": fmt.Sprintf("Successfully parsed data for county: %s", req.CountyName),
	})
}

func (h *CountyHandler) HandleDirectBulkParse(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req BulkParseRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	results := make([]Result, 0, len(req.Links))
	
	for _, link := range req.Links {
		result := Result{
			CountyName: link.CountyName,
			Success:    true,
		}

		// Get the parser
		p, err := h.manager.GetParser(link.ParseMethod)
		if err != nil {
			result.Success = false
			result.Error = fmt.Sprintf("Failed to get parser: %v", err)
			results = append(results, result)
			continue
		}

		// Set county name for the parser
		p.SetCountyName(link.CountyName)

		// Parse the URL
		ctx := r.Context()
		if err := p.Parse(ctx, link.Link); err != nil {
			result.Success = false
			result.Error = fmt.Sprintf("Failed to parse data: %v", err)
			results = append(results, result)
			continue
		}

		results = append(results, result)
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"results": results,
		"total": len(req.Links),
		"successful": len(filter(results, func(r Result) bool { return r.Success })),
	})
}

// Results Handlers
func (h *CountyHandler) HandleGetCountyResults(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Get county ID from path
	countyID := r.PathValue("id")
	if countyID == "" {
		http.Error(w, "County ID is required", http.StatusBadRequest)
		return
	}

	// Get optional type filter from query params
	resultType := r.URL.Query().Get("type") // "candidate" or "measure"

	// Get results from PocketBase
	collectionName := fmt.Sprintf("county_%s_results", countyID)
	collection, err := h.store.GetPocketBase().Dao().FindCollectionByNameOrId(collectionName)
	if err != nil {
		http.Error(w, "County results not found", http.StatusNotFound)
		return
	}

	// Build query
	query := h.store.GetPocketBase().Dao().RecordQuery(collection)
	if resultType != "" {
		query.AndWhere(dbx.HashExp{"type": resultType})
	}

	// Execute query
	var records []*pb.Record
	if err := query.All(&records); err != nil {
		log.Printf("Error fetching results: %v", err)
		http.Error(w, "Error fetching results", http.StatusInternalServerError)
		return
	}

	// Convert records to response format
	type Result struct {
		ID          string  `json:"id"`
		Type        string  `json:"type"`
		ContestName string  `json:"contest_name"`
		ChoiceName  string  `json:"choice_name"`
		Votes       int     `json:"votes"`
		Percentage  float64 `json:"percentage"`
		IsBond      bool    `json:"is_bond,omitempty"`
	}

	results := make([]Result, len(records))
	for i, record := range records {
		results[i] = Result{
			ID:          record.Id,
			Type:        record.GetString("type"),
			ContestName: record.GetString("contest_name"),
			ChoiceName:  record.GetString("choice_name"),
			Votes:       int(record.GetInt("votes")),
			Percentage:  record.GetFloat("percentage"),
		}
		if results[i].Type == "measure" {
			results[i].IsBond = record.GetBool("is_bond")
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"total":   len(results),
		"results": results,
	})
}

func (h *CountyHandler) HandleGetMeasuresHTML(w http.ResponseWriter, r *http.Request) {
	countyID := r.PathValue("id")
	log.Printf("Starting measures request for county: %s", countyID)

	// Parse latest data first
	if err := h.parseCountyData(countyID); err != nil {
		log.Printf("Warning: Failed to parse latest data: %v", err)
	} else {
		log.Printf("Successfully parsed latest data for county: %s", countyID)
	}

	// Get results from PocketBase - use the same collection name format as the parser
	collectionName := fmt.Sprintf("county_%s_results", "marin") // Hardcode to "marin" for now
	collection, err := h.store.GetPocketBase().Dao().FindCollectionByNameOrId(collectionName)
	if err != nil {
		log.Printf("Error finding collection %s: %v", collectionName, err)
		http.Error(w, "County results not found", http.StatusNotFound)
		return
	}

	// Query measure results
	query := h.store.GetPocketBase().Dao().RecordQuery(collection).
		AndWhere(dbx.HashExp{"type": "measure"})

	var records []*pb.Record
	if err := query.All(&records); err != nil {
		log.Printf("Error fetching results: %v", err)
		http.Error(w, "Error fetching results", http.StatusInternalServerError)
		return
	}

	// Group measures by contest
	groupMap := make(map[string]*MeasureGroup)
	for _, record := range records {
		contestName := record.GetString("contest_name")
		if _, exists := groupMap[contestName]; !exists {
			groupMap[contestName] = &MeasureGroup{
				Title: contestName,
			}
		}

		measure := Measure{
			Name:        record.GetString("choice_name"),
			Description: record.GetString("description"),
			YesVotes:    formatVotes(record.GetInt("yes_votes")),
			NoVotes:     formatVotes(record.GetInt("no_votes")),
		}
		groupMap[contestName].Measures = append(groupMap[contestName].Measures, measure)
	}

	// Convert map to slice
	var groups []MeasureGroup
	for _, group := range groupMap {
		groups = append(groups, *group)
	}

	// Parse and execute template
	tmpl, err := template.ParseFiles("internal/templates/measures.html")
	if err != nil {
		log.Printf("Error parsing template: %v", err)
		http.Error(w, "Error parsing template", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/html")
	if err := tmpl.Execute(w, map[string]interface{}{
		"Groups": groups,
	}); err != nil {
		log.Printf("Error executing template: %v", err)
		http.Error(w, "Error executing template", http.StatusInternalServerError)
		return
	}
}

func (h *CountyHandler) HandleGetCandidatesHTML(w http.ResponseWriter, r *http.Request) {
	countyID := r.PathValue("id")
	log.Printf("Starting candidates request for county: %s", countyID)

	// Parse latest data first
	if err := h.parseCountyData(countyID); err != nil {
		log.Printf("Warning: Failed to parse latest data: %v", err)
	} else {
		log.Printf("Successfully parsed latest data for county: %s", countyID)
	}

	// Get results from PocketBase - use the same collection name format as the parser
	collectionName := fmt.Sprintf("county_%s_results", "marin") // Hardcode to "marin" for now
	collection, err := h.store.GetPocketBase().Dao().FindCollectionByNameOrId(collectionName)
	if err != nil {
		log.Printf("Error finding collection %s: %v", collectionName, err)
		http.Error(w, "County results not found", http.StatusNotFound)
		return
	}

	// Query candidate results
	query := h.store.GetPocketBase().Dao().RecordQuery(collection).
		AndWhere(dbx.HashExp{"type": "candidate"})

	var records []*pb.Record
	if err := query.All(&records); err != nil {
		log.Printf("Error fetching results: %v", err)
		http.Error(w, "Error fetching results", http.StatusInternalServerError)
		return
	}

	// Group candidates by race
	raceMap := make(map[string]*Race)
	for _, record := range records {
		contestName := record.GetString("contest_name")
		if _, exists := raceMap[contestName]; !exists {
			raceMap[contestName] = &Race{
				Title: contestName,
			}
		}

		candidate := Candidate{
			Name:       record.GetString("choice_name"),
			Position:   record.GetString("description"),
			Votes:      formatVotes(record.GetInt("votes")),
			Percentage: formatPercentage(record.GetFloat("percentage")),
		}
		raceMap[contestName].Candidates = append(raceMap[contestName].Candidates, candidate)
	}

	// Convert map to slice
	var races []Race
	for _, race := range raceMap {
		races = append(races, *race)
	}

	// Parse and execute template
	tmpl, err := template.ParseFiles("internal/templates/candidates.html")
	if err != nil {
		log.Printf("Error parsing template: %v", err)
		http.Error(w, "Error parsing template", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/html")
	if err := tmpl.Execute(w, map[string]interface{}{
		"Races": races,
	}); err != nil {
		log.Printf("Error executing template: %v", err)
		http.Error(w, "Error executing template", http.StatusInternalServerError)
		return
	}
}

// System Operation Handlers
func (h *CountyHandler) HandleCleanupCollections(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	log.Printf("Starting collection cleanup...")
	
	// Get all collections using FindCollectionsByType
	collections, err := h.store.GetPocketBase().Dao().FindCollectionsByType("base")
	if err != nil {
		log.Printf("Error fetching collections: %v", err)
		http.Error(w, "Failed to fetch collections", http.StatusInternalServerError)
		return
	}
	
	var deleted []string
	var skipped []string

	// Delete all collections except county_links
	for _, collection := range collections {
		if collection.Name == "county_links" {
			skipped = append(skipped, collection.Name)
			continue
		}

		// Skip system collections (those starting with underscore)
		if strings.HasPrefix(collection.Name, "_") {
			skipped = append(skipped, collection.Name)
			continue
		}

		log.Printf("Deleting collection: %s", collection.Name)
		if err := h.store.GetPocketBase().Dao().DeleteCollection(collection); err != nil {
			log.Printf("Error deleting collection %s: %v", collection.Name, err)
			http.Error(w, fmt.Sprintf("Failed to delete collection %s", collection.Name), http.StatusInternalServerError)
			return
		}
		deleted = append(deleted, collection.Name)
	}

	// Prepare response
	response := map[string]interface{}{
		"deleted": deleted,
		"skipped": skipped,
		"message": "Collections cleanup completed successfully",
	}

	log.Printf("Cleanup completed. Deleted: %d collections, Skipped: %d collections", 
		len(deleted), len(skipped))

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}

// Helper method for parsing county data
func (h *CountyHandler) parseCountyData(countyID string) error {
	// Get the county link info
	countyLink, err := h.store.GetCountyLink(countyID)
	if err != nil {
		return fmt.Errorf("failed to get county link: %w", err)
	}

	// Get the parser
	p, err := h.manager.GetParser(string(countyLink.ParseMethod))
	if err != nil {
		return fmt.Errorf("failed to get parser: %w", err)
	}

	// Set county name for the parser
	p.SetCountyName(countyLink.CountyName)

	// Parse the URL
	ctx := context.Background()
	if err := p.Parse(ctx, countyLink.Link); err != nil {
		return fmt.Errorf("failed to parse data: %w", err)
	}

	return nil
}

// Update the ParseRequest structure to include result type


// Add this new handler
func (h *CountyHandler) HandleParseAndFormat(w http.ResponseWriter, r *http.Request) {
	// Handle preflight OPTIONS request
	if r.Method == http.MethodOptions {
		enableCORS(w, r)  // Pass the request to enableCORS
		w.WriteHeader(http.StatusOK)
		return
	}

	enableCORS(w, r)  // Pass the request to enableCORS for all other requests

	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req ParseRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	// Validate request
	if req.CountyName == "" || req.Link == "" || req.ParseMethod == "" || req.ResultType == "" {
		http.Error(w, "Missing required fields", http.StatusBadRequest)
		return
	}

	if req.ResultType != "measures" && req.ResultType != "candidates" {
		http.Error(w, "ResultType must be either 'measures' or 'candidates'", http.StatusBadRequest)
		return
	}

	// Get the parser
	p, err := h.manager.GetParser(req.ParseMethod)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to get parser: %v", err), http.StatusInternalServerError)
		return
	}

	// Set county name for the parser
	p.SetCountyName(req.CountyName)

	// Parse the URL
	ctx := r.Context()
	if err := p.Parse(ctx, req.Link); err != nil {
		http.Error(w, fmt.Sprintf("Failed to parse data: %v", err), http.StatusInternalServerError)
		return
	}

	// Get results from PocketBase
	collectionName := fmt.Sprintf("county_%s_results", req.CountyName)
	collection, err := h.store.GetPocketBase().Dao().FindCollectionByNameOrId(collectionName)
	if err != nil {
		http.Error(w, "Results not found", http.StatusNotFound)
		return
	}

	// Query results based on type
	query := h.store.GetPocketBase().Dao().RecordQuery(collection)
	if req.ResultType == "measures" {
		query.AndWhere(dbx.HashExp{"type": "measure"})
	} else {
		query.AndWhere(dbx.HashExp{"type": "candidate"})
	}

	var records []*pb.Record
	if err := query.All(&records); err != nil {
		http.Error(w, "Error fetching results", http.StatusInternalServerError)
		return
	}

	// Set content type to HTML
	w.Header().Set("Content-Type", "text/html")

	// Format and return results based on type
	if req.ResultType == "measures" {
		// Group measures
		groupMap := make(map[string]*MeasureGroup)
		for _, record := range records {
			contestName := record.GetString("contest_name")
			if _, exists := groupMap[contestName]; !exists {
				groupMap[contestName] = &MeasureGroup{
					Title: contestName,
				}
			}

			measure := Measure{
				Name:        record.GetString("choice_name"),
				Description: record.GetString("description"),
				YesVotes:    formatVotes(record.GetInt("yes_votes")),
				NoVotes:     formatVotes(record.GetInt("no_votes")),
			}
			groupMap[contestName].Measures = append(groupMap[contestName].Measures, measure)
		}

		var groups []MeasureGroup
		for _, group := range groupMap {
			groups = append(groups, *group)
		}

		tmpl, err := template.ParseFiles("internal/templates/measures.html")
		if err != nil {
			http.Error(w, "Error parsing template", http.StatusInternalServerError)
			return
		}

		if err := tmpl.Execute(w, map[string]interface{}{
			"Groups": groups,
		}); err != nil {
			http.Error(w, "Error executing template", http.StatusInternalServerError)
			return
		}
	} else {
		// Group candidates
		raceMap := make(map[string]*Race)
		for _, record := range records {
			contestName := record.GetString("contest_name")
			if _, exists := raceMap[contestName]; !exists {
				raceMap[contestName] = &Race{
					Title: contestName,
				}
			}

			candidate := Candidate{
				Name:       record.GetString("choice_name"),
				Position:   record.GetString("description"),
				Votes:      formatVotes(record.GetInt("votes")),
				Percentage: formatPercentage(record.GetFloat("percentage")),
			}
			raceMap[contestName].Candidates = append(raceMap[contestName].Candidates, candidate)
		}

		var races []Race
		for _, race := range raceMap {
			races = append(races, *race)
		}

		tmpl, err := template.ParseFiles("internal/templates/candidates.html")
		if err != nil {
			http.Error(w, "Error parsing template", http.StatusInternalServerError)
			return
		}

		if err := tmpl.Execute(w, map[string]interface{}{
			"Races": races,
		}); err != nil {
			http.Error(w, "Error executing template", http.StatusInternalServerError)
			return
		}
	}
}

func formatPercentage(percentage float64) string {
	if percentage == 0 {
		return "0%"
	}
	return fmt.Sprintf("%.1f%%", percentage)
}