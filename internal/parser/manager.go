package parser

import (
	"context"
	"fmt"
	"github.com/pocketbase/pocketbase"
	"log"
)

// ParserManager manages different types of parsers
type ParserManager struct {
	parsers map[string]Parser
	pb      *pocketbase.PocketBase
}

// NewParserManager creates a new parser manager
func NewParserManager(pb *pocketbase.PocketBase) (*ParserManager, error) {
	m := &ParserManager{
		parsers: make(map[string]Parser),
		pb:      pb,
	}

	// Initialize ZIP parser
	zipParser, err := NewZIPParser(pb)
	if err != nil {
		return nil, fmt.Errorf("failed to create ZIP parser: %w", err)
	}
	m.RegisterParser(zipParser)

	return m, nil
}

// RegisterParser adds a new parser to the manager
func (m *ParserManager) RegisterParser(parser Parser) {
	m.parsers[parser.Method()] = parser
}

// GetParser retrieves a parser by method
func (m *ParserManager) GetParser(method string) (Parser, error) {
	parser, ok := m.parsers[method]
	if !ok {
		return nil, fmt.Errorf("no parser found for method: %s", method)
	}
	return parser, nil
}

// ParseURL parses data from a URL using the appropriate parser
func (m *ParserManager) ParseURL(ctx context.Context, method, url string) error {
	parser, err := m.GetParser(method)
	if err != nil {
		return err
	}

	return parser.Parse(ctx, url)
}

// Cleanup performs any necessary cleanup
func (m *ParserManager) Cleanup() {
	for _, p := range m.parsers {
		if err := p.Cleanup(); err != nil {
			log.Printf("Error cleaning up parser: %v", err)
		}
	}
} 