package models

import "fmt"

// ParseMethod represents the method used to parse county data
type ParseMethod string

const (
    ParseMethodZIP  ParseMethod = "zip"
    ParseMethodHTML ParseMethod = "html"
)

// ValidateParseMethod checks if the parse method is valid
func ValidateParseMethod(method ParseMethod) error {
    switch method {
    case ParseMethodZIP, ParseMethodHTML:
        return nil
    default:
        return fmt.Errorf("invalid parse method: %s", method)
    }
}

// CountyLink represents a county's election data source
type CountyLink struct {
    ID          string      `json:"id,omitempty"`
    CountyName  string      `json:"county_name"`
    Link        string      `json:"link"`
    ParseMethod ParseMethod `json:"parse_method"`
}

// Validate ensures all required fields are present and valid
func (c *CountyLink) Validate() error {
    if c.CountyName == "" {
        return fmt.Errorf("county name is required")
    }
    if c.Link == "" {
        return fmt.Errorf("link is required")
    }
    return ValidateParseMethod(c.ParseMethod)
} 