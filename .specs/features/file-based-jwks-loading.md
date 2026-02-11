---
type: "feature"
feature: "file-based-jwks-loading"
repository_type: "single-product"
status: "proposed"
priority: "medium"
complexity: "standard"
technology_stack: ["go"]
azure_services: []
external_services: []
on_premises_dependencies: []
multi_instance_support: "compatible"
observability: "basic"
related_prd: "PRD.md"
cross_product_dependencies: []
---

# Feature: File-Based JWKS and Well-Known Configuration Loading

## Problem Statement

Organizations deploying rest-rego need to load JWT signing keys and OIDC well-known configurations from local files for testing, offline scenarios, air-gapped environments, and CI/CD pipelines. Currently, rest-rego only supports loading JWKS and well-known configurations from HTTP(S) endpoints, which creates challenges:

1. **Testing Limitations**: Cannot run integration tests without external OIDC providers
2. **Air-Gapped Environments**: Cannot deploy in isolated networks without internet access
3. **CI/CD Pipeline Issues**: External dependencies make builds fragile and slow
4. **Development Friction**: Developers need network access and external services for local testing
5. **Disaster Recovery**: No fallback mechanism if OIDC providers are temporarily unavailable

**Use Case**: Development, testing, and deployment scenarios where:
- Integration tests need predictable, version-controlled JWKS files
- Air-gapped production environments cannot reach external OIDC providers
- CI/CD pipelines need fast, reliable builds without external dependencies
- Local development requires offline capability
- Static JWKS keys are sufficient (no key rotation needed)

**Current Limitation**: The `LoadWellKnowns()` and `LoadJWKS()` methods in `internal/jwtsupport/jwt.go` only support HTTP(S) URLs via the `go-resthelp` library and `jwk.Cache`, requiring network connectivity to external OIDC providers.

## User Stories

### US-001: Local Development with File-Based JWKS
**As a** developer  
**I want** to use local JWKS files during development  
**So that** I can test JWT authentication without external dependencies

**Acceptance Criteria**:
- Configure `WELLKNOWN_OIDC=file:///path/to/well-known.json`
- Well-known file contains `jwks_uri: "file:///path/to/jwks.json"`
- rest-rego loads both files successfully
- JWT validation works with keys from local file
- No network requests are made
- Log messages indicate "loaded from file"

**Example Configuration**:
```bash
export WELLKNOWN_OIDC="file:///home/dev/config/well-known.json"
export JWT_AUDIENCES="api://my-api"
```

**Example well-known.json**:
```json
{
  "issuer": "https://login.example.com",
  "jwks_uri": "file:///home/dev/config/jwks.json",
  "id_token_signing_alg_values_supported": ["RS256"]
}
```

**Invalid Example (Cross-Source - REJECTED)**:
```json
{
  "issuer": "https://login.example.com",
  "jwks_uri": "https://login.example.com/jwks",
  "id_token_signing_alg_values_supported": ["RS256"]
}
```
If loaded via `WELLKNOWN_OIDC=file:///config/well-known.json`, this would be rejected with:
```
ERROR source type mismatch well-known=file:///config/well-known.json well-known-type=file 
jwks-uri=https://login.example.com/jwks jwks-type=http 
error="well-known and jwks_uri must use matching source types (both file or both http)"
```

### US-002: Air-Gapped Production Deployment
**As a** platform engineer  
**I want** to deploy rest-rego in air-gapped environments  
**So that** I can secure isolated networks without internet access

**Acceptance Criteria**:
- JWKS and well-known files mounted via Kubernetes ConfigMap/Secret
- rest-rego uses `file:` URLs to load configurations
- No external network calls attempted
- JWT validation works with static keys
- Audit logs show successful file loading
- Documentation includes Kubernetes deployment example

**Example Kubernetes ConfigMap**:
```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: rest-rego-jwks
data:
  well-known.json: |
    {
      "jwks_uri": "file:///config/jwks.json",
      "id_token_signing_alg_values_supported": ["RS256", "ES256"]
    }
  jwks.json: |
    {
      "keys": [
        {
          "kty": "RSA",
          "use": "sig",
          "kid": "key-2024",
          "n": "...",
          "e": "AQAB"
        }
      ]
    }
```

### US-003: Reliable CI/CD Testing
**As a** DevOps engineer  
**I want** integration tests to use file-based JWKS  
**So that** CI/CD pipelines are fast and don't depend on external services

**Acceptance Criteria**:
- Test fixtures include JWKS and well-known JSON files
- Test scripts use `file:` URLs
- Tests run without network access
- Tests execute faster than HTTP-based loading
- No test flakes from network issues
- Tests can run in parallel without rate limiting

### US-004: Mixed Configuration Support (Multiple Issuers)
**As a** operations engineer  
**I want** to use both file and HTTP configurations simultaneously for different issuers  
**So that** I can transition gradually or support multiple issuers

**Acceptance Criteria**:
- Configure multiple well-known URLs: `WELLKNOWN_OIDC="file:///local.json,https://remote.com/.well-known"`
- rest-rego loads from both sources (different issuers)
- Each issuer's well-known and jwks_uri must use matching source types
- Cross-source within single issuer rejected: file well-known → http jwks_uri ✗
- JWT validation attempts all configured issuers
- Logging distinguishes between file and HTTP sources
- One failure doesn't prevent loading others
- Clear error if well-known and jwks_uri source types don't match

### US-005: Error Handling and Diagnostics
**As a** operations engineer  
**I want** clear error messages for file loading failures  
**So that** I can quickly diagnose configuration issues

**Acceptance Criteria**:
- File not found → Clear error with absolute path
- Invalid JSON → Parse error with line/column if possible
- Invalid JWKS format → Validation error details
- Permission denied → File permission guidance
- Startup fails if no valid JWKS loaded (same as HTTP behavior)
- Structured logging includes file paths and error details

## Requirements

### Functional Requirements

#### FR-001: File URL Parsing
- **Priority**: High
- **Description**: Parse and normalize `file:` URL notation
- **Supported Formats**:
  - `file:/path/to/file` (relative or absolute)
  - `file:///absolute/path/to/file` (absolute with triple slash)
  - `file://localhost/path` (localhost prefix)
- **Implementation**:
  - Utility function `isFileURL(url string) bool`
  - Utility function `readFileURL(url string) ([]byte, error)`
  - Handle URL decoding (spaces, special characters)
  - Convert to absolute paths for reading
- **Validation**:
  - Unit tests for various URL formats
  - Cross-platform compatibility (Linux, Windows, macOS)

#### FR-002: Well-Known File Loading
- **Priority**: High
- **Description**: Load OIDC well-known configuration from local files
- **Implementation**:
  - Modify `LoadWellKnowns()` to detect file URLs
  - Read file content using `os.ReadFile()`
  - Parse JSON using `json.Unmarshal()`
  - Support both file and HTTP URLs in same configuration
- **Validation**:
  - File successfully parsed into `wellKnownData` struct
  - `jwks_uri` field properly extracted
  - Logging indicates file source

#### FR-003: JWKS File Loading
- **Priority**: High
- **Description**: Load JWKS (JSON Web Key Set) from local files
- **Implementation**:
  - Modify `LoadJWKS()` to detect file URLs in `jwks_uri`
  - Read JWKS file using `readFileURL()`
  - Parse using `jwk.Parse(data)` from `lestrrat-go/jwx/v2/jwk`
  - Apply `PostFetch()` processing for algorithm handling
  - Store as static `jwk.Set` (no caching/refresh for file sources)
- **Validation**:
  - Keys properly loaded and available
  - Algorithm postprocessing applied
  - Key count logged correctly

#### FR-004: Authentication with File-Based Keys
- **Priority**: High
- **Description**: JWT validation works with file-loaded keys
- **Implementation**:
  - Modify `Authenticate()` to handle both cached (HTTP) and static (file) JWKS
  - Track JWKS source (file vs HTTP) to determine lookup strategy
  - Use static set lookup for file-based keys (no cache refresh)
  - Use cache lookup for HTTP-based keys
- **Validation**:
  - Valid JWTs signed with file-based keys accepted
  - Invalid JWTs rejected
  - Logging shows correct issuer/key source

#### FR-005: Mixed Source Support (Between Issuers)
- **Priority**: Medium
- **Description**: Support mixing file and HTTP sources **across different issuers**
- **Scope**: Different well-known configurations can use different source types
- **Constraint**: Within a single issuer, well-known and jwks_uri must match source types (see FR-006)
- **Implementation**:
  - Iterate through all `wellknownList` entries
  - Apply appropriate loading strategy per entry
  - Maintain consistent validation logic
  - Track which JWKS corresponds to which well-known
- **Validation**:
  - Can configure multiple well-known URLs with different source types
  - Both file-based and HTTP-based issuers used for validation
  - Failure in one doesn't block the other
- **Example**: `WELLKNOWN_OIDC="file:///config/test.json,https://login.microsoftonline.com/.well-known/openid-configuration"` ✓ Valid

#### FR-006: Source Type Validation
- **Priority**: High
- **Description**: Enforce matching source types within issuer configuration
- **Rule**: If well-known uses `file:` URL, its `jwks_uri` must also use `file:` URL (and vice versa)
- **Rationale**: 
  - Prevents configuration errors (e.g., file well-known pointing to unreachable HTTP JWKS)
  - Simplifies security model (consistent source trust level)
  - Avoids partial failures in air-gapped environments
  - Makes deployment intent explicit
- **Implementation**:
  - After loading well-known, check source type mismatch
  - If well-known is file: and jwks_uri is http(s): → error
  - If well-known is http(s): and jwks_uri is file: → error
  - Log error and skip this issuer (don't exit, allow other issuers)
- **Validation**:
  - Mixed sources within issuer rejected with clear error
  - Error message explains the constraint
  - Other issuers continue loading

#### FR-007: Error Handling
- **Priority**: High
- **Description**: Clear error handling for file operations
- **Implementation**:
  - Check file existence before reading
  - Wrap `os.ErrNotExist` with helpful context
  - Provide absolute path in error messages
  - Validate JSON structure after parsing
  - Validate source type consistency (FR-006)
  - Log errors with structured logging (slog)
- **Validation**:
  - Missing files produce clear errors
  - Invalid JSON shows parse errors
  - Permission issues include file path
  - Source type mismatches caught early

### Non-Functional Requirements

#### NFR-001: Performance
- **Requirement**: File loading should not significantly impact startup time
- **Metrics**:
  - File read < 10ms for typical JWKS files (< 10KB)
  - No network latency overhead
  - Faster than HTTP loading for local development
- **Implementation**:
  - Direct file I/O (no caching overhead)
  - Single read per file at startup
  - No periodic refresh for file sources

#### NFR-002: Security
- **Requirement**: File-based JWKS maintains same security guarantees
- **Requirements**:
  - File contents validated before use (JSON parsing, JWKS structure)
  - File path traversal prevention (validate paths)
  - Clear logging of file sources for auditing
  - No weakening of JWT validation logic
  - File permissions respected (Unix file permissions)
- **Validation**:
  - Security review of file handling code
  - Path traversal tests
  - Permission-denied scenarios handled

#### NFR-003: Kubernetes Multi-Instance Support
- **Requirement**: Compatible with multiple pod instances
- **Considerations**:
  - Static JWKS files mounted identically across all pods
  - No state sharing needed (each pod loads its own files)
  - No coordination required between instances
  - ConfigMap/Secret updates require pod restart (acceptable)
- **Implementation Pattern**:
  - Mount JWKS files via Kubernetes ConfigMap or Secret
  - All pods use same file paths via volume mounts
  - No pod-to-pod communication needed

#### NFR-004: Observability
- **Requirement**: Clear visibility into file loading operations
- **Metrics**:
  - Structured logs for file operations (slog)
  - Log level DEBUG for file paths and content size
  - Log level INFO for successful loading with key counts
  - Log level ERROR for failures with full context
- **Prometheus Metrics** (reuse existing):
  - JWT validation success/failure rates (no new metrics needed)
  - Existing metrics still work with file-based keys

#### NFR-005: Backward Compatibility
- **Requirement**: Existing HTTP-based configurations continue working
- **Requirements**:
  - No breaking changes to configuration format
  - HTTP URLs processed exactly as before
  - No performance regression for HTTP-based loading
  - Existing deployments unaffected
- **Validation**:
  - All existing tests pass
  - HTTP-based loading still uses cache with refresh
  - Log messages clearly distinguish sources

## Technical Design

### Architecture Overview

**Current Flow (HTTP Only)**:
```
WELLKNOWN_OIDC → LoadWellKnowns() → HTTP GET → Parse JSON → wellknownList
                                                                ↓
                                                        wellknownList.JwksURI
                                                                ↓
                 LoadJWKS() → jwk.Cache.Register() → HTTP GET → jwk.Set
                                                                ↓
                 Authenticate() → cache.Get() → Validate JWT
```

**Enhanced Flow (File + HTTP)**:
```
WELLKNOWN_OIDC → LoadWellKnowns() → isFileURL() → Yes → os.ReadFile() → Parse JSON
                                        ↓                                      ↓
                                       No                             wellknownList
                                        ↓                                      ↓
                                   HTTP GET → Parse JSON              wellknownList.JwksURI
                                                                              ↓
                 LoadJWKS() → isFileURL() → Yes → os.ReadFile() → jwk.Parse() → PostFetch()
                                  ↓                                                  ↓
                                 No                                            Static jwk.Set
                                  ↓                                                  ↓
                        jwk.Cache.Register() → HTTP GET                       JWKS array
                                  ↓
                          Cached jwk.Set
                                  ↓
                 Authenticate() → Check JWKS source → Use static or cached → Validate JWT
```

### Key Components to Modify

#### 1. File URL Utilities (New)
**Location**: `internal/jwtsupport/jwt.go`

```go
// isFileURL checks if the URL uses the file: scheme
func isFileURL(url string) bool {
    return strings.HasPrefix(url, "file:")
}

// readFileURL reads content from a file: URL
// Supports: file:/path, file:///path, file://localhost/path
func readFileURL(fileURL string) ([]byte, error) {
    // Remove "file:" prefix
    filePath := strings.TrimPrefix(fileURL, "file:")
    // Handle file:// or file:/// formats
    filePath = strings.TrimPrefix(filePath, "//")
    filePath = strings.TrimPrefix(filePath, "localhost/")
    
    // Ensure absolute path for Unix
    if !filepath.IsAbs(filePath) {
        filePath = "/" + filePath
    }
    
    slog.Debug("jwtsupport: reading file", "path", filePath)
    return os.ReadFile(filePath)
}

// sourceType returns a human-readable source type for logging
func sourceType(isFile bool) string {
    if isFile {
        return "file"
    }
    return "http"
}
```

#### 2. LoadWellKnowns() Enhancement
**Location**: `internal/jwtsupport/jwt.go` (existing method)

**Changes**:
- Add `encoding/json` import
- Check each URL with `isFileURL()`
- If file URL: Use `readFileURL()` → `json.Unmarshal()`
- If HTTP URL: Use existing `resthelp` logic
- Continue processing remaining URLs on error (existing behavior)

**Pseudocode**:
```go
for _, wellKnown := range j.wellKnowns {
    var wc wellKnownData
    
    if isFileURL(wellKnown) {
        data, err := readFileURL(wellKnown)
        if err != nil {
            slog.Error("failed to read well-known file", "url", wellKnown, "error", err)
            continue
        }
        if err = json.Unmarshal(data, &wc); err != nil {
            slog.Error("failed to parse well-known file", "url", wellKnown, "error", err)
            continue
        }
        slog.Info("loaded well-known from file", "url", wellKnown)
    } else {
        // Existing HTTP logic unchanged
        helper := resthelp.New()
        // ... existing code
    }
    
    j.wellknownList = append(j.wellknownList, &wc)
}
```

#### 3. LoadJWKS() Enhancement
**Location**: `internal/jwtsupport/jwt.go` (existing method)

**Changes**:
- Check each `wk.JwksURI` with `isFileURL()`
- If file URL: Use `readFileURL()` → `jwk.Parse()` → `wk.PostFetch()` → append to `j.JWKS`
- If HTTP URL: Use existing `jwk.Cache` logic
- Track correspondence between `wellknownList` and `JWKS` arrays

**Pseudocode**:
```go
for i, wk := range j.wellknownList {
    // Validate source type consistency (FR-006)
    wellKnownSource := j.wellKnowns[i]
    wellKnownIsFile := isFileURL(wellKnownSource)
    jwksIsFile := isFileURL(wk.JwksURI)
    
    if wellKnownIsFile != jwksIsFile {
        slog.Error("source type mismatch",
            "well-known", wellKnownSource,
            "well-known-type", sourceType(wellKnownIsFile),
            "jwks-uri", wk.JwksURI,
            "jwks-type", sourceType(jwksIsFile),
            "error", "well-known and jwks_uri must use matching source types (both file or both http)")
        continue
    }
    
    if isFileURL(wk.JwksURI) {
        data, err := readFileURL(wk.JwksURI)
        if err != nil {
            slog.Error("failed to read jwks file", "url", wk.JwksURI, "error", err)
            continue
        }
        
        set, err := jwk.Parse(data)
        if err != nil {
            slog.Error("failed to parse jwks file", "url", wk.JwksURI, "error", err)
            continue
        }
        
        // Apply algorithm postprocessing
        set, err = wk.PostFetch(wk.JwksURI, set)
        if err != nil {
            slog.Error("failed to post-process jwks", "url", wk.JwksURI, "error", err)
            continue
        }
        
        slog.Info("loaded jwks from file", "url", wk.JwksURI, "keys", set.Len())
        j.JWKS = append(j.JWKS, set)
    } else {
        // Existing cache-based logic unchanged
        err := j.cache.Register(wk.JwksURI, jwk.WithPostFetcher(wk))
        // ... existing code
    }
}
```

#### 4. Authenticate() Enhancement
**Location**: `internal/jwtsupport/jwt.go` (existing method)

**Changes**:
- Track which JWKS entries are file-based vs cached
- For file-based: Use direct array lookup
- For cached: Use `j.cache.Get()` (existing)
- Maintain index correspondence between `wellknownList` and `JWKS`

**Pseudocode**:
```go
for i, wc := range j.wellknownList {
    var ks jwk.Set
    var err error
    
    if isFileURL(wc.JwksURI) {
        // Use pre-loaded static JWKS
        if i < len(j.JWKS) {
            ks = j.JWKS[i]
        } else {
            slog.Warn("JWKS not found for file URL", "url", wc.JwksURI)
            lastError = types.ErrAuthenticationUnavailable
            continue
        }
    } else {
        // Existing cache lookup
        ks, err = j.cache.Get(context.Background(), wc.JwksURI)
        if err != nil {
            slog.Warn("failed to fetch JWKS", "url", wc.JwksURI, "error", err)
            lastError = err
            continue
        }
    }
    
    // Existing validation logic unchanged
    for _, aud := range j.audiences {
        // ... existing JWT validation
    }
}
```

### Data Structure Changes

**JWTSupport struct** (no changes needed):
```go
type JWTSupport struct {
    audiences     []string
    audienceKey   string
    authKind      string
    wellKnowns    []string          // Mixed file: and https: URLs
    wellknownList []*wellKnownData  // Parsed configs (file or HTTP)
    cache         *jwk.Cache        // Only used for HTTP sources
    JWKS          []jwk.Set         // Mixed: static (file) and cached (HTTP)
    permissive    bool
}
```

**Note**: The existing array structure naturally supports mixed sources. Index correspondence between `wellknownList[i]` and `JWKS[i]` is maintained by processing them in the same order.

### Configuration Examples

#### Environment Variables
```bash
# File-based (local development)
export WELLKNOWN_OIDC="file:///home/dev/.jwks/well-known.json"
export JWT_AUDIENCES="api://my-api"

# Mixed (production + testing) - Different issuers
export WELLKNOWN_OIDC="https://login.microsoftonline.com/tenant/.well-known/openid-configuration,file:///config/test-well-known.json"

# Multiple file sources
export WELLKNOWN_OIDC="file:///config/issuer1.json,file:///config/issuer2.json"

# INVALID: Cross-source within single issuer (file well-known → HTTP JWKS)
# This would be rejected with error:
# export WELLKNOWN_OIDC="file:///config/well-known-with-http-jwks.json"
# where well-known.json contains: {"jwks_uri": "https://example.com/jwks"}
```

#### Kubernetes Deployment
```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: rest-rego-jwks
data:
  well-known.json: |
    {
      "issuer": "https://our-issuer.example.com",
      "jwks_uri": "file:///config/jwks.json",
      "id_token_signing_alg_values_supported": ["RS256", "ES256"]
    }
  jwks.json: |
    {
      "keys": [
        {
          "kty": "RSA",
          "use": "sig",
          "kid": "2024-key-1",
          "n": "base64-encoded-modulus...",
          "e": "AQAB",
          "alg": "RS256"
        }
      ]
    }
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: rest-rego
spec:
  template:
    spec:
      containers:
      - name: rest-rego
        image: rest-rego:latest
        env:
        - name: WELLKNOWN_OIDC
          value: "file:///config/well-known.json"
        - name: JWT_AUDIENCES
          value: "api://my-backend"
        volumeMounts:
        - name: jwks-config
          mountPath: /config
          readOnly: true
      volumes:
      - name: jwks-config
        configMap:
          name: rest-rego-jwks
```

### Error Scenarios and Handling

| Scenario | Detection | Error Handling | Logging |
|----------|-----------|----------------|---------|
| Source type mismatch | Compare well-known vs jwks_uri source | Log error, skip issuer, continue | ERROR with both URLs and types |
| File not found | `os.ErrNotExist` | Log error, skip source, try next | ERROR with file path |
| Invalid JSON | `json.Unmarshal` error | Log error, skip source | ERROR with parse details |
| Invalid JWKS structure | `jwk.Parse` error | Log error, skip source | ERROR with validation message |
| Permission denied | `os.ErrPermission` | Log error, skip source | ERROR with file path and permissions hint |
| No valid sources loaded | `len(j.JWKS) == 0` | `os.Exit(1)` (existing behavior) | ERROR + exit |
| Mixed sources, one fails | Individual error checks | Continue processing others | ERROR for failed, INFO for successful |
| Empty file | `len(data) == 0` | Treat as parse error | ERROR "empty file" |
| Relative path in file: URL | `filepath.IsAbs()` check | Convert to absolute | DEBUG with resolved path |

### Testing Strategy

#### Unit Tests
**Location**: `internal/jwtsupport/jwt_test.go` (new tests)

1. **TestIsFileURL**: Validate URL detection
   - `file:/path` → true
   - `file:///path` → true
   - `https://example.com` → false
   - Empty string → false

2. **TestReadFileURL**: Validate file reading
   - Valid file → content returned
   - Non-existent file → error
   - Various URL formats work correctly
   - Cross-platform path handling

3. **TestLoadWellKnowns_File**: Well-known from file
   - Valid JSON file → parsed correctly
   - Invalid JSON → error logged, skipped
   - File not found → error logged, skipped

4. **TestLoadJWKS_File**: JWKS from file
   - Valid JWKS → keys loaded
   - PostFetch processing applied
   - Key count correct
   - Invalid JWKS → error handled

5. **TestAuthenticate_FileBasedKeys**: JWT validation
   - JWT signed with file-based key → accepted
   - Invalid JWT → rejected
   - Mixed file + HTTP sources → both work

6. **TestMixedSources**: Combined file and HTTP (across issuers)
   - Multiple issuers: one file-based, one HTTP-based
   - Both sources loaded successfully
   - Validation attempts both
   - Independent failure handling

7. **TestSourceTypeMismatch**: Reject cross-source configuration
   - File well-known with HTTP jwks_uri → error logged, issuer skipped
   - HTTP well-known with file jwks_uri → error logged, issuer skipped
   - Error message includes both URLs and source types
   - Other issuers continue loading

#### Integration Tests
**Location**: `tests/` (new .http and .k6 files)

1. **File-Based JWKS Test**:
   - Create test JWKS file
   - Generate test JWT with matching key
   - Send authenticated request
   - Verify policy evaluation succeeds

2. **Air-Gapped Simulation**:
   - Disable network access
   - Use only file-based config
   - Verify full authentication flow works

3. **Mixed Source Test**:
   - Configure both file and mock HTTP endpoint
   - Verify both issuers accepted
   - Test fallback behavior

#### Manual Testing Checklist
- [ ] Local development with file: URL works
- [ ] Kubernetes ConfigMap mounting works
- [ ] Error messages are clear and actionable
- [ ] Logs distinguish file vs HTTP sources
- [ ] Performance is acceptable (startup < 1s with large JWKS)
- [ ] HTTP-based loading still works (no regression)
- [ ] Documentation examples work as written

### Security Considerations

#### Threat Model
1. **Configuration Errors**: Mismatched source types within issuer
   - **Risk**: File well-known pointing to unreachable HTTP JWKS in air-gapped env
   - **Mitigation**: Enforce source type consistency (FR-006), fail issuer loading with clear error
   - **Defense**: Prevents partial configurations that could bypass security

2. **Path Traversal**: Malicious file: URLs attempting to read sensitive files
   - **Mitigation**: Validate paths, use filepath.Clean(), restrict to expected directories
   - **Note**: Configuration is typically controlled by deployment, not users

3. **Exposure of Sensitive Files**: Incorrect file permissions
   - **Mitigation**: Respect Unix file permissions, log permission errors clearly
   - **Best Practice**: Document recommended permissions (0600 for secrets)

4. **JWKS Tampering**: Unauthorized modification of JWKS files
   - **Mitigation**: Use Kubernetes Secrets (immutable), file integrity monitoring
   - **Best Practice**: Mount as read-only volumes

5. **Weak Key Distribution**: JWKS files in version control
   - **Mitigation**: Documentation warning against committing production keys
   - **Best Practice**: Use Kubernetes Secrets, not ConfigMaps, for sensitive keys

#### Security Best Practices Documentation
To be added to `docs/JWT.md`:
- Never commit production JWKS files to version control
- Use Kubernetes Secrets for sensitive keys, ConfigMaps for test keys
- Mount JWKS files as read-only volumes
- Set restrictive file permissions (0600) for JWKS files
- Use file-based JWKS for testing/air-gapped only; prefer HTTP(S) for production
- Rotate keys regularly even when using static files
- Audit logs for file access and loading

### Documentation Updates

#### 1. docs/JWT.md
Add new section: **"File-Based JWKS Loading"**
- Overview and use cases
- Configuration examples
- **Source type consistency requirement**: well-known and jwks_uri must match (both file or both HTTP)
- Kubernetes deployment pattern
- Security considerations
- Limitations (no auto-refresh)

#### 2. docs/CONFIGURATION.md
Update `WELLKNOWN_OIDC` documentation:
- Note that file: URLs are supported
- Provide file: URL syntax examples
- Document mixed source configuration (different issuers can use different sources)
- **Document source type consistency requirement** (within single issuer, both must be file or both must be HTTP)

#### 3. README.md
Add to "Features" section:
- "File-based JWKS loading for testing and air-gapped deployments"

Update "Quick Start" with alternative file-based example for local development

#### 4. examples/kubernetes/
Add new example: `examples/kubernetes/file-based-jwks/`
- `configmap-jwks.yaml`: JWKS ConfigMap
- `deployment.yaml`: Deployment with volume mount
- `README.md`: Step-by-step guide
- `test-jwks.json`: Sample JWKS for testing

### Deployment Strategy

#### Phase 1: Development & Testing (Week 1-2)
- Implement core functionality (file URL parsing, loading)
- Write unit tests
- Test locally with file-based configs
- Document implementation approach

#### Phase 2: Integration & Documentation (Week 3)
- Integration tests with Kubernetes ConfigMaps
- Update documentation (JWT.md, CONFIGURATION.md)
- Create example deployment
- Security review

#### Phase 3: Release (Week 4)
- Merge to main branch
- Release notes highlighting new capability
- Update Docker image
- Announce to users with migration guide

### Known Limitations

1. **No Cross-Source Within Issuer**: Well-known and jwks_uri must use matching source types
   - **Rationale**: Prevents configuration errors and maintains consistent security posture per issuer
   - **Enforcement**: Configuration validation during `LoadJWKS()` with clear error messages
   - **Workaround**: To mix sources, configure multiple separate issuers

2. **No Auto-Refresh**: File-based JWKS are loaded once at startup. Changes require pod restart.
   - **Rationale**: Complexity of file watching not justified; key rotation should trigger deployment
   - **Workaround**: Use rolling restart when updating ConfigMap/Secret

3. **No Key Rotation**: Static files don't support automatic key rotation
   - **Rationale**: File-based JWKS intended for testing and air-gapped scenarios with manual rotation
   - **Best Practice**: For production with rotation, prefer HTTP-based JWKS

4. **Path Handling Differences**: Windows vs Unix file: URL formats differ
   - **Mitigation**: Document platform-specific examples, normalize paths in code

5. **No Validation of JWKS Quality**: Files may contain weak or test keys
   - **Mitigation**: Document security best practices, warn against production use of test keys

### Success Metrics

- **Adoption**: Number of deployments using file-based JWKS (via logs/telemetry)
- **Testing**: CI/CD pipeline speed improvement (target: 30% faster without external HTTP calls)
- **Air-Gapped**: Successful production deployment in isolated environments
- **Error Rate**: < 1% of file loading attempts fail due to bugs (vs configuration issues)

## Implementation Phases

### MVP (Minimum Viable Product) - Sprint 1
**Goal**: Basic file loading capability for local development

**Scope**:
- File URL detection (`isFileURL()`)
- File reading utility (`readFileURL()`)
- `LoadWellKnowns()` file support
- `LoadJWKS()` file support
- `Authenticate()` static JWKS handling
- Basic error handling and logging
- Unit tests for core functions

**Success Criteria**:
- Can load well-known and JWKS from local files
- JWT validation works with file-based keys
- Unit tests pass
- Local development example works

### Enhancement - Sprint 2
**Goal**: Production-ready features and documentation

**Scope**:
- Mixed source support (file + HTTP)
- Enhanced error handling and diagnostics
- Kubernetes ConfigMap example
- Integration tests
- Complete documentation suite
- Security review and hardening

**Success Criteria**:
- Can mix file and HTTP sources
- Clear error messages for all failure scenarios
- Kubernetes example deployable
- Integration tests pass
- Documentation complete

### Future Enhancements (Post-MVP)
- **File Watching**: Hot-reload JWKS when files change (using fsnotify, similar to policy watching)
- **JWKS Validation**: Pre-validate JWKS structure and key quality at load time
- **Metrics**: Prometheus metrics for file loading success/failure
- **Cache Invalidation API**: Management endpoint to trigger JWKS reload
- **Multi-File Support**: Support loading multiple JWKS from a directory pattern

## Integration

### PRD Link
Related to [PRD.md](../PRD.md) sections:
- **Section 6.2**: Authentication Providers (extends JWT authentication)
- **Section 8.1**: Deployment Patterns (enables air-gapped deployments)
- **Section 9.7**: Testing (improves testability)

### README Impact
User-facing changes to [README.md](../../README.md):
- Update "Features" section with file-based loading capability
- Add "Quick Start" alternative for local development
- Update "Configuration" with file: URL examples

### Cross-Product Dependencies
None. This is an internal enhancement to rest-rego with no external dependencies.

## Quality Checklist

- [x] Clear problem statement and user stories
- [x] Technology stack aligns with Lindex standards (Go)
- [x] Kubernetes multi-instance support considerations included
- [x] Deployment targets specified (no changes to existing deployment model)
- [x] Authentication approach documented (enhances existing JWT support)
- [x] Observability requirements defined (structured logging with slog)
- [x] Security requirements specified (file handling, permissions, best practices)
- [x] Error handling and diagnostics detailed
- [x] Cross-references to PRD and README maintained
- [x] Backward compatibility preserved (HTTP loading unchanged)
- [x] Testing strategy comprehensive (unit + integration + manual)
- [x] Documentation updates planned
- [x] Implementation phases defined (MVP + enhancements)
- [x] Success metrics identified
