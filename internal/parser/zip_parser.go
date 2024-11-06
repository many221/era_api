package parser

import (
    "archive/zip"
    "context"
    "encoding/csv"
    "fmt"
    "io"
    "log"
    "net/http"
    "os"
    "path/filepath"
    "strconv"
    "strings"
    "time"
    
    "github.com/pocketbase/pocketbase"
    "github.com/pocketbase/pocketbase/models/schema"
    pbModels "github.com/pocketbase/pocketbase/models"
    "era/internal/formatter"
    "era/internal/models"
)

// ZIPParser implements Parser interface for ZIP files containing CSV data
type ZIPParser struct {
    tempDir    string
    pb         *pocketbase.PocketBase
    countyName string
}

// NewZIPParser creates a new ZIP parser instance
func NewZIPParser(pb *pocketbase.PocketBase) (*ZIPParser, error) {
    tempDir, err := os.MkdirTemp("", "election_data_*")
    if err != nil {
        return nil, fmt.Errorf("failed to create temp directory: %w", err)
    }
    
    return &ZIPParser{
        tempDir: tempDir,
        pb:      pb,
    }, nil
}

// Method returns the parser type
func (p *ZIPParser) Method() string {
	return "zip"
}

// Parse implements the Parser interface
func (p *ZIPParser) Parse(ctx context.Context, url string) error {
	log.Printf("Starting to parse URL: %s", url)
	
	// Download ZIP file
	log.Printf("Downloading ZIP file...")
	zipPath, err := p.downloadZIP(ctx, url)
	if err != nil {
		log.Printf("Error downloading ZIP: %v", err)
		return NewParseError("download", err)
	}
	defer os.Remove(zipPath)
	log.Printf("Successfully downloaded ZIP to: %s", zipPath)
	
	// Extract and process CSV files
	log.Printf("Processing ZIP file...")
	if err := p.processZIPFile(ctx, zipPath); err != nil {
		log.Printf("Error processing ZIP: %v", err)
		return NewParseError("process", err)
	}
	
	log.Printf("Successfully completed parsing")
	return nil
}

// downloadZIP downloads a ZIP file from the given URL
func (p *ZIPParser) downloadZIP(ctx context.Context, url string) (string, error) {
	log.Printf("Creating HTTP request for URL: %s", url)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}
 
	// Add headers
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36")
	req.Header.Set("Accept", "*/*")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")
	req.Header.Set("Connection", "keep-alive")
	log.Printf("Added browser-like headers to request")
	
	client := &http.Client{
		Timeout: 30 * time.Second,
	}
	
	log.Printf("Sending HTTP request...")
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to download file: %w", err)
	}
	defer resp.Body.Close()
	
	log.Printf("Received response with status code: %d", resp.StatusCode)
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}
	
	// Create temporary file
	zipPath := filepath.Join(p.tempDir, "download.zip")
	log.Printf("Creating temporary file at: %s", zipPath)
	f, err := os.Create(zipPath)
	if err != nil {
		return "", fmt.Errorf("failed to create temp file: %w", err)
	}
	defer f.Close()
	
	// Copy data
	log.Printf("Copying response data to file...")
	written, err := io.Copy(f, resp.Body)
	if err != nil {
		os.Remove(zipPath)
		return "", fmt.Errorf("failed to save file: %w", err)
	}
	log.Printf("Successfully wrote %d bytes to file", written)
	
	return zipPath, nil
}

// processZIPFile extracts and processes CSV files from the ZIP
func (p *ZIPParser) processZIPFile(ctx context.Context, zipPath string) error {
	log.Printf("Opening ZIP file: %s", zipPath)
	r, err := zip.OpenReader(zipPath)
	if err != nil {
		return fmt.Errorf("failed to open ZIP: %w", err)
	}
	defer r.Close()
	
	log.Printf("Found %d files in ZIP archive", len(r.File))
	
	// Process each file
	for _, f := range r.File {
		select {
		case <-ctx.Done():
			log.Printf("Context cancelled, stopping processing")
			return ctx.Err()
		default:
			log.Printf("Processing file: %s", f.Name)
			if err := p.processZIPEntry(ctx, f); err != nil {
				log.Printf("Error processing file %s: %v", f.Name, err)
				return fmt.Errorf("failed to process %s: %w", f.Name, err)
			}
		}
	}
	
	log.Printf("Finished processing all files in ZIP")
	return nil
}

// processZIPEntry handles a single file from the ZIP archive
func (p *ZIPParser) processZIPEntry(ctx context.Context, f *zip.File) error {
	// Skip if not CSV
	if !strings.HasSuffix(strings.ToLower(f.Name), ".csv") {
		log.Printf("Skipping non-CSV file: %s", f.Name)
		return nil
	}
	
	log.Printf("Processing CSV file: %s for county: %s", f.Name, p.countyName)
	
	// Create formatter
	resultsFormatter := formatter.New(p.pb)
	
	// Open the file in ZIP
	rc, err := f.Open()
	if err != nil {
		return fmt.Errorf("failed to open file in ZIP: %w", err)
	}
	defer rc.Close()
	
	// Create CSV reader
	reader := csv.NewReader(rc)
	
	// Read headers
	log.Printf("Reading CSV headers...")
	headers, err := reader.Read()
	if err != nil {
		return fmt.Errorf("failed to read CSV headers: %w", err)
	}
	log.Printf("Found %d columns: %v", len(headers), headers)
	
	// Create a map for easier column access
	rowData := make(map[string]interface{})
	headerMap := make(map[string]int)
	
	// Create header map for easier access
	for i, header := range headers {
		headerMap[strings.ToLower(header)] = i
	}
 
	// Process rows
	rowCount := 0
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			row, err := reader.Read()
			if err == io.EOF {
				log.Printf("Finished reading CSV, processed %d rows", rowCount)
				return nil
			}
			if err != nil {
				return fmt.Errorf("failed to read CSV row: %w", err)
			}

			// Safely get values with fallbacks
			var contestName, choiceName, totalVotes, percent string
			
			if idx, ok := headerMap["contest name"]; ok && idx < len(row) {
				contestName = row[idx]
			}
			if idx, ok := headerMap["choice name"]; ok && idx < len(row) {
				choiceName = row[idx]
			}
			if idx, ok := headerMap["total votes"]; ok && idx < len(row) {
				totalVotes = row[idx]
			}
			if idx, ok := headerMap["percent of votes"]; ok && idx < len(row) {
				percent = row[idx]
			}

			// Store all row data for raw access
			for i, header := range headers {
				if i < len(row) {
					rowData[strings.ToLower(header)] = row[i]
				}
			}

			// Create election entry with safe values
			entry := &models.ElectionEntry{
				CountyID:    p.countyName,
				Title:       contestName,
				ChoiceName:  choiceName,
				Votes:       parseVotes(totalVotes),
				Percentage:  parsePercentage(percent),
				RawData:     rowData,
			}

			log.Printf("Processing entry - Title: %s, Choice: %s, Votes: %d, Percentage: %.2f",
				entry.Title, entry.ChoiceName, entry.Votes, entry.Percentage)

			// Process entry through formatter
			if err := resultsFormatter.ProcessEntry(ctx, entry); err != nil {
				log.Printf("Warning: failed to process row %d: %v", rowCount, err)
				continue
			}

			rowCount++
			if rowCount%1000 == 0 {
				log.Printf("Processed %d rows...", rowCount)
			}
		}
	}
}

// Helper functions for parsing data
func parseVotes(s string) int {
	votes, err := strconv.Atoi(strings.TrimSpace(s))
	if err != nil {
		return 0
	}
	return votes
}

func parsePercentage(s string) float64 {
	// Remove % sign and convert to float
	s = strings.TrimSpace(strings.TrimSuffix(s, "%"))
	percentage, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0
	}
	return percentage
}

func makeRawData(headers, row []string) map[string]interface{} {
	rawData := make(map[string]interface{})
	for i, header := range headers {
		if i < len(row) {
			rawData[header] = row[i]
		}
	}
	return rawData
}

// createCollectionFromHeaders creates a PocketBase collection based on CSV headers
func (p *ZIPParser) createCollectionFromHeaders(ctx context.Context, filename string, headers []string) error {
	if p.countyName == "" {
		return fmt.Errorf("county name not set")
	}
	
	collectionName := fmt.Sprintf("county_%s_results", p.countyName)
	log.Printf("Creating collection: %s", collectionName)
	
	// Check if collection already exists
	collection, err := p.pb.Dao().FindCollectionByNameOrId(collectionName)
	if err == nil {
		log.Printf("Collection %s already exists", collectionName)
		return nil
	}
	
	// Create schema fields from headers
	schemaFields := make([]*schema.SchemaField, len(headers))
	for i, header := range headers {
		fieldName := strings.ToLower(strings.ReplaceAll(header, " ", "_"))
		schemaFields[i] = &schema.SchemaField{
			Name:     fieldName,
			Type:     schema.FieldTypeText,
			Required: false,
		}
	}
	
	// Create new collection
	collection = &pbModels.Collection{
		Name:       collectionName,
		Type:       pbModels.CollectionTypeBase,
		CreateRule: nil,
		Schema:     schema.NewSchema(schemaFields...),
	}
	
	if err := p.pb.Dao().SaveCollection(collection); err != nil {
		return fmt.Errorf("failed to create collection: %w", err)
	}
	
	log.Printf("Successfully created collection %s with %d fields", collectionName, len(headers))
	return nil
}

// storeRow stores a single row of data in PocketBase
func (p *ZIPParser) storeRow(ctx context.Context, filename string, headers, row []string) error {
	if p.countyName == "" {
		return fmt.Errorf("county name not set")
	}
	
	collectionName := fmt.Sprintf("county_%s_results", p.countyName)
	
	collection, err := p.pb.Dao().FindCollectionByNameOrId(collectionName)
	if err != nil {
		return fmt.Errorf("failed to find collection: %w", err)
	}
	
	// Create new record
	record := pbModels.NewRecord(collection)
	
	// Set field values
	for i, header := range headers {
		if i < len(row) {
			fieldName := strings.ToLower(strings.ReplaceAll(header, " ", "_"))
			record.Set(fieldName, row[i])
		}
	}
	
	// Save record
	if err := p.pb.Dao().SaveRecord(record); err != nil {
		return fmt.Errorf("failed to save record: %w", err)
	}
	
	return nil
}

// Cleanup removes temporary files
func (p *ZIPParser) Cleanup() error {
	return os.RemoveAll(p.tempDir)
}

// SetCountyName sets the county name for the ZIPParser
func (p *ZIPParser) SetCountyName(name string) {
	p.countyName = strings.ToLower(strings.ReplaceAll(name, " ", "_"))
} 