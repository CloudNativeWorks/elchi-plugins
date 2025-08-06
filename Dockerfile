# Multi-stage build for elchi plugins
FROM golang:1.21-alpine AS builder

# Install git for go modules
RUN apk add --no-cache git ca-certificates

# Set working directory
WORKDIR /workspace

# Copy go workspace files
COPY go.work go.work
COPY pkg/ pkg/

# Get plugin name from build arg
ARG PLUGIN_NAME
ARG PROJECT_VERSION

# Validate plugin directory exists
COPY ${PLUGIN_NAME}/ ${PLUGIN_NAME}/

# Build the specific plugin
WORKDIR /workspace/${PLUGIN_NAME}

# Download dependencies
RUN go mod download

# Build static binary
RUN CGO_ENABLED=0 GOOS=linux GOARCH=${TARGETARCH} \
    go build -a -installsuffix cgo \
    -ldflags="-w -s -X main.Version=${PROJECT_VERSION}" \
    -o /plugin \
    .

# Final minimal image
FROM scratch

# Copy CA certificates for HTTPS connections
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/

# Copy the plugin binary
COPY --from=builder /plugin /plugin

# Use non-root user ID
USER 65534

# Set entrypoint
ENTRYPOINT ["/plugin"]