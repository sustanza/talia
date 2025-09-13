# Multi-stage build for Talia
# Build stage
FROM golang:1.24-alpine AS builder

# Install build dependencies
RUN apk add --no-cache git make

# Set working directory
WORKDIR /build

# Copy go mod files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build the binary
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o talia ./cmd/talia

# Final stage
FROM alpine:latest

# Install runtime dependencies
RUN apk --no-cache add ca-certificates

# Create non-root user
RUN addgroup -g 1000 -S talia && \
    adduser -u 1000 -S talia -G talia

# Set working directory
WORKDIR /app

# Copy binary from builder
COPY --from=builder /build/talia /app/talia

# Change ownership
RUN chown -R talia:talia /app

# Switch to non-root user
USER talia

# Expose volume for data files
VOLUME ["/data"]

# Set entrypoint
ENTRYPOINT ["/app/talia"]

# Default command (show help)
CMD ["--help"]
