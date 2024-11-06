package formatter

import (
	"context"
	"era/internal/models"
	"fmt"
	"log"
	"strings"
	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/models/schema"
	pbModels "github.com/pocketbase/pocketbase/models"
)

// ResultsFormatter handles the formatting of election results
type ResultsFormatter struct {
	pb *pocketbase.PocketBase
}

// New creates a new ResultsFormatter
func New(pb *pocketbase.PocketBase) *ResultsFormatter {
	return &ResultsFormatter{pb: pb}
}

// Add helper function at the top of the file
func intPtr(i int) *int {
	return &i
}

// ensureCollection creates a single collection for all results
func (f *ResultsFormatter) ensureCollection(countyName string) error {
	log.Printf("=== Ensuring collection for county: %s ===", countyName)

	collectionName := fmt.Sprintf("county_%s_results", countyName)
	
	// Check if collection exists
	collection, err := f.pb.Dao().FindCollectionByNameOrId(collectionName)
	if err == nil {
		log.Printf("Collection %s already exists", collectionName)
		return nil
	}

	// Create new collection with all necessary fields
	collection = &pbModels.Collection{
		Name: collectionName,
		Type: pbModels.CollectionTypeBase,
		Schema: schema.NewSchema(
			&schema.SchemaField{
				Name:     "county_link",
				Type:     schema.FieldTypeRelation,
				Required: true,
				Options: &schema.RelationOptions{
					CollectionId: "county_links",
					MaxSelect:    intPtr(1),
					MinSelect:    intPtr(1),
				},
			},
			&schema.SchemaField{
				Name:     "type",
				Type:     schema.FieldTypeSelect,
				Required: true,
				Options: &schema.SelectOptions{
					Values: []string{"candidate", "measure"},
				},
			},
			&schema.SchemaField{
				Name:     "contest_name",
				Type:     schema.FieldTypeText,
				Required: true,
			},
			&schema.SchemaField{
				Name:     "choice_name",
				Type:     schema.FieldTypeText,
				Required: true,
			},
			&schema.SchemaField{
				Name:     "votes",
				Type:     schema.FieldTypeNumber,
				Required: true,
			},
			&schema.SchemaField{
				Name:     "percentage",
				Type:     schema.FieldTypeNumber,
				Required: true,
			},
			&schema.SchemaField{
				Name:     "is_bond",
				Type:     schema.FieldTypeBool,
				Required: false,
			},
		),
	}

	if err := f.pb.Dao().SaveCollection(collection); err != nil {
		return fmt.Errorf("failed to create collection: %w", err)
	}

	log.Printf("Successfully created collection: %s", collectionName)
	return nil
}

// ProcessEntry processes and stores a single entry
func (f *ResultsFormatter) ProcessEntry(ctx context.Context, entry *models.ElectionEntry) error {
	log.Printf("Processing entry for county: %s", entry.CountyID)
	log.Printf("Entry details - Title: %s, Choice: %s", entry.Title, entry.ChoiceName)

	if err := f.ensureCollection(entry.CountyID); err != nil {
		log.Printf("Error ensuring collection: %v", err)
		return fmt.Errorf("failed to ensure collection: %w", err)
	}

	// Determine entry type
	entryType := "candidate"
	isBond := false
	choiceLower := strings.ToLower(entry.ChoiceName)
	titleLower := strings.ToLower(entry.Title)
	if choiceLower == "yes" || choiceLower == "no" || strings.Contains(choiceLower, "bond") || strings.Contains(choiceLower, "bonds") {
		entryType = "measure"
		isBond = strings.Contains(titleLower, "bond") || strings.Contains(titleLower, "bonds")
	}

	// Create record
	collection, err := f.pb.Dao().FindCollectionByNameOrId(
		fmt.Sprintf("county_%s_results", entry.CountyID))
	if err != nil {
		return fmt.Errorf("failed to find collection: %w", err)
	}

	record := pbModels.NewRecord(collection)
	record.Set("county_link", entry.CountyID)
	record.Set("type", entryType)
	record.Set("contest_name", entry.Title)
	record.Set("choice_name", entry.ChoiceName)
	record.Set("votes", entry.Votes)
	record.Set("percentage", entry.Percentage)
	if entryType == "measure" {
		record.Set("is_bond", isBond)
	}

	if err := f.pb.Dao().SaveRecord(record); err != nil {
		return fmt.Errorf("failed to save record: %w", err)
	}

	log.Printf("Successfully saved %s record for %s", entryType, entry.CountyID)
	return nil
}

// Remove unused methods since we're using a single collection
func (f *ResultsFormatter) CategorizeEntry(entry *models.ElectionEntry) (string, error) {
	choiceLower := strings.ToLower(entry.ChoiceName)
	if choiceLower == "yes" || choiceLower == "no" {
		return "measure", nil
	}
	return "candidate", nil
}