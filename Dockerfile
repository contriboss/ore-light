# Build stage
FROM golang:1.25.1-alpine AS builder

WORKDIR /build

# Install build dependencies
RUN apk add --no-cache git

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source
COPY . .

# Build ore
RUN go build -ldflags="-s -w" -o /ore ./cmd/ore

# Runtime stage
FROM alpine:latest

# Install runtime dependencies (Ruby for native extensions, git for git sources)
RUN apk add --no-cache \
    ruby \
    ruby-dev \
    build-base \
    git \
    ca-certificates

# Copy ore binary
COPY --from=builder /ore /usr/local/bin/ore

# Set working directory
WORKDIR /workspace

# Default command
ENTRYPOINT ["ore"]
CMD ["--help"]
