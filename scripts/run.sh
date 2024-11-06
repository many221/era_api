#!/bin/bash

# Disable CGO
export CGO_ENABLED=0

# Create necessary directories
mkdir -p pb_data
mkdir -p bin

# Run the application directly
echo "Starting server..."
go run cmd/server/main.go 