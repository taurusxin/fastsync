# Build stage
FROM golang:1.25-alpine AS builder

WORKDIR /app

# Install build dependencies
RUN apk add --no-cache git

# Copy go mod and sum files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -o fastsync ./cmd/fastsync

# Final stage
FROM alpine:latest

WORKDIR /app

# Install runtime dependencies (if any)
RUN apk add --no-cache ca-certificates tzdata

# Copy binary from builder
COPY --from=builder /app/fastsync .
COPY --from=builder /app/config.toml.example ./config.toml.example

# Create directories for data and logs
RUN mkdir -p /data /logs

# Expose default port
EXPOSE 7963

# Define volumes
VOLUME ["/data", "/config", "/logs"]

# Set entrypoint
ENTRYPOINT ["./fastsync"]
CMD ["-c", "/config/config.toml"]
