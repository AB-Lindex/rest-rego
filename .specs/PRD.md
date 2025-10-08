---
type: "prd"
project: "rest-rego"
version: "2.1"
status: "active"
last_updated: "2025-10-08"
repository_type: "single-product"
stakeholders:
  product_owner: "AB-Lindex Team"
  tech_lead: "AB-Lindex Team"
target_audience: ["developers", "devops", "security-team", "platform-engineers"]
complexity: "complex"
deployment: "multi-instance"
observability: "required"
technology_stack: ["go", "opa-rego", "prometheus", "docker", "kubernetes"]
azure_services: ["azure-ad", "microsoft-graph", "container-registry", "kubernetes-service"]
related_features: []
---

# PRD: rest-rego - High-Performance Authorization Sidecar for REST APIs

## 1. Product Overview

### 1.1 Document Title and Version
* **PRD**: rest-rego - High-Performance Authorization Sidecar for REST APIs
* **Version**: 2.1
* **Last Updated**: 2025-10-08
* **Status**: Active Production Project
* **Previous Version**: 2.0 (2025-10-08)
* **Major Changes**: Added WSO2 API Manager integration documentation, custom JWT header and claim configurations (AUTH_HEADER, AUTH_KIND, JWT_AUDIENCE_KEY)

### 1.2 Product Summary

rest-rego is a high-performance authorization sidecar that protects REST APIs using Open Policy Agent (OPA) Rego policies. Built in Go for minimal latency (<5ms overhead) and high throughput (5000+ req/s per instance), it acts as a reverse proxy with policy-based access control, supporting both JWT (OIDC) and Azure Graph authentication.

**Value Proposition**: Focus on business logic, not authorization boilerplate. Deploy rest-rego as a sidecar and let it handle authentication and authorization so your application doesn't have to.

**Core Problem Solved**: Authorization is hard to get right. Developers typically repeat authentication and authorization logic across every service, leading to inconsistent security, scattered code, slow iteration, and maintenance nightmares. rest-rego centralizes this as a specialized sidecar with policy-as-code.

**Technology Stack**: Go 1.25+, OPA Rego, Prometheus, Docker, Kubernetes

**Deployment Model**: Multi-instance sidecar (designed for concurrent operation across multiple instances)

**Core Architecture**:
- **Reverse proxy** using Go's `httputil.ReverseProxy`
- **OPA integration** via `github.com/open-policy-agent/opa/v1/rego`
- **Pluggable authentication** providers (`types.AuthProvider` interface)
- **Hot-reload** of Rego policies using `fsnotify`
- **Prometheus metrics** for comprehensive observability
- **Middleware chain**: CleanupHandler → WrapHandler → Metrics → Auth → Policy

**Key Differentiators**:
- ✅ **Zero-trust authorization** with deny-by-default security model
- ✅ **Policy-as-code** using OPA Rego - version controlled, testable, auditable
- ✅ **Hot policy reload** - update authorization rules in <1 second without restart
- ✅ **High performance** - <5ms latency overhead, 5000+ req/s throughput per instance
- ✅ **Dual authentication** - JWT (recommended) or Azure Graph validation
- ✅ **Zero code changes** - deploy as sidecar, your app stays the same
- ✅ **Production-grade** - Prometheus metrics, health checks, structured logging
- ✅ **Lightweight** - 50-100MB memory footprint vs 200-500MB+ for full API gateways

**Performance vs Alternatives**:
| Solution          | Latency Overhead       | Memory Usage | Development Time     |
|-------------------|------------------------|--------------|----------------------|
| rest-rego         | <5ms                   | 50-100MB     | 30 min deployment    |
| DIY Auth Code     | Varies (likely higher) | N/A          | 2-5 days per service |
| Heavy API Gateway | 10-50ms                | 200-500MB+   | 1-2 weeks setup      |

### 1.3 Documentation Ecosystem

* **README.md**: User-facing quick start guide, configuration reference, deployment patterns
* **WHY.md**: Detailed value proposition, comparison with DIY and API gateway alternatives, real-world scenarios
* **SECURITY.md**: Security policy, best practices, vulnerability reporting
* **docs/**: 
  - **JWT.md**: JWT authentication configuration with OIDC providers, custom header and claim configurations
  - **AZURE.md**: Azure Graph authentication setup and caching
  - **WSO2.md**: WSO2 API Manager integration guide with non-standard JWT handling
* **examples/**: 
  - **kubernetes/**: Complete Kubernetes deployment manifests (sidecar pattern)
  - Sample policies demonstrating common patterns
* **tests/**: 
  - **.http files**: Manual testing with VS Code REST Client
  - **.k6 files**: Load testing scenarios
* **policies/**: Default policy examples (request.rego, roles.rego)
* **/.specs/PRD.md** (this document): Comprehensive product requirements covering architecture, deployment patterns, security, performance, and operational excellence

## 2. Goals

### 2.1 Business Goals

* **Accelerate Development**: Enable teams to ship features 70% faster by eliminating authorization boilerplate from application code
* **Zero-Trust Security**: Provide enterprise-grade authorization for REST APIs with deny-by-default security model that fails closed, not open
* **Reduce Security Incidents**: Eliminate authorization bypass vulnerabilities through centralized, auditable policy enforcement
* **Developer Autonomy**: Enable teams to maintain their own authorization policies alongside their code in Git
* **Operational Simplicity**: Minimal infrastructure requirements with straightforward sidecar deployment pattern
* **Compliance Support**: Enable audit trails and policy version control for regulatory requirements (SOC2, ISO27001, GDPR)
* **Cost Efficiency**: Lightweight sidecar approach reduces infrastructure costs 50-80% compared to heavy API gateways
* **Consistency at Scale**: Ensure identical authorization behavior across dozens of microservices
* **Rapid Policy Iteration**: Deploy policy changes in <1 second vs 10+ minute code deployment cycles

### 2.2 User Goals

* **Platform Engineers**: Deploy and manage authorization consistently across multiple services with minimal operational overhead
* **Backend Developers**: Write and test authorization policies without changing application code, iterate on policies rapidly
* **Security Teams**: Enforce consistent security policies across all services, audit authorization decisions in real-time
* **DevOps Engineers**: Monitor authorization layer health and performance with Prometheus metrics alongside application metrics
* **API Consumers**: Experience transparent authorization without application code changes or performance degradation
* **Compliance Officers**: Access complete audit trail of authorization policy changes and access decisions

### 2.3 Non-Goals

* **User-facing authentication UI**: No login pages or user interface (B2B/M2M authentication only, not B2C)
* **Rate limiting**: Not a rate limiter (use nginx, cloud services, or dedicated rate limiting solutions)
* **Request/response transformation**: No API transformation beyond metrics URL labeling (not an ETL tool)
* **Service mesh replacement**: Not a full service mesh (focused on authorization only, not traffic management)
* **Database authorization**: Not for database access control (API-level authorization only)
* **Non-REST protocols**: HTTP/REST only (no gRPC, WebSocket, MQTT, or other protocols)
* **Backend URL rewriting**: `url` result does NOT change backend request paths (see REQ-015 for actual purpose)
* **Identity provider**: Not an identity provider (integrates with existing OIDC providers)

## 3. User Personas

### 3.1 Key User Types

* **Platform Engineers**: Deploy and configure rest-rego across multiple services and environments
* **Backend Developers**: Define authorization policies for their APIs
* **Security Engineers**: Audit policies and authorization decisions
* **DevOps Engineers**: Monitor and troubleshoot authorization layer
* **API Consumers**: Client applications and services making API calls

### 3.2 Basic Persona Details

* **Platform Engineer (Primary)**: Deploys rest-rego as sidecar or gateway, configures authentication providers, manages Kubernetes deployments, monitors system health
* **Backend Developer (Primary)**: Writes Rego policies, tests authorization logic, deploys policy changes, integrates with CI/CD pipelines
* **Security Engineer (Secondary)**: Reviews policies for security compliance, analyzes authorization decisions, defines security requirements
* **DevOps Engineer (Secondary)**: Monitors metrics, sets up alerting, troubleshoots authorization failures, manages policy deployments
* **API Consumer (Indirect)**: Obtains JWT tokens from identity provider, makes authenticated API requests

### 3.3 Role-Based Access

* **Platform Administrator**: Full configuration access, deployment management, authentication provider setup
* **Developer**: Policy write access for owned services, read access to shared policies, metrics access
* **Security Auditor**: Read-only access to all policies and authorization logs
* **Operator**: Metrics and health check access, incident response capabilities

## 4. Functional Requirements

### REQ-001: Reverse Proxy with Policy Enforcement (Priority: High)
* **Type**: Functional
* **Complexity**: Complex
* **Technology**: Go httputil.ReverseProxy + OPA Rego
* **Multi-Instance Considerations**: Stateless design, no shared state between instances
* **Observability**: Request latency (p50/p95/p99), authorization decisions (allow/deny), proxy performance, backend connection health
* **Dependencies**: REQ-002 or REQ-003 (Authentication), REQ-004 (Policy Evaluation)
* **Description**: Act as a reverse proxy that intercepts all incoming HTTP requests, evaluates authorization policies, and forwards allowed requests to backend services
* **Acceptance Criteria**:
  - AC-001: Forward all HTTP methods (GET, POST, PUT, PATCH, DELETE, HEAD, OPTIONS) to backend
  - AC-002: Preserve request headers, body, and query parameters (no modification)
  - AC-003: Add <5ms latency overhead (p99) for policy evaluation
  - AC-004: Handle 5000+ requests/second per instance with 2 CPU cores
  - AC-005: Return 403 Forbidden for policy-denied requests with clear error message
  - AC-006: Return 500 Internal Server Error for policy evaluation failures (fail closed)
  - AC-007: Return 502 Bad Gateway for backend connection failures
  - AC-008: Support configurable backend scheme (http/https), host, and port
  - AC-009: Maintain correlation IDs throughout request lifecycle for tracing

### REQ-002: JWT Authentication (Priority: High)
* **Type**: Functional
* **Complexity**: Standard
* **Technology**: Go lestrrat-go/jwx/v2 library + OIDC discovery
* **Multi-Instance Considerations**: Each instance caches JWK keys locally, refreshed from OIDC well-known endpoint
* **Observability**: JWT validation success/failure rates, JWK cache hits/misses, token expiration tracking, provider-specific metrics
* **Dependencies**: None
* **Description**: Authenticate requests using standard JWT tokens with OIDC well-known endpoint discovery for automatic cryptographic verification. Works with any standards-compliant OIDC provider (cloud-based or on-premises), including non-standard implementations like WSO2 API Manager.
* **Acceptance Criteria**:
  - AC-001: Fetch OIDC configuration from well-known endpoint(s) on startup with retry logic
  - AC-002: Cache JWK public keys with automatic refresh from OIDC endpoint (no manual key rotation needed)
  - AC-003: Verify JWT signature using cached public keys from any configured provider
  - AC-004: Validate JWT expiration (`exp`), audience (`aud`), and issuer (`iss`) claims
  - AC-005: Extract all JWT claims and make available to policies as `input.jwt`
  - AC-006: Support multiple OIDC providers via array of well-known URLs
  - AC-007: Handle JWT verification with <1ms overhead (with cached keys)
  - AC-008: **Require** audience claim validation when JWT mode enabled (fail fast on startup if not configured)
  - AC-009: Support custom JWT audience claim key via `JWT_AUDIENCE_KEY` (default: "aud") for non-standard providers
  - AC-010: Support custom JWT header via `AUTH_HEADER` (default: "Authorization") for non-standard providers
  - AC-011: Support custom token prefix via `AUTH_KIND` (default: "Bearer") for non-standard providers
  - AC-012: Work with any OIDC-compliant provider (Azure AD, Auth0, Okta, Keycloak, WSO2, self-hosted)
  - AC-013: Automatic JWK key rotation support (picks up provider key changes seamlessly)

### REQ-003: Azure Graph Authentication (Priority: Medium)
* **Type**: Functional
* **Complexity**: Standard
* **Technology**: Microsoft Graph API + Go cache
* **Multi-Instance Considerations**: Local cache per instance, eventual consistency acceptable
* **Observability**: Graph API call rates, cache hit ratios, API latency
* **Dependencies**: None
* **Description**: Authenticate requests by validating Azure AD application tokens against Microsoft Graph API with caching
* **Acceptance Criteria**:
  - AC-001: Parse JWT without cryptographic verification
  - AC-002: Extract appId from token claims
  - AC-003: Query Microsoft Graph API for application details
  - AC-004: Cache application information with configurable TTL
  - AC-005: Make application details available to policies
  - AC-006: Handle Graph API rate limits gracefully
  - AC-007: Fail closed on Graph API errors

### REQ-004: Policy Evaluation Engine (Priority: High)
* **Type**: Functional
* **Complexity**: Complex
* **Technology**: OPA Rego v1.7+
* **Multi-Instance Considerations**: Each instance evaluates policies independently with identical policy files
* **Observability**: Policy evaluation time (p50/p95/p99), allow/deny ratios by endpoint, policy compilation errors, evaluation failures
* **Dependencies**: REQ-002 or REQ-003 (Authentication)
* **Description**: Evaluate Rego policies against structured request input to make authorization decisions. Policies must explicitly allow access (deny-by-default security model).
* **Acceptance Criteria**:
  - AC-001: Load Rego policies from configurable directory (default: `./policies`, env: `POLICY_DIR`)
  - AC-002: Support wildcard pattern for policy files (default: `*.rego`, env: `FILE_PATTERN`)
  - AC-003: Provide structured input with `request` (method, path[], headers, auth), `jwt` (JWT claims) or `user` (Azure app data)
  - AC-004: Evaluate policy entry point (default: `request.rego`, env: `REQUEST_REGO`) and extract `allow` boolean result
  - AC-005: Support optional `url` result for **metrics URL label customization** (NOT backend URL rewriting)
  - AC-006: Complete policy evaluation in <3ms for typical policies (RBAC, path-based authorization)
  - AC-007: Log policy input and result when debug mode enabled (`--debug` or `DEBUG=true`)
  - AC-008: Extract package name from policy files for correct evaluation path
  - AC-009: Fail closed (deny access) on policy evaluation errors
  - AC-010: Support complex policy logic (functions, rules, data structures)

### REQ-005: Hot Policy Reload (Priority: High)
* **Type**: Functional
* **Complexity**: Standard
* **Technology**: fsnotify file watcher
* **Multi-Instance Considerations**: Each instance watches its own policy directory
* **Observability**: Policy reload events, reload success/failure, policy validation errors
* **Dependencies**: REQ-004 (Policy Evaluation)
* **Description**: Automatically detect and reload policy files when changed without service restart
* **Acceptance Criteria**:
  - AC-001: Watch policy directory for file changes using fsnotify
  - AC-002: Detect create, modify, and delete operations on .rego files
  - AC-003: Recompile and reload policies within 1 second of file change
  - AC-004: Validate policy syntax before applying changes
  - AC-005: Continue using previous policies if new policies are invalid
  - AC-006: Log all reload events with success/failure status
  - AC-007: Emit metrics for policy reload operations

### REQ-006: Health and Readiness Probes (Priority: High)
* **Type**: Non-Functional
* **Complexity**: Simple
* **Technology**: Go HTTP server
* **Multi-Instance Considerations**: Each instance reports its own health independently
* **Observability**: Health check response times, failure counts
* **Dependencies**: REQ-004 (Policy Evaluation)
* **Description**: Provide HTTP endpoints for Kubernetes/container health and readiness checks
* **Acceptance Criteria**:
  - AC-001: `/healthz` endpoint returns 200 OK when service is healthy
  - AC-002: `/readyz` endpoint returns 200 OK when policies loaded and auth configured
  - AC-003: Health endpoints available on management port (default: 8182)
  - AC-004: Respond to health checks in <10ms
  - AC-005: Health checks do not require authentication
  - AC-006: Include basic status information in response body
  - AC-007: Support HEAD requests for minimal overhead checks

### REQ-007: Prometheus Metrics (Priority: High)
* **Type**: Non-Functional
* **Complexity**: Standard
* **Technology**: Prometheus client_golang
* **Multi-Instance Considerations**: Each instance exports its own metrics
* **Observability**: Metrics scrape success rate, metric cardinality
* **Dependencies**: All functional requirements
* **Description**: Export detailed operational and business metrics in Prometheus format
* **Acceptance Criteria**:
  - AC-001: `/metrics` endpoint on management port (default: 8182)
  - AC-002: Track request counts by method, path, and authorization result
  - AC-003: Measure request duration histograms (p50, p95, p99)
  - AC-004: Count authentication successes and failures by type
  - AC-005: Measure policy evaluation duration
  - AC-006: Track policy reload events and outcomes
  - AC-007: Include Go runtime metrics (memory, goroutines, GC)
  - AC-008: Support metric labels for multi-service deployments

### REQ-008: Structured Logging (Priority: Medium)
* **Type**: Non-Functional
* **Complexity**: Simple
* **Technology**: Go log/slog
* **Multi-Instance Considerations**: Each instance logs independently with instance identifier
* **Observability**: Log volume, error rates, log parsing success
* **Dependencies**: All functional requirements
* **Description**: Provide structured JSON logging for observability and debugging
* **Acceptance Criteria**:
  - AC-001: Use Go slog for structured logging
  - AC-002: Log levels: debug, info, warn, error
  - AC-003: Include request ID in all request-related logs
  - AC-004: Log authentication events (success, failure, method)
  - AC-005: Log authorization decisions in verbose mode
  - AC-006: Log policy reload events with file names and outcomes
  - AC-007: Support debug mode for policy input/output logging
  - AC-008: Include timestamps in ISO8601 format

### REQ-009: Configuration Management (Priority: High)
* **Type**: Functional
* **Complexity**: Standard
* **Technology**: alexflint/go-arg library
* **Multi-Instance Considerations**: Each instance configured independently via env vars or CLI
* **Observability**: Configuration errors, validation failures, startup configuration logging
* **Dependencies**: None
* **Description**: Support flexible configuration via environment variables and command-line arguments with comprehensive validation
* **Acceptance Criteria**:
  - AC-001: Support both environment variables and CLI arguments (flags take precedence)
  - AC-002: Validate configuration on startup (fail fast with clear error messages)
  - AC-003: Prevent conflicting authentication modes (cannot use both `AZURE_TENANT` and `WELLKNOWN_OIDC`)
  - AC-004: **Require** `JWT_AUDIENCES` when using `WELLKNOWN_OIDC` (fail on startup if missing)
  - AC-005: Provide sensible defaults for all optional settings
  - AC-006: Display version information on `--version` flag
  - AC-007: Support `--help` for comprehensive configuration documentation
  - AC-008: Log final configuration on startup (redact secrets: tokens, client IDs)
  - AC-009: Support comprehensive timeout configuration (see REQ-016)
  - AC-010: Support custom JWT audience claim key via `JWT_AUDIENCE_KEY`
  - AC-011: Support custom JWT header name via `AUTH_HEADER`
  - AC-012: Support custom token prefix via `AUTH_KIND`
  - AC-013: Support permissive authentication mode via `PERMISSIVE_AUTH`
  - AC-014: Validate timeout ranges (1s - 10m) and logical constraints

### REQ-010: Middleware Pipeline (Priority: High)
* **Type**: Technical
* **Complexity**: Standard
* **Technology**: go-chi/chi/v5 router
* **Multi-Instance Considerations**: Each instance executes middleware independently
* **Observability**: Middleware execution time, error propagation
* **Dependencies**: All functional requirements
* **Description**: Implement ordered middleware chain for request processing
* **Acceptance Criteria**:
  - AC-001: Execute middleware in order: Cleanup → Wrap → Metrics → Auth → Policy
  - AC-002: CleanupHandler initializes request context and ensures cleanup
  - AC-003: WrapHandler creates types.Info structure from HTTP request
  - AC-004: MetricsWrap records Prometheus metrics for request lifecycle
  - AC-005: AuthHandler authenticates using configured provider
  - AC-006: PolicyHandler evaluates Rego policies and makes decision
  - AC-007: Propagate types.Info through request context
  - AC-008: Handle middleware errors with appropriate HTTP status codes

### REQ-011: Graceful Shutdown (Priority: High)
* **Type**: Non-Functional
* **Complexity**: Standard
* **Technology**: Go context and signal handling
* **Multi-Instance Considerations**: Each instance shuts down independently on termination signal
* **Observability**: Shutdown duration, in-flight request completion
* **Dependencies**: REQ-001 (Reverse Proxy)
* **Description**: Handle termination signals gracefully, complete in-flight requests before shutdown
* **Acceptance Criteria**:
  - AC-001: Listen for SIGTERM and SIGINT signals
  - AC-002: Stop accepting new requests on shutdown signal
  - AC-003: Allow in-flight requests to complete (with timeout)
  - AC-004: Close file watchers and policy engine cleanly
  - AC-005: Shutdown within 30 seconds maximum
  - AC-006: Log shutdown initiation and completion
  - AC-007: Return appropriate status codes during shutdown

### REQ-012: Docker Container Support (Priority: High)
* **Type**: Technical
* **Complexity**: Standard
* **Technology**: Docker multi-stage build
* **Multi-Instance Considerations**: Each container instance is independent
* **Observability**: Container health, startup time
* **Dependencies**: All functional requirements
* **Description**: Package rest-rego as lightweight Docker container
* **Acceptance Criteria**:
  - AC-001: Multi-stage Dockerfile for minimal image size
  - AC-002: Run as non-root user for security
  - AC-003: Expose ports 8181 (proxy) and 8182 (management)
  - AC-004: Support volume mount for policy directory
  - AC-005: Include health check in Dockerfile
  - AC-006: Published to Docker Hub as lindex/rest-rego
  - AC-007: Tag releases with semantic version numbers
  - AC-008: Container startup in <3 seconds

### REQ-013: Kubernetes Deployment Support (Priority: High)
* **Type**: Technical
* **Complexity**: Standard
* **Technology**: Kubernetes manifests
* **Multi-Instance Considerations**: Designed for horizontal pod autoscaling
* **Observability**: Pod health, scaling events
* **Dependencies**: REQ-012 (Docker Container)
* **Description**: Provide Kubernetes deployment examples for sidecar and gateway patterns
* **Acceptance Criteria**:
  - AC-001: Example deployment.yaml for sidecar pattern
  - AC-002: Example service.yaml for service exposure
  - AC-003: Example ingress.yaml for external access
  - AC-004: ServiceAccount with minimal required permissions
  - AC-005: ConfigMap/Secret examples for configuration
  - AC-006: Kustomization support for environment variants
  - AC-007: Resource requests and limits defined
  - AC-008: Liveness and readiness probe configurations

### REQ-014: Permissive Authentication Mode (Priority: Low)
* **Type**: Functional
* **Complexity**: Simple
* **Technology**: Go authentication middleware
* **Multi-Instance Considerations**: Each instance applies permissive mode independently
* **Observability**: Anonymous request counts, authentication attempt distribution
* **Dependencies**: REQ-002 (JWT Authentication) or REQ-003 (Azure Graph Authentication)
* **Description**: Support treating authentication failures as anonymous users rather than rejecting requests. Useful for gradual migration scenarios or development environments.
* **Acceptance Criteria**:
  - AC-001: Enable via `--permissive-auth` flag or `PERMISSIVE_AUTH=true` environment variable
  - AC-002: Invalid tokens treated as unauthenticated (anonymous) rather than rejected with 401
  - AC-003: Policy receives empty/null authentication context for unauthenticated requests
  - AC-004: Policy can distinguish between valid auth, invalid auth, and no auth
  - AC-005: Log authentication failures even in permissive mode (for security monitoring)
  - AC-006: Default: disabled (fail closed, not permissive)
  - AC-007: Documented warning: permissive mode reduces security, use only when appropriate

### REQ-015: Metrics URL Label Customization (Priority: Medium)
* **Type**: Functional
* **Complexity**: Standard
* **Technology**: OPA Rego + Prometheus metrics
* **Multi-Instance Considerations**: Each instance applies URL customization independently
* **Observability**: URL pattern distribution, cardinality metrics
* **Dependencies**: REQ-004 (Policy Evaluation), REQ-007 (Prometheus Metrics)
* **Description**: Allow policies to customize the URL label in Prometheus metrics for GDPR compliance (anonymizing personal identifiers) and metric cardinality reduction (preventing explosion from unique IDs).
* **Purpose**: The `url` result is **NOT** for rewriting backend request URLs. It only affects the `path` label in Prometheus metrics.
* **Acceptance Criteria**:
  - AC-001: Policy can return optional `url` string in evaluation result
  - AC-002: Returned `url` string used as `path` label in `restrego_requests_total` metric
  - AC-003: Original request path used if policy does not return `url` result
  - AC-004: Support anonymization patterns (e.g., `/api/users/123` → `/api/users/:id`)
  - AC-005: Backend request sent to **original URL** (not the customized label)
  - AC-006: Reduce metric cardinality by grouping similar paths
  - AC-007: GDPR compliance: anonymize user IDs, email addresses, etc. in metrics
  - AC-008: Example use cases documented clearly to prevent confusion with URL rewriting

### REQ-016: Comprehensive Timeout Configuration (Priority: Medium)
* **Type**: Non-Functional
* **Complexity**: Simple
* **Technology**: Go HTTP server and client configuration
* **Multi-Instance Considerations**: Each instance configured independently
* **Observability**: Timeout events by type, request duration distribution
* **Dependencies**: REQ-001 (Reverse Proxy)
* **Description**: Support granular timeout configuration for all HTTP operations to prevent resource exhaustion and improve reliability.
* **Acceptance Criteria**:
  - AC-001: `READ_HEADER_TIMEOUT` (default: 10s): Timeout for reading request headers
  - AC-002: `READ_TIMEOUT` (default: 30s): Timeout for reading entire request
  - AC-003: `WRITE_TIMEOUT` (default: 90s): Timeout for writing response
  - AC-004: `IDLE_TIMEOUT` (default: 120s): Timeout for idle connections
  - AC-005: `BACKEND_DIAL_TIMEOUT` (default: 10s): Timeout for connecting to backend
  - AC-006: `BACKEND_RESPONSE_TIMEOUT` (default: 30s): Timeout for backend response headers
  - AC-007: `BACKEND_IDLE_TIMEOUT` (default: 90s): Timeout for idle backend connections
  - AC-008: Validate timeouts on startup: must be between 1s and 10m
  - AC-009: Validate logical constraints: read-timeout >= read-header-timeout
  - AC-010: Log timeout events with specific timeout type for debugging

## 5. User Experience

### 5.1 Entry Points & First-Time User Flow

* **Developer First Experience**:
  1. Read README.md quick start guide
  2. Pull Docker image or download binary
  3. Create simple allow policy in ./policies/request.rego
  4. Run rest-rego with minimal configuration
  5. Test authorization with curl commands
  6. See policy decisions in debug logs

* **Platform Engineer Deployment**:
  1. Review examples/kubernetes/ for deployment patterns
  2. Customize deployment for environment (dev/staging/prod)
  3. Configure authentication provider (JWT or Azure)
  4. Deploy policies alongside application code
  5. Monitor health checks and metrics
  6. Verify authorization decisions

### 5.2 Core Experience

* **Policy Development**: Write Rego policies in familiar editor, test with local rest-rego instance, see immediate feedback via debug logs
* **Deployment**: Deploy as sidecar container alongside application, zero application code changes, policies deploy with application
* **Monitoring**: View metrics in Prometheus/Grafana, track authorization decisions, measure performance impact
* **Policy Updates**: Edit policy files, automatic hot reload without restart, validate syntax before applying
* **Troubleshooting**: Enable debug mode for detailed logging, review policy input/output, check metrics for patterns

### 5.3 Advanced Features & Edge Cases

* **Metrics URL Customization**: Policy returns custom URL label for Prometheus metrics to anonymize personal data (GDPR) or reduce cardinality
* **Multiple OIDC Providers**: Configure multiple well-known URLs for different identity providers (Azure AD + Keycloak simultaneously)
* **Policy Testing**: Use OPA testing framework (`opa test`) to validate policies before deployment
* **Custom Metrics**: Expose application-specific authorization patterns via Prometheus metrics
* **Multi-Tenant Policies**: Write tenant-aware policies using JWT claims (`input.jwt.tenant_id`) or headers
* **Performance Tuning**: Optimize policy complexity for high-throughput scenarios (avoid expensive operations in hot path)
* **Permissive Auth Mode**: Gradual migration support - treat invalid tokens as anonymous for backwards compatibility
* **Graceful Degradation**: Backend failures result in clear error messages, not cascading failures

### 5.4 UI/UX Highlights

* **No UI**: Command-line and configuration-driven (by design)
* **Developer-Friendly Logs**: Clear structured logs with request IDs for correlation
* **Helpful Error Messages**: Configuration errors explain exactly what's wrong
* **Quick Feedback**: Debug mode shows policy input/output for immediate understanding
* **Standard Interfaces**: Prometheus metrics, health checks, standard HTTP status codes

## 6. Narrative

A development team has built 15 microservices that need consistent authorization. Initially, they implemented JWT validation and authorization logic in each service - 2-3 days per service, 30-45 developer days total. The code is scattered across 15 codebases with subtle differences. When authorization requirements change, they face deployment delays and inconsistencies.

They decide to deploy rest-rego as a sidecar. In 2-3 days, all 15 services are protected with consistent authorization. The team writes Rego policies that express their authorization rules: "App X can access endpoint Y", "Role admin has full access", "Public endpoints require no auth". These policies live in Git alongside application code.

**Request Flow**: Client → rest-rego (validates JWT in <1ms, evaluates policy in <3ms) → Backend (or 403 if denied)

The authorization layer adds only 3-5ms of latency. The team monitors authorization decisions via Prometheus metrics in Grafana dashboards: they see exactly which applications access which endpoints, track authorization denial patterns (possible attacks?), and measure performance impact.

**Policy Updates**: When requirements change weekly, they simply edit .rego files. Changes take effect within 1 second via hot reload - no deployment, no restart, no risk. Compare this to code changes requiring 10-30 minute deployments.

**Real Incident**: A developer forgets to add auth to a new endpoint. With custom code, this would be a security hole. With rest-rego, the endpoint is denied by default until a policy explicitly allows it (zero-trust, fail-closed model).

**GDPR Compliance**: Metrics track API usage, but user IDs in URLs would violate GDPR. The team writes a policy that returns `url := "/api/users/:id"` for metrics, anonymizing user IDs while still grouping similar requests. The actual backend request still uses the original URL with the real user ID.

**Scale**: Platform engineers deploy the same rest-rego pattern across dozens of services. Security engineers audit all policies in Git (complete version history). Compliance teams have complete audit trails. The authorization layer scales horizontally with applications. Teams maintain autonomy over their own policies while ensuring consistency.

**Result**: 90% less development time, 10x faster policy iteration, zero authorization bypass incidents, complete audit trails, consistent security posture across all services.

## 7. Success Metrics

### 7.1 User-Centric Metrics

* **Time to First Authorization**: <5 minutes from container start to first allowed request
* **Policy Development Time**: <30 minutes to write and test typical authorization policy
* **Deployment Complexity**: Single container sidecar, no external dependencies beyond backend
* **Debugging Efficiency**: Average <10 minutes to diagnose authorization issues with debug mode
* **Developer Satisfaction**: Survey scores >4/5 for ease of policy development

### 7.2 Business Metrics

* **Service Adoption Rate**: Number of services protected by rest-rego
* **Policy Updates Frequency**: Number of policy deployments per service per month
* **Security Incidents**: Reduction in authorization bypass incidents
* **Compliance Audit Time**: Reduction in time to audit authorization policies
* **Infrastructure Cost**: Cost per service compared to alternative solutions

### 7.3 Technical Metrics

* **Performance Metrics**:
  - Request latency overhead: <5ms (p99)
  - Throughput per instance: >5000 requests/second
  - Memory usage per instance: <100MB
  - CPU usage: <5% at 1000 req/s (2 core baseline)

* **Reliability Metrics**:
  - Service uptime: >99.9%
  - Policy reload success rate: >99.5%
  - Authentication success rate: >99.8% (excluding invalid tokens)
  - Error rate: <0.1% (excluding policy denials)

* **Scalability Metrics**:
  - Horizontal scaling efficiency: Linear to 10+ instances
  - Startup time: <3 seconds
  - Policy reload time: <1 second
  - Concurrent connections per instance: >10,000

### 7.4 Observability and Progress Tracking

* **Business Dashboards**:
  - Authorization Decision Overview: Allow/deny ratios by service, endpoint, application
  - Service Adoption: Number of protected services, policy coverage
  - Security Posture: Unauthorized access attempts, policy violations
  - Compliance: Policy change frequency, audit trail completeness

* **Operational Dashboards**:
  - Service Health: Instance health, readiness status, error rates
  - Performance: Request latency (p50, p95, p99), throughput, resource usage
  - Authentication: JWT validation rates, Graph API call rates, cache hit ratios
  - Policy Engine: Evaluation times, reload events, compilation errors

* **Custom Metrics**:
  - `restrego_requests_total`: Total requests by method, path, result (allow/deny)
  - `restrego_request_duration_seconds`: Request processing duration histogram
  - `restrego_auth_total`: Authentication attempts by method and result
  - `restrego_policy_evaluation_seconds`: Policy evaluation duration histogram
  - `restrego_policy_reload_total`: Policy reload events by result
  - `restrego_jwk_cache_hits_total`: JWK cache hit rate
  - `restrego_graph_api_calls_total`: Microsoft Graph API call count

* **Alert Thresholds**:
  - **Critical**: Service down (health check failing), >50% error rate, >100ms p99 latency
  - **Warning**: >5% error rate, >50ms p99 latency, policy reload failures
  - **Info**: Policy reload success, configuration changes, scaling events

## 8. Technical Considerations

### 8.1 Technology Stack Integration

* **Primary Technologies**:
  - Go 1.25+: Core implementation language for performance and concurrency
  - OPA Rego: Policy language and evaluation engine
  - Prometheus: Metrics collection and monitoring
  - Docker: Container packaging and distribution
  - Kubernetes: Orchestration and deployment platform

* **Integration Points**:
  - OIDC Providers: Azure AD, Auth0, Keycloak, Okta, WSO2 API Manager
  - Microsoft Graph API: Application validation for Azure authentication
  - Backend Services: Any HTTP/HTTPS REST API
  - Monitoring: Prometheus, Grafana, Azure Monitor, Datadog
  - CI/CD: GitHub Actions, Azure DevOps, Jenkins

* **Architecture Patterns**:
  - Sidecar Pattern: Deploy alongside each service instance
  - Gateway Pattern: Central authorization point for multiple backends
  - Middleware Chain: Ordered request processing pipeline
  - Plugin Architecture: Pluggable authentication providers

### 8.2 Multi-Instance Deployment Requirements

* **Concurrency Requirements**:
  - Each instance operates completely independently
  - No shared state between instances
  - Concurrent policy evaluation using goroutines
  - Safe for horizontal scaling across multiple pods/containers

* **State Management**:
  - No persistent state (stateless design)
  - Policy files mounted from ConfigMap or volume
  - JWK keys cached locally per instance (refreshed from OIDC endpoint)
  - Azure Graph app cache local per instance (eventual consistency)

* **Coordination Needs**:
  - No inter-instance coordination required
  - Policy updates via ConfigMap updates trigger rolling restart
  - Load balancer distributes requests across instances
  - Each instance makes independent authorization decisions

* **Platform Considerations**:
  - **Kubernetes**: HorizontalPodAutoscaler for scaling, ConfigMap for policies
  - **Azure Functions**: Not recommended (designed for long-running proxy)
  - **App Services**: Supported as sidecar in multi-container groups
  - **Traditional VMs**: Systemd service with policy directory mount

### 8.3 Data Storage & Privacy

* **Data Models**:
  - `types.Info`: Request context with structured request/auth data
  - `RequestInfo`: Normalized request data for policy evaluation
  - JWT claims: Standard OIDC claims (aud, iss, exp, appid, roles)
  - Azure app data: Application metadata from Graph API

* **Storage Strategy**:
  - Policy files: Read-only file system or ConfigMap
  - JWK cache: In-memory cache with TTL refresh
  - App cache: In-memory cache with configurable TTL (default: 1 hour)
  - No persistent storage required

* **Privacy Requirements**:
  - JWT tokens not logged (hidden in debug output)
  - Sensitive headers masked in logs
  - No storage of user personal data
  - Request IDs for correlation without PII
  - Compliance with data retention policies (ephemeral only)

* **Backup and Recovery**:
  - Policy files versioned in Git
  - No data to backup (stateless)
  - Recovery via container restart with policy volume
  - Cache rebuilds automatically on startup

### 8.4 Scalability & Performance

* **Performance Expectations**:
  - Request latency overhead: <5ms (p99)
  - Throughput per instance: 5000+ requests/second
  - Policy evaluation: <3ms for typical policies
  - JWT verification: <1ms with cached JWK
  - Memory footprint: 50-100MB per instance

* **Scaling Triggers**:
  - CPU usage >70%: Scale out
  - Request latency p99 >50ms: Scale out
  - Memory usage >80%: Scale out
  - Load balancer connection queue depth: Scale out
  - Request rate >4000 req/s per instance: Scale out

* **Resource Planning**:
  - Minimum: 0.5 CPU, 128MB memory per instance
  - Recommended: 1 CPU, 256MB memory per instance
  - High throughput: 2 CPU, 512MB memory per instance
  - Network: 100Mbps minimum, 1Gbps recommended

* **Capacity Considerations**:
  - Linear scaling up to 100+ instances per cluster
  - Policy complexity impacts throughput (test realistic policies)
  - Azure Graph mode: Limited by Graph API rate limits (500-1000 req/s)
  - JWT mode: No external rate limits (local verification)

### 8.5 Observability and Monitoring Requirements

* **Business Metrics**:
  - Authorization decisions by application (allow/deny ratios)
  - API endpoint usage patterns
  - Application access frequency
  - Policy violation trends
  - Service adoption rate

* **Operational Metrics**:
  - Request processing latency (p50, p95, p99)
  - Error rates by type (auth, policy, proxy)
  - Instance health and readiness
  - Resource usage (CPU, memory, goroutines)
  - Policy reload success/failure rates

* **Custom Metrics**:
  - Per-endpoint authorization metrics
  - Per-application access patterns
  - Policy evaluation performance by policy file
  - Cache hit rates (JWK, Graph API app cache)
  - Backend response times (via proxy)

* **Dashboard Strategy**:
  - **Executive Dashboard**: Service adoption, security posture, top applications
  - **Operations Dashboard**: Service health, performance, error rates, resource usage
  - **Security Dashboard**: Authorization denials, policy violations, suspicious patterns
  - **Developer Dashboard**: Per-service metrics, policy performance, debugging info

* **Alerting Framework**:
  - **Critical Alerts**: Service down, >50% error rate, security incidents
  - **Warning Alerts**: >5% error rate, high latency, policy reload failures
  - **Info Notifications**: Policy updates, configuration changes, scaling events

### 8.6 Operational Excellence

* **Health Check Requirements**:
  - **Startup**: Policy files loaded, authentication configured, backend reachable
  - **Readiness**: All components initialized, first policy evaluation successful
  - **Liveness**: HTTP server responding, goroutines not deadlocked
  - Health checks respond in <10ms on dedicated port (8182)

* **Graceful Lifecycle**:
  - **Startup**: Load policies → Configure auth → Start file watcher → Start HTTP servers
  - **Shutdown**: Stop accepting requests → Complete in-flight requests → Close watchers → Exit
  - Maximum shutdown time: 30 seconds
  - Signal handling: SIGTERM and SIGINT for graceful shutdown

* **Error Handling**:
  - Configuration errors: Fail fast on startup with clear error messages
  - Policy errors: Log error, continue with last valid policies
  - Authentication errors: Return 401 Unauthorized with error details
  - Policy evaluation errors: Fail closed (deny access), return 500 Internal Server Error
  - Backend errors: Proxy error response to client with appropriate status code

* **Security Considerations**:
  - **Authentication**: JWT signature verification, token expiration enforcement
  - **Authorization**: Deny by default, explicit allow rules only
  - **Secrets Management**: Environment variables for sensitive config, no secrets in logs
  - **TLS**: Support TLS for backend connections, certificate validation
  - **Container Security**: Non-root user, minimal image, no shell access
  - **Policy Isolation**: Policies cannot access file system or network beyond input data

### 8.7 Potential Challenges

* **Technical Risks**:
  - Policy complexity leading to performance degradation (mitigation: policy testing, benchmarking)
  - OPA version compatibility issues (mitigation: pinned dependency versions, testing)
  - JWK endpoint unavailability on startup (mitigation: retry logic, cached keys)
  - Policy hot reload race conditions (mitigation: atomic policy replacement)

* **Integration Complexity**:
  - Backend service discovery in dynamic environments (mitigation: Kubernetes DNS, configuration)
  - Multiple OIDC provider configuration (mitigation: clear documentation, examples)
  - Policy synchronization across instances (mitigation: ConfigMap updates, health checks)
  - Cross-service policy coordination (mitigation: shared policy library, documentation)

* **Performance Bottlenecks**:
  - Complex Rego policies with expensive operations (mitigation: policy optimization guide)
  - Azure Graph API rate limits (mitigation: caching, JWT mode recommendation)
  - Network latency to backend services (mitigation: sidecar deployment pattern)
  - Large number of policy files (mitigation: policy consolidation, lazy loading)

* **Operational Concerns**:
  - Policy syntax errors causing denials (mitigation: validation before deployment, testing)
  - Debugging authorization decisions (mitigation: debug mode, structured logging)
  - Policy version management across environments (mitigation: GitOps, semantic versioning)
  - Monitoring alert fatigue (mitigation: tuned thresholds, actionable alerts)

## 9. Milestones & Sequencing

### 9.1 Project Estimate

* **Size**: Large (existing production project with ongoing enhancements)
* **Time Estimate**: Maintenance mode with quarterly feature releases

### 9.2 Team Size & Composition

* **Team Size**: 2-3 contributors (open source project)
* **Roles Involved**:
  - Go Developer (primary maintainer)
  - DevOps Engineer (deployment patterns, examples)
  - Security Engineer (policy patterns, security review)
  - Technical Writer (documentation)

### 9.3 Suggested Phases

* **Phase 1: Foundation (Completed)**: Core reverse proxy, JWT authentication, policy evaluation, hot reload
  - Key Deliverables: Working sidecar, Docker image, basic policies
  
* **Phase 2: Azure Integration (Completed)**: Azure Graph authentication, app caching
  - Key Deliverables: Azure auth provider, Graph API integration, caching

* **Phase 3: Observability (Completed)**: Prometheus metrics, health checks, structured logging
  - Key Deliverables: Metrics endpoint, dashboards, logging improvements

* **Phase 4: Documentation & Examples (Completed)**: Comprehensive documentation, deployment examples
  - Key Deliverables: PRD documentation, Kubernetes examples, policy library, test suite

* **Phase 5: Enhancement & Optimization (Ongoing)**: Performance optimization, additional features
  - Key Deliverables: Policy caching improvements, additional auth providers, advanced examples, community contributions

## 10. User Stories

### 10.1 Deploy rest-rego as Sidecar

* **ID**: US-001
* **Type**: Functional
* **Priority**: High
* **Complexity**: Standard
* **Technology**: Docker, Kubernetes
* **Multi-Instance Considerations**: Each pod gets independent sidecar instance
* **Observability**: Pod health, sidecar startup time, resource usage
* **Description**: As a platform engineer, I want to deploy rest-rego as a sidecar container alongside my application so that authorization is co-located with the application
* **Dependencies**: REQ-012 (Docker Container), REQ-013 (Kubernetes Support)
* **Related Features**: examples/kubernetes/deployment.yaml
* **Acceptance Criteria**:
  - AC-001: Sidecar container starts before application container
  - AC-002: Application container can reach rest-rego on localhost:8181
  - AC-003: rest-rego forwards requests to application on configured port
  - AC-004: Pod health checks monitor both containers
  - AC-005: Policy ConfigMap mounted into sidecar container
  - AC-006: Rolling updates deploy new policies without downtime
  - AC-007: Resource requests prevent resource starvation

### 10.2 Write Authorization Policy

* **ID**: US-002
* **Type**: Functional
* **Priority**: High
* **Complexity**: Standard
* **Technology**: OPA Rego
* **Multi-Instance Considerations**: Same policy evaluated independently by each instance
* **Observability**: Policy evaluation time, syntax validation
* **Description**: As a backend developer, I want to write Rego policies that express authorization rules so that I can control who accesses my API
* **Dependencies**: REQ-004 (Policy Evaluation), REQ-005 (Hot Reload)
* **Related Features**: policies/request.rego, examples/
* **Acceptance Criteria**:
  - AC-001: Create .rego file in policies/ directory
  - AC-002: Policy has access to request (method, path, headers) and authentication (jwt/user) data
  - AC-003: Policy returns `allow` boolean for authorization decision
  - AC-004: Policy can optionally return `url` string for URL rewriting
  - AC-005: Syntax errors logged clearly on policy reload
  - AC-006: Policy changes take effect within 1 second via hot reload
  - AC-007: Can test policy locally using OPA CLI tools

### 10.3 Authenticate with JWT

* **ID**: US-003
* **Type**: Functional
* **Priority**: High
* **Complexity**: Simple
* **Technology**: JWT, OIDC
* **Multi-Instance Considerations**: Each instance validates JWT independently using cached JWK
* **Observability**: JWT validation success/failure rate, cache hit ratio
* **Description**: As an API consumer, I want to authenticate using JWT tokens so that rest-rego can verify my identity and authorize my requests
* **Dependencies**: REQ-002 (JWT Authentication)
* **Related Features**: docs/JWT.md
* **Acceptance Criteria**:
  - AC-001: Obtain JWT token from configured OIDC provider
  - AC-002: Include token in Authorization header (Bearer format)
  - AC-003: rest-rego validates JWT signature using OIDC public keys
  - AC-004: rest-rego validates token expiration, audience, issuer
  - AC-005: JWT claims available to policies as input.jwt
  - AC-006: Invalid tokens result in 401 Unauthorized response
  - AC-007: Token validation adds <1ms latency overhead

### 10.4 Monitor Authorization Decisions

* **ID**: US-004
* **Type**: Non-Functional
* **Priority**: High
* **Complexity**: Simple
* **Technology**: Prometheus, Grafana
* **Multi-Instance Considerations**: Aggregate metrics across all instances
* **Observability**: Metrics scrape success, dashboard functionality
* **Description**: As a DevOps engineer, I want to monitor authorization decisions in Prometheus so that I can track access patterns and troubleshoot issues
* **Dependencies**: REQ-007 (Prometheus Metrics)
* **Related Features**: None (monitoring infrastructure)
* **Acceptance Criteria**:
  - AC-001: Prometheus scrapes /metrics endpoint on port 8182
  - AC-002: View request counts by method, path, and allow/deny result
  - AC-003: View request latency histograms (p50, p95, p99)
  - AC-004: View authentication success/failure rates
  - AC-005: Create Grafana dashboards for authorization overview
  - AC-006: Set up alerts for high error rates or latency
  - AC-007: Correlate authorization metrics with application metrics

### 10.5 Debug Authorization Failures

* **ID**: US-005
* **Type**: Functional
* **Priority**: Medium
* **Complexity**: Simple
* **Technology**: Structured logging
* **Multi-Instance Considerations**: Each instance logs independently with instance identifier
* **Observability**: Log volume, error patterns
* **Description**: As a developer, I want to see detailed policy input and evaluation results so that I can debug why a request was denied
* **Dependencies**: REQ-008 (Structured Logging), REQ-004 (Policy Evaluation)
* **Related Features**: None (debugging feature)
* **Acceptance Criteria**:
  - AC-001: Enable debug mode with --debug flag or DEBUG=true env var
  - AC-002: Logs show complete policy input structure (request, jwt/user)
  - AC-003: Logs show policy evaluation result (allow, url)
  - AC-004: Request ID correlates logs across middleware chain
  - AC-005: Logs include timestamp, level, message, structured fields
  - AC-006: Can filter logs by request ID for specific request debugging
  - AC-007: Sensitive tokens masked in log output

### 10.6 Update Policies Without Downtime

* **ID**: US-006
* **Type**: Functional
* **Priority**: High
* **Complexity**: Standard
* **Technology**: fsnotify, Kubernetes ConfigMap
* **Multi-Instance Considerations**: Each instance reloads independently, rolling update pattern
* **Observability**: Policy reload events, validation failures
* **Description**: As a platform engineer, I want to update authorization policies without restarting services so that policy changes deploy quickly without downtime
* **Dependencies**: REQ-005 (Hot Reload)
* **Related Features**: None (operational feature)
* **Acceptance Criteria**:
  - AC-001: Edit policy file in ConfigMap or mounted volume
  - AC-002: rest-rego detects file change within 1 second
  - AC-003: Policy syntax validated before applying changes
  - AC-004: Invalid policies rejected, previous policies remain active
  - AC-005: Policy reload logged with success/failure status
  - AC-006: Metrics track policy reload events
  - AC-007: In-flight requests complete using previous policy version

### 10.7 Configure Multiple OIDC Providers

* **ID**: US-007
* **Type**: Functional
* **Priority**: Medium
* **Complexity**: Standard
* **Technology**: JWT, OIDC
* **Multi-Instance Considerations**: All instances use same OIDC configuration
* **Observability**: JWK fetch success by provider, validation distribution
* **Description**: As a platform engineer, I want to support multiple OIDC providers so that different applications can authenticate with different identity providers
* **Dependencies**: REQ-002 (JWT Authentication)
* **Related Features**: docs/JWT.md
* **Acceptance Criteria**:
  - AC-001: Configure multiple WELLKNOWN_OIDC URLs via environment variable
  - AC-002: rest-rego fetches JWK keys from all configured providers
  - AC-003: JWT validation tries each provider's keys until match found
  - AC-004: Policy can distinguish tokens from different providers via issuer claim
  - AC-005: Failed JWK fetch from one provider doesn't block others
  - AC-006: Metrics track validation success by provider
  - AC-007: Support audience validation per provider

### 10.8 Implement Role-Based Access Control

* **ID**: US-008
* **Type**: Functional
* **Priority**: High
* **Complexity**: Standard
* **Technology**: OPA Rego
* **Multi-Instance Considerations**: Same RBAC rules evaluated by all instances
* **Observability**: Authorization by role, role distribution
* **Description**: As a backend developer, I want to implement role-based access control so that different user roles have appropriate permissions
* **Dependencies**: REQ-004 (Policy Evaluation)
* **Related Features**: policies/roles.rego
* **Acceptance Criteria**:
  - AC-001: Define role permissions in Rego policy data structures
  - AC-002: Extract roles from JWT claims (e.g., input.jwt.roles array)
  - AC-003: Check user has required role for requested operation
  - AC-004: Support role hierarchy (admin inherits editor permissions)
  - AC-005: Log authorization decisions with role information
  - AC-006: Different endpoints require different roles
  - AC-007: Policy complexity doesn't significantly impact performance (<5ms evaluation)

### 10.9 Customize Metrics URL Labels (GDPR Compliance)

* **ID**: US-009
* **Type**: Functional
* **Priority**: Medium
* **Complexity**: Standard
* **Technology**: OPA Rego, Prometheus metrics
* **Multi-Instance Considerations**: Each instance applies URL customization independently in metrics
* **Observability**: URL pattern distribution, metric cardinality monitoring
* **Description**: As a backend developer, I want policies to customize URL labels in Prometheus metrics so that I can anonymize personal identifiers for GDPR compliance and reduce metric cardinality
* **Dependencies**: REQ-004 (Policy Evaluation), REQ-007 (Prometheus Metrics), REQ-015 (Metrics URL Customization)
* **Related Features**: None (metrics customization)
* **Important**: The `url` result does **NOT** change the backend request URL. It only affects the `path` label in Prometheus metrics.
* **Acceptance Criteria**:
  - AC-001: Policy returns optional `url` field in evaluation result
  - AC-002: Returned `url` used as `path` label in `restrego_requests_total` metric
  - AC-003: Backend request sent to **original URL**, not customized label
  - AC-004: Can anonymize user IDs (e.g., `/api/users/123` → `/api/users/:id` in metrics)
  - AC-005: Can anonymize email addresses (e.g., `/api/accounts/user@example.com` → `/api/accounts/:email`)
  - AC-006: Reduce metric cardinality by grouping similar paths
  - AC-007: GDPR compliance: no personal data in metric labels
  - AC-008: Example policies documented with clear explanations

### 10.10 Scale Horizontally

* **ID**: US-010
* **Type**: Non-Functional
* **Priority**: High
* **Complexity**: Simple
* **Technology**: Kubernetes HorizontalPodAutoscaler
* **Multi-Instance Considerations**: Core requirement - designed for multiple instances
* **Observability**: Instance count, load distribution, scaling events
* **Description**: As a platform engineer, I want to scale rest-rego horizontally so that authorization capacity grows with application demand
* **Dependencies**: REQ-013 (Kubernetes Support), all multi-instance requirements
* **Related Features**: examples/kubernetes/
* **Acceptance Criteria**:
  - AC-001: Configure HorizontalPodAutoscaler based on CPU or request rate
  - AC-002: New instances start and become ready within 5 seconds
  - AC-003: Load balancer distributes requests across all healthy instances
  - AC-004: Each instance makes independent authorization decisions
  - AC-005: No shared state or coordination between instances
  - AC-006: Scaling up/down doesn't impact active requests
  - AC-007: Linear performance improvement with additional instances (up to 10x)

### 10.11 Gradual Migration with Permissive Auth Mode

* **ID**: US-011
* **Type**: Functional
* **Priority**: Low
* **Complexity**: Simple
* **Technology**: Go authentication middleware
* **Multi-Instance Considerations**: Each instance applies permissive mode independently
* **Observability**: Anonymous request counts, authentication failure patterns
* **Description**: As a platform engineer, I want to enable permissive authentication mode during migration so that invalid tokens are treated as anonymous rather than rejected
* **Dependencies**: REQ-014 (Permissive Authentication Mode)
* **Related Features**: None (migration feature)
* **Use Case**: Migrating from no auth to JWT auth incrementally, or supporting mixed authenticated/unauthenticated traffic
* **Security Warning**: Reduces security posture - use only during controlled migrations
* **Acceptance Criteria**:
  - AC-001: Enable with `PERMISSIVE_AUTH=true` environment variable
  - AC-002: Invalid JWT tokens treated as unauthenticated (not rejected)
  - AC-003: Policy receives null/empty authentication context for failed auth
  - AC-004: Policy can distinguish valid auth vs anonymous via `input.jwt` presence
  - AC-005: Log authentication failures even in permissive mode
  - AC-006: Default disabled (fail closed for security)
  - AC-007: Documentation warns about security implications

### 10.12 Configure Comprehensive Timeouts

* **ID**: US-012
* **Type**: Non-Functional
* **Priority**: Medium
* **Complexity**: Simple
* **Technology**: Go HTTP server configuration
* **Multi-Instance Considerations**: Each instance configured independently
* **Observability**: Timeout events by type, timeout distributions
* **Description**: As a DevOps engineer, I want to configure granular HTTP timeouts so that I can prevent resource exhaustion and tune for my environment
* **Dependencies**: REQ-016 (Comprehensive Timeout Configuration)
* **Related Features**: None (operational configuration)
* **Acceptance Criteria**:
  - AC-001: Configure separate timeouts for: read-header, read, write, idle (server)
  - AC-002: Configure separate timeouts for: dial, response, idle (backend client)
  - AC-003: Timeouts validated on startup (1s - 10m range)
  - AC-004: Logical validation (read-timeout >= read-header-timeout)
  - AC-005: Timeout events logged with specific timeout type
  - AC-006: Sensible defaults work for most deployments
  - AC-007: Can tune for slow backends (increase response timeout) or fast backends (decrease for fail-fast)

---

## Appendix A: rest-rego vs Alternatives

### Why rest-rego? (See WHY.md for detailed analysis)

rest-rego solves the authorization problem that every REST API faces. This section summarizes the value proposition compared to alternatives.

### rest-rego vs DIY Authorization Code

| Aspect | DIY Implementation | rest-rego |
|--------|---------------------|-----------|
| **Initial Development** | 2-5 days per service | 30 minutes deployment |
| **Consistency** | Copy-paste prone to drift | Identical across all services |
| **Policy Updates** | Code change + deployment (10-30 min) | Edit file, 1-second reload |
| **Security Model** | Easy to forget checks (fail open) | Deny-by-default (fail closed) |
| **Testing** | Write unit tests for every service | OPA testing framework + examples |
| **OIDC Support** | Manual well-known parsing | Auto-discovery, automatic key rotation |
| **Maintenance** | Ongoing per-service maintenance | Central component updates |
| **Audit Trail** | Custom logging implementation | Built-in structured logs + metrics |
| **Multi-Service** | 15 services = 30-45 developer days | 2-3 days for all services |

**Real Cost Example**: 
- **DIY**: 15 microservices × 3 days = 45 developer days initial development + ongoing maintenance
- **rest-rego**: 2-3 days total for all 15 services + centralized maintenance

### rest-rego vs Heavy API Gateways (Kong, Apigee, Tyk)

| Aspect | Heavy API Gateway | rest-rego |
|--------|-------------------|-----------|
| **Complexity** | 100+ features (rate limiting, caching, transformation, etc.) | Focused on authorization only |
| **Resource Usage** | 200-500MB+ per instance | 50-100MB per instance |
| **Latency** | 10-50ms overhead | <5ms overhead |
| **Learning Curve** | Steep (gateway concepts, plugins) | Moderate (Rego policies) |
| **Cost** | Enterprise licenses ($$$) | Open source (free) |
| **Flexibility** | Opinionated workflows | Policy-as-code - full control |
| **Deployment** | Complex central setup | Single sidecar container |
| **Policy Language** | Gateway-specific config | Standard OPA Rego |

**When to Use Each**:
- **Heavy Gateway**: Need rate limiting, transformation, caching, protocol translation, etc.
- **rest-rego**: Need authorization only, prefer lightweight sidecar, want policy-as-code

### rest-rego vs Standalone OPA

| Aspect | Standalone OPA | rest-rego |
|--------|----------------|-----------|
| **Purpose** | General policy engine | REST API authorization sidecar |
| **Setup** | Build reverse proxy yourself | Complete proxy + policy engine |
| **JWT Handling** | Write JWT verification code | Built-in OIDC integration |
| **Hot Reload** | Implement file watching | Built-in with fsnotify |
| **Metrics** | Implement Prometheus integration | Built-in comprehensive metrics |
| **Health Checks** | Implement yourself | Kubernetes-ready endpoints |
| **Development Time** | 1-2 weeks to build proxy + integration | Deploy in 30 minutes |

**Relationship**: rest-rego **uses** OPA as its policy engine, providing a complete authorization sidecar solution.

### Key Advantages of rest-rego

1. **Ship Features 70% Faster**: No authorization boilerplate in application code
2. **Fail Closed by Default**: Deny-by-default security model prevents authorization bypass bugs
3. **Policy-as-Code**: Version control policies in Git alongside application code
4. **10x Faster Policy Iteration**: 1-second hot reload vs 10-30 minute deployments
5. **Lightweight**: 50-100MB vs 200-500MB+ for full gateways
6. **High Performance**: <5ms latency overhead vs 10-50ms for heavy gateways
7. **Zero Code Changes**: Deploy as sidecar, application unchanged
8. **Production-Ready**: Prometheus metrics, health checks, structured logging built-in
9. **OIDC Flexibility**: Works with any standards-compliant OIDC provider (cloud or on-premises)
10. **Automatic Key Rotation**: JWK keys automatically refreshed from OIDC endpoints

### When NOT to Use rest-rego

- ❌ **Ultra-low latency requirements**: If every millisecond counts (< 1ms response times required)
- ❌ **Simple public APIs**: If API is fully public with no auth, you don't need this
- ❌ **Non-REST protocols**: rest-rego is HTTP/REST only (no gRPC, WebSocket, MQTT)
- ❌ **Database-level authorization**: This protects HTTP APIs, not database queries
- ❌ **Need full API gateway features**: If you need rate limiting, caching, transformation, use full gateway

### Real-World Impact

**Scenario 1: 15 Microservices Deployment**
- **Before rest-rego**: 45 developer days, inconsistent implementations, maintenance nightmare
- **After rest-rego**: 2-3 days deployment, consistent security, centralized maintenance
- **Result**: 90% reduction in authorization development time

**Scenario 2: Weekly Authorization Changes**
- **Before rest-rego**: Code change + build + deploy = 10-30 minutes per change, risk of bugs
- **After rest-rego**: Edit policy file = 1 second, no deployment, no risk
- **Result**: 10x faster policy iteration

**Scenario 3: Emergency Access Grant**
- **Before rest-rego**: Emergency code change + hotfix deployment = 10+ minutes minimum
- **After rest-rego**: Edit policy file, save = <1 second
- **Result**: Sub-second emergency policy updates

---

## Appendix B: Technology Stack Details

### Core Dependencies
- **go 1.25.0**: Programming language
- **github.com/open-policy-agent/opa v1.7.1**: Policy evaluation engine
- **github.com/lestrrat-go/jwx/v2 v2.1.6**: JWT verification and JWKS handling
- **github.com/go-chi/chi/v5 v5.2.2**: HTTP router and middleware
- **github.com/fsnotify/fsnotify v1.9.0**: File system watching for hot reload
- **github.com/prometheus/client_golang v1.23.0**: Prometheus metrics
- **github.com/patrickmn/go-cache v2.1.0**: In-memory caching
- **github.com/alexflint/go-arg v1.6.0**: Configuration parsing

### Deployment Platforms
- **Docker**: Primary container packaging
- **Kubernetes**: Primary orchestration platform
- **Azure Kubernetes Service**: Recommended Azure deployment
- **Docker Compose**: Development and simple deployments

### Monitoring Stack
- **Prometheus**: Metrics collection
- **Grafana**: Visualization and dashboards
- **Azure Monitor**: Azure-native monitoring
- **Log aggregation**: Any compatible with structured JSON logs

---

## Appendix B: Technology Stack Details

### Core Dependencies
- **go 1.25.0**: Programming language (performance, concurrency, cross-platform)
- **github.com/open-policy-agent/opa v1.7.1**: Policy evaluation engine (Rego language)
- **github.com/lestrrat-go/jwx/v2 v2.1.6**: JWT verification and JWKS handling with OIDC discovery
- **github.com/go-chi/chi/v5 v5.2.2**: HTTP router and middleware framework
- **github.com/fsnotify/fsnotify v1.9.0**: File system watching for hot policy reload
- **github.com/prometheus/client_golang v1.23.0**: Prometheus metrics exposition
- **github.com/patrickmn/go-cache v2.1.0**: In-memory caching (Azure Graph, JWK)
- **github.com/alexflint/go-arg v1.6.0**: Configuration parsing (env vars + CLI args)

### Authentication Integrations
- **Any OIDC Provider**: Azure AD, Auth0, Okta, Keycloak, WSO2 API Manager, self-hosted
- **Microsoft Graph API**: Azure AD application metadata (Azure auth mode)
- **JWT Standards**: RFC 7519 (JWT), RFC 7517 (JWK), OpenID Connect Discovery
- **Non-Standard JWT**: Custom headers and claim keys for providers like WSO2

### Deployment Platforms
- **Docker**: Primary container packaging (multi-stage builds, non-root user)
- **Kubernetes**: Primary orchestration platform (sidecar pattern, HPA, ConfigMaps)
- **Azure Kubernetes Service**: Recommended Azure deployment
- **Docker Compose**: Development and simple deployments
- **Systemd**: Traditional VM deployments (supported but not primary)

### Monitoring Stack
- **Prometheus**: Metrics collection (scrape model)
- **Grafana**: Visualization and dashboards
- **Azure Monitor**: Azure-native monitoring integration
- **Log Aggregation**: Any system compatible with structured JSON logs (ELK, Splunk, Datadog)

### Development Tools
- **OPA CLI**: Local policy testing and validation
- **VS Code REST Client**: Manual API testing (.http files)
- **k6**: Load and performance testing (.k6 files)
- **Taskfile**: Build automation (Taskfile.yml)

---

## Appendix C: Related Documentation

### Existing Documentation (Comprehensive)
- **README.md**: Quick start guide, configuration reference, deployment patterns, troubleshooting
- **WHY.md**: Detailed value proposition, comparison with DIY auth and API gateways, real-world scenarios
- **SECURITY.md**: Security policy, best practices, deployment security, vulnerability reporting
- **docs/JWT.md**: JWT authentication detailed configuration, OIDC provider setup, multi-provider support, custom headers and claims
- **docs/AZURE.md**: Azure Graph authentication configuration, app caching, performance tuning
- **docs/WSO2.md**: WSO2 API Manager integration guide, non-standard JWT handling, application authorization patterns
- **examples/**: Kubernetes deployment manifests (deployment, service, ingress, ConfigMap examples)
- **examples/kubernetes/request.rego**: Sample policies demonstrating common patterns
- **policies/**: Default policy examples (request.rego, roles.rego)
- **tests/**: Manual testing (.http) and load testing (.k6) examples
- **/.specs/PRD.md** (this document): Comprehensive product requirements and technical specifications

### Documentation Quality
- ✅ **Complete**: All features documented with examples
- ✅ **Accurate**: Synchronized with actual code implementation (v2.0 corrections applied)
- ✅ **Practical**: Real-world examples and troubleshooting guides
- ✅ **Updated**: Regular synchronization with code changes
- ✅ **Multi-Level**: Quick start (README) to deep dive (PRD, WHY.md)

### Key Corrections in v2.0
1. **URL Result Purpose**: Corrected from "backend URL rewriting" to "metrics URL label customization"
2. **JWT Audience Requirement**: Confirmed as required (not optional) when using JWT mode
3. **Permissive Auth Mode**: Documented newly discovered feature
4. **Comprehensive Timeouts**: Documented all timeout configuration options
5. **Audience Key Configuration**: Documented custom JWT claim key support
6. **Multi-OIDC Support**: Documented array-based configuration for multiple providers

### Updates in v2.1
1. **WSO2 API Manager Integration**: Comprehensive documentation for WSO2 non-standard JWT handling
2. **Custom JWT Headers**: Documented `AUTH_HEADER` configuration for non-standard JWT header names
3. **Custom Token Prefix**: Documented `AUTH_KIND` configuration for custom or empty token prefixes
4. **Enhanced JWT Documentation**: Updated JWT.md reference to include custom header and claim configurations

### Future Documentation Enhancements
- Interactive policy playground/sandbox (web-based policy testing)
- Video tutorials and walkthroughs (deployment, policy writing)
- Community-contributed policy library (common patterns repository)
- Performance benchmarking suite (automated performance regression testing)
- Certification/compliance guides (SOC2, ISO27001, PCI-DSS mapping)
- Migration guides from alternatives (Authz, standalone OPA, custom code)
- Advanced policy patterns (attribute-based access control, time-based rules, geo-fencing)

---

## Appendix D: Policy Input Structure Reference

### Complete Policy Input Schema

```json
{
  "request": {
    "method": "string",           // HTTP method (GET, POST, PUT, DELETE, etc.)
    "path": ["string"],           // URL path split as array (e.g., ["api", "users", "123"])
    "headers": {                  // Request headers (canonical key names)
      "Authorization": "string",  // Hidden in debug logs
      "Content-Type": "string",
      "Custom-Header": "string"
    },
    "auth": {
      "kind": "string",           // Authentication type ("Bearer", "Basic", etc.)
      "token": "string"           // Token value (hidden in debug logs)
    },
    "size": 1234                  // Request body size in bytes
  },
  "jwt": {                        // Present when using JWT authentication
    "aud": "string",              // Audience claim (validated against JWT_AUDIENCES)
    "iss": "string",              // Issuer claim
    "exp": 1234567890,            // Expiration timestamp
    "iat": 1234567890,            // Issued at timestamp
    "appid": "guid",              // Application ID (Azure AD)
    "roles": ["string"],          // Role array (if present in token)
    "custom_claim": "value"       // Any custom JWT claims available
  },
  "user": {                       // Present when using Azure Graph authentication
    "appId": "guid",              // Azure AD application ID
    "displayName": "string",      // Application display name
    "publisherDomain": "string",  // Publisher domain
    // Additional Graph API fields cached per application
  }
}
```

### Policy Output Schema

```json
{
  "allow": true,                  // REQUIRED: Boolean authorization decision
  "url": "/api/users/:id"         // OPTIONAL: Custom URL label for metrics (not backend URL)
}
```

### Example Policy Patterns

**Simple Allow/Deny:**
```rego
package policies
default allow := false
allow { input.jwt.appid == "11112222-3333-4444-5555-666677778888" }
```

**Role-Based Access Control:**
```rego
package policies
default allow := false
allow { "admin" in input.jwt.roles }
allow { "editor" in input.jwt.roles; input.request.method == "GET" }
```

**Path-Based Authorization:**
```rego
package policies
default allow := false
allow { input.request.path[0] == "public" }
allow { input.request.path[0] == "api"; input.jwt.appid != "" }
```

**Metrics URL Customization (GDPR):**
```rego
package policies
default allow := false
default url := ""

allow { "admin" in input.jwt.roles }

# Anonymize user IDs in metrics
url := "/api/users/:id" {
  input.request.path[0] == "api"
  input.request.path[1] == "users"
  count(input.request.path) > 2
}
```

---

## Appendix E: Performance Characteristics

### Latency Breakdown (Typical Request)

| Component | Latency (p99) | Notes |
|-----------|---------------|-------|
| JWT Verification | <1ms | With cached JWK keys |
| Policy Evaluation | <3ms | Simple to moderate policies |
| Middleware Overhead | <1ms | Request context setup |
| **Total rest-rego** | **<5ms** | End-to-end overhead |
| Backend Processing | Varies | Not included (application-specific) |

### Throughput Characteristics

| Resource Configuration | Expected Throughput | Notes |
|------------------------|---------------------|-------|
| 0.5 CPU, 128MB RAM | ~1,000 req/s | Minimum viable |
| 1 CPU, 256MB RAM | ~5,000 req/s | Recommended baseline |
| 2 CPU, 512MB RAM | ~10,000 req/s | High throughput |
| Linear scaling | Up to 100+ instances | Kubernetes HPA |

### Policy Complexity Impact

| Policy Type | Evaluation Time | Recommendation |
|-------------|-----------------|----------------|
| Simple allow/deny | <1ms | Ideal |
| Role-based (RBAC) | <2ms | Common pattern |
| Path + claims checks | <3ms | Acceptable |
| Complex data structures | 3-10ms | Optimize if possible |
| External data lookups | Not supported | Use cached data only |

### Scaling Characteristics

- **Horizontal Scaling**: Linear performance improvement up to 100+ instances
- **Startup Time**: <3 seconds per instance (policy loading + auth config)
- **Memory Growth**: Stable after startup (no memory leaks)
- **CPU Pattern**: Scales linearly with request rate
- **Network**: Minimal overhead (localhost backend in sidecar pattern)

### Comparison Benchmarks

| Solution | Latency (p99) | Throughput/Instance | Memory |
|----------|---------------|---------------------|--------|
| **rest-rego** | **<5ms** | **5,000+ req/s** | **50-100MB** |
| DIY JWT Code | 5-15ms | Varies | N/A |
| Heavy Gateway | 10-50ms | 2,000-5,000 req/s | 200-500MB |
| Database Auth | 10-50ms | Limited by DB | N/A |

---

*This PRD v2.1 represents the accurate, comprehensive state of the rest-rego project. It incorporates WSO2 API Manager integration, custom JWT configurations (AUTH_HEADER, AUTH_KIND), and aligns with all documentation in README.md, WHY.md, JWT.md, AZURE.md, and WSO2.md. It serves as the definitive reference for features, requirements, and technical specifications.*
