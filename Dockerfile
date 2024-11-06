FROM golang:1.22-alpine

WORKDIR /app

# Install build dependencies
RUN apk add --no-cache git

# Copy go mod files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code and templates
COPY . .

# Make sure templates directory exists
RUN mkdir -p /app/internal/templates

# Copy templates specifically
COPY internal/templates/*.html /app/internal/templates/

# Build the application
RUN CGO_ENABLED=0 go build -o /app/bin/server cmd/server/main.go

# Create data directory
RUN mkdir -p /app/pb_data

# Expose ports
EXPOSE 8080
EXPOSE 8090

# Run the server
CMD ["/app/bin/server"] 