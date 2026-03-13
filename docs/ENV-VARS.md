# Environment Variable Expansion in Policies

rest-rego expands environment variables inside Rego policy files at load time — before OPA compiles the policy. This lets you inject runtime secrets (API keys, allowed client IDs, shared tokens) without hardcoding them in source files or Kubernetes ConfigMaps.

Expansion also runs on every **hot-reload**, so rotating a secret only requires updating the environment variable and touching the policy file (or restarting the pod).

## Syntax

Use standard shell parenthesis syntax:

```
$(VAR_NAME)
```

Any occurrence of `$(VAR_NAME)` in a `.rego` file is replaced with the value of the `VAR_NAME` environment variable before the policy is compiled.

### Configuring prefix and wrapper

Variable expansion is powered by [`github.com/ninlil/envsubst`](https://github.com/ninlil/envsubst). The prefix character and wrapper pair can be changed via environment variables if your policy variables or conventions require a different style:

| Option  | Env variable       | Default | Valid values    | Example  |
|---------|--------------------|---------|-----------------|----------|
| Prefix  | `ENVSUBST_PREFIX`  | `$`     | `$` `%` `&` `#` | `%{VAR}` |
| Wrapper | `ENVSUBST_WRAPPER` | `(`     | `{` `(` `[` `<` | `$(VAR)` |

The wrapper value is the **opening** character; the matching closing character is inferred automatically (`{` → `}`, `(` → `)`, `[` → `]`, `<` → `>`).

Examples:

| `ENVSUBST_PREFIX` | `ENVSUBST_WRAPPER` | Syntax used in policy |
|-------------------|--------------------|-----------------------|
| `$` (default)     | `(` (default)      | `$(VAR_NAME)`         |
| `%`               | `(`                | `%(VAR_NAME)`         |
| `$`               | `{`                | `${VAR_NAME}`         |
| `#`               | `[`                | `#[VAR_NAME]`         |

> **Security note**: Use the default `$(VAR_NAME)` syntax unless you have a specific reason to change it. Non-default prefixes may be less recognisable to reviewers auditing your policies.

## Example

Policy file (`policies/request.rego`):

```rego
package policies

default allow := false

allowed_apps := {
    "$(ALLOWED_APP_ID_1)",
    "$(ALLOWED_APP_ID_2)",
}

allow if {
    input.jwt.appid in allowed_apps
}
```

At startup, if `ALLOWED_APP_ID_1=11112222-3333-4444-5555-666677778888` is set, the compiled policy is equivalent to:

```rego
allowed_apps := {
    "11112222-3333-4444-5555-666677778888",
    "$(ALLOWED_APP_ID_2)",   # unset → empty string
}
```

## Behaviour

| Scenario                                       | Result                                                                |
|------------------------------------------------|-----------------------------------------------------------------------|
| Variable is set                                | Replaced with its value                                               |
| Variable is unset                              | Replaced with empty string `""`                                       |
| No placeholders in policy                      | Policy unchanged, no overhead                                         |
| Variable value contains Rego syntax characters | Characters are inserted as-is; ensure values are valid string content |

## Kubernetes Integration

Store secrets in a Kubernetes Secret and inject them into the sidecar container via `env`:

```yaml
# secret.yaml
apiVersion: v1
kind: Secret
metadata:
  name: restrego-secrets
  namespace: my-namespace
type: Opaque
stringData:
  allowed-app-id-1: "11112222-3333-4444-5555-666677778888"
  allowed-app-id-2: "22223333-4444-5555-6666-777788889999"
```

```yaml
# deployment.yaml (sidecar container)
- name: rest-rego
  image: lindex/rest-rego:latest
  env:
  - name: BACKEND_PORT
    value: "8080"
  - name: ALLOWED_APP_ID_1
    valueFrom:
      secretKeyRef:
        name: restrego-secrets
        key: allowed-app-id-1
  - name: ALLOWED_APP_ID_2
    valueFrom:
      secretKeyRef:
        name: restrego-secrets
        key: allowed-app-id-2
  volumeMounts:
  - name: policies
    mountPath: /policies
    readOnly: true
```

Alternatively, use `envFrom` to bulk-load all keys from a Secret:

```yaml
  envFrom:
  - secretRef:
      name: restrego-secrets
```

## Security Considerations

- Secrets are **never written to disk** — expansion happens in memory at load time only.
- Policy source files and ConfigMaps contain only placeholders, not secret values.
- If a variable is unset, it expands to `""`. A policy that compares against an empty string will effectively be inoperable for that rule — consider this when designing policies.
- Secrets are visible in the process environment. Ensure the pod's security context restricts access (non-root user, `readOnlyRootFilesystem: true`).

## Troubleshooting

**Placeholder not expanded (literal `$(VAR_NAME)` appears in evaluation):**
- Verify the environment variable is set in the sidecar container.
- Use `kubectl exec` to confirm: `kubectl exec -n <ns> <pod> -c rest-rego -- env | grep VAR_NAME`

**Policy denied unexpectedly after secret rotation:**
- After updating a Kubernetes Secret, the pod must be restarted (or the policy file touched to trigger hot-reload) for the new value to take effect.

**OPA compilation error after expansion:**
- The expanded value may contain characters that break Rego string syntax (e.g. a value containing `"`). Ensure secret values are safe to embed inside a Rego string literal.
