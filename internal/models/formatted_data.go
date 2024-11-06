package models

// ElectionEntry represents raw election data
type ElectionEntry struct {
    ID          string
    CountyID    string
    Title       string
    ChoiceName  string
    Votes       int
    Percentage  float64
    RawData     map[string]interface{}
}

// Candidate represents a formatted candidate entry
type Candidate struct {
    ID          string
    CountyID    string
    Race        string
    Name        string
    Votes       int
    Percentage  float64
    Additional  map[string]interface{}
}

// Measure represents a formatted measure entry
type Measure struct {
    ID          string
    CountyID    string
    Title       string
    Choice      string // Yes/No
    Votes       int
    Percentage  float64
    IsBond      bool
    Additional  map[string]interface{}
} 