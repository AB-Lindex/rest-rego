---
goal: E2E Memory Leak Detection — Implement and Run REQ-012
version: "1.0"
date_created: 2026-05-21
last_updated: 2026-05-21
owner: rest-rego team
status: 'Planned'
tags: [feature, e2e-tests, observability, memory, k6]
---

# Introduction

![Status: Planned](https://img.shields.io/badge/status-Planned-blue)

This plan delivers the minimum infrastructure required to **run and diagnose REQ-012** (Memory Leak Detection) from the e2e-tests PRD. The scope is limited to exactly what is needed to execute `e2e-tests/memleak.sh` against the **no-auth scenario**: the `BackendServer` shared type, the no-auth helpers binary and its Taskfile, the `load.js` k6 preset, a no-auth soak k6 script, and the `memleak.sh` script itself.

No-auth mode eliminates JWT, OIDC, and credential-verification overhead, so any measured heap growth is attributable solely to the proxy path — making it the correct diagnostic target for REQ-012. JWT-specific memory behaviour is a separate concern covered by US-010.

The full e2e assertion suite (Phases 2–4 of the parent PRD) is out of scope for this plan.

## 1. Requirements & Constraints

- **REQ-012**: `e2e-tests/memleak.sh` captures `go_memstats_heap_inuse_bytes` from rest-rego's `/metrics` endpoint before and after a k6 soak run, using `curl` and `awk`
- **REQ-012a**: A 30-second GC settle period elapses after k6 exits before the final reading is taken
- **REQ-012b**: A configurable growth threshold (default 50 MB) triggers a non-zero exit code from `memleak.sh`
- **REQ-012c**: The k6 script used for the soak run uses the `load.js` preset — no Prometheus scraping from inside k6
- **REQ-010**: No-auth helpers binary (`e2e-tests/noauth/helpers/main.go`) starts mock backend on fixed port; exposes `GET /health` and `GET /e2e/{name}`
- **REQ-014**: `e2e-tests/noauth/Taskfile.yml` defines exactly two tasks: `helpers` and `restrego`
- **REQ-011**: `e2e-tests/noauth/k6/noauth-allow.js` imports `load.js` and accepts `PROXY` via `--env`; no `TOKEN_URL` required
- **CON-001**: The shared package (`e2e-tests/shared`) must have zero compile-time dependency on rest-rego's internal packages
- **CON-002**: All components run on `127.0.0.1`; no network calls leave the host machine
- **CON-003**: `memleak.sh` verifies `curl`, `awk`, and `k6` are in `PATH` and exits 1 with a clear message if any is absent
- **CON-005**: k6 major version **2** must be installed — a new major release with potential breaking changes; every k6-related implementation step must verify API compatibility with 2.x before writing or finalising the file; `memleak.sh` checks that the installed version starts with `v2.` and exits 1 with a clear message if it does not
- **CON-004**: `memleak.sh` preserves k6's exit code — k6 failures are not masked by the heap comparison
- **GUD-001**: No-auth scenario port allocation: backend `18204`, proxy `18201`, mgmt `18203`
- **GUD-002**: rest-rego no-auth mode enabled via env var `NO_AUTH=true`

Update the status tag on each task (`[📋 Planned]` → `[⏳ In Progress]` → `[✅ Completed: YYYY-MM-DD]`) as work progresses.

## 1.1. Repository Context

- **Repository Type**: Single-Product
- **PRD**: `/.specs/PRD.md`
- **E2E PRD**: `/.specs/PRD-e2e-tests.md`
- **Technology Stack**: Go, Bash, k6 JavaScript
- **Metrics endpoint confirmed**: `GET /metrics` is registered in `internal/application/mgmt.go`; `go_memstats_heap_inuse_bytes` is exposed via `collectors.NewGoCollector()` registered in `internal/metrics/metrics.go`
- **Module dependencies confirmed**: `github.com/lestrrat-go/jwx/v2 v2.1.6` and `golang.org/x/crypto v0.49.0` already present in `go.mod`

## 2. Implementation Steps

### Implementation Phase 1 — Shared Go Package

- **GOAL-001**: Create the `e2eshared` package in `e2e-tests/shared/`. Only `BackendServer` and the `htpasswd` helper are required for the no-auth scenario; `OIDCProvider` and `TokenSigner` are deferred to the full suite delivery. The package must have no dependency on rest-rego's internal packages.

- **TASK-001**: Create `e2e-tests/shared/backend_server.go` `[✅ Completed: 2026-05-21]`
  - Package declaration: `package e2eshared`
  - Struct: `BackendServer` with fields `server *httptest.Server`, `mux *http.ServeMux`, `mu sync.Mutex`, `Requests map[string][]*http.Request`
  - Constructor: `NewBackendServer(port int) (*BackendServer, error)` — binds to `127.0.0.1:<port>`; registers `GET /health` returning `200 OK`
  - Method: `Mount(name string, status int, body string)` — registers handler at `/e2e/<name>`; captures each incoming `*http.Request` into `Requests["/e2e/<name>"]` under mutex; then applies query-parameter overrides before writing the response:
    - `?size=N` — pad body with `'x'` or truncate to exactly N bytes
    - `?cl=missing` — call `w.Header().Del("Content-Length")` and flush body without setting it; set `Transfer-Encoding: chunked`
    - `?cl=bad` — write response header `Content-Length: notanumber` via `w.Header().Set("Content-Length","notanumber")` before `WriteHeader`
    - `?cl=N` — write `Content-Length: N` explicitly regardless of actual body length
  - Exported methods: `URL() string`, `Close()`

- **TASK-002**: Create `e2e-tests/shared/htpasswd.go` `[� Ignored]`
  - **Reason**: This plan targets the no-auth scenario exclusively; htpasswd/bcrypt support is not required. Defer to the basic-auth scenario plan.
  - ~~Package declaration: `package e2eshared`~~
  - ~~Function: `WriteHTPasswd(dir string, users map[string]string) (string, error)`~~
  - ~~For each `user → password` entry: generate bcrypt hash at cost 4 using `golang.org/x/crypto/bcrypt`; write line `user:$2a$04$<hash>` to file~~
  - ~~Output file path: `filepath.Join(dir, "users.htpasswd")`; returns absolute path on success~~

### Implementation Phase 2 — No-Auth Helpers Binary and Taskfile

- **GOAL-002**: Create the no-auth helpers binary that binds the mock backend to a fixed documented port and provides a `/health` readiness probe. Create the scenario Taskfile so `memleak.sh` can be run against a long-lived process.

- **TASK-003**: Create `e2e-tests/noauth/helpers/main.go` `[✅ Completed: 2026-05-21]`
  - Package declaration: `package main`
  - Import `e2e-tests/shared` (module path `github.com/AB-Lindex/rest-rego/e2e-tests/shared`)
  - Start `BackendServer` on port `18204`
  - Mount the following backend path — returns `200 OK`, body `"ok"`:
    - `noauth-allow`
  - Print to stdout on startup: `BACKEND: http://127.0.0.1:18204`
  - Block on `signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)`
  - On context cancellation: call `backendServer.Close()`, `os.Exit(0)`

- **TASK-004**: Create `e2e-tests/noauth/policies/request.rego` `[✅ Completed: 2026-05-21]`
  - Package declaration: `package policies`
  - Default: `default allow := false`
  - Allow rule: `allow if { endswith(input.request.path[count(input.request.path)-1], "-allow") }`
  - This single rule handles all test case paths: paths ending in `-allow` are permitted; all others are denied — no per-case policy changes are needed when new test cases are added

- **TASK-005**: Create `e2e-tests/noauth/Taskfile.yml` `[✅ Completed: 2026-05-21]`
  - `version: "3"`
  - Task `helpers`:
    - `desc: Start mock backend for the no-auth scenario`
    - `cmds: ["go run github.com/AB-Lindex/rest-rego/e2e-tests/noauth/helpers"]`
  - Task `restrego`:
    - `desc: Start rest-rego proxy for the no-auth scenario (requires helpers to be running)`
    - `env`:
      - `NO_AUTH: "true"`
      - `BACKEND_HOST: 127.0.0.1`
      - `BACKEND_PORT: "18204"`
      - `LISTEN_ADDR: :18201`
      - `MGMT_ADDR: :18203`
      - `POLICY_DIR: e2e-tests/noauth/policies`
    - `cmds: ["./bin/restrego"]`
    - `preconditions`: check that `./bin/restrego` exists with message `"run 'task build' first to compile the restrego binary"`

### Implementation Phase 3 — k6 Load Preset

- **GOAL-003**: Create the `load.js` shared k6 module imported by all soak and memleak k6 scripts. Must not include any Prometheus scraping.

- **TASK-008**: Create `e2e-tests/k6-lib/load.js` `[✅ Completed: 2026-05-21]`
  - **k6 2.0.0 verification required before writing this file**: Confirm that the `ramping-vus` executor name, `stages` array shape, `thresholds` schema, and `http_req_failed` built-in metric name are unchanged in k6 2.0.0. Run `k6 version` and consult the k6 2.0.0 migration guide / changelog; update the options object below if any name or shape has changed.
  - Export a named `options` constant conforming to k6's `Options` type:
    ```js
    export const options = {
      scenarios: {
        sustained: {
          executor: 'ramping-vus',
          startVUs: 1,
          stages: [
            { duration: '2m', target: 100 },
            { duration: '5m', target: 100 },
          ],
        },
      },
      thresholds: {
        http_req_failed: ['rate<0.01'],
      },
    };
    ```
  - No `http_req_duration` threshold — latency is not the focus of soak runs
  - No setup/teardown functions — all service management is external

### Implementation Phase 4 — No-Auth Soak k6 Script

- **GOAL-004**: Create the no-auth k6 soak script that `memleak.sh` runs. No token handling required — requests go straight to the proxy.

- **TASK-007**: Create `e2e-tests/noauth/k6/noauth-allow.js` `[✅ Completed: 2026-05-21]`
  - **k6 2.0.0 verification required before writing this file**: Confirm that `http.get()`, `check()`, `__ENV`, `fail()`, and local ES-module import syntax (`import { options } from '../../k6-lib/load.js'`) are all supported and unchanged in k6 2.0.0. Adjust any API calls that have been renamed or restructured in the 2.0.0 release.
  - Read env var: `const PROXY = __ENV.PROXY`
  - Import: `import { options } from '../../k6-lib/load.js'`; re-export: `export { options }`
  - Default function body:
    1. `const res = http.get(` `` `${PROXY}/e2e/noauth-allow` `` `);`
    2. `check(res, { 'status 200': r => r.status === 200 });`
  - No hardcoded URLs or tokens
  - Missing `PROXY` exits with `fail('PROXY env var is required')`

### Implementation Phase 5 — memleak.sh

- **GOAL-005**: Create `e2e-tests/memleak.sh` — the REQ-012 deliverable. Captures `go_memstats_heap_inuse_bytes` before and after a k6 run; prints a delta; exits non-zero if the delta exceeds the threshold.

- **TASK-008**: Create `e2e-tests/memleak.sh` `[✅ Completed: 2026-05-21]`
  - Shebang: `#!/usr/bin/env bash`
  - Strict mode: `set -euo pipefail`
  - Usage line printed to stderr on bad arguments: `Usage: memleak.sh <mgmt-url> <proxy-url> <k6-script> [--token-url=URL] [--threshold-mb=N]`
  - Require exactly 3 positional arguments; exit 1 on wrong count
  - Parse optional named arguments from `${@:4}`:
    - `--token-url=*` → `TOKEN_URL`
    - `--threshold-mb=*` → `THRESHOLD_MB` (default `50`)
  - Prerequisite checks (exit 1 with message if any fail):
    - `command -v curl`
    - `command -v awk`
    - `command -v k6`
    - `k6 version` output contains `v2.` — exit 1 with message `"k6 v2.x required, got: <actual version>"` if the major version is not 2
    - `[[ -f "$K6_SCRIPT" ]]`
  - `heap()` function: `curl -sf "${MGMT}/metrics" | awk '/^go_memstats_heap_inuse_bytes / {print $2}'`
  - `heap_mb()` function: prints heap in MB for human-readable output
  - Sequence:
    1. `BASELINE=$(heap)` — print baseline in MB
    2. Build k6 args array: always `--env PROXY="$PROXY"`; append `--env TOKEN_URL="$TOKEN_URL"` only if `TOKEN_URL` is non-empty
    3. Run k6 with the built args array; capture exit code in `K6_EXIT`; do not let `set -e` intercept k6's failure
    4. `echo "waiting 30s for GC to settle..."`; `sleep 30`
    5. `FINAL=$(heap)` — print final in MB
    6. `awk` comparison: print `"heap delta: %.1f MB"` and set exit code based on `(FINAL - BASELINE) / 1048576 > THRESHOLD_MB`
  - Final exit: `exit $(( awk_exit || K6_EXIT ))` — non-zero if either k6 failed or threshold was breached

### Implementation Phase 6 — Documentation

- **GOAL-006**: Create `e2e-tests/README.md` covering the prerequisites, quick-start invocation for REQ-012, result interpretation, and troubleshooting for the most common failure modes.

- **TASK-009**: Create `e2e-tests/README.md` `[✅ Completed: 2026-05-21]`
  - Section **Prerequisites**:
    - `./bin/restrego` must be built: `task build` or `go build -o bin/restrego ./cmd`
    - k6 **2.0.0** must be in `PATH`; verify with `k6 version`
    - `curl` and `awk` must be available (standard on Linux/macOS)
  - Section **Memory Leak Detection (REQ-012)**:
    - Three-terminal quick start with exact commands using no-auth ports (`18201`–`18204`)
    - Example `memleak.sh` invocation: `./e2e-tests/memleak.sh http://127.0.0.1:18203 http://127.0.0.1:18201 e2e-tests/noauth/k6/noauth-allow.js`
  - Section **Interpreting Results**:
    - `heap delta: X.Y MB` — what constitutes a leak vs. normal GC variance
    - Exit code 0 = within threshold; exit code 1 = threshold breached or k6 failure
    - How to tune `--threshold-mb` for longer runs
    - How to re-run k6 against the already-running restrego process
  - Section **Troubleshooting**:

    | Symptom | Likely cause | Resolution |
    |---|---|---|
    | `command not found: ./bin/restrego` | Binary not compiled | Run `task build` |
    | `curl: (7) Failed to connect` on `/metrics` | restrego not started or wrong MGMT_ADDR | Verify `task -d e2e-tests/noauth restrego` is running; check port 18203 |
    | `heap()` returns empty string | `/metrics` path doesn't match awk pattern | Run `curl http://127.0.0.1:18203/metrics \| grep heap_inuse` to inspect |
    | k6 returns `403 Forbidden` on all iterations | Policy denying; path does not end in `-allow` | Check `e2e-tests/noauth/policies/request.rego` and that the k6 URL ends in `-allow` |
    | Port already in use | Another process on 18201–18204 | `lsof -i :18201` to identify; kill or change ports in Taskfile |

## 3. Alternatives

- **ALT-001**: Use `pprof` heap snapshots via `/debug/pprof/heap` instead of Prometheus metrics — rejected because it requires Go tooling on the test machine and produces point-in-time snapshots that are harder to compare with a shell `awk` one-liner; the PRD explicitly specifies `curl` + `awk` with the Prometheus endpoint
- **ALT-002**: Emit heap readings from inside k6 using the `k6/x/prometheus-remote-write` extension — rejected by REQ-012c; no Prometheus scraping from inside k6 for the soak script
- **ALT-003**: Implement `memleak` as a Go binary in the shared package — rejected because a Bash script is more portable across CI environments and the PRD specifies `memleak.sh`
- **ALT-004**: Use the JWT scenario as the primary memleak target — rejected because JWT introduces OIDC fetch, JWK caching, and RS256 verification overhead that conflates with proxy-path allocations; no-auth isolates the proxy path. JWT-specific memory behaviour is separately covered by US-010.

## 4. Dependencies

- **DEP-001**: `golang.org/x/crypto v0.49.0` — in `go.mod`; not required by this plan (TASK-002 is ignored)
- **DEP-002**: `k6 2.0.0` — runtime dependency; must be installed separately; checked by `memleak.sh` at startup (version string match, not just presence). k6 2.0.0 is a new major release; breaking changes to executor names, options schema, built-in metric names, JS module imports, or CLI flags are possible — verify compatibility at each k6-related implementation step (TASK-007, load.js TASK-008)
- **DEP-003**: `curl` — runtime dependency for `memleak.sh`; present on standard Linux/macOS CI runners
- **DEP-004**: `awk` — runtime dependency for `memleak.sh`; present on standard Linux/macOS CI runners
- **DEP-005**: `./bin/restrego` compiled binary — built via `task build`; runtime dependency of the `restrego` Taskfile task

## 5. Files

- **FILE-001**: `e2e-tests/shared/backend_server.go` — configurable mock backend with request capture; `?size`, `?cl` query param handling; `/health` endpoint
- **FILE-002**: `e2e-tests/shared/htpasswd.go` — bcrypt htpasswd file generator (completes the shared package; not used by this scenario)
- **FILE-003**: `e2e-tests/noauth/helpers/main.go` — fixed-port backend binary on port `18204`; `/health` readiness probe
- **FILE-004**: `e2e-tests/noauth/policies/request.rego` — Rego allow rule based on path suffix (`-allow` / `-deny`)
- **FILE-005**: `e2e-tests/noauth/Taskfile.yml` — `helpers` and `restrego` tasks; `NO_AUTH=true`; precondition checks for `./bin/restrego`
- **FILE-006**: `e2e-tests/k6-lib/load.js` — sustained-load k6 preset (ramp 1→100 VUs over 2 min, hold 5 min)
- **FILE-007**: `e2e-tests/noauth/k6/noauth-allow.js` — no-auth soak k6 script; imports `load.js`; no token handling
- **FILE-008**: `e2e-tests/memleak.sh` — REQ-012 deliverable; prerequisite checks; optional `--token-url`; heap baseline/final capture; 30-second settle; delta comparison; dual exit code
- **FILE-009**: `e2e-tests/README.md` — quick-start, result interpretation, troubleshooting table

## 6. Testing

- **TEST-001**: `go build ./e2e-tests/shared` exits 0 — confirms shared package compiles with no dependency on rest-rego internals
- **TEST-002**: `go build ./e2e-tests/noauth/helpers` exits 0 — confirms helpers binary compiles
- **TEST-003**: `task -d e2e-tests/noauth helpers` starts; `curl -sf http://127.0.0.1:18204/health` returns `200 OK`
- **TEST-004**: `task -d e2e-tests/noauth restrego` starts; `curl -sf http://127.0.0.1:18203/metrics` returns a response containing `go_memstats_heap_inuse_bytes`
- **TEST-005**: `./e2e-tests/memleak.sh http://127.0.0.1:18203 http://127.0.0.1:18201 e2e-tests/noauth/k6/noauth-allow.js` exits 0 and prints a line matching `heap delta: [0-9.]+ MB`
- **TEST-006**: `./e2e-tests/memleak.sh ... --threshold-mb=0` exits 1 (zero threshold forces failure on any positive heap delta)
- **TEST-007**: `PATH="" ./e2e-tests/memleak.sh ...` exits 1 and prints a message identifying the missing tool (`curl`, `awk`, or `k6`)
- **TEST-008**: Running `memleak.sh` with a k6 binary that reports a version other than `2.0.0` exits 1 and prints a message containing the detected version

## 7. Risks & Assumptions

- **RISK-001**: Ports `18201`–`18204` may be in use on the developer machine or CI runner; the Taskfile and helpers binary do not perform port-availability checks before binding — callers must ensure ports are free
- **RISK-002**: The 30-second GC settle window may be insufficient after a very long soak run under sustained memory pressure; operators should tune `--threshold-mb` per environment and run duration
- **RISK-003**: `go_memstats_heap_inuse_bytes` reflects in-use heap pages, not total allocated objects; heap growth that the GC has already reclaimed within the 30-second window will not appear in the delta — this is intentional (the script detects sustained leaks, not transient spikes)
- **RISK-004**: k6 2.0.0 is a new major release; executor names (`ramping-vus`), options schema (`stages`, `thresholds`), built-in metric names (`http_req_failed`), JavaScript module import syntax, and CLI flags (`--env`) may have changed. Each k6-related task must consult the k6 2.0.0 migration guide before implementation; if a breaking change is found, update the affected task's specification before writing any code.
- **ASSUMPTION-001**: `./bin/restrego` exposes `/metrics` on `MGMT_ADDR` — verified; `GET /metrics` is registered in `internal/application/mgmt.go` via `metrics.Handler()`
- **ASSUMPTION-002**: `go_memstats_heap_inuse_bytes` is present in the `/metrics` output — verified; `collectors.NewGoCollector()` is registered in `internal/metrics/metrics.go`
- **ASSUMPTION-003**: `NO_AUTH=true` starts rest-rego without any authentication requirement — confirmed in `internal/config/config.go` (`NoAuth bool` with env var `NO_AUTH`)

## 8. Related Specifications / Further Reading

- [.specs/PRD-e2e-tests.md](.specs/PRD-e2e-tests.md) — full e2e PRD; REQ-012 at §8, §7.5; US-010 (memory leak detection run); US-012 (no-auth variable response size)
- [.specs/PRD.md](.specs/PRD.md) — parent product PRD
- [docs/OBSERVABILITY.md](../docs/OBSERVABILITY.md) — Prometheus metrics endpoint documentation
- [internal/metrics/metrics.go](../internal/metrics/metrics.go) — GoCollector registration (`go_memstats_heap_inuse_bytes` source)
- [internal/application/mgmt.go](../internal/application/mgmt.go) — `/metrics` HTTP handler registration and `MGMT_ADDR` configuration
