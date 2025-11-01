# Build stage
# Note: Update the Go version to match go.mod if needed
# For Go 1.23+, use: golang:1.23-alpine
FROM golang:1.23-alpine AS builder

# Install build dependencies
RUN apk add --no-cache git ca-certificates

# Set working directory
WORKDIR /build

# Copy go mod files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build the application
# CGO_ENABLED=0 creates a statically linked binary
# -ldflags="-s -w" reduces binary size by stripping debug info
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo \
    -ldflags="-s -w" -o server ./cmd/server

# Runtime stage - using distroless for security and small size
FROM gcr.io/distroless/static-debian12:nonroot

# Copy the binary from builder
COPY --from=builder /build/server /server

# Cloud Run sets PORT environment variable automatically
# The application already reads PORT from environment
ENV PORT=8080

# Use non-root user (distroless images come with nonroot user)
USER nonroot:nonroot

# Expose port
EXPOSE 8080

# Run the server
ENTRYPOINT ["/server"]