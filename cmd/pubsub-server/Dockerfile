# Build stage
FROM golang:1.25-alpine AS builder

# Install build dependencies
RUN apk add --no-cache git gcc musl-dev

# Set working directory
WORKDIR /build

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build the application
WORKDIR /build/cmd/pubsub-server
RUN CGO_ENABLED=1 GOOS=linux go build -a -installsuffix cgo -o pubsub-server .

# Final stage
FROM alpine:latest

# Install runtime dependencies
RUN apk --no-cache add ca-certificates

# Create non-root user
RUN addgroup -g 1000 pubsub && \
    adduser -D -u 1000 -G pubsub pubsub

WORKDIR /app

# Copy binary from builder
COPY --from=builder /build/cmd/pubsub-server/pubsub-server .

# Change ownership
RUN chown -R pubsub:pubsub /app

# Switch to non-root user
USER pubsub

# Expose port
EXPOSE 8080

# Health check
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
  CMD wget --no-verbose --tries=1 --spider http://localhost:8080/api/v1/health || exit 1

# Run the application
CMD ["./pubsub-server"]
