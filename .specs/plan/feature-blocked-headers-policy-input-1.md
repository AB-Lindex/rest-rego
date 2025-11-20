---
goal: Implement Blocked Headers Policy Input Feature
version: 1.0
date_created: 2025-11-20
last_updated: 2025-11-20
owner: AB-Lindex Team
status: 'Planned'
tags: [feature, security, policy, multi-layer, observability]
---

# Introduction

![Status: Planned](https://img.shields.io/badge/status-Planned-blue)

This implementation plan enables rest-rego policies to access `X-Restrego-*` headers that are removed during request cleanup. This feature supports multi-layer authorization architectures where upstream rest-rego instances set context headers that downstream instances need to validate and use in policy decisions.

**Key Use Cases:**
- Multi-layer deployments with upstream context propagation
- Security auditing of header spoofing attempts
- Layered authorization with trust verification
- Validated header forwarding after policy approval

## 1. Requirements & Constraints

### Requirements

- **REQ-001**: Capture `X-Restrego-*` headers before removal in `CleanupHandler`
- **REQ-002**: Store captured headers in request context for policy access
- **REQ-003**: Add `blocked_headers` field to `RequestInfo` struct for Rego policy input
- **REQ-004**: Implement `EXPOSE_BLOCKED_HEADERS` configuration flag with default `false`
- **REQ-005**: Headers must ALWAYS be removed from backend requests (security preserved)
- **REQ-006**: Policy can extract and forward validated header values as `X-Restrego-*` to backend
- **REQ-007**: Add Prometheus metrics for blocked headers tracking
- **REQ-008**: Maintain backward compatibility with existing policies

### Security Constraints

- **SEC-001**: Headers MUST be removed from backend requests regardless of feature flag state
- **SEC-002**: Blocked headers only accessible to policy, never automatically forwarded
- **SEC-003**: Default behavior must be secure (feature disabled by default)
- **SEC-004**: Policy validation required before any header forwarding to backend

### Performance Constraints

- **CON-001**: Zero performance overhead when feature disabled (`EXPOSE_BLOCKED_HEADERS=false`)
- **CON-002**: <1ms overhead when feature enabled (simple map copy operation)
- **CON-003**: Early return in cleanup handler if flag disabled

### Implementation Guidelines

- **GUD-001**: Use canonical header names via `http.CanonicalHeaderKey()`
- **GUD-002**: Follow existing context key pattern (`ctxKey` int type)
- **GUD-003**: Use snake_case `blocked_headers` for Rego consistency
- **GUD-004**: Maintain separation of concerns (cleanup removes, policy validates, backend receives approved)

### Architecture Patterns

- **PAT-001**: Context propagation pattern for cross-middleware data sharing
- **PAT-002**: Feature flag pattern for zero-cost abstraction when disabled
- **PAT-003**: Policy-as-gatekeeper pattern for header validation and forwarding

## 1.1. Repository Context

- **Repository Type**: Single-Product
- **PRD Location**: `/.specs/PRD.md`
- **Related Features**: `/.specs/features/multi-layer-header-passthrough.md`
- **Technology Stack**: Go 1.25+, OPA Rego, Prometheus
- **Cross-Product Dependencies**: None

## 2. Implementation Steps

### Implementation Phase 1: Configuration and Type Definitions

**GOAL-001**: Add configuration flag and update type definitions to support blocked headers feature

- **TASK-001**: Add `ExposeBlockedHeaders` field to `Fields` struct in `internal/config/config.go` `[✅ Completed]`
  - Files: `internal/config/config.go`
  - Add field: `ExposeBlockedHeaders bool` with arg tags `--expose-blocked-headers,env:EXPOSE_BLOCKED_HEADERS`
  - Set default: `false`
  - Add help text: `"expose X-Restrego-* headers to policy as blocked_headers (security: headers still removed from backend)"`
  - Location: After line 26 (after `PermissiveAuth` field)

- **TASK-002**: Add `BlockedHeaders` field to `RequestInfo` struct in `internal/types/request.go` `[✅ Completed]`
  - Files: `internal/types/request.go`
  - Add field to `RequestInfo` struct: `BlockedHeaders map[string]interface{} json:"blocked_headers,omitempty"`
  - Location: After line 23 (after `Headers` field)
  - Use `omitempty` tag to maintain backward compatibility

- **TASK-003**: Define context key for blocked headers storage in `internal/router/cleanup.go` `[✅ Completed]`
  - Files: `internal/router/cleanup.go`
  - Add `type ctxKey int` type definition at package level
  - Add constant `const ctxBlockedHeadersKey ctxKey = 1`
  - Location: After package and import statements

### Implementation Phase 2: Header Capture and Context Storage

**GOAL-002**: Implement header capture logic in cleanup handler with context storage

- **TASK-004**: Enhance `CleanupHandler` to capture blocked headers when feature enabled `[✅ Completed]`
  - Files: `internal/router/cleanup.go`
  - Prerequisites: TASK-001, TASK-003 completed
  - Implementation steps:
    1. Add conditional map allocation based on `proxy.config.ExposeBlockedHeaders`
    2. Capture header values during iteration (single value as string, multiple as slice)
    3. Store in context using `context.WithValue()` before header removal
    4. Update logging: debug level when captured, warn level when not exposed
  - Estimated effort: 2-3 hours

- **TASK-005**: Add `GetBlockedHeaders()` helper function in `internal/router/cleanup.go` `[✅ Completed]`
  - Files: `internal/router/cleanup.go`
  - Dependencies: TASK-004
  - Function signature: `func GetBlockedHeaders(r *http.Request) map[string]interface{}`
  - Returns: blocked headers from context or `nil` if not present
  - Location: After `CleanupHandler` function
  - Estimated effort: 30 minutes

### Implementation Phase 3: Policy Input Integration

**GOAL-003**: Integrate blocked headers into policy input structure `[✅ Completed]`

- **TASK-006**: Update `NewInfo()` to retrieve blocked headers from request context `[✅ Completed]`
  - Files: `internal/types/request.go`
  - Dependencies: TASK-002, TASK-005
  - Implementation:
    1. Moved `GetBlockedHeaders()` function to `types` package to avoid import cycle
    2. Exported `CtxBlockedHeadersKey` constant (value 1) in `types` package
    3. After creating base `Info` struct, call `GetBlockedHeaders(r)`
    4. If result has length > 0, assign to `i.Request.BlockedHeaders`
    5. Updated `cleanup.go` to use `types.CtxBlockedHeadersKey` instead of local constant
    6. Location: In `NewInfo()` function, after populating `RequestInfo.Auth` field
  - Note: No changes needed to `WrapHandler` - it continues calling `NewInfo()` as before
  - Completed: Implementation successfully moved context key and helper function to types package to avoid circular dependency

### Implementation Phase 4: Metrics and Observability

**GOAL-004**: Add comprehensive metrics and logging for blocked headers feature `[✅ Completed]`

- **TASK-009**: Add blocked headers metrics to `internal/metrics/metrics.go` `[✅ Completed]`
  - Files: `internal/metrics/metrics.go`
  - Prerequisites: Configuration flag available
  - Add to metrics struct (around line 18-26):
    1. `blockedHeadersExposed prometheus.Gauge` - Feature state (0=disabled, 1=enabled)
    2. `blockedHeadersCaptured prometheus.Counter` - Total headers captured
    3. `requestsWithBlockedHeaders prometheus.Counter` - Total requests with blocked headers
  - Register in `New()` function using `promauto.With(metrics.reg).NewGauge/NewCounter`
  - Estimated effort: 2 hours

- **TASK-010**: Export metrics for external access `[✅ Completed]`
  - Files: `internal/metrics/metrics.go`
  - Dependencies: TASK-009
  - Create exported functions:
    1. `SetBlockedHeadersExposed(enabled bool)` - Set gauge to 0 or 1
    2. `IncrementBlockedHeadersCaptured(count int)` - Increment counter
    3. `IncrementRequestsWithBlockedHeaders()` - Increment counter
  - Location: After `Wrap()` function
  - Estimated effort: 1 hour

- **TASK-011**: Instrument `CleanupHandler` with metrics calls `[✅ Completed]`
  - Files: `internal/router/cleanup.go`
  - Dependencies: TASK-010
  - Add metrics calls:
    1. `metrics.IncrementBlockedHeadersCaptured(1)` when header captured
    2. `metrics.IncrementRequestsWithBlockedHeaders()` when `len(blocked) > 0`
  - Estimated effort: 30 minutes

- **TASK-012**: Set initial metric state in application startup `[✅ Completed]`
  - Files: `internal/application/mgmt.go`
  - Dependencies: TASK-010
  - Add `metrics.SetBlockedHeadersExposed(config.ExposeBlockedHeaders)` during initialization
  - Add info-level log: "blocked headers feature enabled/disabled"
  - Estimated effort: 30 minutes

### Implementation Phase 5: Testing

**GOAL-005**: Comprehensive testing coverage for all scenarios

- **TASK-013**: Update existing cleanup tests in `internal/router/cleanup_test.go` `[✅ Completed]`
  - Files: `internal/router/cleanup_test.go`
  - Dependencies: TASK-004, TASK-005
  - Add test cases:
    1. Feature disabled (default): headers removed, NOT in context
    2. Feature enabled: headers removed AND captured in context
    3. No blocked headers: empty/nil context value
    4. Multiple blocked headers: all captured correctly
    5. Single vs multiple header values: proper type handling
  - Extend existing test table with `exposeEnabled` and `expectInContext` fields
  - Estimated effort: 3-4 hours

- **TASK-014**: Create policy input integration tests `[✅ Completed]`
  - Files: `internal/types/request_test.go` (new test cases)
  - Dependencies: TASK-006
  - Test scenarios:
    1. `NewInfo()` with no blocked headers in context
    2. `NewInfo()` with empty map in context
    3. `NewInfo()` with populated blocked headers in context
    4. Verify JSON marshaling includes `blocked_headers` field
    5. Verify `omitempty` excludes field when empty
  - Estimated effort: 2-3 hours

- **TASK-015**: Create end-to-end integration test policy `[📋 Planned]`
  - Files: `policies/test-blocked-headers.rego` (new file)
  - Testing: Rego policy that validates blocked headers usage
  - Policy content:
    1. Test accessing `input.request.blocked_headers`
    2. Test validation logic (e.g., only allow if no spoofing)
    3. Test extraction and forwarding pattern
  - Estimated effort: 2 hours

- **TASK-016**: Create HTTP test scenarios in `tests/blocked-headers.http` `[📋 Planned]`
  - Files: `tests/blocked-headers.http` (new file)
  - Dependencies: TASK-015
  - Test scenarios:
    1. Request with `X-Restrego-*` header, feature disabled
    2. Request with `X-Restrego-*` header, feature enabled
    3. Verify backend doesn't receive blocked headers
    4. Verify policy can access headers when enabled
  - Estimated effort: 1-2 hours

- **TASK-017**: Add performance benchmarks `[✅ Completed]`
  - Files: `internal/router/cleanup_benchmark_test.go` (new file)
  - Testing: Benchmark feature disabled vs enabled overhead
  - Benchmarks:
    1. `BenchmarkCleanupHandler_Disabled` - baseline performance (43580 ns/op)
    2. `BenchmarkCleanupHandler_Enabled_NoBlockedHeaders` - enabled but no headers (1088 ns/op)
    3. `BenchmarkCleanupHandler_Enabled_WithBlockedHeaders` - enabled with 3 headers (2034 ns/op)
    4. `BenchmarkCleanupHandler_Enabled_ManyBlockedHeaders` - enabled with 10 headers (4585 ns/op)
    5. `BenchmarkCleanupHandler_Enabled_MultiValueHeaders` - enabled with multi-value headers (1772 ns/op)
  - Results: All scenarios well under 1ms target (~1-4µs overhead when enabled)
  - Completed: 2025-11-20

### Implementation Phase 6: Documentation

**GOAL-006**: Comprehensive documentation for feature usage and patterns

- **TASK-018**: Update README.md with blocked headers section `[✅ Completed]`
  - Files: `README.md`
  - Dependencies: All implementation tasks completed
  - Add section: "🔒 Blocked Headers for Policy Evaluation"
  - Content:
    1. Feature overview and use cases ✅
    2. Configuration example (`EXPOSE_BLOCKED_HEADERS=true`) ✅
    3. Policy access examples (validation, spoofing detection, forwarding) ✅
    4. Security notes and guarantees ✅
    5. Metrics reference ✅
  - Location: After authentication sections, before deployment ✅
  - Completed: 2025-11-20

- **TASK-019**: Create detailed policy development guide `[📋 Planned]`
  - Files: `docs/POLICY.md` (new file)
  - Content:
    1. Policy input structure reference
    2. Blocked headers usage patterns
    3. Multi-layer architecture patterns
    4. Validation and forwarding examples
    5. Security best practices
    6. Common pitfalls and troubleshooting
  - Estimated effort: 3-4 hours

- **TASK-020**: Create example policies directory `[📋 Planned]`
  - Files: `examples/policies/blocked-headers-*.rego` (multiple examples)
  - Example policies:
    1. `blocked-headers-validation.rego` - Upstream header validation
    2. `blocked-headers-spoofing-detection.rego` - Security auditing
    3. `blocked-headers-forwarding.rego` - Validated header forwarding
    4. `blocked-headers-multi-layer.rego` - Complete multi-layer example
  - Estimated effort: 2-3 hours

- **TASK-021**: Update feature specification with implementation notes `[📋 Planned]`
  - Files: `.specs/features/multi-layer-header-passthrough.md`
  - Add "Implementation Notes" section:
    1. Actual implementation decisions made
    2. Any deviations from original spec
    3. Performance test results
    4. Known limitations or future improvements
  - Estimated effort: 1 hour

- **TASK-022**: Update PRD to reference new feature `[📋 Planned]`
  - Files: `.specs/PRD.md`
  - Update `related_features` array to include `multi-layer-header-passthrough`
  - Add brief mention in relevant requirements sections
  - Estimated effort: 30 minutes

## 3. Alternatives

### Alternative Approaches Considered

- **ALT-001**: Create separate `NewInfoWithBlocked()` function that accepts blocked headers as parameter
  - **Why rejected**: Adds unnecessary API surface; `NewInfo()` already has access to request context and can retrieve headers directly

- **ALT-002**: Always capture blocked headers regardless of flag, only expose to policy when enabled
  - **Why rejected**: Violates zero-cost abstraction principle; still incurs allocation overhead when not needed

- **ALT-003**: Use separate middleware for blocked header capture instead of enhancing `CleanupHandler`
  - **Why rejected**: Increases middleware chain complexity; cleanup handler already has access to headers before removal

- **ALT-004**: Add blocked headers to existing `Headers` field with special prefix (e.g., `__blocked__`)
  - **Why rejected**: Pollutes main headers namespace; less explicit; harder for policy developers to understand

- **ALT-005**: Use `blockedHeaders` (camelCase) instead of `blocked_headers` (snake_case) for JSON field
  - **Why rejected**: Inconsistent with Rego naming conventions; other Rego inputs use snake_case

## 4. Dependencies

### External Dependencies

- **DEP-001**: OPA Rego SDK `github.com/open-policy-agent/opa/v1/rego` - Already present, no version change needed
- **DEP-002**: Prometheus client `github.com/prometheus/client_golang` - Already present, no version change needed
- **DEP-003**: Go 1.25+ - Already required by project

### Internal Dependencies

- **DEP-004**: `internal/config` - Configuration flag addition
- **DEP-005**: `internal/types` - Request info type modification, imports `internal/router` for `GetBlockedHeaders()`
- **DEP-006**: `internal/router` - Cleanup handler changes, exports `GetBlockedHeaders()` function
- **DEP-007**: `internal/metrics` - New metrics addition
- **DEP-008**: Existing middleware chain order must be preserved: `CleanupHandler` → `WrapHandler` → `metrics.Wrap` → `authHandler` → `policyHandler`

### Cross-Component Dependencies

- **DEP-009**: Context key values must not conflict with existing `ctxInfoKey` (value 0) in `types/request.go`
- **DEP-010**: Header removal must happen before policy evaluation (existing guarantee)
- **DEP-011**: Policy result variables (snake_case) to `X-Restrego-*` header conversion (existing mechanism)

## 5. Files

### Files to Modify

- **FILE-001**: `internal/config/config.go`
  - Description: Add `ExposeBlockedHeaders` configuration field
  - Changes: Add field to `Fields` struct with appropriate tags

- **FILE-002**: `internal/types/request.go`
  - Description: Add `BlockedHeaders` field to `RequestInfo` struct and update `NewInfo()` to retrieve from context
  - Changes: Extend struct definition, enhance `NewInfo()` function to call `router.GetBlockedHeaders()`

- **FILE-003**: `internal/router/cleanup.go`
  - Description: Implement header capture logic with context storage and export `GetBlockedHeaders()` helper
  - Changes: Add context key, enhance `CleanupHandler`, add exported `GetBlockedHeaders()` function

- **FILE-004**: `internal/metrics/metrics.go`
  - Description: Add blocked headers metrics
  - Changes: Add metric definitions, registration, and exported functions

- **FILE-005**: `internal/application/app.go` or `cmd/main.go`
  - Description: Initialize metrics state on startup
  - Changes: Set initial gauge value based on config

- **FILE-006**: `README.md`
  - Description: Add blocked headers feature documentation
  - Changes: Add new section with usage examples and security notes

- **FILE-007**: `.specs/PRD.md`
  - Description: Reference new feature in related features
  - Changes: Update `related_features` array

### Files to Create

- **FILE-008**: `internal/router/cleanup_benchmark_test.go`
  - Description: Performance benchmarks for feature overhead measurement

- **FILE-009**: `docs/POLICY.md`
  - Description: Comprehensive policy development guide with blocked headers patterns

- **FILE-010**: `policies/test-blocked-headers.rego`
  - Description: Test policy for integration testing

- **FILE-011**: `tests/blocked-headers.http`
  - Description: HTTP test scenarios for manual/automated testing

- **FILE-012**: `examples/policies/blocked-headers-validation.rego`
  - Description: Example policy showing upstream header validation

- **FILE-013**: `examples/policies/blocked-headers-spoofing-detection.rego`
  - Description: Example policy showing spoofing detection

- **FILE-014**: `examples/policies/blocked-headers-forwarding.rego`
  - Description: Example policy showing validated header forwarding

- **FILE-015**: `examples/policies/blocked-headers-multi-layer.rego`
  - Description: Complete multi-layer authorization example

### Files to Update (Tests)

- **FILE-016**: `internal/router/cleanup_test.go`
  - Description: Extend existing tests with blocked headers scenarios
  - Changes: Add test cases for feature enabled/disabled, context storage validation

- **FILE-017**: `internal/types/request_test.go`
  - Description: Add tests for `NewInfo()` blocked headers retrieval from context
  - Changes: New test cases for blocked headers handling with context setup

## 6. Testing

### Unit Tests

- **TEST-001**: `TestCleanupHandler_BlockedHeaders_FeatureDisabled`
  - Verify: Headers removed, NOT stored in context, no metrics incremented
  - Files: `internal/router/cleanup_test.go`

- **TEST-002**: `TestCleanupHandler_BlockedHeaders_FeatureEnabled`
  - Verify: Headers removed AND captured in context, metrics incremented
  - Files: `internal/router/cleanup_test.go`

- **TEST-003**: `TestCleanupHandler_BlockedHeaders_NoBlockedHeaders`
  - Verify: No context value created when no blocked headers present
  - Files: `internal/router/cleanup_test.go`

- **TEST-004**: `TestCleanupHandler_BlockedHeaders_MultipleValues`
  - Verify: Multiple header values stored as slice, single as string
  - Files: `internal/router/cleanup_test.go`

- **TEST-005**: `TestGetBlockedHeaders_ContextPresent`
  - Verify: Function retrieves blocked headers from context correctly
  - Files: `internal/router/cleanup_test.go`

- **TEST-006**: `TestGetBlockedHeaders_ContextMissing`
  - Verify: Function returns nil when context value not present
  - Files: `internal/router/cleanup_test.go`

- **TEST-007**: `TestNewInfo_BlockedHeaders_NotInContext`
  - Verify: `BlockedHeaders` field not set when context value missing
  - Files: `internal/types/request_test.go`

- **TEST-008**: `TestNewInfo_BlockedHeaders_EmptyInContext`
  - Verify: `BlockedHeaders` field not set when context contains empty map
  - Files: `internal/types/request_test.go`

- **TEST-009**: `TestNewInfo_BlockedHeaders_PopulatedInContext`
  - Verify: `BlockedHeaders` field correctly populated from context
  - Files: `internal/types/request_test.go`

- **TEST-010**: `TestRequestInfo_JSONMarshal_WithBlockedHeaders`
  - Verify: JSON output includes `blocked_headers` field with correct structure
  - Files: `internal/types/request_test.go`

- **TEST-011**: `TestRequestInfo_JSONMarshal_WithoutBlockedHeaders`
  - Verify: JSON output omits `blocked_headers` field when empty (omitempty)
  - Files: `internal/types/request_test.go`

### Integration Tests

- **TEST-012**: End-to-end policy evaluation with blocked headers
  - Verify: Policy receives blocked headers in input, can make decisions based on them
  - Files: `tests/blocked-headers.http`, `policies/test-blocked-headers.rego`

- **TEST-013**: Backend request verification
  - Verify: Backend NEVER receives blocked headers regardless of feature state
  - Files: `tests/blocked-headers.http`

- **TEST-014**: Metrics collection verification
  - Verify: Metrics correctly track captured headers and requests
  - Files: Integration test or manual verification via `/metrics` endpoint

### Performance Tests

- **TEST-015**: Benchmark feature disabled overhead
  - Target: Zero overhead compared to baseline
  - Files: `internal/router/cleanup_benchmark_test.go`

- **TEST-016**: Benchmark feature enabled overhead (no blocked headers)
  - Target: <100µs overhead for conditional check
  - Files: `internal/router/cleanup_benchmark_test.go`

- **TEST-017**: Benchmark feature enabled overhead (with blocked headers)
  - Target: <1ms overhead for capture and context storage
  - Files: `internal/router/cleanup_benchmark_test.go`

### Security Tests

- **TEST-018**: Verify headers always removed from backend request
  - Security guarantee test across all scenarios
  - Files: `tests/blocked-headers.http`

- **TEST-019**: Verify headers only accessible to policy, not automatically forwarded
  - Security guarantee test - backend must not receive original headers
  - Files: `tests/blocked-headers.http`

## 7. Risks & Assumptions

### Risks

- **RISK-001**: Performance impact on high-throughput deployments when feature enabled
  - Mitigation: Thorough benchmarking, feature disabled by default, early return optimization
  - Severity: Low (feature is opt-in, <1ms target is conservative)

- **RISK-002**: Memory consumption with many blocked headers or large header values
  - Mitigation: Context values are request-scoped (garbage collected after request), consider size limits in future
  - Severity: Low (headers already in memory, just copying references)

- **RISK-003**: Breaking changes if policy developers rely on headers being completely invisible
  - Mitigation: Feature disabled by default, requires explicit opt-in, backward compatible
  - Severity: Very Low (new field is additive, uses omitempty)

- **RISK-004**: Complexity in understanding multi-layer authorization flows
  - Mitigation: Comprehensive documentation, clear examples, policy development guide
  - Severity: Medium (requires education and clear patterns)

- **RISK-005**: Potential for policy developers to accidentally forward sensitive header data
  - Mitigation: Documentation emphasizes validation requirement, examples show best practices
  - Severity: Medium (mitigated by documentation and examples)

### Assumptions

- **ASSUMPTION-001**: Multi-layer deployments will use this feature primarily with Azure or JWT authentication
  - Validation: Feature works with all authentication providers

- **ASSUMPTION-002**: Blocked header names will follow standard HTTP header naming (ASCII, no special chars)
  - Validation: Standard Go HTTP header handling applies

- **ASSUMPTION-003**: Number of blocked headers per request will be small (<10)
  - Validation: Map allocation and copy is efficient for small maps

- **ASSUMPTION-004**: Policy developers understand trust boundaries in multi-layer architectures
  - Validation: Documentation and examples make trust model explicit

- **ASSUMPTION-005**: Existing middleware order (CleanupHandler first) is correct and won't change
  - Validation: Architecture review confirms this is fundamental design

- **ASSUMPTION-006**: Context values are request-scoped and properly cleaned up by Go runtime
  - Validation: Standard Go context behavior, no custom cleanup needed

## 8. Related Specifications / Further Reading

### Internal Documentation

- [Feature Specification: Blocked Headers Policy Input](/.specs/features/multi-layer-header-passthrough.md)
- [Product Requirements Document (PRD)](/.specs/PRD.md)
- [Security Documentation](/.specs/security/) - General security architecture

### External References

- [Open Policy Agent Documentation](https://www.openpolicyagent.org/docs/latest/)
- [OPA Rego Policy Reference](https://www.openpolicyagent.org/docs/latest/policy-reference/)
- [Go Context Package](https://pkg.go.dev/context)
- [Prometheus Metrics Best Practices](https://prometheus.io/docs/practices/naming/)
- [HTTP Header Security Best Practices](https://owasp.org/www-project-web-security-testing-guide/latest/4-Web_Application_Security_Testing/02-Configuration_and_Deployment_Management_Testing/06-Test_HTTP_Methods)

### Related Patterns

- **Sidecar Pattern**: Authorization as a separate concern
- **Policy as Code**: Declarative authorization rules
- **Context Propagation**: Passing data through middleware chains
- **Feature Flags**: Zero-cost abstraction for optional features
- **Defense in Depth**: Multiple layers of authorization
