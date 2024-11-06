package handlers

import (
	"encoding/json"
	"era/internal/models"
	"era/internal/parser"
	"era/internal/storage"
	"fmt"
	"log"
	"net/http"
	"strings"
	"html/template"
	pb "github.com/pocketbase/pocketbase/models"
	
	"github.com/pocketbase/dbx"
)

type CountyHandler struct {
	store   *storage.PocketBaseStore
	manager *parser.ParserManager
}

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
}

const perPage = 100 // Number of records per page

func NewCountyHandler(store *storage.PocketBaseStore, manager *parser.ParserManager) *CountyHandler {
	return &CountyHandler{
		store:   store,
		manager: manager,
	}
}

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

	// Get results from PocketBase
	collectionName := fmt.Sprintf("county_%s_results", countyID)
	collection, err := h.store.GetPocketBase().Dao().FindCollectionByNameOrId(collectionName)
	if err != nil {
		http.Error(w, "County results not found", http.StatusNotFound)
		return
	}

	// Query only measure results
	query := h.store.GetPocketBase().Dao().RecordQuery(collection).
		AndWhere(dbx.HashExp{"type": "measure"})

	// Initialize slice to hold all records
	var allRecords []*pb.Record

	// Get all records with pagination
	page := 1
	for {
		pageRecords := []*pb.Record{}
		err := query.Offset(int64((page - 1) * perPage)).
			Limit(int64(perPage)).
			All(&pageRecords)

		if err != nil {
			log.Printf("Error fetching page %d: %v", page, err)
			http.Error(w, "Error fetching results", http.StatusInternalServerError)
			return
		}

		// If no records returned, we've reached the end
		if len(pageRecords) == 0 {
			break
		}

		allRecords = append(allRecords, pageRecords...)
		page++
	}

	// Group measures by contest (using all records)
	groupMap := make(map[string]*MeasureGroup)
	for _, record := range allRecords {
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

// Helper function to format vote counts
func formatVotes(votes int) string {
	if votes == 0 {
		return "NA"
	}
	return fmt.Sprintf("%d", votes)
}

func (h *CountyHandler) HandleGetCandidatesHTML(w http.ResponseWriter, r *http.Request) {
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

	// Get results from PocketBase
	collectionName := fmt.Sprintf("county_%s_results", countyID)
	collection, err := h.store.GetPocketBase().Dao().FindCollectionByNameOrId(collectionName)
	if err != nil {
		http.Error(w, "County results not found", http.StatusNotFound)
		return
	}

	// Query only candidate results
	query := h.store.GetPocketBase().Dao().RecordQuery(collection).
		AndWhere(dbx.HashExp{"type": "candidate"})

	// Initialize slice to hold all records
	var allRecords []*pb.Record

	// Get all records with pagination
	page := 1
	for {
		pageRecords := []*pb.Record{}
		err := query.Offset(int64((page - 1) * perPage)).
			Limit(int64(perPage)).
			All(&pageRecords)

		if err != nil {
			log.Printf("Error fetching page %d: %v", page, err)
			http.Error(w, "Error fetching results", http.StatusInternalServerError)
			return
		}

		// If no records returned, we've reached the end
		if len(pageRecords) == 0 {
			break
		}

		allRecords = append(allRecords, pageRecords...)
		page++
	}

	// Group candidates by race (using all records)
	raceMap := make(map[string]*Race)
	for _, record := range allRecords {
		contestName := record.GetString("contest_name")
		if _, exists := raceMap[contestName]; !exists {
			raceMap[contestName] = &Race{
				Title: contestName,
			}
		}

		candidate := Candidate{
			Name:     record.GetString("choice_name"),
			Position: record.GetString("description"),
			Votes:    formatVotes(record.GetInt("votes")),
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