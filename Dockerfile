# Multi-stage build for elchi plugins
FROM golang:1.21-alpine AS builder

# Install git for go modules
RUN apk add --no-cache git ca-certificates

# Get plugin name from build arg
ARG PLUGIN_NAME
ARG PROJECT_VERSION

# Set working directory
WORKDIR /workspace

# Copy all workspace files (build context should be from root)
COPY . .

# Check if plugin exists
RUN test -d "${PLUGIN_NAME}" || (echo "Plugin directory ${PLUGIN_NAME} not found" && exit 1)

# Build the specific plugin
WORKDIR /workspace/${PLUGIN_NAME}

# Download dependencies (this will use the workspace and local pkg module)
RUN go mod download

# Build static binary
RUN CGO_ENABLED=0 GOOS=linux GOARCH=${TARGETARCH:-amd64} \
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