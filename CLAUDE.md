# Claude Instructions

This repository contains elchi plugins for cloud-native workflows built with Go.

## Development Commands

When working on this codebase, please use the following commands:

### Build and Test
- `go build ./...` - Build all modules
- `go test ./...` - Run tests for all modules
- `go mod tidy` - Clean up module dependencies
- `go work sync` - Sync workspace dependencies

### Development
- `go run ./elchi-endpoint-discovery` - Run the endpoint discovery plugin
- `go mod download` - Download dependencies

## Code Standards

- Follow existing Go conventions and patterns
- Use the shared pkg module for common functionality (logger, config)
- Ensure all tests pass before committing
- Run `go mod tidy` in each module before submitting changes
- Use Go 1.21+ features when appropriate

## Project Structure

This is a plugin-based architecture using Go workspaces:

```
elchi-plugins/
├── go.work                ← Go workspace configuration
├── pkg/                   ← Common packages (logger, config)
│   ├── go.mod
│   ├── logger/logger.go
│   └── config/config.go
└── elchi-endpoint-discovery/  ← First plugin for K8s node discovery
    ├── go.mod
    └── main.go
```

Each plugin is a separate Go module that can import the shared pkg module. New plugins should follow this pattern and use the common logging and configuration utilities.