# Election Results API

A Go-based API for managing and processing election results data using PocketBase.

## Features
- County election data management
- ZIP file parsing
- Results categorization (candidates/measures)
- RESTful API endpoints

## Requirements
- Go 1.22+
- PocketBase
- Docker (for deployment)

## Development Setup
1. Clone the repository
2. Run `go mod download`
3. Run `./scripts/run.sh`

## API Endpoints
- `POST /api/county-links` - Create county link
- `GET /api/county-links` - List all county links
- `GET /api/county-links/{id}` - Get specific county link
- `PUT /api/county-links/{id}` - Update county link
- `DELETE /api/county-links/{id}` - Delete county link
- `POST /api/county-links/{id}/parse` - Parse county data
- `POST /api/bulk-parse/{method}` - Bulk parse by method
- `GET /api/county-results/{id}` - Get county results
- `POST /api/cleanup` - Clean up collections

## Deployment 