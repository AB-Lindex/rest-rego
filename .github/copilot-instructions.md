# rest-rego AI Assistant Guide

## Project Overview

rest-rego is a Go-based authorization sidecar that protects REST APIs using Open Policy Agent (OPA) Rego policies. It acts as a reverse proxy with policy-based access control, supporting both JWT and Azure Graph authentication.

## Architecture

### Core Components
- **Reverse Proxy**: Built with `httputil.ReverseProxy`, routes requests through policy validation
- **Policy Engine**: OPA integration via `github.com/open-policy-agent/opa/v1/rego` 
- **Authentication**: Pluggable auth providers (`types.AuthProvider` interface)
- **File Watching**: Hot-reload of Rego policies using `fsnotify`

### Key Flow
1. Request → `router.Proxy` middleware chain → Auth → Policy → Backend
2. Middleware order: `CleanupHandler` → `WrapHandler` → `metrics.Wrap` → `authHandler` → `policyHandler`
3. Policy result determines access (`allow` boolean) and optional URL rewriting

### Data Structures
- `types.Info`: Request context passed through middleware chain
- `RequestInfo`: Standardized request data for Rego policies (`method`, `path[]`, `headers`, `auth`)
- Policy input format: `{"request": RequestInfo, "jwt": JWTClaims, "user": AzureUser}`

## Development Patterns

### Configuration
- Uses `github.com/alexflint/go-arg` for CLI/env config in `internal/config/config.go`
- Validates mutually exclusive auth modes (Azure vs JWT)
- Canonical header names via `http.CanonicalHeaderKey()`

### Authentication Providers
Implement `types.AuthProvider` interface:
```go
type AuthProvider interface {
    Authenticate(*Info, *http.Request) error
}
```
- **JWT**: `internal/jwtsupport` - OIDC well-known endpoints, JWK caching
- **Azure**: `internal/azure` - Graph API integration with app caching

### Policy Integration
- Policies must be in `policies/` directory matching `*.rego` pattern
- Default entry point: `request.rego` (configurable via `REQUEST_REGO`)
- Policy package extraction from `package` directive in Rego files
- Hot-reload via `filecache` package with `fsnotify`

### Error Handling
- Use structured logging with `log/slog`
- Exit with `os.Exit(1)` for configuration errors
- HTTP errors: 403 for denied access, 500 for internal errors

## Key Workflows

### Building & Testing
```bash
# Use Task runner (Taskfile.yml)
task choose    # Interactive task selection
task test      # Run all tests
go test ./...  # Standard Go testing

# Docker build
docker build -t rest-rego .
```

### Policy Development
- Place `.rego` files in `policies/` directory
- Must define `package policies` or appropriate namespace
- Required rule: `allow` (boolean) - determines access
- Optional rule: `url` (string) - rewrites target URL
- Input structure: `{"request": {...}, "jwt": {...}, "user": {...}}`

### Testing
- Manual: `.http` files in `tests/` for VS Code REST Client
- Performance: `.k6` files for load testing
- Environment file: `.env` with `TENANT`, `CLIENT_ID`, `CLIENT_SECRET`

## Integration Points

### Deployment Modes
- **Sidecar**: Deployed alongside target service (common in Kubernetes)
- **Gateway**: Central authorization point for multiple services
- Default ports: 8181 (proxy), 8182 (management/metrics)

### External Dependencies
- **OPA**: Policy evaluation engine
- **Azure Graph API**: For Azure authentication mode
- **OIDC Providers**: For JWT well-known endpoints
- **Prometheus**: Metrics via `/metrics` endpoint

### Environment Variables
Critical configs: `AZURE_TENANT`, `WELLKNOWN_OIDC`, `JWT_AUDIENCES`, `BACKEND_PORT`, `POLICY_DIR`

## Project-Specific Conventions

- Interface-driven design: `AuthProvider`, `Validator` interfaces
- Context propagation: `types.Info` attached to request context
- Middleware chaining: Each handler calls `next.ServeHTTP()` 
- Graceful shutdown: Server shutdown with context timeout
- File watching: Automatic policy reload without restart
- Dual auth modes: Never allow both Azure and JWT simultaneously
