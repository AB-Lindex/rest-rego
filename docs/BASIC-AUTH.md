# Basic Authentication

rest-rego supports HTTP Basic Auth as an authentication provider. Credentials are stored in an Apache 2.4 htpasswd file using bcrypt hashes, hot-reloaded on change, and the verified username is made available to Rego policies. Passwords are never forwarded to the policy engine.

## Table of Contents

- [Overview](#overview)
- [Generating Credentials](#generating-credentials)
- [Supported Hash Formats](#supported-hash-formats)
- [Policy Input](#policy-input)
- [Example Rego Policy](#example-rego-policy)
- [Kubernetes Secret Mounting](#kubernetes-secret-mounting)
- [Permissive Mode](#permissive-mode)
- [Hot-Reload](#hot-reload)
- [Security Notes](#security-notes)

## Overview

Enable Basic Auth by pointing `BASIC_AUTH_FILE` at an htpasswd file:

```bash
export BASIC_AUTH_FILE="/etc/rest-rego/users.htpasswd"
rest-rego
```

**Basic Auth is mutually exclusive with `AZURE_TENANT` and `WELLKNOWN_OIDC`.** Configuring more than one provider causes a startup failure.

On startup, rest-rego:

1. Reads and validates every entry in the htpasswd file
2. Rejects entries with unsupported hash formats (see [Supported Hash Formats](#supported-hash-formats))
3. Exits with an error if no valid bcrypt entries are found

## Generating Credentials

Use the `htpasswd` tool from the Apache `httpd-tools` / `apache2-utils` package. Always use bcrypt (`-B`) with a cost factor of at least 12 (`-C 12`):

```bash
# Create a new file with the first user
htpasswd -B -C 12 -c /etc/rest-rego/users.htpasswd alice

# Add more users to an existing file
htpasswd -B -C 12 /etc/rest-rego/users.htpasswd bob

# Verify a password interactively
htpasswd -v /etc/rest-rego/users.htpasswd alice
```

The resulting file looks like:

```
# rest-rego htpasswd
alice:$2y$12$WtZ16rjMlh5lXqcY4cI3ROJAb7Kh1GrzZ6UkqxS7vpBPHIY9wBZPi
bob:$2y$12$sN1qI8G5u1M9sFEPJvB.zO4mKklCb.zQKH1n7dXzb8GBHXQ1YDrC2
```

**Tip**: Higher cost factors (`-C 14`, `-C 15`) provide stronger security at the cost of slower verification. For most APIs, cost 12 is a good default.

## Supported Hash Formats

| Hash format             | Prefix   | Status                |
|-------------------------|----------|-----------------------|
| bcrypt (variant `$2y$`) | `$2y$`   | Accepted              |
| bcrypt (variant `$2b$`) | `$2b$`   | Accepted              |
| bcrypt (variant `$2a$`) | `$2a$`   | Accepted              |
| MD5 (APR1)              | `$apr1$` | Skipped — WARN logged |
| SHA-1                   | `{SHA}`  | Skipped — WARN logged |
| All other formats       | —        | Skipped — WARN logged |

rest-rego enforces minimum bcrypt cost:

| Cost  | Behaviour                                               |
|-------|---------------------------------------------------------|
| < 10  | Entry rejected, WARN logged                             |
| 10–11 | Entry accepted, WARN logged (below recommended minimum) |
| ≥ 12  | Entry accepted                                          |

## Policy Input

When a request carries a valid `Authorization: Basic …` header, rest-rego sets the following fields on `input.request.auth`:

| Field                         | Value                                          |
|-------------------------------|------------------------------------------------|
| `input.request.auth.kind`     | `"basic"`                                      |
| `input.request.auth.user`     | Authenticated username                         |
| `input.request.auth.password` | Always `""` — cleared before policy evaluation |

Requests without an `Authorization` header (or with a non-Basic scheme) are passed through as anonymous, with `input.request.auth` set to `null`.

## Example Rego Policy

```rego
package request.rego

import rego.v1

default allow := false

user := input.request.auth.user

# Admin endpoints: restricted users only
allow if {
    input.request.path[0] == "admin"
    user in {"alice"}
}

# Reports endpoints: authorized users
allow if {
    input.request.path[0] == "reports"
    user in {"alice", "bob"}
}
```

A simpler policy that allows any authenticated user:

```rego
package request.rego

import rego.v1

default allow := false

allow if {
    input.request.auth.kind == "basic"
    input.request.auth.user != ""
}

# Assign custom header forwarded to backend
user := input.request.auth.user
```

**Resulting header:** `X-Restrego-User: alice`

## Kubernetes Secret Mounting

Store the htpasswd file as a Kubernetes Secret and mount it into the sidecar container.

### Create the Secret

```bash
# Create the htpasswd file locally
htpasswd -B -C 12 -c users.htpasswd alice
htpasswd -B -C 12 users.htpasswd bob

# Create the Secret
kubectl create secret generic rest-rego-htpasswd \
  --from-file=users.htpasswd \
  --namespace=your-namespace
```

### Pod Spec

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: my-service
spec:
  template:
    spec:
      containers:
        - name: rest-rego
          image: lindex/rest-rego:latest
          env:
            - name: BASIC_AUTH_FILE
              value: /users/users.htpasswd
            - name: BACKEND_PORT
              value: "8080"
          volumeMounts:
            - name: htpasswd
              mountPath: /etc/rest-rego
              readOnly: true
        - name: app
          image: your-app:latest
      volumes:
        - name: htpasswd
          secret:
            secretName: rest-rego-htpasswd
```

**Note**: When mounted from a Kubernetes Secret the file is updated in the pod automatically when the Secret changes. rest-rego's `fsnotify`-based hot-reload picks up the new file without requiring a pod restart.

## Permissive Mode

Set `PERMISSIVE_AUTH=true` to allow anonymous access alongside authenticated requests:

```bash
export BASIC_AUTH_FILE="/etc/rest-rego/users.htpasswd"
export PERMISSIVE_AUTH=true
rest-rego
```

| Scenario                  | `PERMISSIVE_AUTH=false` (default)   | `PERMISSIVE_AUTH=true`              |
|---------------------------|-------------------------------------|-------------------------------------|
| No `Authorization` header | `null` auth, pass to policy         | `null` auth, pass to policy         |
| Unknown username          | `401 Unauthorized`                  | `null` auth, pass to policy         |
| Correct password          | Pass to policy with `auth.user` set | Pass to policy with `auth.user` set |
| Wrong password            | `401 Unauthorized`                  | `401 Unauthorized`                  |

**Wrong passwords always return `401 Unauthorized` regardless of permissive mode.** This prevents credential-stuffing attacks from silently downgrading to anonymous access.

See [PERMISSIVE.md](PERMISSIVE.md) for complete documentation, including how to detect anonymous requests in the backend service.

## Hot-Reload

rest-rego watches the htpasswd file with `fsnotify` and reloads credentials atomically on every write or replace. During a reload:

- Requests in flight continue to use the previous credential set
- The new credential map replaces the old one atomically after a successful parse
- If the new file is invalid or empty, rest-rego retains the last valid credential set and logs an error

No restart is required after updating the htpasswd file.

## Security Notes

- **Passwords never reach Rego**: The `password` field of `input.request.auth` is always an empty string. Policies cannot access raw passwords.
- **Timing-safe comparison**: bcrypt comparison is inherently constant-time for the hash length; no additional timing mitigations are needed.
- **TLS strongly recommended**: HTTP Basic Auth credentials are base64-encoded, not encrypted. Always terminate TLS at an ingress or sidecar before traffic reaches rest-rego.
- **Minimum cost enforcement**: Entries with bcrypt cost < 10 are rejected at load time to prevent trivially crackable stored hashes.
