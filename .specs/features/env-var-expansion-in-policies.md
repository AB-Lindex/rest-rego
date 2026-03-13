---
type: "feature"
feature: "env-var-expansion-in-policies"
repository_type: "single-product"
status: "active"
priority: "medium"
complexity: "simple"
technology_stack: ["go"]
ci_cd_system: "azure-devops"
deployment_targets: ["onprem-k8s", "azure-aks"]
lindex_components:
  authentication: "rest-rego"
  security_scanning: "trivy"
azure_services: []
external_services: []
on_premises_dependencies: []
multi_instance_support: "compatible"
observability: "basic"
related_prd: "PRD.md"
cross_product_dependencies: []
implemented_in_commit: "a7066771def2c880ea166ea5d50934c13bc331f9"
---

# Feature: Environment Variable Expansion in Rego Policies

## Problem Statement

Rego policy files are loaded from disk and often committed to source control or bundled into Kubernetes ConfigMaps. This means hardcoded secrets (API keys, shared secrets, allowed client IDs) would be present in plain text in version-controlled files — a security antipattern.

Operators need a way to inject runtime secrets into policies without embedding them in source files or ConfigMaps.

## User Stories

### US-001: Secret Injection via Environment Variables
**As an** operator deploying rest-rego in Kubernetes  
**I want** to reference environment variable values inside Rego policies using `$(VAR_NAME)` syntax  
**So that** secrets can be stored in Kubernetes Secrets and injected at runtime without appearing in policy source files

**Acceptance Criteria**:
- Policy files may contain `$(VAR_NAME)` placeholders
- Placeholders are expanded at policy load time (and on hot-reload)
- Unset variables expand to empty string
- Policies without any placeholders are unaffected

### US-002: Kubernetes Secret Integration
**As a** platform engineer  
**I want** to store allowed client IDs or API tokens in a Kubernetes Secret and reference them from a Rego policy  
**So that** secret rotation does not require updating ConfigMaps or rebuilding images

**Acceptance Criteria**:
- Environment variables sourced from a Kubernetes Secret (via `envFrom` or `env.valueFrom.secretKeyRef`) are expanded correctly
- Variable substitution happens before OPA compiles the policy

## Requirements

### Functional
1. On policy file load (initial scan and fsnotify hot-reload), each file's bytes are processed through env-var expansion before being passed to the OPA compiler.
2. Syntax: `$(VAR_NAME)` — standard shell-style parenthesis wrapper.
3. Missing/unset variables expand to an empty string (no error, no panic).
4. All existing policy behaviour (no placeholders) is fully preserved.

### Non-Functional
- **Security**: Secrets never written to disk; expansion happens in-memory at load time only.
- **Kubernetes Multi-Instance Support**: Compatible — each pod reads vars from its own environment; no shared state required.
- **Performance**: Expansion is O(n) in file size and runs only on load/reload, not per-request.

## Technical Design

### Architecture
Policy loading pipeline in `pkg/regocache/rego.go`:

```
filecache.Get() → []byte (raw file content)
    ↓
envsubst.ConvertBytes(data, envsubst.Getenv)   ← NEW
    ↓
OPA rego.Module compilation
    ↓
rego.PreparedEvalQuery (cached)
```

### Implementation

**Package**: [`github.com/ninlil/envsubst`](https://github.com/ninlil/envsubst) v0.2.0

This package was chosen over `os.ExpandEnv` because it:
- Works directly on `[]byte` with no typecasting
- Supports configurable prefix characters (`$`, `%`, `&`, `#`)
- Supports configurable wrapper pairs (`{}`, `()`, `[]`, `<>`)
- Returns an error if expansion fails, allowing proper error handling

**Default configuration** (library defaults, no `init()` override needed):
- Prefix: `$` (default)
- Wrapper: `()` → standard `$(VAR)` syntax

**Changed files**:
- `pkg/regocache/rego.go` — replaced file-content handling in `GetRego()` with `envsubst.ConvertBytes`
- `go.mod` / `go.sum` — added `github.com/ninlil/envsubst v0.2.0`
- `pkg/regocache/rego_test.go` — new test file (see Testing section)

### Example Policy Usage

```rego
package policies

default allow := false

# Allowed app IDs injected from environment at load time
allowed_apps := {
    "$(ALLOWED_APP_ID_1)",
    "$(ALLOWED_APP_ID_2)",
}

allow if {
    input.user.appId in allowed_apps
}
```

Kubernetes Secret + sidecar env:
```yaml
env:
  - name: ALLOWED_APP_ID_1
    valueFrom:
      secretKeyRef:
        name: restrego-secrets
        key: allowed-app-id-1
```

## Testing

Five unit tests in `pkg/regocache/rego_test.go`:

| Test                                        | Scenario                                              | Expected             |
|---------------------------------------------|-------------------------------------------------------|----------------------|
| `TestEnvExpansion_matchingValue`            | `$(VAR)` expands to set env value; input matches      | `allow = true`       |
| `TestEnvExpansion_wrongValue`               | `$(VAR)` expands correctly; input does not match      | `allow = false`      |
| `TestEnvExpansion_missingVarExpandsToEmpty` | Unset var expands to `""`; empty string input matches | `allow = true`       |
| `TestEnvExpansion_multipleVars`             | Two vars expanded; both/one/neither matching checked  | matches expectations |
| `TestEnvExpansion_noVarsInPolicy`           | Policy with no placeholders                           | unchanged behaviour  |

## Integration

- **PRD Link**: Extends the Policy Engine section — enables secret-safe policy authoring.
- **README Impact**: Document `$(VAR_NAME)` syntax in `docs/POLICY.md`.
- **Deployment**: No changes to deployment manifests needed; operators add `env`/`envFrom` entries to the sidecar container spec.
