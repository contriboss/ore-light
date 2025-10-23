# Multi-stage build for minimal runtime image
# Builder stage: Compile ore binary with version injection
FROM golang:1.25-alpine AS builder

WORKDIR /build

# Install build dependencies
RUN apk add --no-cache git ca-certificates

# Copy go mod files first for better layer caching
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build static binary with version injection (matching mage build)
# Ruby developers: This is like bundle install --deployment but for Go
# CGO_ENABLED=0 creates a truly static binary (no C dependencies)
RUN VERSION=$(cat VERSION 2>/dev/null || echo "dev") && \
    COMMIT=$(git rev-parse --short HEAD 2>/dev/null || echo "unknown") && \
    TIME=$(date -u +%Y-%m-%dT%H:%M:%SZ) && \
    CGO_ENABLED=0 GOOS=linux go build \
      -ldflags="-s -w -X main.version=${VERSION} -X main.buildCommit=${COMMIT} -X main.buildTime=${TIME}" \
      -o ore ./cmd/ore

# Runtime stage: Minimal distroless image
# Ruby developers: Like Alpine but even smaller (~2MB vs ~5MB)
# No shell, no package manager - just the binary
FROM gcr.io/distroless/static:latest

# Copy ca-certificates for HTTPS (distroless doesn't include them)
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/

# Copy the static binary
COPY --from=builder /build/ore /usr/local/bin/ore

# Set working directory for mounted projects
WORKDIR /workspace

# Default entrypoint
ENTRYPOINT ["ore"]
