---
type: "prd"
project: "rest-rego-e2e-tests"
version: "1.6"
status: "draft"
last_updated: "2026-05-21"
repository_type: "single-product"
technology_stack: ["go"]
parent_prd: ".specs/PRD.md"
related_features: []
---

# PRD: End-to-End Test Suite for rest-rego

## 1. Product Overview

### 1.1 Document Title and Version

- **PRD**: End-to-End Test Suite for rest-rego
- **Version**: 1.6
- **Last Updated**: 2026-05-21
- **Status**: Draft
- **Parent PRD**: [PRD.md](PRD.md) — rest-rego core product

### 1.2 Summary

This document describes a self-contained end-to-end test suite for rest-rego. Each test acts simultaneously as the **HTTP client** calling rest-rego and as the **backend** receiving the proxied request. This dual role allows each test to fully observe both sides of the proxy interaction and verify the complete request/response lifecycle.

The suite is located in `/e2e-tests/` at the repository root, with reusable infrastructure in the `/e2e-tests/shared/` package. Each scenario is a standalone Go `main` program — not a `_test.go` file — so the suite is completely invisible to `go test ./...` and has no dependency on the `testing` package.

Every scenario has two programs: an **assertion binary** (`main.go`) and a **helpers binary** (`helpers/main.go`).

The assertion binary accepts the URLs of already-running services as flags:

- **`--proxy`** — base URL of the rest-rego proxy
- **`--oidc`** — base URL of the OIDC provider (JWT scenarios only)
- **`--backend`** — base URL of the mock backend

It starts `./bin/restrego` as a subprocess, runs all assertions, exits 0 on pass or 1 on fail. Intended for CI and local correctness checks.

The helpers binary starts only the supporting servers (OIDC provider, mock backend) on fixed, documented localhost ports and keeps them running until SIGINT/SIGTERM. Intended as the companion to a separately-started rest-rego process for load and memory diagnostics.

For load testing and memory-leak diagnostics, each scenario directory contains a `Taskfile.yml` with two tasks that are started independently:
- **`task helpers`** — runs the scenario's helpers binary (`helpers/main.go`)
- **`task restrego`** — starts the compiled rest-rego binary configured to connect to the helpers' fixed ports

This separation allows rest-rego to remain running after a k6 run completes, giving Prometheus metrics time to settle and enabling the scenario to be run again against the same process.

### 1.3 Documentation Ecosystem

- **Parent PRD**: [PRD.md](PRD.md) — overall product requirements
- **[docs/JWT.md](../docs/JWT.md)**: JWT authentication, OIDC well-known, JWKS configuration
- **[docs/FILE-BASED-JWKS.md](../docs/FILE-BASED-JWKS.md)**: File-based JWKS loading (used by the JWT helpers internally)
- **[docs/BASIC-AUTH.md](../docs/BASIC-AUTH.md)**: Basic auth configuration and htpasswd format
- **[docs/POLICY.md](../docs/POLICY.md)**: Rego policy input structure and `allow`/`url` rules
- **[docs/PERMISSIVE.md](../docs/PERMISSIVE.md)**: Permissive auth mode

---

## 2. Goals

### 2.1 Goals

- Provide deterministic, dependency-free E2E correctness tests invoked with `go run ./e2e-tests/<scenario>`
- Always run the pre-built `/bin/restrego` binary as a subprocess — the suite has no compile-time dependency on rest-rego's internal packages
- Exercise every supported auth mode (JWT, basic auth, no-auth) against the real rest-rego binary
- Allow each scenario to assert both what rest-rego **allows/denies** and what the **backend actually receives**
- Serve as living documentation and regression safety net for all auth and policy interactions
- Expose every scenario's helper servers (separate `helpers/main.go` binary) so a separately-started rest-rego process can be targeted by k6 for performance, load, and memory-leak diagnostics
- Provide k6 scripts for every scenario covering throughput benchmarking, sustained-load profiling, and memory growth detection

### 2.2 Non-Goals

- Running as part of `go test ./...` — the suite is intentionally excluded from the standard test runner
- Manual integration testing against real Azure AD or WSO2 (covered by `tests/*.http`)
- Unit testing of individual packages (covered by existing `*_test.go` files throughout the project)
- Testing the Rego policy language itself — only the integration boundary between rest-rego and policies is tested
- Replacing the existing `tests/*.k6` scripts (those target deployed instances; these target the controlled in-process test setup)

---

## 3. Test Architecture

### 3.1 Topology per Test

Every test case — in both assert and helpers mode — uses the same external subprocess topology:

```
[Test HTTP Client / k6]
        │  HTTP request
        ▼
[/bin/restrego]          ←── committed policies/ dir
        │  Auth validation
        │  Policy evaluation
        │  Proxied request (if allowed)
        ▼
[Test Backend Server]
        │  Response
        ▼
[/bin/restrego]          ──► response forwarded to caller
        │
        ▼
[Test HTTP Client / k6]  ──► assert status, headers; inspect backend capture
```

The difference between modes is only **who manages the restrego process**:
- **Assert mode**: the scenario binary starts `/bin/restrego` as a subprocess, runs assertions, and stops it automatically
- **Helpers mode**: the scenario binary starts only the OIDC provider and mock backend; the user starts `/bin/restrego` separately via `task restrego`

All components run on `127.0.0.1`. No network calls leave the host machine.

### 3.2 Auth Mode Topologies

For **JWT** tests, an additional in-process OIDC provider is started by the scenario binary alongside the mock backend:

```
[/bin/restrego]
        │  fetches on startup
        ▼
[In-process OIDC Provider]   ←─ started by the scenario binary
  /.well-known/openid-configuration  → points to /jwks
  /jwks                              → serves JWK public key set
```

The OIDC provider generates an RSA key pair at startup. The scenario binary signs test tokens with the private key; `/bin/restrego` verifies them against the public key served over loopback.

For **basic auth** tests, a temporary `htpasswd` file is written to a directory created with `os.MkdirTemp` and removed with `defer os.RemoveAll(dir)`. No additional server is needed.

---

## 4. Folder Structure

```
/e2e-tests/
├── shared/                      # Package e2eshared — no dependency on "testing"
│   ├── oidc_provider.go         # In-process OIDC/JWKS server
│   ├── token_signer.go          # JWT token builder and signer
│   ├── backend_server.go        # Configurable mock backend (net/http)
│   └── htpasswd.go              # bcrypt htpasswd file generator
│
├── jwt/                         # go run ./e2e-tests/jwt --proxy=... --oidc=... --backend=...
│   ├── main.go                  # All JWT auth scenarios (assertions only)
│   ├── helpers/
│   │   └── main.go              # Starts OIDC + backend on fixed ports; blocks until SIGINT
│   ├── policies/
│   ├── setup.sh                 # Start OIDC + backend + restrego; export env vars
│   ├── teardown.sh              # Kill processes; remove temp dirs
│   ├── Taskfile.yml             # task helpers / task restrego
│   └── k6/
│       ├── jwt-allow.js         # k6: throughput + latency, valid token
│       └── jwt-deny.js          # k6: throughput + latency, deny policy
│
├── basicauth/
│   ├── main.go                  # All basic auth scenarios (assertions only)
│   ├── helpers/
│   │   └── main.go              # Starts backend on fixed port; blocks until SIGINT
│   ├── policies/
│   ├── setup.sh
│   ├── teardown.sh
│   ├── Taskfile.yml
│   └── k6/
│       └── basicauth-allow.js
│
├── noauth/
│   ├── main.go                  # All no-auth scenarios (assertions only)
│   ├── helpers/
│   │   └── main.go              # Starts backend on fixed port; blocks until SIGINT
│   ├── policies/
│   ├── setup.sh
│   ├── teardown.sh
│   ├── Taskfile.yml
│   └── k6/
│       ├── noauth-allow.js
│       ├── noauth-memory.js         # k6: soak with variable response sizes
│       └── noauth-content-length.js # k6: Content-Length mismatch stress test
│
├── policy/
│   ├── main.go                  # Cross-cutting policy behaviour (assertions only)
│   ├── helpers/
│   │   └── main.go
│   ├── policies/
│   ├── setup.sh
│   ├── teardown.sh
│   ├── Taskfile.yml
│   └── k6/
│       ├── url-label.js
│       └── blocked-headers.js
│
├── k6-lib/                      # Shared k6 preset modules
│   ├── throughput.js
│   └── load.js
│
├── memleak.sh                   # Capture heap before/after a k6 run; exit non-zero on growth
├── k6run.sh                     # setup.sh → k6 run → teardown.sh
└── README.md                    # How to run, extend, and troubleshoot
```

Each assertion `main.go` receives the URLs it needs as flags — the calling script is responsible for starting services:

```bash
# Run assertions for the JWT scenario
./e2e-tests/jwt/setup.sh   # starts OIDC + backend + restrego, writes env to .env.jwt
source .env.jwt
go run ./e2e-tests/jwt --proxy=$PROXY_URL --oidc=$OIDC_URL --backend=$BACKEND_URL
./e2e-tests/jwt/teardown.sh

# Or via the convenience wrapper (setup → go run → teardown):
./e2e-tests/k6run.sh e2e-tests/jwt e2e-tests/jwt/k6/jwt-allow.js
```

---

## 5. Shared Package (`e2e-tests/shared`)

### 5.1 Server Types and `RunCase`

The shared package provides three independent, composable server types. Each scenario's `main.go` starts only the servers it needs, wires them together, and passes the resulting URLs to its assertions as plain function arguments.

```go
// TestCase describes one assertion: what to send and what to expect.
type TestCase struct {
    Name           string
    Path           string                 // request path, e.g. "/e2e/valid-token-allow"
    Claims         map[string]interface{} // JWT claims to mint (JWT scenarios only)
    BasicAuthUser  string                 // "user:password" (basic-auth scenarios only)
    BackendStatus  int                    // status the mock backend should return
    BackendBody    string                 // body the mock backend should return
    ExpectedStatus int                    // expected HTTP status from rest-rego
}

// RunCase sends one HTTP request to proxyURL and asserts the response.
// Returns true on pass, false on fail (prints reason to stdout).
func RunCase(proxyURL string, tc TestCase, token string) bool
```

Each scenario's `main.go` is responsible for:
1. Starting the servers it needs (`OIDCProvider`, `BackendServer`)
2. Starting `./bin/restrego` via `os/exec`, passing the scenario's committed `policies/` directory as `POLICY_DIR`
3. Running `RunCase` for each test case
4. Stopping everything with deferred `server.Close()` calls

The helpers binaries (`jwt/helpers/main.go`, etc.) are separate `package main` programs that start only the support servers on fixed ports and block until SIGINT. They share the same `OIDCProvider` and `BackendServer` types from the `shared` package — no separate constructor variants are needed.

### 5.2 `OIDCProvider`

An `httptest.Server` that implements the minimum OIDC discovery surface required by rest-rego:

- `GET /.well-known/openid-configuration` — returns a JSON document pointing `jwks_uri` at the `/jwks` endpoint on the same server
- `GET /jwks` — returns a `JWKS` document containing the public key(s)

Key generation uses `crypto/rsa` with a 2048-bit key pair generated fresh for every `OIDCProvider` instance. The well-known document also sets `id_token_signing_alg_values_supported` so rest-rego's `PostFetch` hook applies the algorithm to bare keys.

### 5.3 `TokenSigner`

Builds and signs JWTs compatible with the rest-rego JWT validator:

```go
type TokenClaims struct {
    Subject  string
    Issuer   string
    Audience []string
    Extra    map[string]interface{}   // arbitrary extra claims
    TTL      time.Duration            // defaults to 5 minutes
}

func (s *TokenSigner) Sign(claims TokenClaims) string
```

The signer uses the private key from the paired `OIDCProvider`. `Sign` returns a compact-serialised JWT string suitable for use in an `Authorization: Bearer <token>` header.

### 5.4 `BackendServer`

A per-path routing HTTP server with request capture:

```go
type BackendServer struct {
    // Captured requests, keyed by path (e.g. "/e2e/valid-token-allow")
    Requests map[string][]*http.Request
}

// Mount registers a handler at /e2e/<name> that captures requests
// and responds with the given status and body.
func (b *BackendServer) Mount(name string, status int, body string)

func (b *BackendServer) URL() string   // base URL, e.g. "http://127.0.0.1:PORT"
func (b *BackendServer) Close()
```

The assertion binary calls `Mount` for each test case before starting rest-rego. After `RunCase` returns, it inspects `Requests["/e2e/<name>"]` to verify what the backend received (path, headers, body).

The helpers binary calls `Mount` for every test case it exposes — because rest-rego is external in helpers mode, all paths must exist before the load begins. No assertion is made in helpers mode; the backend simply responds with the configured status and body.

All mounted handlers recognise two optional query parameters on incoming requests:

- **`?size=N`** — the response body is padded (or truncated) to exactly `N` bytes, allowing k6 scripts to vary response sizes through the same path without registering extra test cases.
- **`?cl=missing`** — `Content-Length` is suppressed entirely, forcing chunked transfer encoding or connection close.
- **`?cl=bad`** — `Content-Length` is set to the non-numeric string `"notanumber"`, producing a malformed header.
- **`?cl=N`** — `Content-Length` is set to exactly `N` bytes regardless of the actual body length; when `N` differs from `?size`, the response carries a mismatched `Content-Length`.

### 5.5 `htpasswd` helper

Generates a temporary htpasswd file with bcrypt-hashed entries:

```go
func WriteHTPasswd(dir string, users map[string]string) (string, error)
// Writes to the provided dir; caller is responsible for removing dir via Close().
```

Uses `golang.org/x/crypto/bcrypt` at cost 4 (minimum) for fast execution.

---

## 6. Test Scenarios

### 6.1 JWT Auth Tests (`e2e-tests/jwt/`)

| Scenario | Input | Expected outcome |
|---|---|---|
| Valid token, allow policy | Signed JWT with matching audience | `200`, backend receives request |
| Valid token, deny policy | Signed JWT, policy `allow := false` | `403`, backend receives nothing |
| Expired token, strict mode | Token with past `exp` | `401`, backend receives nothing |
| Expired token, permissive mode | Token with past `exp` | Request passes as anonymous |
| Missing token, strict mode | No `Authorization` header | `401` |
| Missing token, permissive mode | No `Authorization` header | Request passes as anonymous |
| Wrong audience | Token with audience `other-api` | `401` |
| JWT claims in policy input | Token with custom claim `role=admin` | Policy can read `input.jwt.role` |
| JWKS key rotation | Replace key in OIDC provider mid-test | Token signed with new key validates |

### 6.2 Basic Auth Tests (`e2e-tests/basicauth/`)

| Scenario | Input | Expected outcome |
|---|---|---|
| Valid credentials, allow policy | `alice:password` | `200`, backend receives request |
| Valid credentials, deny policy | `alice:password`, policy denies | `403` |
| Wrong password | `alice:wrongpassword` | `401` |
| Unknown user | `nobody:pass` | `401` |
| Missing credentials, strict mode | No `Authorization` header | `401` with `WWW-Authenticate: Basic` |
| Missing credentials, permissive mode | No `Authorization` header | Request passes as anonymous |
| Username in policy input | `alice:password`, allow policy reads `input.request.auth.user` | Policy sees `"alice"` |
| Password NOT in policy input | `alice:password` | `input.request.auth.password` is absent |

### 6.3 No-Auth Tests (`e2e-tests/noauth/`)

| Scenario | Input | Expected outcome |
|---|---|---|
| Allow policy | Any request | `200`, backend receives request |
| Deny policy | Any request | `403` |
| No auth header required | Request without `Authorization` | `200` (policy permitting) |
| Variable response size | `GET /e2e/noauth-allow?size=N` | Backend returns exactly N bytes; used to correlate response size with heap growth |
| Missing Content-Length | `GET /e2e/noauth-allow?cl=missing` | Backend omits `Content-Length`; caller receives full body; rest-rego must not crash or hang |
| Malformed Content-Length | `GET /e2e/noauth-allow?cl=bad` | Backend sets `Content-Length: notanumber`; rest-rego forwards response without crashing; no goroutine leak |
| Content-Length too small | `GET /e2e/noauth-allow?size=100&cl=50` | Backend sends 100 bytes but declares 50; rest-rego forwards response; no crash |
| Content-Length too large | `GET /e2e/noauth-allow?size=50&cl=100` | Backend sends 50 bytes but declares 100; caller receives 50 bytes; no hang or goroutine leak |

### 6.4 Policy Behaviour Tests (`e2e-tests/policy/`)

These tests use JWT auth as the vehicle but focus on cross-cutting policy features:

| Scenario | Policy rule | Expected outcome |
|---|---|---|
| URL label — static | `url := "/users/--"` | Prometheus `url` label is `/users/--`; backend still receives the original path |
| URL label — conditional | `url := "/v2" if input.jwt.version == "v2"` | Metrics label changes per token claim; backend path is unchanged |
| Blocked headers stripped | `X-Restrego-Var1` header present | Backend does not receive the header |
| Blocked headers in policy | `ExposeBlockedHeaders=true` + policy reads `input.request.blocked_headers` | Policy can read the value |
| Path array in policy | Request to `/a/b/c` | `input.request.path == ["a","b","c"]` |
| Method in policy input | `DELETE /resource` | `input.request.method == "DELETE"` |
| Backend response forwarded | Backend returns `201 Created` + body | Caller receives `201` + body |

---

## 7. k6 Integration

### 7.1 Two-Task Approach per Scenario

For load and memory-leak testing, each scenario directory contains a `Taskfile.yml` with two independent tasks meant to be run in separate terminals:

```yaml
# e2e-tests/jwt/Taskfile.yml
tasks:
  helpers:
    desc: Start OIDC provider and mock backend for the JWT scenario
    cmds:
      - go run ./e2e-tests/jwt/helpers

  restrego:
    desc: Start rest-rego binary pointed at the JWT helpers
    env:
      WELLKNOWN_OIDC: http://127.0.0.1:18182/.well-known/openid-configuration
      JWT_AUDIENCES: e2e-test-audience
      BACKEND_HOST: 127.0.0.1
      BACKEND_PORT: "18184"
      LISTEN_ADDR: :18181
      MGMT_ADDR: :18183
      POLICY_DIR: ./policies
    cmds:
      - rest-rego
```

Startup sequence:
1. `task helpers` — starts OIDC + backend; waits until the backend `/health` probe returns 200
2. `task restrego` — starts rest-rego; waits until its proxy port responds
3. k6 runs using `PROXY`, `TOKEN_URL`, and `MGMT_URL` env vars passed on the command line
4. After k6 completes, rest-rego remains running; Prometheus metrics continue to settle
5. Optionally re-run a k6 script (or a different one) against the same process
6. SIGINT both tasks to shut down

The `e2e-tests/k6run.sh` script automates steps 1–3: runs `setup.sh`, waits for both processes to be ready, then runs k6.

### 7.2 Helpers Server Endpoints

The helpers binary (`helpers/main.go`) exposes the following endpoints on its fixed port:

| Endpoint | Description |
|---|---|
| `GET /health` | Readiness probe; returns `200 OK` when the server is ready |
| `GET /e2e/token/{name}` | Mint a fresh JWT with the claims for the named test case (JWT scenarios only) |
| `GET /e2e/{name}` | Mock backend handler for the named test case |

k6 scripts receive the proxy URL and token URL directly via `--env` variables. No manifest discovery is needed.

### 7.3 k6 Script Structure

Every k6 script receives all URLs via `--env` variables — no manifest fetching, no setup phase:

```js
import http from 'k6/http';
import { check } from 'k6';
import { options as throughputOptions } from '../k6-lib/throughput.js';

const PROXY     = __ENV.PROXY;      // e.g. http://127.0.0.1:18181
const TOKEN_URL = __ENV.TOKEN_URL;  // e.g. http://127.0.0.1:18184/e2e/token/valid-token-allow

export const options = throughputOptions;

export default function () {
    const token = http.get(TOKEN_URL).body;
    const res = http.get(`${PROXY}/e2e/valid-token-allow`, {
        headers: { Authorization: `Bearer ${token}` }
    });
    check(res, { 'status is 200': r => r.status === 200 });
}
```

k6 scripts are invoked by `k6run.sh` (or manually) with `--env PROXY=... --env TOKEN_URL=...` already set from the scenario's environment. No manifest parsing or dynamic path discovery is needed.

### 7.4 k6 Scenario Types

Two k6 option presets are defined as shared JS modules in `e2e-tests/k6-lib/`:

| Preset file | Purpose | Typical config |
|---|---|---|
| `throughput.js` | Peak throughput / latency benchmark | 50 VUs, 30 s, p95 < 10 ms threshold |
| `load.js` | Sustained load, check for degradation | ramp 1 → 100 VUs over 5 min, then steady 5 min |

Each scenario's k6 script imports the appropriate preset and may override thresholds. Memory leak detection uses `memleak.sh` (see §7.5) rather than a k6 preset.

### 7.5 Memory Leak Detection Strategy

Memory baseline and final readings are taken by `memleak.sh` using `curl` and `awk` — no Prometheus scraping from inside k6. The k6 script used for the soak run uses the `load.js` preset (sustained VUs, no metric scraping).

The **no-auth scenario** is the canonical target for this script. No-auth eliminates JWT, OIDC, and credential-verification overhead so that any measured heap growth is attributable solely to the proxy path. JWT-specific memory behaviour is covered separately by US-010.

`TOKEN_URL` is an optional named argument (`--token-url=URL`). Omit it for no-auth scenarios; supply it for JWT scenarios so k6 can refresh tokens.

```bash
#!/usr/bin/env bash
# memleak.sh <mgmt-url> <proxy-url> <k6-script> [--token-url=URL] [--threshold-mb=50]

MGMT=$1; PROXY=$2; K6_SCRIPT=$3
TOKEN_URL=""; THRESHOLD_MB=50
for arg in "${@:4}"; do
  case "$arg" in
    --token-url=*)    TOKEN_URL="${arg#--token-url=}" ;;
    --threshold-mb=*) THRESHOLD_MB="${arg#--threshold-mb=}" ;;
  esac
done

heap() { curl -sf "$MGMT/metrics" | awk '/^go_memstats_heap_inuse_bytes / {print $2}'; }

K6_ARGS=(--env PROXY="$PROXY")
[[ -n "$TOKEN_URL" ]] && K6_ARGS+=(--env TOKEN_URL="$TOKEN_URL")

BASELINE=$(heap)
k6 run "${K6_ARGS[@]}" "$K6_SCRIPT"
sleep 30   # let GC settle
FINAL=$(heap)

awk -v b="$BASELINE" -v f="$FINAL" -v t="$THRESHOLD_MB" \
  'BEGIN { diff=(f-b)/1048576; printf "heap delta: %.1f MB\n", diff; exit (diff > t) ? 1 : 0 }'
```

Protocol for a full soak run (no-auth scenario):
1. Start helpers: `task -d e2e-tests/noauth helpers`
2. Start rest-rego: `task -d e2e-tests/noauth restrego`
3. Run: `./e2e-tests/memleak.sh http://127.0.0.1:18203 http://127.0.0.1:18201 e2e-tests/noauth/k6/noauth-allow.js`
4. `memleak.sh` exits non-zero if heap growth exceeds the threshold (default 50 MB)
5. Optionally run the k6 script again against the still-running rest-rego process

### 7.6 Port Allocation

To avoid conflicts when multiple scenarios run simultaneously, ports are allocated from a configurable base (default 18100). Each scenario uses a documented offset:

| Scenario | Helper ports (OIDC / backend) | Rest-rego proxy | Rest-rego mgmt |
|---|---|---|---|
| `jwt` | 18182 / 18184 | 18181 | 18183 |
| `basicauth` | — / 18194 | 18191 | 18193 |
| `noauth` | — / 18204 | 18201 | 18203 |
| `policy` | 18212 / 18214 | 18211 | 18213 |

---

## 8. Functional Requirements

### REQ-001: Standalone Execution

- Each scenario is a `package main` program invoked with `go run ./e2e-tests/<scenario>`
- The suite has no `_test.go` files and is invisible to `go test ./...`
- The `shared` package has no compile-time dependency on rest-rego's internal packages
- No Docker; no network calls leave the host machine
- In assert mode the binary exits 0 on pass and 1 on fail; all assertion output goes to stdout
- The compiled rest-rego binary must exist at `./bin/restrego` before any scenario can run; the binary is git-ignored and must be built separately (`task build` or equivalent)

### REQ-002: Self-Contained OIDC Provider

- `OIDCProvider` generates its own RSA key pair per test run
- Serves a standards-compliant `/.well-known/openid-configuration` and `/jwks` endpoint
- `rest-rego` must be able to start with `WELLKNOWN_OIDC` pointing at the provider's loopback URL

### REQ-003: Token Signing

- `TokenSigner` must produce tokens that pass rest-rego's full JWT validation chain (signature, audience, expiry)
- Must support arbitrary extra claims in the token body so policy input tests can read them

### REQ-004: Backend Observability

- In assert mode: `Requests[name]` must contain the request(s) the backend received for each named test case, enabling assertions on forwarded path, headers, and body
- In helpers mode: the backend routes each registered test case to its own path handler; no assertion is made — the backend simply responds with the configured status/body
- Both modes: test cases must configure the backend response (status, headers, body) per `TestCase`

### REQ-005: Fast Assertion Mode

- `htpasswd` entries use bcrypt cost 4 to avoid slow execution
- OIDC key pair generation uses 2048-bit RSA (balance between speed and realistic key size)
- Each scenario binary in assert mode must complete in under 10 seconds

### REQ-010: Helpers Mode

- Every scenario has a separate `helpers/main.go` program that starts only the OIDC provider and mock backend on fixed, documented localhost ports
- The backend exposes `GET /health` as a readiness probe and `GET /e2e/{name}` for each test case path
- The JWT helpers binary additionally exposes `GET /e2e/token/{name}` to mint fresh JWTs
- The helpers binary blocks until SIGINT/SIGTERM, then calls `server.Close()` on each server and exits cleanly
- Rest-rego is **not** started by the helpers binary; it is started independently via `task restrego`

### REQ-014: Per-Scenario Taskfile

- Each scenario directory (`jwt/`, `basicauth/`, `noauth/`, `policy/`) contains a `Taskfile.yml`
- Each `Taskfile.yml` defines exactly two tasks: `helpers` and `restrego`
- The `restrego` task starts the compiled rest-rego binary with env vars pre-configured for that scenario's fixed ports and policy directory
- The `restrego` task sets `MGMT_ADDR` so rest-rego's Prometheus endpoint is reachable for metrics collection

### REQ-011: k6 Script per Scenario

- Every test scenario listed in §6 must have at least one corresponding k6 script
- Each k6 script accepts `PROXY` and `TOKEN_URL` (JWT scenarios) via `--env`
- Each k6 script imports one of the two standard presets (`throughput`, `load`) from `e2e-tests/k6-lib/`
- k6 scripts must not contain hardcoded URLs or tokens

### REQ-012: Memory Leak Detection

- `e2e-tests/memleak.sh` captures `go_memstats_heap_inuse_bytes` from rest-rego's `/metrics` endpoint before and after a k6 run, using `curl` and `awk`
- A 30-second settle period elapses after k6 exits before the final reading is taken
- A configurable growth threshold (default 50 MB) triggers a non-zero exit code from `memleak.sh`
- The k6 script used for the soak run uses the `load.js` preset (no Prometheus scraping from inside k6)

### REQ-013: k6 Run Helper

- `e2e-tests/k6run.sh <scenario-dir> <k6-script>` runs `setup.sh` for the given scenario, waits for both helpers and rest-rego to be ready, runs k6 with the correct `--env` variables, and exits with k6's exit code
- Both helper and rest-rego processes are left running after k6 exits (to allow metrics settling and re-runs); the script prints their PIDs
- The script accepts an optional `--shutdown` flag to also run `teardown.sh` after k6 completes

### REQ-006: Isolation Within a Scenario

- Each assertion case in a scenario binary starts its own `./bin/restrego` subprocess and stops it before the next case runs
- No restrego subprocess or server is shared between assertion cases
- Cleanup (subprocess termination, server shutdown) is done with `defer server.Close()` and `defer cmd.Process.Kill()`

### REQ-007: Rest-Rego Instantiation Strategy

- Rest-rego is always run as the compiled binary at `./bin/restrego`
- The assertion binary starts it via `os/exec`, passes all configuration as environment variables, and kills it after the case completes
- Both assertion and helpers modes pass the scenario's committed `policies/` directory as `POLICY_DIR`
- After starting the subprocess, the assertion binary probes rest-rego's proxy port and returns an error if it does not become ready within a configurable timeout (default 5 s)

### REQ-008: Auth Mode Coverage

- Separate scenario binaries for each auth mode: `jwt/main.go`, `basicauth/main.go`, `noauth/main.go`
- Cross-cutting policy-focused assertions run in `policy/main.go`

---

## 8. Technical Considerations

### 8.1 Rest-Rego as a Subprocess

The test suite never imports rest-rego's internal packages. The binary at `./bin/restrego` is started via `os/exec` with all configuration passed as environment variables:

```
os/exec.Command("./bin/restrego")
    Env: LISTEN_ADDR, BACKEND_HOST, BACKEND_PORT,
         WELLKNOWN_OIDC / BASIC_AUTH_FILE / NO_AUTH,
         JWT_AUDIENCES, POLICY_DIR, MGMT_ADDR
```

In **assertion binaries**: ports are chosen at startup (random high port for the proxy, ephemeral ports for helpers started in-process). `MGMT_ADDR` is omitted or set to a disabled value — the management server is not needed for assertions.

In **helpers mode** (`task restrego`): rest-rego is started separately by the Taskfile with the documented fixed ports and `MGMT_ADDR` set for Prometheus scraping.

### 8.2 OIDC Well-Known Bootstrap

`jwtsupport.New` fetches the well-known document at startup. The `OIDCProvider` server must be started **before** `./bin/restrego` is launched so the HTTP fetch can succeed. The assertion binary handles this ordering explicitly.

### 8.3 JWT Audience Configuration

The assertion binary configures rest-rego with a fixed test audience (`e2e-test-audience`). `TokenSigner.Sign` defaults to this audience unless overridden, so tests that want to exercise a wrong-audience case pass a different audience explicitly.

### 8.4 bcrypt Cost for Tests

Standard cost (12+) would add ~300ms per credential verification. Cost 4 reduces this to ~1ms, making parallel test suites practical. Cost 4 is only acceptable in test code; production htpasswd files must use cost ≥ 12.

### 8.5 Metrics Server

In assert mode the management server is not started — only the proxy server is needed.

In helpers mode, the management server is part of the separately-started rest-rego process. The `restrego` Taskfile task sets `MGMT_ADDR` to the scenario's documented mgmt port. No flag is needed on the scenario binary itself.

### 8.6 Readiness Probing

After starting the helpers binary, `setup.sh` polls `GET /health` on the backend server until it returns 200 (or times out). After starting rest-rego, `setup.sh` polls the proxy port until it accepts connections. `k6run.sh` waits for `setup.sh` to complete before starting k6, ensuring both processes are ready before load begins.

### 8.7 k6 Token Refresh

JWT tokens have a TTL (default 5 min). Long-running k6 scripts refresh tokens by calling `GET /e2e/token/{name}` on the helpers server — the endpoint mints a fresh JWT each time it is called. The helpers server is independent of rest-rego, so token minting remains available regardless of rest-rego's state. k6 scripts pass `TOKEN_URL` via `--env` and call it at the start of each iteration (or every N iterations for performance).

### 8.8 Dependencies

The `shared` package has no compile-time dependency on rest-rego's internal packages. It uses only:

- Standard library: `net/http`, `os/exec`, `crypto/rsa`, `crypto/rand`, `encoding/json`, `os`, `sync`
- Already-required module: `github.com/lestrrat-go/jwx/v2` (JWT signing)
- Already-required module: `golang.org/x/crypto/bcrypt` (htpasswd generation)

The compiled `./bin/restrego` binary is a runtime dependency, not a compile-time one. The `k6run.sh` script and scenario binaries check for its existence and exit with a clear error if absent.

For k6 scripts, k6 must be installed separately (`k6 >= 0.50`). It is not a Go module dependency. The `k6run.sh` script checks for k6 in `PATH` and exits with a clear error if absent.

### 8.9 Policy Path Convention

Because all test cases share a single rest-rego process in helpers mode, the committed `policies/` directory must handle all registered test-case paths. The recommended convention is:

- All test-case paths are under `/e2e/`
- Test cases that expect `200` use names ending in `-allow` (e.g., `valid-token-allow`)
- Test cases that expect `403` use names ending in `-deny` (e.g., `wrong-audience-deny`)
- The policy uses the final path segment's suffix to determine `allow`: `allow if endswith(input.request.path[1], "-allow")`

This means the policy for each scenario is predictable and does not need to change when new test cases are added, as long as the naming convention is followed.

---

## 9. User Stories

### US-001: JWT Allow/Deny Cycle

- **Description**: As a developer, I want a test that sends a valid JWT and verifies both that rest-rego allows it and that the backend receives the proxied request, and separately that a deny policy results in a 403 with the backend receiving nothing.
- **Acceptance criteria**:
  - Assertion binary starts `OIDCProvider`, `BackendServer`, and `./bin/restrego` subprocess with an allow policy; client sends `Authorization: Bearer <valid-token>`; asserts `200`; asserts `BackendServer.Requests["/e2e/valid-token-allow"]` is non-empty
  - Second run with a deny policy; asserts `403`; asserts `BackendServer.Requests["/e2e/valid-token-deny"]` is empty

### US-002: JWT Claims Reach Policy

- **Description**: As a policy author, I want to verify that custom JWT claims are available in `input.jwt` inside my Rego policy.
- **Acceptance criteria**:
  - Token is signed with claim `"department": "engineering"`
  - Policy reads `input.jwt.department == "engineering"` as the allow condition
  - Test asserts `200` when the claim is present; `403` when it is absent

### US-003: Basic Auth Username Reaches Policy

- **Description**: As a policy author, I want to verify that the authenticated username is available in `input.request.auth.user`.
- **Acceptance criteria**:
  - Test configures user `alice` in the htpasswd file
  - Policy allows only if `input.request.auth.user == "alice"`
  - Request with `alice:password` returns `200`; request with `bob:password` returns `403`

### US-004: Basic Auth Password Never in Policy Input

- **Description**: As a security reviewer, I want to verify that passwords are never forwarded to the Rego policy.
- **Acceptance criteria**:
  - Policy attempts to read `input.request.auth.password`; test asserts this field evaluates to `undefined` (deny by default)
  - Verified by a policy that `allow := input.request.auth.password == "anything"` which must always deny

### US-005: URL Metric Label Normalisation

- **Description**: As an operator, I want to verify that the Rego `url` rule controls the Prometheus metric label without affecting the path forwarded to the backend. This is used to collapse high-cardinality paths (e.g. `/user/123`, `/user/456` → `/user/--`) so dashboards and alerts are not flooded with unique label values.
- **Acceptance criteria**:
  - Request arrives at rest-rego as `GET /user/123`
  - Policy sets `url := "/user/--"`
  - `BackendServer` receives the request at the **original** path `/user/123` (backend path is unchanged)
  - The Prometheus `http_requests_total` counter uses the label `url="/user/--"`
  - A second request to `/user/456` also records `url="/user/--"` in metrics, confirming label normalisation

### US-006: Permissive JWT Mode

- **Description**: As a developer, I want to verify that `permissive=true` allows requests with missing or invalid tokens to pass through with anonymous identity.
- **Acceptance criteria**:
  - Rest-rego is started with `permissive=true` and a policy that allows anonymous requests
  - Request without `Authorization` header returns `200`
  - Request with an expired token returns `200` (policy permitting)
  - `input.jwt` is nil/absent in the policy input for anonymous requests

### US-007: Blocked Headers

- **Description**: As a security reviewer, I want to verify that `X-Restrego-*` headers are stripped from requests forwarded to the backend.
- **Acceptance criteria**:
  - Client sends `X-Restrego-Var1: secret` in the request
  - `BackendServer.LastRequest.Header.Get("X-Restrego-Var1") == ""`
  - When `ExposeBlockedHeaders=true`, the policy can read `input.request.blocked_headers["X-Restrego-Var1"] == "secret"`

### US-008: JWKS Key Rotation

- **Description**: As an operator, I want to verify that rest-rego continues to validate tokens after a JWKS key rotation without restart.
- **Acceptance criteria**:
  - Token signed with key-pair A validates successfully
  - `OIDCProvider` rotates to key-pair B; JWKS cache refresh is triggered (or TTL-based)
  - Token signed with key-pair B validates successfully after rotation

### US-009: k6 Throughput Benchmark

- **Description**: As a developer, I want to run a k6 throughput benchmark against the JWT scenario to establish a latency baseline and catch performance regressions.
- **Acceptance criteria**:
  - `k6run.sh e2e-tests/jwt e2e-tests/jwt/k6/jwt-allow.js` starts helpers and rest-rego, runs k6, and exits
  - k6 reports p95 latency and requests/s; script fails if p95 > 10 ms threshold
  - Both helpers and rest-rego processes are still running after k6 exits

### US-010: Memory Leak Detection Run

- **Description**: As a developer, I want to run a 30-minute k6 soak test against the JWT scenario, then observe that heap metrics have settled before concluding there is no leak.
- **Acceptance criteria**:
  - `task -d e2e-tests/jwt helpers` and `task -d e2e-tests/jwt restrego` start their respective processes
  - `k6run.sh e2e-tests/jwt e2e-tests/jwt/k6/jwt-memory.js` runs for 30 min at 10 VUs
  - Rest-rego continues running after k6 finishes; a post-settle heap reading is taken
  - k6 exits non-zero if heap growth exceeds 50 MB; exits zero otherwise
  - The scenario can be run a second time against the already-running rest-rego process

### US-011: Token Refresh During Long k6 Run

- **Description**: As a developer running the load or memleak k6 presets, I want tokens to be refreshed automatically from the helpers server so the run does not fail with 401 after token expiry.
- **Acceptance criteria**:
  - k6 script calls `GET /e2e/token` on the helpers server (not rest-rego) every 4 minutes
  - Refreshed token is used for subsequent virtual user iterations
  - No 401 responses appear in k6 output during a 30-minute run

### US-012: Memory Leak Detection — Variable Response Size (No-Auth)

- **Description**: As a developer, I want to run a soak test in no-auth mode where the k6 script cycles through increasing response sizes, to determine whether the size of proxied response bodies contributes to heap growth in rest-rego. No-auth mode removes JWT overhead so any heap growth is attributable to the proxy path itself.
- **Acceptance criteria**:
  - `task -d e2e-tests/noauth helpers` and `task -d e2e-tests/noauth restrego` start their respective processes
  - `k6run.sh e2e-tests/noauth e2e-tests/noauth/k6/noauth-memory.js` runs a 30-min soak at 10 VUs
  - The k6 script cycles through response sizes (e.g. 1 KB, 10 KB, 100 KB, 1 MB) by appending `?size=N` to the request URL
  - Each size step runs for a fixed interval (e.g. 5 min); heap readings are taken at the end of each interval
  - The script prints a per-size heap delta table; exits non-zero if any single step shows growth exceeding the configured threshold (default 20 MB)
  - The test can be re-run against the already-running rest-rego process to confirm results are consistent

### US-013: Content-Length Mismatch Handling (No-Auth)

- **Description**: As a developer, I want to verify that rest-rego handles backend responses where `Content-Length` is missing, non-numeric, smaller than the actual body, or larger than the actual body — ensuring the proxy does not crash, leak goroutines, or silently corrupt the response.
- **Acceptance criteria**:
  - `GET /e2e/noauth-allow?cl=missing` — caller receives the full body with no `Content-Length` crash; rest-rego exits cleanly after the request
  - `GET /e2e/noauth-allow?cl=bad` — rest-rego forwards the response without panicking; caller receives a non-5xx status; goroutine count does not grow
  - `GET /e2e/noauth-allow?size=100&cl=50` (under-declared) — rest-rego forwards the response; actual bytes received and proxy behavior are both documented in the test output; no crash
  - `GET /e2e/noauth-allow?size=50&cl=100` (over-declared) — caller receives the 50-byte body; connection is not left hanging; no goroutine leak
  - All four cases are exercised as assertion cases in `go run ./e2e-tests/noauth` and the binary exits 0
  - `k6run.sh e2e-tests/noauth e2e-tests/noauth/k6/noauth-content-length.js` runs a 5-minute stress test cycling through all four `?cl=` variants; exits non-zero if any iteration returns a 5xx or if goroutine count (scraped from `/metrics`) grows by more than 50 above baseline

---

## 10. Milestones and Sequencing

### 10.1 Phase 1 — Shared Infrastructure

Deliverables: `e2e-tests/shared/` package complete and all types compile. No `testing` package import.

- `OIDCProvider` with RSA key generation
- `TokenSigner`
- `BackendServer` with request capture and `Mount`
- `htpasswd` helper (bcrypt cost 4)
- `RunCase` function

### 10.2 Phase 2 — JWT Scenario Binary

Deliverables: `go run ./e2e-tests/jwt` exits 0 in assert mode.

- All scenarios from §6.1 implemented as assertion cases in `jwt/main.go`
- Uses shared OIDC provider and token signer

### 10.3 Phase 3 — Basic Auth Scenario Binary

Deliverables: `go run ./e2e-tests/basicauth` exits 0 in assert mode.

- All scenarios from §6.2 in `basicauth/main.go`
- Uses shared htpasswd helper

### 10.4 Phase 4 — No-Auth and Policy Scenario Binaries

Deliverables: `go run ./e2e-tests/noauth` and `go run ./e2e-tests/policy` exit 0 in assert mode.

- All scenarios from §6.3 and §6.4

### 10.5 Phase 5 — Helpers Binaries and Per-Scenario Taskfiles

Deliverables: `helpers/main.go` working for all four scenarios; each scenario has `setup.sh`, `teardown.sh`, and `Taskfile.yml`.

- Fixed-port binding in each helpers binary; `/health` readiness probe
- `/e2e/token/{name}` refresh endpoint on JWT helpers server
- Per-scenario `Taskfile.yml` with `helpers` and `restrego` tasks
- Per-scenario `setup.sh` / `teardown.sh` scripts
- Committed `policies/` directories under each scenario with the scenario's Rego source

### 10.6 Phase 6 — k6 Infrastructure and Scripts

Deliverables: `k6-lib/` presets complete; `k6run.sh` and `memleak.sh` written; all scenario k6 scripts present.

- `k6-lib/throughput.js`, `k6-lib/load.js` presets
- `memleak.sh` with configurable threshold; reads heap via `curl`/`awk`
- `k6run.sh` with `--shutdown` flag
- JWT allow/deny scripts
- Basic auth allow script
- No-auth allow script
- No-auth memory + variable response-size script (`noauth-memory.js`)
- No-auth Content-Length mismatch stress script (`noauth-content-length.js`)
- URL label normalisation and blocked-headers scripts

### 10.7 Phase 7 — Taskfile Integration

Deliverables: New `task e2e:*` targets added alongside existing tasks; no existing targets modified.

- Add `task e2e:assert` — runs all four scenario binaries in assert mode sequentially; sits beside existing `task test`, does not replace it
- Add `task e2e:perf` — invokes `k6run.sh` for throughput benchmarking across all scenarios
- Add `task e2e:memleak` — invokes `k6run.sh` with the `memleak` preset for memory growth detection
- Document in `e2e-tests/README.md`
