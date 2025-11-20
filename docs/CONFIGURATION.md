# Configuration Reference

Complete reference for all rest-rego configuration options.

## Table of Contents

- [Overview](#overview)
- [Core Configuration](#core-configuration)
- [Network Configuration](#network-configuration)
- [Authentication Configuration](#authentication-configuration)
- [Timeout Configuration](#timeout-configuration)
- [Configuration Examples](#configuration-examples)
- [Configuration Validation](#configuration-validation)

## Overview

rest-rego supports configuration via:

1. **Environment variables** (recommended for containers)
2. **Command-line arguments** (useful for local development)
3. **Configuration files** (not currently supported, use env vars)

**Precedence**: Command-line arguments override environment variables.

## Core Configuration

| Option | Env Variable | Default | Description |
|--------|--------------|---------|-------------|
| `-v, --verbose` | - | `false` | Enable verbose logging (debug level) |
| `--debug` | `DEBUG` | `false` | Print policy input and result for all requests |
| `-d, --directory` | `POLICY_DIR` | `./policies` | Directory containing policy files |
| `--pattern` | `FILE_PATTERN` | `*.rego` | File pattern to match for policies |
| `-r, --requestrego` | `REQUEST_REGO` | `request.rego` | Main policy file for requests |
| `--expose-blocked-headers` | `EXPOSE_BLOCKED_HEADERS` | `false` | Expose blocked `X-Restrego-*` headers to policies |

### Examples

```bash
# Enable debug mode
rest-rego --debug

# Custom policy directory
rest-rego -d /etc/rest-rego/policies

# Different file pattern (e.g., only .policy files)
rest-rego --pattern "*.policy"

# Verbose logging
rest-rego --verbose
```

## Network Configuration

| Option                 | Env Variable     | Default     | Description                            |
|------------------------|------------------|-------------|----------------------------------------|
| `-l, --listen`         | `LISTEN_ADDR`    | `:8181`     | Address/port for API proxy             |
| `-m, --management`     | `MGMT_ADDR`      | `:8182`     | Address/port for health/metrics        |
| `-s, --backend-scheme` | `BACKEND_SCHEME` | `http`      | Backend URL scheme (`http` or `https`) |
| `-h, --backend-host`   | `BACKEND_HOST`   | `localhost` | Backend hostname or IP                 |
| `-p, --backend-port`   | `BACKEND_PORT`   | `8080`      | Backend port number                    |

### Port Configuration

rest-rego uses three ports:

| Port     | Purpose                      | Default          | Exposed To                |
|----------|------------------------------|------------------|---------------------------|
| **8181** | API proxy (main traffic)     | `:8181`          | External clients          |
| **8182** | Management (health, metrics) | `:8182`          | Monitoring systems, K8s   |
| **8080** | Backend service              | `localhost:8080` | Internal only (via proxy) |

### Examples

```bash
# Bind to specific IP
rest-rego --listen 0.0.0.0:8181

# Custom backend
rest-rego \
  --backend-scheme https \
  --backend-host api.example.com \
  --backend-port 443

# Different management port
rest-rego --management :9090

# Environment variables
export LISTEN_ADDR=":8181"
export MGMT_ADDR=":8182"
export BACKEND_SCHEME="http"
export BACKEND_HOST="localhost"
export BACKEND_PORT="8080"
rest-rego
```

## Authentication Configuration

rest-rego supports two mutually exclusive authentication modes:

1. **JWT Authentication** (recommended)
2. **Azure Graph Authentication**

### JWT Authentication

| Option | Env Variable | Default | Description |
|--------|--------------|---------|-------------|
| `-w, --well-known` | `WELLKNOWN_OIDC` | - | OIDC well-known URL(s) for JWT verification |
| `-u, --audience` | `JWT_AUDIENCES` | - | Expected JWT audience value(s) **(required)** |
| `--audience-key` | `JWT_AUDIENCE_KEY` | `aud` | JWT claim key for audience validation |
| `-a, --auth-header` | `AUTH_HEADER` | `Authorization` | HTTP header for authentication token |
| `-k, --auth-kind` | `AUTH_KIND` | `bearer` | Expected authentication type (case-insensitive) |
| `--permissive-auth` | `PERMISSIVE_AUTH` | `false` | Allow unauthenticated requests (treat as anonymous) |

#### Standard OIDC (Azure AD, Okta, Auth0)

```bash
export WELLKNOWN_OIDC="https://login.microsoftonline.com/TENANT-ID/v2.0/.well-known/openid-configuration"
export JWT_AUDIENCES="api://your-api-audience"
rest-rego
```

#### WSO2 API Manager

```bash
export WELLKNOWN_OIDC="https://api-manager.example.com/oauth2/token/.well-known/openid-configuration"
export JWT_AUDIENCE_KEY="http://wso2.org/claims/apiname"
export JWT_AUDIENCES="YourAPIName"
export AUTH_HEADER="X-Jwt-Assertion"
export AUTH_KIND=""  # Empty to skip prefix validation
rest-rego
```

#### Multiple OIDC Providers

```bash
# Comma-separated URLs
export WELLKNOWN_OIDC="https://provider1.com/.well-known/openid-configuration,https://provider2.com/.well-known/openid-configuration"
export JWT_AUDIENCES="audience1,audience2"
rest-rego

# Or use multiple -w flags
rest-rego \
  -w https://provider1.com/.well-known/openid-configuration \
  -w https://provider2.com/.well-known/openid-configuration \
  -u audience1,audience2
```

### Azure Graph Authentication

| Option | Env Variable | Default | Description |
|--------|--------------|---------|-------------|
| `-t, --azure-tenant` | `AZURE_TENANT` | - | Azure Tenant ID for Graph authentication |
| `-a, --auth-header` | `AUTH_HEADER` | `Authorization` | HTTP header for authentication token |
| `-k, --auth-kind` | `AUTH_KIND` | `bearer` | Expected authentication type |

```bash
export AZURE_TENANT="your-tenant-id"
rest-rego
```

**Note**: Azure Graph mode requires managed identity or service principal with `Application.Read.All` permission.

### Permissive Authentication Mode

Allow requests without authentication (useful for migration scenarios):

```bash
export PERMISSIVE_AUTH=true
rest-rego
```

**Warning**: In permissive mode, unauthenticated requests are passed to policies with empty auth context. Your policies must handle this explicitly.

## Timeout Configuration

| Option | Env Variable | Default | Description |
|--------|--------------|---------|-------------|
| `--read-header-timeout` | `READ_HEADER_TIMEOUT` | `10s` | Timeout for reading request headers |
| `--read-timeout` | `READ_TIMEOUT` | `30s` | Timeout for reading entire request (headers + body) |
| `--write-timeout` | `WRITE_TIMEOUT` | `90s` | Timeout for writing response |
| `--idle-timeout` | `IDLE_TIMEOUT` | `120s` | Timeout for idle keep-alive connections |
| `--backend-dial-timeout` | `BACKEND_DIAL_TIMEOUT` | `10s` | Timeout for establishing backend connection |
| `--backend-response-timeout` | `BACKEND_RESPONSE_TIMEOUT` | `30s` | Timeout for receiving backend response headers |
| `--backend-idle-timeout` | `BACKEND_IDLE_TIMEOUT` | `90s` | Timeout for idle backend connections |

### Timeout Guidelines

- **Short timeouts** (1-10s): Health checks, metadata endpoints
- **Medium timeouts** (30-60s): Standard API requests
- **Long timeouts** (90-300s): File uploads, batch processing

### Examples

```bash
# Strict timeouts for fast APIs
rest-rego \
  --read-timeout 10s \
  --write-timeout 30s \
  --backend-response-timeout 10s

# Lenient timeouts for slow backends
rest-rego \
  --read-timeout 60s \
  --write-timeout 300s \
  --backend-response-timeout 120s

# Environment variables
export READ_HEADER_TIMEOUT="10s"
export READ_TIMEOUT="30s"
export WRITE_TIMEOUT="90s"
export IDLE_TIMEOUT="120s"
export BACKEND_DIAL_TIMEOUT="10s"
export BACKEND_RESPONSE_TIMEOUT="30s"
export BACKEND_IDLE_TIMEOUT="90s"
rest-rego
```

### Timeout Validation

All timeouts must be:
- Minimum: `1s` (1 second)
- Maximum: `10m` (10 minutes)
- Format: Go duration string (e.g., `30s`, `2m`, `1h`)

## Configuration Examples

### Production JWT Setup

```bash
#!/bin/bash
export WELLKNOWN_OIDC="https://login.microsoftonline.com/YOUR-TENANT/v2.0/.well-known/openid-configuration"
export JWT_AUDIENCES="api://production-api"
export BACKEND_PORT="8080"
export POLICY_DIR="/etc/rest-rego/policies"
export LISTEN_ADDR=":8181"
export MGMT_ADDR=":8182"
export READ_TIMEOUT="30s"
export WRITE_TIMEOUT="90s"
export BACKEND_RESPONSE_TIMEOUT="30s"

rest-rego
```

### Docker Compose

```yaml
version: '3.8'

services:
  rest-rego:
    image: lindex/rest-rego:latest
    ports:
      - "8181:8181"
      - "8182:8182"
    environment:
      WELLKNOWN_OIDC: "https://login.microsoftonline.com/TENANT/v2.0/.well-known/openid-configuration"
      JWT_AUDIENCES: "api://your-audience"
      BACKEND_HOST: "backend"
      BACKEND_PORT: "8080"
      DEBUG: "false"
      VERBOSE: "false"
    volumes:
      - ./policies:/policies:ro
    depends_on:
      - backend
  
  backend:
    image: your-backend:latest
    ports:
      - "8080"
```

### Kubernetes ConfigMap

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: rest-rego-config
data:
  WELLKNOWN_OIDC: "https://login.microsoftonline.com/TENANT/v2.0/.well-known/openid-configuration"
  JWT_AUDIENCES: "api://k8s-api"
  BACKEND_PORT: "8080"
  LISTEN_ADDR: ":8181"
  MGMT_ADDR: ":8182"
  POLICY_DIR: "/policies"
  READ_TIMEOUT: "30s"
  WRITE_TIMEOUT: "90s"
---
apiVersion: v1
kind: Pod
metadata:
  name: rest-rego
spec:
  containers:
  - name: rest-rego
    image: lindex/rest-rego:latest
    envFrom:
    - configMapRef:
        name: rest-rego-config
    volumeMounts:
    - name: policies
      mountPath: /policies
      readOnly: true
  volumes:
  - name: policies
    configMap:
      name: rest-rego-policies
```

### Development Setup

```bash
# .env file
WELLKNOWN_OIDC=https://login.microsoftonline.com/DEV-TENANT/v2.0/.well-known/openid-configuration
JWT_AUDIENCES=api://dev-api
BACKEND_PORT=8080
DEBUG=true
VERBOSE=true
POLICY_DIR=./policies

# Run with .env
export $(cat .env | xargs)
rest-rego
```

## Configuration Validation

rest-rego validates all configuration on startup and exits with clear error messages if invalid.

### Common Validation Errors

#### Conflicting Authentication

```
❌ Error: cannot use both Azure Tenant and OIDC well-known endpoints
```

**Solution**: Choose either JWT (`WELLKNOWN_OIDC`) or Azure (`AZURE_TENANT`), not both.

#### Missing JWT Audience

```
❌ Error: JWT audiences required when using OIDC well-known endpoints
```

**Solution**: Set `JWT_AUDIENCES` when using `WELLKNOWN_OIDC`.

#### Invalid Timeout

```
❌ Error: read timeout must be between 1s and 10m, got: 15m
```

**Solution**: Use timeouts between 1 second and 10 minutes.

#### Invalid Directory

```
❌ Error: policy directory does not exist: /etc/policies
```

**Solution**: Ensure `POLICY_DIR` points to an existing, readable directory.

#### Invalid Port

```
❌ Error: invalid listen address: :abc
```

**Solution**: Use valid port numbers (1-65535).

### Validation Checklist

Before deploying rest-rego:

- [ ] Choose one authentication mode (JWT or Azure)
- [ ] Set required variables for chosen auth mode
- [ ] Verify policy directory exists and contains `.rego` files
- [ ] Test backend connectivity (host/port reachable)
- [ ] Validate timeout values are reasonable
- [ ] Check port conflicts (8181, 8182 available)
- [ ] Test configuration with `rest-rego --help`

## Version Information

Check rest-rego version and build information:

```bash
rest-rego --version
```

Output:
```
rest-rego version v1.2.3
Build date: 2025-11-20
Git commit: abc1234
Go version: go1.25.0
```

## Environment Variable Priority

When both environment variables and command-line flags are set:

1. **Command-line flags** take precedence
2. **Environment variables** are used as defaults
3. **Built-in defaults** apply if neither is set

Example:

```bash
export BACKEND_PORT="8080"
rest-rego --backend-port 9090  # Uses 9090 (flag overrides env)
```

## Related Documentation

- [Policy Development Guide](./POLICY.md) - Writing and testing policies
- [JWT Authentication](./JWT.md) - JWT setup for Azure AD, Okta, Auth0
- [WSO2 Authentication](./WSO2.md) - WSO2 API Manager integration
- [Azure Graph Authentication](./AZURE.md) - Azure Graph setup
- [Deployment Guide](./DEPLOYMENT.md) - Production deployment patterns
- [Troubleshooting](./TROUBLESHOOTING.md) - Configuration issues and solutions
