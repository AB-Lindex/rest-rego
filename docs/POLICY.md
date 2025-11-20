# Policy Development Guide

This guide covers everything you need to know about writing, testing, and deploying OPA Rego policies for rest-rego.

## Table of Contents

- [Policy Basics](#policy-basics)
- [Policy Input Structure](#policy-input-structure)
- [Example Policies](#example-policies)
- [Policy Testing](#policy-testing)
- [Hot Reload](#hot-reload)
- [Best Practices](#best-practices)

## Policy Basics

rest-rego uses the [Rego policy language](https://www.openpolicyagent.org/docs/latest/policy-language/) from Open Policy Agent (OPA).

### Required Elements

Every policy must include:

- `package policies` declaration
- `default allow := false` (deny-by-default security)
- `allow` rule(s) that return boolean

### Optional Elements

- `url` result for customizing URL labels in metrics (e.g., for granularity or GDPR compliance)
- Additional helper rules and functions
- Custom variables for policy results (forwarded as `X-Restrego-*` headers)

### Minimal Policy Example

```rego
package policies

default allow := false

allow if {
  input.jwt.appid == "11112222-3333-4444-5555-666677778888"
}
```

## Policy Input Structure

The policy receives structured input with request details and authentication context:

```json
{
  "request": {
    "method": "GET",
    "path": ["api", "users", "123"],
    "headers": {
      "Authorization": "Bearer <HIDDEN>",
      "Content-Type": "application/json"
    },
    "auth": {
      "kind": "Bearer",
      "token": "<HIDDEN>"
    },
    "size": 0,
    "blocked_headers": {
      "X-Restrego-Custom": "value"
    }
  },
  "jwt": {
    "appid": "<APPLICATION-ID>",
    "aud": ["<AUDIENCE>"],
    "exp": "2025-03-24T11:41:37Z",
    "iat": "2025-03-24T10:36:37Z",
    "iss": "https://sts.windows.net/<TENANT>/",
    "roles": ["admin", "reader"],
    "tenant_id": "<TENANT-ID>",
    "sub": "<USER-ID>"
  },
  "user": {
    "appId": "<APPLICATION-ID>",
    "displayName": "Application Name"
  }
}
```

### Input Fields Reference

| Field | Description | Always Present |
|-------|-------------|----------------|
| `request.method` | HTTP method (GET, POST, PUT, DELETE, etc.) | ✅ |
| `request.path` | URL path split as array (e.g., `/api/users/123` → `["api", "users", "123"]`) | ✅ |
| `request.headers` | Request headers (sensitive values hidden in debug logs) | ✅ |
| `request.auth.kind` | Authentication type (usually "Bearer" or "Basic") | ❌ (only if auth header present) |
| `request.auth.token` | Token value (hidden in logs) | ❌ (only if auth header present) |
| `request.size` | Request body size in bytes | ✅ |
| `request.blocked_headers` | Blocked `X-Restrego-*` headers (only if `EXPOSE_BLOCKED_HEADERS=true`) | ❌ |
| `jwt.*` | JWT claims when using JWT authentication | ❌ (only in JWT mode) |
| `user.*` | Application info when using Azure Graph authentication | ❌ (only in Azure mode) |

## Example Policies

### Simple App-Based Authorization

```rego
package policies

default allow := false

# Allow specific applications
allow if {
  valid_apps := {
    "11112222-3333-4444-5555-666677778888", # app-name-1
    "22223333-4444-5555-6666-777788889999", # app-name-2
  }
  input.jwt.appid in valid_apps
}
```

### Role-Based Access Control (RBAC)

```rego
package policies

default allow := false

# Admins can do everything
allow if {
  "admin" in input.jwt.roles
}

# Readers can only GET
allow if {
  "reader" in input.jwt.roles
  input.request.method == "GET"
}

# Order managers can manage orders
allow if {
  "order-manager" in input.jwt.roles
  input.request.path[0] == "orders"
}

# Support staff can view but not modify
allow if {
  "support" in input.jwt.roles
  input.request.method in ["GET", "HEAD", "OPTIONS"]
}
```

### Path-Based Authorization

```rego
package policies

default allow := false

# Public endpoints don't require authentication
allow if {
  input.request.path[0] == "public"
}

# Health check endpoint
allow if {
  input.request.path[0] == "health"
  input.request.method == "GET"
}

# Protected API requires valid application
allow if {
  input.request.path[0] == "api"
  input.jwt.appid != ""
}

# Admin-only endpoints
allow if {
  input.request.path[0] == "admin"
  "admin" in input.jwt.roles
}
```

### Method-Based Authorization

```rego
package policies

default allow := false

# Anyone authenticated can read
allow if {
  input.request.method in ["GET", "HEAD", "OPTIONS"]
  input.jwt.appid != ""
}

# Only specific apps can write
allow if {
  input.request.method in ["POST", "PUT", "PATCH", "DELETE"]
  input.jwt.appid in ["write-app-id-1", "write-app-id-2"]
}
```

### Tenant-Aware Authorization

```rego
package policies

default allow := false

# Extract tenant from path (e.g., /tenants/{tenant-id}/resources)
tenant_from_path := input.request.path[1] if {
  input.request.path[0] == "tenants"
  count(input.request.path) > 1
}

# Allow access only to user's own tenant
allow if {
  tenant_from_path == input.jwt.tenant_id
}

# Admins can access all tenants
allow if {
  "global-admin" in input.jwt.roles
}
```

### Customizing Metrics URL Labels (GDPR Compliance)

```rego
package policies

default allow := false
default url := ""

# Allow access to user endpoints
allow if {
  input.request.path[0] == "api"
  input.request.path[1] == "users"
  "admin" in input.jwt.roles
}

# Anonymize user IDs in metrics for GDPR compliance
url := "/api/users/:id" if {
  input.request.path[0] == "api"
  input.request.path[1] == "users"
  count(input.request.path) > 2
}

# Generalize order IDs in metrics
url := "/api/orders/:id" if {
  input.request.path[0] == "api"
  input.request.path[1] == "orders"
  count(input.request.path) > 2
}

# Generalize all resource IDs
url := concat("/", [input.request.path[0], input.request.path[1], ":id"]) if {
  count(input.request.path) > 2
  input.request.path[0] == "api"
}
```

### Header Validation

```rego
package policies

default allow := false

# Require specific header to be present
allow if {
  input.request.headers["X-Api-Version"] == "v2"
  input.jwt.appid != ""
}

# Validate content type for POST/PUT
allow if {
  input.request.method in ["POST", "PUT"]
  input.request.headers["Content-Type"] == "application/json"
  input.jwt.appid in ["valid-app-1", "valid-app-2"]
}
```

### Time-Based Authorization

```rego
package policies

import future.keywords.if

default allow := false

# Allow only during business hours (UTC)
allow if {
  hour := time.clock([time.now_ns()])[0]
  hour >= 8
  hour < 18
  input.jwt.appid != ""
}

# Emergency access 24/7 for admins
allow if {
  "emergency-admin" in input.jwt.roles
}
```

### Forwarding Custom Headers to Backend

```rego
package policies

default allow := false

# Allow request
allow if {
  input.jwt.appid != ""
}

# Forward tenant ID to backend (becomes X-Restrego-Tenant-Id header)
tenant_id := input.jwt.tenant_id if {
  input.jwt.tenant_id != ""
}

# Forward user roles (becomes X-Restrego-User-Roles header)
user_roles := concat(",", input.jwt.roles) if {
  count(input.jwt.roles) > 0
}

# Forward custom claim (becomes X-Restrego-Department header)
department := input.jwt.department if {
  input.jwt.department != ""
}
```

### Multi-Layer Authorization with Blocked Headers

```rego
package policies

default allow := false

# Layer 1: Set tenant context for downstream
allow if {
  input.jwt.appid == "gateway-app-id"
}

tenant_id := input.jwt.tenant_id

# Layer 2: Validate upstream tenant (requires EXPOSE_BLOCKED_HEADERS=true)
allow if {
  # Verify upstream set tenant header
  upstream_tenant := input.request.blocked_headers["X-Restrego-Tenant-Id"]
  
  # Verify it matches our JWT
  upstream_tenant == input.jwt.tenant_id
  
  # Only trust specific upstream apps
  input.jwt.appid in ["layer1-app-id", "trusted-gateway-id"]
}
```

### Combining Multiple Conditions

```rego
package policies

default allow := false

# Helper function: check if user is admin
is_admin if {
  "admin" in input.jwt.roles
}

# Helper function: check if path is public
is_public_path if {
  input.request.path[0] in ["public", "health", "docs"]
}

# Helper function: check if valid app
is_valid_app if {
  input.jwt.appid in [
    "app-1",
    "app-2",
    "app-3"
  ]
}

# Allow public paths to everyone
allow if {
  is_public_path
}

# Allow admins to do everything
allow if {
  is_admin
}

# Allow valid apps to read
allow if {
  is_valid_app
  input.request.method == "GET"
}

# Allow valid apps + specific role to write
allow if {
  is_valid_app
  "writer" in input.jwt.roles
  input.request.method in ["POST", "PUT", "DELETE"]
}
```

## Policy Testing

### Online Testing

Use the [Rego Playground](https://play.openpolicyagent.org/) to test policies interactively with sample input.

### Local Testing with OPA CLI

Install OPA CLI:

```bash
# macOS
brew install opa

# Linux
curl -L -o opa https://openpolicyagent.org/downloads/latest/opa_linux_amd64
chmod +x opa
sudo mv opa /usr/local/bin/

# Windows
# Download from https://www.openpolicyagent.org/docs/latest/#running-opa
```

Test your policies:

```bash
# Run unit tests (if you've written .rego test files)
opa test policies/ -v

# Evaluate policy with sample input
echo '{"request": {"method": "GET", "path": ["api", "users"]}, "jwt": {"appid": "test-id", "roles": ["admin"]}}' | \
  opa eval -d policies/ -I 'data.policies.allow'

# Check policy syntax
opa check policies/

# Profile policy performance
opa eval -d policies/ --profile -I 'data.policies.allow' < sample-input.json
```

### Writing Unit Tests

Create a test file `policies/request_test.rego`:

```rego
package policies

test_allow_admin {
  allow with input as {
    "request": {"method": "GET", "path": ["api"]},
    "jwt": {"appid": "test-app", "roles": ["admin"]}
  }
}

test_deny_no_roles {
  not allow with input as {
    "request": {"method": "GET", "path": ["api"]},
    "jwt": {"appid": "test-app", "roles": []}
  }
}

test_allow_public_path {
  allow with input as {
    "request": {"method": "GET", "path": ["public", "info"]},
    "jwt": {}
  }
}
```

Run tests:

```bash
opa test policies/ -v
```

### Testing with rest-rego Debug Mode

Enable debug mode to see policy input and output for real requests:

```bash
# Docker
docker run -e DEBUG=true ... lindex/rest-rego:latest

# Binary
rest-rego --debug
```

This will log the complete policy input and evaluation result for every request.

## Hot Reload

Policy files are automatically reloaded when changed (typically <1 second):

1. Edit `.rego` file in `./policies/` directory
2. Save the file
3. rest-rego detects change and reloads
4. New requests use updated policy immediately

### How It Works

- rest-rego watches the policy directory using filesystem notifications
- When a file changes, all policies are recompiled
- Invalid policies are rejected; previous valid policies remain active
- Reload events are logged and tracked in `restrego_policy_reload_total` metric
- In-flight requests complete using previous policy version

### Best Practices for Hot Reload

1. **Test before deploying**: Validate syntax with `opa check policies/` before saving
2. **Use Git**: Version control policies to track changes and enable rollback
3. **Monitor metrics**: Watch `restrego_policy_reload_total` for reload failures
4. **Atomic updates**: In Kubernetes, update ConfigMaps atomically (entire config map replaced)
5. **Staged rollout**: Test policy changes in dev/staging before production

### Kubernetes ConfigMap Updates

When using ConfigMaps for policies in Kubernetes:

```bash
# Update ConfigMap
kubectl create configmap my-app-policies \
  --from-file=policies/ \
  --dry-run=client -o yaml | kubectl apply -f -

# Trigger reload (ConfigMap mounted as volume)
# rest-rego will detect the change automatically within ~1 second
```

For faster updates, use `kubectl rollout restart` to recreate pods with new policies:

```bash
kubectl rollout restart deployment/my-app
```

## Best Practices

### Security

1. **Deny by default**: Always use `default allow := false`
2. **Explicit allow rules**: Be specific about what's allowed
3. **Validate inputs**: Check for empty/missing values before using them
4. **Avoid secrets in policies**: Use environment variables or Kubernetes secrets for sensitive data
5. **Audit logging**: Use `trace()` for security-relevant decisions

### Performance

1. **Keep policies simple**: Aim for <3ms evaluation time
2. **Avoid external data**: Don't fetch data from external sources in policies
3. **Use sets for lookups**: Sets are faster than arrays for membership checks
4. **Profile policies**: Use `opa eval --profile` to identify bottlenecks
5. **Cache complex computations**: Define helper rules for reused logic

### Maintainability

1. **Use helper functions**: Break complex logic into reusable functions
2. **Add comments**: Explain business rules and edge cases
3. **Consistent naming**: Use clear, descriptive names for rules
4. **Version policies**: Store in Git with meaningful commit messages
5. **Write tests**: Cover edge cases and security-critical paths

### Example: Well-Structured Policy

```rego
package policies

import future.keywords.if

default allow := false
default url := ""

#################
# Configuration #
#################

# Allowed applications
valid_apps := {
  "11112222-3333-4444-5555-666677778888", # production-api
  "22223333-4444-5555-6666-777788889999", # staging-api
}

# Admin roles
admin_roles := {"admin", "super-admin", "global-admin"}

# Public paths (no auth required)
public_paths := {"public", "health", "docs", "swagger"}

###########
# Helpers #
###########

is_admin if {
  some role in admin_roles
  role in input.jwt.roles
}

is_valid_app if {
  input.jwt.appid in valid_apps
}

is_public_path if {
  count(input.request.path) > 0
  input.request.path[0] in public_paths
}

is_read_request if {
  input.request.method in ["GET", "HEAD", "OPTIONS"]
}

###################
# Allow Rules     #
###################

# Allow public endpoints
allow if {
  is_public_path
}

# Allow admins full access
allow if {
  is_admin
}

# Allow valid apps to read
allow if {
  is_valid_app
  is_read_request
}

# Allow valid apps with writer role to modify
allow if {
  is_valid_app
  "writer" in input.jwt.roles
  input.request.method in ["POST", "PUT", "PATCH", "DELETE"]
}

###################
# URL Rewriting   #
###################

# Anonymize resource IDs in metrics
url := sprintf("/%s/:id", [input.request.path[0]]) if {
  count(input.request.path) > 1
  not is_public_path
}
```

## Related Documentation

- [Configuration Reference](./CONFIGURATION.md) - Environment variables and flags
- [Authentication Guides](./JWT.md) - JWT, WSO2, and Azure authentication setup
- [Blocked Headers Feature](./BLOCKED-HEADERS.md) - Multi-layer authorization patterns
- [Troubleshooting](./TROUBLESHOOTING.md) - Common policy issues and solutions

## External Resources

- [OPA Documentation](https://www.openpolicyagent.org/docs/latest/)
- [Rego Language Reference](https://www.openpolicyagent.org/docs/latest/policy-reference/)
- [Rego Playground](https://play.openpolicyagent.org/)
- [OPA Policy Examples](https://github.com/open-policy-agent/opa/tree/main/examples)
