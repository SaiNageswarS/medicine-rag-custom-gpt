# Build stage
FROM golang:1.24-alpine AS builder

WORKDIR /app

# Install git for version info
RUN apk add --no-cache git ca-certificates

# Copy dependency files first for better caching
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build with production flags
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags="-s -w -X main.Version=$(git describe --tags --always 2>/dev/null || echo 'dev')" \
    -trimpath \
    -o /app/medicine-rag .

# Final stage - minimal image
FROM scratch

# Copy CA certificates for HTTPS
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/

# Copy binary
COPY --from=builder /app/medicine-rag /medicine-rag

# Expose gRPC and HTTP ports
EXPOSE 50051 8081

ENTRYPOINT ["/medicine-rag"]
