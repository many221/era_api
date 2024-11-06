package storage

import (
    "era/internal/models"
    "fmt"
    "github.com/pocketbase/pocketbase"
    "github.com/pocketbase/pocketbase/models/schema"
    pbModels "github.com/pocketbase/pocketbase/models"
    "log"
    
    "time"
)

type PocketBaseStore struct {
    app *pocketbase.PocketBase
}

func NewPocketBaseStore(dataDir string) (*PocketBaseStore, error) {
    // Create a new PocketBase instance with a data directory
    app := pocketbase.New()
    
    // Configure the root command - change from 127.0.0.1 to 0.0.0.0 to allow external access
    app.RootCmd.SetArgs([]string{"serve", "--dir", dataDir, "--http", "0.0.0.0:8090"})
    
    // Start PocketBase in a goroutine
    go func() {
        if err := app.Start(); err != nil {
            log.Printf("Failed to start PocketBase: %v", err)
        }
    }()

    // Give PocketBase a moment to initialize
    time.Sleep(time.Second)
    
    // Initialize the app
    if err := app.Bootstrap(); err != nil {
        return nil, fmt.Errorf("failed to bootstrap PocketBase: %w", err)
    }
    
    // Ensure collection exists
    if err := ensureCollection(app); err != nil {
        return nil, fmt.Errorf("failed to ensure collection exists: %w", err)
    }
    
    return &PocketBaseStore{app: app}, nil
}

func ensureCollection(app *pocketbase.PocketBase) error {
    collection, err := app.Dao().FindCollectionByNameOrId("county_links")
    if err != nil {
        // Create collection if it doesn't exist
        collection = &pbModels.Collection{
            Name:       "county_links",
            Type:       pbModels.CollectionTypeBase,
            CreateRule: nil,
            Schema: schema.NewSchema(
                &schema.SchemaField{
                    Name:     "county_name",
                    Type:     schema.FieldTypeText,
                    Required: true,
                },
                &schema.SchemaField{
                    Name:     "link",
                    Type:     schema.FieldTypeUrl,
                    Required: true,
                },
                &schema.SchemaField{
                    Name:     "parse_method",
                    Type:     schema.FieldTypeSelect,
                    Required: true,
                    Options: &schema.SelectOptions{
                        Values: []string{"zip", "html"},
                    },
                },
            ),
        }
        
        if err := app.Dao().SaveCollection(collection); err != nil {
            return fmt.Errorf("failed to save collection: %w", err)
        }
    }
    return nil
}

func (s *PocketBaseStore) SaveCountyLink(countyLink *models.CountyLink) error {
    collection, err := s.app.Dao().FindCollectionByNameOrId("county_links")
    if err != nil {
        return fmt.Errorf("failed to find collection: %w", err)
    }
    
    record := pbModels.NewRecord(collection)
    record.Set("county_name", countyLink.CountyName)
    record.Set("link", countyLink.Link)
    record.Set("parse_method", string(countyLink.ParseMethod))
    
    if err := s.app.Dao().SaveRecord(record); err != nil {
        return fmt.Errorf("failed to save record: %w", err)
    }
    
    return nil
}

func (s *PocketBaseStore) GetCountyLink(id string) (*models.CountyLink, error) {
    record, err := s.app.Dao().FindRecordById("county_links", id)
    if err != nil {
        return nil, fmt.Errorf("failed to find county link: %w", err)
    }
    
    return &models.CountyLink{
        ID:          record.Id,
        CountyName:  record.GetString("county_name"),
        Link:        record.GetString("link"),
        ParseMethod: models.ParseMethod(record.GetString("parse_method")),
    }, nil
}

func (s *PocketBaseStore) GetAllCountyLinks() ([]models.CountyLink, error) {
    records, err := s.app.Dao().FindRecordsByExpr("county_links")
    if err != nil {
        return nil, fmt.Errorf("failed to fetch county links: %w", err)
    }
    
    links := make([]models.CountyLink, len(records))
    for i, record := range records {
        links[i] = models.CountyLink{
            ID:          record.Id,
            CountyName:  record.GetString("county_name"),
            Link:        record.GetString("link"),
            ParseMethod: models.ParseMethod(record.GetString("parse_method")),
        }
    }
    return links, nil
}

func (s *PocketBaseStore) UpdateCountyLink(id string, countyLink *models.CountyLink) error {
    record, err := s.app.Dao().FindRecordById("county_links", id)
    if err != nil {
        return fmt.Errorf("failed to find county link: %w", err)
    }
    
    record.Set("county_name", countyLink.CountyName)
    record.Set("link", countyLink.Link)
    record.Set("parse_method", string(countyLink.ParseMethod))
    
    if err := s.app.Dao().SaveRecord(record); err != nil {
        return fmt.Errorf("failed to update record: %w", err)
    }
    
    return nil
}

func (s *PocketBaseStore) DeleteCountyLink(id string) error {
    record, err := s.app.Dao().FindRecordById("county_links", id)
    if err != nil {
        return fmt.Errorf("failed to find county link: %w", err)
    }
    
    if err := s.app.Dao().DeleteRecord(record); err != nil {
        return fmt.Errorf("failed to delete record: %w", err)
    }
    
    return nil
}

func (s *PocketBaseStore) GetPocketBase() *pocketbase.PocketBase {
    return s.app
}