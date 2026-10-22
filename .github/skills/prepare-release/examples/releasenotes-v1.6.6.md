# Changes since v1.6.5

## Security / Fixes

- **Toolchain**
  - Minimum Go version raised from 1.27.2 to 1.27.3, so builds use the patched standard library. This fixes [GO-2026-6809](https://pkg.go.dev/vuln/GO-2026-6809) (CVE-2026-111943) in `crypto/x509`, where `Certificate.Verify` could accept known intermediate certificates that are no longer trusted.

## Tests

- No new tests. The full test suite passes on Go 1.27.3.

## Upgrade notes

- No configuration or behaviour changes. Building from source now requires Go 1.27.3 or newer.
