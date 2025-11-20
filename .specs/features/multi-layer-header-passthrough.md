---
type: "feature"
feature: "blocked-headers-policy-input"
repository_type: "single-product"
status: "proposed"
priority: "medium"
complexity: "simple"
technology_stack: ["go"]
azure_services: []
external_services: []
on_premises_dependencies: []
multi_instance_support: "required"
observability: "required"
related_prd: "PRD.md"
cross_product_dependencies: []
---

# Feature: Blocked Headers Policy Input

## Problem Statement

Organizations deploying rest-rego in **multi-layer architectures** need visibility into `X-Restrego-*` headers sent by clients or upstream rest-rego instances for security auditing and policy decisions. Currently, the `CleanupHandler` removes these headers before policy evaluation, making it impossible for policies to:

1. **Detect spoofing attempts**: Policy cannot see if a client tried to inject malicious headers
2. **Validate upstream context**: Policy cannot verify headers from trusted upstream instances
3. **Make layered decisions**: Policy cannot use upstream authorization context in its logic
4. **Audit security events**: No record of blocked headers in policy evaluation

**Use Case**: Multi-layer deployments where:
- Upstream rest-rego instances set context headers (e.g., `X-Restrego-Tenant-Id`)
- Downstream rest-rego needs to validate and use these headers in policy
- Security policies need to audit spoofing attempts
- Headers are removed for security but should be visible to policy

**Current Limitation**: Headers are removed in `CleanupHandler` before `NewInfo()` creates the policy input, so they never reach the Rego policy engine.

## User Stories

### US-001: Upstream Context Validation
**As a** platform engineer  
**I want** downstream rest-rego policies to validate headers from upstream instances  
**So that** I can implement layered authorization with trust verification

**Acceptance Criteria**:
- Upstream rest-rego sets `X-Restrego-Tenant-Id` header
- Downstream rest-rego captures this header before removal
- Policy receives header in `input.request.blocked_headers["X-Restrego-Tenant-Id"]`
- Policy can validate the value and decide to allow/deny based on it
- Headers are still removed from backend request (security preserved)

### US-002: Spoofing Detection
**As a** security engineer  
**I want** policies to detect when clients attempt header spoofing  
**So that** I can audit and block malicious requests

**Acceptance Criteria**:
- Client sends `X-Restrego-Admin: true` (spoofing attempt)
- Header is captured in `input.request.blocked_headers`
- Policy can detect this and deny with `allow := false`
- Metrics track spoofing attempts
- Logging shows blocked header details

### US-003: Conditional Feature Flag
**As a** operations engineer  
**I want** blocked headers in policy input only when needed  
**So that** I minimize performance impact for simple deployments

**Acceptance Criteria**:
- Default behavior: headers removed, NOT added to policy input
- Flag `EXPOSE_BLOCKED_HEADERS=true` enables feature
- When disabled, `blocked_headers` map is empty or omitted
- No performance impact when disabled
- Backward compatible with existing policies

### US-004: Forward Validated Headers to Backend
**As a** policy developer  
**I want** to extract validated values from blocked headers and forward them to the backend  
**So that** I can pass trusted upstream context after validation

**Acceptance Criteria**:
- Policy can extract values: `tenantid := input.request.blocked_headers["X-Restrego-Tenant-Id"]`
- Policy result variables are forwarded as `X-Restrego-*` headers to backend
- Only validated/trusted values are forwarded (policy controls what gets through)
- Backend receives policy-approved headers, not original spoofable headers
- Clear documentation of this pattern

## Requirements

### Functional Requirements

#### FR-001: Capture Blocked Headers
- **Priority**: High
- **Description**: Capture `X-Restrego-*` headers before removal in `CleanupHandler`
- **Implementation**:
  - Extract headers matching `X-Restrego-*` prefix
  - Store in request context for later access
  - Headers still removed from `http.Request` (security preserved)
- **Validation**:
  - Headers never reach backend service
  - Headers available to policy via `Info` struct
  - Case-insensitive prefix matching

#### FR-002: Add Blocked Headers to Policy Input
- **Priority**: High
- **Description**: Expose blocked headers in Rego policy input structure
- **Implementation**:
  - Add `BlockedHeaders map[string]interface{}` field to `RequestInfo` struct
  - Populate during `NewInfo()` creation if feature enabled
  - Map structure mirrors existing `Headers` field format
- **Schema**:
  ```json
  {
    "request": {
      "method": "GET",
      "path": ["api", "users"],
      "headers": { /* normal headers */ },
      "blocked_headers": {
        "X-Restrego-Tenant-Id": "tenant-123",
        "X-Restrego-User-Context": "admin"
      }
    }
  }
  ```

#### FR-003: Configuration Flag
- **Priority**: High
- **Description**: Feature flag to control blocked headers exposure
- **Implementation**:
  - New config: `EXPOSE_BLOCKED_HEADERS` (env var, CLI flag)
  - Default: `false` (zero performance impact, backward compatible)
  - When `true`: Capture and expose headers to policy
  - When `false`: Skip capture logic entirely
- **Validation**:
  - Boolean flag (no partial modes)
  - Works with all authentication providers
  - No breaking changes to existing deployments

#### FR-004: Policy Access to Blocked Headers
- **Priority**: High
- **Description**: Rego policies can access blocked headers for decisions
- **Policy Examples**:
  ```rego
  # Example 1: Validate upstream tenant header
  allow if {
    input.request.blocked_headers["X-Restrego-Tenant-Id"] == "trusted-tenant"
    input.jwt.roles[_] == "admin"
  }
  
  # Example 2: Detect spoofing attempts
  default allow := false
  allow if {
    count(input.request.blocked_headers) == 0  # No spoofing
    # ... other authorization logic
  }
  
  # Example 3: Audit but don't block
  allow if {
    # Log spoofing for audit
    count(input.request.blocked_headers) > 0
    true  # Still allow, but logged
  }
  ```

#### FR-005: Forward Validated Header Values
- **Priority**: High
- **Description**: Policies can extract and forward validated header values to backend
- **Implementation**:
  - Policy extracts values from `blocked_headers`
  - Policy returns result variables (snake_case)
  - rest-rego converts to `X-Restrego-*` headers for backend
  - Only policy-approved values forwarded (security through validation)
- **Policy Pattern**:
  ```rego
  # Extract and validate upstream header
  tenantid := input.request.blocked_headers["X-Restrego-Tenant-Id"]
  
  # Validate it's trusted
  allow if {
    tenantid == "trusted-tenant"
    input.jwt.valid == true
  }
  
  # Backend receives: X-Restrego-Tenantid: trusted-tenant
  # (only if validation passed)
  ```
- **Security Model**:
  - Original headers removed (prevent spoofing)
  - Policy validates before forwarding (trust gate)
  - Backend receives only policy-approved values
  - Policy can transform/sanitize values before forwarding

### Non-Functional Requirements

#### NFR-001: Performance
- **Target**: Zero overhead when feature disabled (`EXPOSE_BLOCKED_HEADERS=false`)
- **Target**: <1ms overhead when enabled (simple map copy)
- **Implementation**: Early return in cleanup handler if flag disabled
- **Validation**: Benchmark tests comparing enabled vs disabled

#### NFR-002: Security
- **Guarantee**: Headers ALWAYS removed from backend request
- **Guarantee**: Headers only visible to policy, never forwarded
- **Design**: Separation of concerns - cleanup removes, policy reads
- **Audit**: Log blocked headers at debug level when captured

#### NFR-003: Observability
- **Metrics**:
  - `restrego_blocked_headers_exposed` (gauge): 0 or 1 (config state)
  - `restrego_blocked_headers_captured_total` (counter): Headers captured
  - `restrego_requests_with_blocked_headers_total` (counter): Requests with blocked headers
- **Logging**:
  - Info-level: Configuration state on startup
  - Debug-level: Individual headers captured
  - Warn-level: High frequency of blocked headers (potential attack)

#### NFR-004: Backward Compatibility
- **Guarantee**: Existing policies unaffected
- **Guarantee**: Default behavior unchanged (headers removed, not exposed)
- **Guarantee**: Rego input structure extended, not changed
- **Migration**: Zero-touch upgrade (feature opt-in)

## Technical Design

### Architecture

**Current Flow**:
```
Incoming Request → CleanupHandler → Headers Removed → WrapHandler → NewInfo() → Policy → Backend
                   (removes X-Restrego-*)              (creates Info)   (no blocked headers)
```

**New Flow with Feature Enabled**:
```
Incoming Request → CleanupHandler → Headers Removed → WrapHandler → NewInfo() → Policy → Backend
                   ↓ Captures first   (still removed   (receives      (sees blocked_headers
                   blocked_headers    for security)     captured map)   in input.request)
                   to context
```

### Data Flow

1. **Cleanup Phase** (`CleanupHandler`):
   - Scan incoming headers for `X-Restrego-*` prefix
   - If `EXPOSE_BLOCKED_HEADERS=true`: Store in request context
   - Remove headers from `http.Request` (always happens)

2. **Wrap Phase** (`WrapHandler`):
   - Create `Info` struct via `NewInfo()`
   - Retrieve blocked headers from request context
   - Add to `Info.Request.BlockedHeaders` map

3. **Policy Phase** (`policyHandler`):
   - Policy receives `input.request.blocked_headers`
   - Can use for authorization decisions
   - Backend receives request WITHOUT blocked headers

### Configuration Changes

**File**: `internal/config/config.go`

```go
type Fields struct {
    // ... existing fields ...
    
    ExposeBlockedHeaders bool `arg:"--expose-blocked-headers,env:EXPOSE_BLOCKED_HEADERS" 
                               default:"false" 
                               help:"expose X-Restrego-* headers to policy as blocked_headers (security: headers still removed from backend)"`
}
```

### Type Changes

**File**: `internal/types/request.go`

```go
// RequestInfo is the request information for the rego-policy
type RequestInfo struct {
    Method         string                 `json:"method"`
    Path           []string               `json:"path"`
    Headers        map[string]interface{} `json:"headers"`
    BlockedHeaders map[string]interface{} `json:"blocked_headers,omitempty"` // NEW
    Auth           *RequestAuth           `json:"auth"`
    Size           int64                  `json:"size"`
    ID             string                 `json:"id,omitempty"`
}
```

**Context Key for Header Storage**:
```go
type ctxKey int

const (
    ctxInfoKey ctxKey = iota
    ctxBlockedHeadersKey
)
```

### Implementation Details

**File**: `internal/router/cleanup.go`

```go
package router

import (
    "context"
    "log/slog"
    "net/http"
    "strings"
)

type ctxKey int

const ctxBlockedHeadersKey ctxKey = 1

// CleanupHandler removes incoming headers that shouldn't be there
// and optionally captures them for policy evaluation
func (proxy *Proxy) CleanupHandler(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        var toClean []string
        var blocked map[string]interface{}
        
        // Only allocate map if feature enabled
        if proxy.config.ExposeBlockedHeaders {
            blocked = make(map[string]interface{})
        }
        
        for key, values := range r.Header {
            if strings.HasPrefix(key, "X-Restrego-") {
                toClean = append(toClean, key)
                
                // Capture header value if feature enabled
                if proxy.config.ExposeBlockedHeaders {
                    if len(values) == 1 {
                        blocked[key] = values[0]
                    } else {
                        blocked[key] = values
                    }
                    slog.Debug("captured blocked header", "header", key)
                }
            }
        }
        
        // Remove headers (always happens for security)
        for _, key := range toClean {
            if !proxy.config.ExposeBlockedHeaders {
                slog.Warn("removing header (possible spoofing)", "header", key)
            }
            r.Header.Del(key)
        }
        
        // Store blocked headers in context if any were captured
        if len(blocked) > 0 {
            ctx := context.WithValue(r.Context(), ctxBlockedHeadersKey, blocked)
            r = r.WithContext(ctx)
            slog.Debug("stored blocked headers in context", "count", len(blocked))
        }
        
        next.ServeHTTP(w, r)
    })
}

// GetBlockedHeaders retrieves blocked headers from request context
func GetBlockedHeaders(r *http.Request) map[string]interface{} {
    if blocked, ok := r.Context().Value(ctxBlockedHeadersKey).(map[string]interface{}); ok {
        return blocked
    }
    return nil
}
```

**File**: `internal/types/request.go` - Update `NewInfo()`

```go
// NewInfo creates a new instance of the Info based on the request
func NewInfo(r *http.Request, authKey string) *Info {
    i := new(Info)
    i.Request.Method = r.Method
    i.Request.Path = strings.Split(strings.TrimPrefix(r.URL.Path, "/"), "/")
    i.Request.Size = r.ContentLength
    i.URL = r.URL.String()

    // Populate normal headers
    i.Request.Headers = make(map[string]interface{})
    for k, v := range r.Header {
        if len(v) == 1 {
            i.Request.Headers[k] = v[0]
        } else {
            i.Request.Headers[k] = v
        }
    }

    // Populate blocked headers from context (if any)
    if blocked := getBlockedHeadersFromContext(r); blocked != nil {
        i.Request.BlockedHeaders = blocked
    }

    // ... existing auth logic ...
    
    return i
}

// getBlockedHeadersFromContext retrieves blocked headers from request context
func getBlockedHeadersFromContext(r *http.Request) map[string]interface{} {
    // Import router package would create circular dependency
    // So we duplicate the context key value here
    type ctxKey int
    const ctxBlockedHeadersKey ctxKey = 1
    
    if blocked, ok := r.Context().Value(ctxBlockedHeadersKey).(map[string]interface{}); ok {
        return blocked
    }
    return nil
}
```

**Alternative Approach (Cleaner)**: Pass blocked headers directly to `NewInfo()`

```go
// In router/wrap.go
func (proxy *Proxy) WrapHandler(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        now := time.Now()
        w2 := bufferedresponse.Wrap(w)
        defer w2.Flush()

        blocked := GetBlockedHeaders(r) // Retrieve from context
        info := types.NewInfoWithBlocked(r, proxy.authKey, blocked)
        
        // ... rest of handler
    })
}

// In types/request.go - cleaner API
func NewInfoWithBlocked(r *http.Request, authKey string, blocked map[string]interface{}) *Info {
    i := NewInfo(r, authKey)
    if len(blocked) > 0 {
        i.Request.BlockedHeaders = blocked
    }
    return i
}
```

### Metrics Integration

**File**: `internal/metrics/metrics.go`

```go
var (
    // ... existing metrics ...
    
    BlockedHeadersExposed = promauto.NewGauge(prometheus.GaugeOpts{
        Name: "restrego_blocked_headers_exposed",
        Help: "Whether blocked headers are exposed to policy (1=enabled, 0=disabled)",
    })
    
    BlockedHeadersCaptured = promauto.NewCounter(prometheus.CounterOpts{
        Name: "restrego_blocked_headers_captured_total",
        Help: "Total number of X-Restrego-* headers captured",
    })
    
    RequestsWithBlockedHeaders = promauto.NewCounter(prometheus.CounterOpts{
        Name: "restrego_requests_with_blocked_headers_total",
        Help: "Total number of requests containing blocked headers",
    })
)
```

**Update Cleanup Handler**:
```go
if proxy.config.ExposeBlockedHeaders {
    blocked[key] = values[0]
    metrics.BlockedHeadersCaptured.Inc()
}

if len(blocked) > 0 {
    metrics.RequestsWithBlockedHeaders.Inc()
}
```

### Testing Strategy

**Unit Tests** (`internal/router/cleanup_test.go`):
```go
func TestCleanupHandler_BlockedHeaders(t *testing.T) {
    testCases := []struct {
        name              string
        exposeEnabled     bool
        setupHeaders      map[string]string
        expectRemoved     []string
        expectInContext   map[string]interface{}
    }{
        {
            name:          "default: removes headers, doesn't capture",
            exposeEnabled: false,
            setupHeaders: map[string]string{
                "X-Restrego-Tenant": "tenant-123",
                "Authorization":     "Bearer token",
            },
            expectRemoved:   []string{"X-Restrego-Tenant"},
            expectInContext: nil,
        },
        {
            name:          "enabled: removes AND captures headers",
            exposeEnabled: true,
            setupHeaders: map[string]string{
                "X-Restrego-Tenant": "tenant-123",
                "X-Restrego-Appid":  "app-456",
                "Authorization":     "Bearer token",
            },
            expectRemoved: []string{"X-Restrego-Tenant", "X-Restrego-Appid"},
            expectInContext: map[string]interface{}{
                "X-Restrego-Tenant": "tenant-123",
                "X-Restrego-Appid":  "app-456",
            },
        },
        {
            name:            "no blocked headers: no context",
            exposeEnabled:   true,
            setupHeaders:    map[string]string{"Authorization": "Bearer token"},
            expectRemoved:   []string{},
            expectInContext: nil,
        },
    }
    // ... test implementation
}
```

**Integration Tests** (`tests/blocked-headers.http`):
```http
### Test 1: Verify headers removed from backend
POST http://localhost:8181/echo
X-Restrego-Tenant: tenant-123
Content-Type: application/json

# Backend should NOT receive X-Restrego-Tenant header

### Test 2: Policy can access blocked headers
# Requires: EXPOSE_BLOCKED_HEADERS=true
# Policy should see input.request.blocked_headers
GET http://localhost:8181/api/validate
X-Restrego-Tenant: tenant-123
X-Restrego-Custom: value
Authorization: Bearer {{jwt_token}}
```

**Policy Test** (`policies/test-blocked.rego`):
```rego
package test

# Test policy that uses blocked headers
default allow := false

allow if {
    # Only allow if client didn't try to spoof
    count(input.request.blocked_headers) == 0
}

# Extract and forward validated tenant
tenantid := input.request.blocked_headers["X-Restrego-Tenant-Id"]
appid := input.request.blocked_headers["X-Restrego-App-Id"]

allow if {
    # Validate upstream headers
    tenantid == "trusted-tenant"
    appid == "trusted-app"
    input.jwt.valid == true
}
# Backend receives:
# - X-Restrego-Tenantid: trusted-tenant
# - X-Restrego-Appid: trusted-app
```

## Implementation Phases

### Phase 1: MVP - Blocked Headers Exposure
**Duration**: 1-2 days  
**Scope**:
1. Add `EXPOSE_BLOCKED_HEADERS` config flag
2. Add `BlockedHeaders` field to `RequestInfo` struct
3. Update `CleanupHandler` to capture headers in context
4. Update `NewInfo()` to populate blocked headers from context
5. Add metrics for observability
6. Add unit tests for all scenarios
7. Update documentation

**Implementation Steps**:
1. **Config** (`internal/config/config.go`): Add `ExposeBlockedHeaders bool` field
2. **Types** (`internal/types/request.go`): Add `BlockedHeaders map[string]interface{}` to `RequestInfo`
3. **Cleanup** (`internal/router/cleanup.go`): 
   - Add context key for blocked headers
   - Capture headers when flag enabled
   - Store in context before removal
4. **Wrap** (`internal/router/wrap.go`): Retrieve blocked headers from context, pass to `NewInfo()`
5. **Metrics** (`internal/metrics/metrics.go`): Add blocked headers metrics
6. **Tests** (`internal/router/cleanup_test.go`): Test all scenarios
7. **Docs**: Update README.md with usage examples

**Deliverables**:
- Feature flag in config
- Updated type definitions
- Enhanced cleanup handler
- Test coverage >90%
- Documentation in README.md
- Example policy using blocked headers

### Phase 2: Enhanced Observability (Future)
**Duration**: 1 day (future work)  
**Scope**:
1. Add histogram for blocked header count distribution
2. Add sampling/rate-limiting for high-frequency warnings
3. Add dashboard examples for Grafana
4. Add alerting rules for anomalous patterns

### Phase 3: Advanced Policy Patterns (Future)
**Duration**: 2-3 days (future work)  
**Scope**:
1. Example policies for common scenarios
2. Cryptographic validation helpers (HMAC signature verification)
3. Policy library for multi-layer trust chains
4. Documentation for security best practices

## Integration

### PRD Link
- **REQ-010: Middleware Pipeline** - Extends cleanup middleware with header capture
- **REQ-009: Configuration Management** - Adds new configuration flag with validation
- **REQ-007: Prometheus Metrics** - Adds blocked headers metrics
- **REQ-008: Structured Logging** - Enhanced logging for header decisions
- **REQ-004: Policy Engine** - Extends policy input with blocked headers

### README Impact
**New Section**: "Blocked Headers for Policy Evaluation"

```markdown
## 🔒 Blocked Headers for Policy Evaluation

### Security Header Capture

rest-rego removes `X-Restrego-*` headers from incoming requests to prevent spoofing. You can optionally expose these blocked headers to your Rego policies for validation and auditing.

**Use Cases**:
- **Multi-layer authorization**: Validate headers from trusted upstream rest-rego instances
- **Spoofing detection**: Audit attempts to inject malicious headers
- **Context propagation**: Use upstream authorization context in downstream policies

**Configuration**:
```bash
EXPOSE_BLOCKED_HEADERS=true  # Expose blocked headers to policy
```

**Policy Access**:
```rego
package request

# Example 1: Validate and forward upstream tenant
tenantid := input.request.blocked_headers["X-Restrego-Tenant-Id"]

allow if {
    tenantid == "trusted-tenant"
    input.jwt.roles[_] == "admin"
}
# Backend receives: X-Restrego-Tenantid: trusted-tenant

# Example 2: Detect spoofing attempts
default allow := false
allow if {
    count(input.request.blocked_headers) == 0  # No spoofing detected
    # ... other authorization logic
}

# Example 3: Validate and transform before forwarding
appid := input.request.blocked_headers["X-Restrego-App-Id"]
userid := input.jwt.sub

allow if {
    appid == "trusted-app-123"
    userid != ""
}
# Backend receives both:
# - X-Restrego-Appid: trusted-app-123
# - X-Restrego-Userid: user@example.com
```

**Security Notes**:
- Headers are ALWAYS removed from backend requests
- Blocked headers only visible to policy, never forwarded
- Default behavior: headers removed, NOT exposed to policy
- Zero performance impact when feature disabled

**Metrics**:
- `restrego_blocked_headers_exposed`: Feature state (0=disabled, 1=enabled)
- `restrego_blocked_headers_captured_total`: Total headers captured
- `restrego_requests_with_blocked_headers_total`: Requests with blocked headers
```

### Documentation Files to Update
1. **README.md**: Add blocked headers section
2. **docs/POLICY.md** (new): Detailed policy development guide with blocked headers
3. **examples/policies/blocked-headers.rego** (new): Example policies
4. **.specs/PRD.md**: Add feature reference to related_features array
5. **CHANGELOG.md**: Feature announcement

### Cross-References
- Related to existing header handling in `CleanupHandler`
- Extends policy input structure from `types.RequestInfo`
- Integrates with existing metrics framework
- Compatible with all authentication providers (JWT, Azure)

## Quality Checklist

- [x] Clear problem statement and user stories
- [x] Technology stack aligns with AB-Lindex standards (Go)
- [x] Multi-instance support considerations included (stateless, per-instance config)
- [x] Observability requirements defined (metrics, logging)
- [x] Cross-references to PRD maintained
- [x] Backward compatibility guaranteed (default behavior unchanged)
- [x] Security model defined (headers never forwarded to backend)
- [x] Testing strategy comprehensive (unit + integration + policy tests)
- [x] Implementation phases clear (MVP → future enhancements)
- [x] Zero performance impact when disabled

## Open Questions

1. **JSON Field Name**: Should blocked headers field be `blocked_headers` (snake_case, consistent with Rego conventions) or `blockedHeaders` (camelCase, consistent with Go)?
   - **Recommendation**: `blocked_headers` for Rego consistency
2. **Maximum Header Count**: Should we limit the number of blocked headers captured to prevent memory issues?
   - **Recommendation**: Add configurable limit (default: 50 headers)
3. **Header Size Limits**: Should we enforce size limits on individual header values?
   - **Recommendation**: Use existing HTTP server limits, no additional restrictions
4. **Case Sensitivity**: Should header name matching be case-sensitive in the blocked headers map?
   - **Recommendation**: Use canonical header names (via `http.CanonicalHeaderKey()`)

## Future Enhancements

1. **Selective Capture**: Whitelist/blacklist specific header names for capture
2. **Header Transformations**: Normalize or sanitize header values before policy input
3. **Signature Validation**: HMAC-based validation for cross-network trust
4. **Rate Limiting**: Throttle requests with excessive blocked headers
5. **Enhanced Metrics**: Histogram of header count per request, top blocked header names
6. **Policy Helpers**: Rego functions for common blocked header patterns
7. **Audit Logging**: Dedicated audit log for blocked header events
