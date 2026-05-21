# E2E Tests

End-to-end tests for rest-rego. Currently covers the **no-auth** scenario with memory leak detection (REQ-012).

## Prerequisites

- **restrego binary** — build it before running any scenario:
  ```bash
  task build
  # or: go build -o bin/restrego ./cmd
  ```
- **k6 v2.x** — must be in `PATH`; verify with `k6 version`
- **curl** and **awk** — standard on Linux and macOS

## Memory Leak Detection (REQ-012)

Captures `go_memstats_heap_inuse_bytes` before and after a sustained k6 soak run and fails if heap growth exceeds a configurable threshold (default 50 MB).

### Quick Start

Open three terminals from the repository root.

**Terminal 1 — mock backend**

```bash
task -d e2e-tests/noauth helpers
```

**Terminal 2 — rest-rego proxy**

```bash
task -d e2e-tests/noauth restrego
```

**Terminal 3 — memory leak check**

```bash
./e2e-tests/memleak.sh \
  http://127.0.0.1:18203 \
  http://127.0.0.1:18201 \
  e2e-tests/noauth/k6/noauth-allow.js
```

Optional flags:

```bash
./e2e-tests/memleak.sh \
  http://127.0.0.1:18203 \
  http://127.0.0.1:18201 \
  e2e-tests/noauth/k6/noauth-allow.js \
  --threshold-mb=100
```

### Port Allocation

| Component | Address |
|---|---|
| Proxy (LISTEN_ADDR) | `127.0.0.1:18201` |
| Management / metrics (MGMT_ADDR) | `127.0.0.1:18203` |
| Mock backend | `127.0.0.1:18204` |

## Interpreting Results

`memleak.sh` prints three lines and exits:

```
baseline heap: 12.3 MB
final heap: 14.1 MB
heap delta: 1.8 MB
```

- A delta of a few MB is normal GC variance — the Go runtime retains heap pages between GC cycles.
- A delta consistently above ~20–30 MB on a fresh process warrants investigation.
- Exit code `0` means the delta is within the threshold and k6 reported no failures.
- Exit code `1` means the threshold was breached, k6 failed, or both.

**Tuning the threshold for longer runs:**

```bash
# Allow up to 200 MB growth for a 30-minute soak
./e2e-tests/memleak.sh ... --threshold-mb=200
```

**Re-running k6 against an already-running proxy** — stop only Terminal 3, then run `memleak.sh` again. The baseline is re-read at the start of each invocation, so a warm process is a valid baseline.

## Troubleshooting

| Symptom | Likely cause | Resolution |
|---|---|---|
| `./bin/restrego: No such file or directory` | Binary not compiled | Run `task build` |
| `curl: (7) Failed to connect` on `/metrics` | restrego not started or wrong port | Verify Terminal 2 is running; check port 18203 |
| `heap()` returns empty string | Metric name not found in output | Run `curl http://127.0.0.1:18203/metrics \| grep heap_inuse` to inspect |
| k6 reports `403 Forbidden` on all iterations | Policy denying the request | Check `e2e-tests/noauth/policies/request.rego`; confirm the k6 URL ends in `-allow` |
| `bind: address already in use` | Another process on ports 18201–18204 | Run `lsof -i :18201` to identify; stop the conflicting process or adjust ports in `e2e-tests/noauth/Taskfile.yml` |
| `k6 v2.x required` | Wrong k6 version installed | Run `k6 version` to check; install k6 v2.x |
