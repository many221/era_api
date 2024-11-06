// internal/parser/parser.go
package parser

import (
    "context"
    "fmt"
)

// Parser defines the interface for different parsing strategies
type Parser interface {
    // Method returns the parser type (e.g., "zip", "html")
    Method() string
    
    // Parse processes data from the given URL
    Parse(ctx context.Context, url string) error
    
    // Cleanup performs any necessary cleanup
    Cleanup() error
    
    // SetCountyName sets the county name for the parser
    SetCountyName(name string)
}

// ParseError represents a parsing error with a specific stage
type ParseError struct {
    Stage string
    Err   error
}

func (e *ParseError) Error() string {
    return fmt.Sprintf("parse error at %s stage: %v", e.Stage, e.Err)
}

// NewParseError creates a new ParseError
func NewParseError(stage string, err error) *ParseError {
    return &ParseError{
        Stage: stage,
        Err:   err,
    }
}