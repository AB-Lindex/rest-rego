# JWT Verification

rest-rego supports JWT-based authentication using standard OIDC (OpenID Connect) providers. This document covers the general JWT verification process and provides specific configuration examples.

## Overview

JWT verification requires:
- An OIDC well-known configuration URL
- Expected audience value(s)
- Optional: Custom header names and claim keys for non-standard implementations

**Supported Identity Providers:**
- **Azure (Microsoft Entra ID)** - Standard OIDC/JWT (documented below)
- **WSO2 API Manager** - Custom JWT format with non-standard claims ([see WSO2.md](WSO2.md))
- Any standard OIDC-compliant identity provider

**Permissive mode**: Set `PERMISSIVE_AUTH=true` to allow requests with missing or invalid tokens to pass through as anonymous. See [PERMISSIVE.md](PERMISSIVE.md) for details.

> ⚠️ **Warning**: `input.jwt` is only set after rest-rego verifies the token's signature, issuer, and audience — never assume a request is authenticated just because an `Authorization` header or JWT-shaped token was present. Always check `input.jwt` for absence/`null` explicitly in your policy rather than parsing the raw token yourself and assuming it's valid. See [Detecting Anonymous Requests in the Backend](PERMISSIVE.md#detecting-anonymous-requests-in-the-backend) for how to signal this to the backend.

## Standard OIDC Configuration

For standard OIDC providers (like Azure), rest-rego automatically:
1. Fetches the OIDC discovery document from the well-known URL
2. Downloads the JSON Web Key Set (JWKS) for signature verification
3. Validates JWT tokens on each request
4. Refreshes JWKS automatically according to the provider's cache headers

### Basic Configuration

```bash
# OIDC discovery endpoint
WELLKNOWN_OIDC=https://your-idp.example.com/.well-known/openid-configuration

# Expected audience(s) - comma-separated for multiple
JWT_AUDIENCES=api://your-app-id,api://another-app-id
```

**Testing and Offline Scenarios:** For local development, integration testing, or air-gapped deployments, rest-rego also supports loading JWKS from local files instead of HTTP endpoints. This enables testing without external identity providers and faster CI/CD pipelines. See [FILE-BASED-JWKS.md](FILE-BASED-JWKS.md) for details.

## Azure (Microsoft Entra ID)

### Setup
You need an Azure Application to act as 'guard'.<br>
This must have an 'Application ID URI' and 'Allow public client flows' enabled (probably in the Advanced settings in the Authentication page).

Add the following arguments to 'rest-rego':
```ini
WELLKNOWN_OIDC=https://login.microsoftonline.com/$TENANT/v2.0/.well-known/openid-configuration
JWT_AUDIENCES=$GUARD_APPIDURI
```

On startup you should see a line stating '..loaded jwks..' ending with 'keys=N' (where N should be >0)

**v1 vs v2 access tokens:** the token version is controlled by `requestedAccessTokenVersion`
in the app registration's manifest, not by which endpoint you call. If you're securing an
MCP server with [RFC 9728 support](#rfc-9728-protected-resource-metadata), set
`requestedAccessTokenVersion` to `2` — this is required before an HTTPS Application ID
URI can be set, and v2 tokens are what MCP clients expect. See [Register your MCP server
in Microsoft Entra ID](https://learn.microsoft.com/en-us/entra/agent-id/secure-mcp-server-with-entra-id#register-your-mcp-server-in-microsoft-entra-id).

### How to get a token
As the api-consumer you also need an Azure Application (normally in the same tenant as the 'guard')


```sh
curl --request POST \
  --url https://login.microsoftonline.com/$TENANT/oauth2/v2.0/token \
  --header 'content-type: application/x-www-form-urlencoded' \
  --data "scope=$GUARD_APPIDURI/.default" \
  --data "grant_type=client_credentials" \
  --data "client_id=$APPID" \
  --data "client_secret=$APPSECRET"
```

## Request Flow

```mermaid
sequenceDiagram
  participant u as API Consumer
  box gray Sidecar
  participant p as HTTP Proxy
  participant pol as Policy
  end
  participant b as Backend to<br>protect
  participant idp as Identity Provider<br>(Azure/WSO2/etc.)

  note over p,idp: Startup (with automatic JWKS refresh)
  p->>idp: Fetch OIDC config
  idp->>p: WellKnown-config
  p->>idp: Fetch JWKS
  idp->>p: JSON Web Key Set

  note over u,p: API Call
  u->>p: Call API with JWT
  note over p: Verify JWT signature<br>using loaded JWKS<br>Validate audience & expiration
  note left of pol: Policy runs even<br/>if token is invalid
  p->>pol: Run request-policy
  pol->>p: Policy result
  p-->>u: 401/403 if denied
  p->>b: Forward authorized call
  b->>p: Response
  p->>u: Response
```

## OIDC Configuration and JWKS Management

rest-rego automatically manages OIDC configuration and cryptographic keys:

### Automatic Key Refresh
- JWKS (JSON Web Key Set) is fetched from the URL in the OIDC discovery document
- Refresh checks run every two minutes. Fetch intervals follow `Cache-Control`/`Expires`, with a minimum of 15 minutes (also the default without cache headers)
- Changes are detected and applied without restart
- Both the fetched JWKS and the keyset after algorithm enrichment must contain at least one key; empty sets are rejected before caching
- Failed refreshes retain the last successfully cached keys and are retried on the normal refresh schedule
- An empty JWKS at startup is rejected rather than accepted as a usable keyset
- File-based JWKS uses the same non-empty checks but is loaded only at startup

### JWKS Refresh Logging

JWKS fetches and background refresh failures are logged without enabling debug logging:

- **INFO** `jwtsupport: JWKS fetched and validated` includes `url` and `keys` after a successful fetch, parse, and non-empty validation (at startup and on every successful refresh)
- **ERROR** `jwtsupport: JWKS refresh failed; keeping cached keys` includes the refresh error and endpoint URL in `error`, including network failures, non-200 responses, malformed JSON, and rejected empty keysets
- Startup failures use the existing `failed to get jwks` or `failed to post-process jwks` error messages

These logs do not include JWTs or key material.

### Algorithm Detection
- rest-rego uses the algorithm (`alg`) specified in each key
- If keys don't specify an algorithm, it falls back to supported algorithms from the OIDC configuration
- Commonly supported: RS256, RS384, RS512, ES256, ES384, ES512

### Validation Process
For each incoming request with a JWT:
1. **Signature verification** - JWT is verified against JWKS public keys
2. **Audience validation** - `aud` claim must match `JWT_AUDIENCES` configuration
3. **Expiration check** - `exp` claim must be in the future
4. **Not-before check** - `nbf` claim must be in the past (if present)
5. **Issuer validation** - `iss` claim must match OIDC configuration

### Debugging Token Issues

Enable debug logging to troubleshoot JWT validation:

```bash
# Add to rest-rego arguments
--debug --verbose
```

**Common log messages:**
- `loaded jwks from {url} (keys=N)` - Successful JWKS load with N keys
- `jwt validation failed: ...` - Token signature or claim validation failed
- `audience validation failed` - Token audience doesn't match expected value

## Custom JWT Configurations

Some identity providers use non-standard JWT formats. rest-rego supports customizations:

### Custom Header Name

By default, JWTs are expected in the `Authorization` header with `Bearer` prefix. Override with:

```bash
# Custom header name (e.g., WSO2 uses X-Jwt-Assertion)
AUTH_HEADER=X-Jwt-Assertion

# Custom auth kind prefix (empty for no prefix)
AUTH_KIND=
```

### Custom Audience Claim

Standard OIDC uses the `aud` claim for audience validation. Override with:

```bash
# Custom claim key for audience (e.g., WSO2 uses a custom URI claim)
JWT_AUDIENCE_KEY=http://wso2.org/claims/apiname

# Expected audience value
JWT_AUDIENCES=YourAPIName
```

**Use Case:** WSO2 API Manager stores the API name in `http://wso2.org/claims/apiname` instead of the standard `aud` claim.

For detailed WSO2 configuration, see [WSO2.md](WSO2.md).

## Azure JWT Structure

Azure AD issues JWTs with standard claims in a flat structure. The Rego policies receive an `input` object like this:

_(Use the `--debug` option to see the exact structure for your setup)_

**Note:** the example below is a **v1** access token (`"ver": "1.0"`, `iss` under
`sts.windows.net`). A **v2** token (`"ver": "2.0"`) instead has an `iss` like
`https://login.microsoftonline.com/tenant-id/v2.0` and an `aud` that's a plain GUID
or client ID rather than an Application ID URI array. If you're using [RFC 9728
support](#rfc-9728-protected-resource-metadata) for MCP clients, your app must issue
v2 tokens so `iss` matches the `.../v2.0` value advertised in `authorization_servers`.

```json
{
  "request": {
    "method": "GET",
    "path": ["hello"],
    "headers": {
      "Authorization": "Bearer <TOKEN>",
      "User-Agent": "...",
      ...
    },
    "auth": {
      "kind": "Bearer",
      "token": "<TOKEN>"
    },
    "size": 0
  },
  "jwt": {
    "appid": "11112222-3333-4444-5555-666677778888",
    "appidacr": "1",
    "aud": ["api://your-app-id"],
    "exp": "2025-10-08T21:17:13Z",
    "iat": "2025-10-08T20:12:13Z",
    "iss": "https://sts.windows.net/tenant-id/",
    "nbf": "2025-10-08T20:12:13Z",
    "oid": "object-id-of-app",
    "sub": "object-id-of-app",
    "tid": "tenant-id",
    "ver": "1.0"
  }
}
```

### Key Azure Claims

| Claim   | Description                              | Use in Policy                      |
|---------|------------------------------------------|------------------------------------|
| `appid` | Azure AD App Registration client ID      | Primary application identifier     |
| `aud`   | Audience (your API's Application ID URI) | Validated automatically            |
| `iss`   | Issuer (Azure AD tenant)                 | Validated automatically            |
| `oid`   | Object ID of the service principal       | Optional: track specific instances |
| `tid`   | Tenant ID                                | Optional: multi-tenant scenarios   |
| `exp`   | Token expiration                         | Validated automatically            |

## Authorization Policies

### Basic Application Authorization (Azure)

Authorize based on Azure AD App Registration client ID:

```rego
package request.rego

# Deny by default
default allow := false

# Allow authorized applications
allow if {
    valid_apps := {
        "11112222-3333-4444-5555-666677778888", # app-name-1
        "22223333-4444-5555-6666-777788889999", # app-name-2
        "33334444-5555-6666-7777-888899990000", # app-name-3
    }
    input.jwt.appid in valid_apps
}
```

**Best Practice:** Always add comments with application names for maintainability.

### Forward Application Identity to Backend

Export the application ID as a custom header:

```rego
package request.rego

default allow := false

allow if {
    valid_apps := {
        "11112222-3333-4444-5555-666677778888", # app-name-1
    }
    input.jwt.appid in valid_apps
}

# Assign custom header forwarded to backend
appid := input.jwt.appid
```

**Resulting header:** `X-Restrego-Appid: 11112222-3333-4444-5555-666677778888`

### Advanced: Tenant-Based Authorization

Multi-tenant scenarios with different authorized apps per tenant:

```rego
package request.rego

default allow := false

appid := input.jwt.appid
tenant := input.jwt.tid

# Tenant 1 applications
allow if {
    tenant == "tenant-id-1"
    tenant1_apps := {
        "app-id-1",
        "app-id-2",
    }
    appid in tenant1_apps
}

# Tenant 2 applications
allow if {
    tenant == "tenant-id-2"
    tenant2_apps := {
        "app-id-3",
        "app-id-4",
    }
    appid in tenant2_apps
}
```

### Advanced: Path-Based Authorization

Different applications authorized for different endpoints:

```rego
package request.rego

default allow := false

appid := input.jwt.appid

# Public endpoints: all authorized apps
allow if {
    input.request.path[0] == "public"
    public_apps := {
        "app-id-1",
        "app-id-2",
    }
    appid in public_apps
}

# Admin endpoints: restricted apps only
allow if {
    input.request.path[0] == "admin"
    admin_apps := {
        "admin-app-id",
    }
    appid in admin_apps
}
```

## Troubleshooting

### Token Validation Failures

**Symptom:** 401 Unauthorized responses

**Common Causes:**

1. **Incorrect `WELLKNOWN_OIDC` URL**
   - Verify the OIDC discovery endpoint is accessible
   - Check rest-rego logs for "loaded jwks" message with keys count > 0
   - Test endpoint manually: `curl https://your-oidc-endpoint/.well-known/openid-configuration`

2. **Wrong audience configuration**
   - Token's `aud` claim must match `JWT_AUDIENCES` exactly
   - Azure: Use Application ID URI (e.g., `api://your-app-id`)
   - Check token claims using [jwt.io](https://jwt.io) debugger

3. **JWT expired or not yet valid**
   - Check `exp` (expiration) and `nbf` (not before) claims
   - Verify system clocks are synchronized

4. **JWKS key mismatch**
   - Token's `kid` (key ID) header must match a key in JWKS
   - JWKS is auto-refreshed every 24h; restart if immediate update needed

### Policy Failures

**Symptom:** 403 Forbidden responses with valid tokens

**Debugging Steps:**

1. **Enable debug logging:**
   ```bash
   # Add to rest-rego arguments or environment
   --debug --verbose
   ```

2. **Check policy input:**
   Debug logs show the exact `input` object passed to policies:
   ```json
   {
     "jwt": {
       "appid": "actual-app-id-from-token"
     }
   }
   ```

3. **Verify application ID:**
   - Extract `appid` from token using jwt.io
   - Ensure it's included in your policy's `valid_apps` set
   - Check for typos in the GUID

4. **Test policy syntax:**
   ```bash
   # Use OPA CLI to test policy locally
   opa eval -d policies/ -i test-input.json "data.request.rego.allow"
   ```

### Common Configuration Mistakes

| Issue | Symptom | Fix |
|-------|---------|-----|
| Wrong tenant in WELLKNOWN_OIDC | 401, "issuer validation failed" | Use correct Azure tenant ID |
| Missing `.default` in token scope | 401, "audience validation failed" | Request token with `{APPIDURI}/.default` |
| Wrong Application ID URI | 401, "audience validation failed" | Match `JWT_AUDIENCES` to App Registration |
| Case-sensitive claim names | Policy doesn't match | Use exact claim names: `appid`, not `AppId` |
| Missing `default allow := false` | Security risk, all requests allowed | Always deny by default |

## Best Practices

### Security

1. **Deny by default** - Start all policies with `default allow := false`
2. **Validate critical claims** - Check issuer, audience, expiration automatically
3. **Minimize authorized apps** - Only add necessary application IDs
4. **Use descriptive comments** - Document which application each ID represents
5. **Rotate secrets regularly** - Azure client secrets should expire and be rotated

### Policy Management

1. **Version control policies** - Track all changes in git
2. **Test in development first** - Validate policy changes before production
3. **Use sets for collections** - More efficient than multiple OR conditions
4. **Group related apps** - Organize by team or service
5. **Keep policies simple** - Complex logic is harder to audit

### Operations

1. **Monitor JWKS refresh** - Automatic refresh every 24 hours
2. **Set resource limits** - Prevent sidecar resource exhaustion
3. **Configure health checks** - Enable Kubernetes self-healing
4. **Alert on authorization failures** - High failure rates indicate issues
5. **Log denied requests** - Monitor for security events or misconfigurations

## Identity Provider Comparison

| Aspect | Azure (Entra ID) | WSO2 API Manager |
|--------|------------------|------------------|
| **OIDC Standard** | ✅ Full compliance | ⚠️ Custom extensions |
| **JWT Header** | `Authorization: Bearer` | `X-Jwt-Assertion` |
| **Audience Claim** | `aud` (standard) | `http://wso2.org/claims/apiname` |
| **Config Required** | `WELLKNOWN_OIDC`, `JWT_AUDIENCES` | + `JWT_AUDIENCE_KEY`, `AUTH_HEADER`, `AUTH_KIND` |
| **App Identifier** | `appid` (GUID) | `applicationname` + `sub` (tuple) |
| **Claims Format** | Flat, short names | Nested, URI-based names |
| **Authorization Pattern** | Simple set membership | Tuple matching |
| **Use Case** | Cloud-native, standard OIDC | On-premise API gateway |

## Integration Examples

### Kubernetes Deployment with Azure

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: your-service
spec:
  template:
    spec:
      containers:
        - name: app
          image: your-app:latest
          ports:
            - containerPort: 8080
        
        - name: restrego
          image: lindex/rest-rego:latest
          env:
            - name: BACKEND_PORT
              value: "8080"
            - name: WELLKNOWN_OIDC
              value: "https://login.microsoftonline.com/your-tenant-id/v2.0/.well-known/openid-configuration"
            - name: JWT_AUDIENCES
              value: "api://your-app-id"
          ports:
            - containerPort: 8181
              name: http
          livenessProbe:
            httpGet:
              path: /healthz
              port: 8182
          readinessProbe:
            httpGet:
              path: /readyz
              port: 8182
          volumeMounts:
            - name: policies
              mountPath: /policies
      
      volumes:
        - name: policies
          configMap:
            name: your-service-policies
```

### ConfigMap with Azure Policy

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: your-service-policies
data:
  request.rego: |
    package request.rego
    
    default allow := false
    
    allow if {
        valid_apps := {
            "11112222-3333-4444-5555-666677778888", # production-api-client
            "22223333-4444-5555-6666-777788889999", # monitoring-service
        }
        input.jwt.appid in valid_apps
    }
    
    appid := input.jwt.appid
```

## RFC 9728 Protected Resource Metadata

MCP-aware clients (such as VS Code's MCP client) expect a server fronted by an
authorization layer to advertise the real upstream authorization server(s) via
[RFC 9728](https://www.rfc-editor.org/rfc/rfc9728) (OAuth 2.0 Protected Resource
Metadata), rather than assuming the fronting layer is itself the authorization
server. rest-rego supports this in JWT (OIDC) auth mode.

### Enabling the endpoint

```bash
export WELLKNOWN_OIDC="https://login.microsoftonline.com/TENANT-ID/v2.0/.well-known/openid-configuration"
export JWT_AUDIENCES="api://your-api-audience"
export RESOURCE_URL="https://api.example.com/mcp"
rest-rego
```

`GET https://api.example.com/.well-known/oauth-protected-resource/mcp` (the plain
`RESOURCE_METADATA_PATH` route on rest-rego itself, reached via an ingress rewrite —
see below) then returns:

```json
{
  "resource": "https://api.example.com/mcp",
  "authorization_servers": ["https://login.microsoftonline.com/tenant-id/v2.0"],
  "bearer_methods_supported": ["header"]
}
```

`authorization_servers` is the de-duplicated list of `issuer` values collected from
every configured `WELLKNOWN_OIDC` document, so multiple comma-separated well-knowns
produce multiple entries.

### `scopes_supported` and MCP client token requests

Set `RESOURCE_SCOPES` to advertise the resource's own app-specific scope(s) as
`scopes_supported` in the metadata document:

```bash
export RESOURCE_SCOPES="https://api.example.com/mcp/access_as_user"
```

```json
{
  "resource": "https://api.example.com/mcp",
  "authorization_servers": ["https://login.microsoftonline.com/tenant-id/v2.0"],
  "bearer_methods_supported": ["header"],
  "scopes_supported": ["https://api.example.com/mcp/access_as_user"]
}
```

If `RESOURCE_SCOPES` is omitted, `scopes_supported` is left out of the document.
MCP-aware clients (e.g. VS Code) then fall back to the connecting IdP tenant's
generic OIDC `scopes_supported` (`openid profile email offline_access`) when
requesting a token — which, combined with an explicit `resource` parameter, Entra ID
rejects with `AADSTS9010010: The resource parameter provided in the request doesn't
match with the requested scopes`. Setting `RESOURCE_SCOPES` resolves this by giving
clients a scope value that matches the `resource` they're requesting a token for.

### 401 vs 403 on policy deny

- A request with **no** `Authorization` header at all, denied by policy, now gets:
  ```
  HTTP/1.1 401 Unauthorized
  WWW-Authenticate: Bearer resource_metadata="https://api.example.com/.well-known/oauth-protected-resource/mcp"
  ```
- A request that presented credentials — valid, invalid, or downgraded to anonymous
  by `PERMISSIVE_AUTH` — still gets `403 Forbidden` on deny, unchanged from today

### Ingress rewrite requirement when `RESOURCE_URL` has a path

The well-known URL is built by inserting `RESOURCE_METADATA_PATH` **before** the path
component of `RESOURCE_URL` (the RFC 8414/9728 convention), e.g.
`RESOURCE_URL=https://api.example.com/mcp` advertises
`https://api.example.com/.well-known/oauth-protected-resource/mcp`. rest-rego itself
only ever listens on the plain `RESOURCE_METADATA_PATH` — your ingress must rewrite
the path-suffixed well-known URL back to the plain path before it reaches rest-rego.
See [DEPLOYMENT.md](DEPLOYMENT.md#rfc-9728-ingress-rewrite) for a worked Kubernetes
example.

### Entra ID `resource` / `identifierUris` precedent

If `authorization_servers` or the token audience your client requests doesn't match a
registered `identifierUris` entry on the Entra ID application, token acquisition fails
with `AADSTS9010010` or `AADSTS500011`. This is an IdP-side app registration mismatch,
not a rest-rego defect — verify `RESOURCE_URL` and `JWT_AUDIENCES` line up with the
application's configured identifier URIs. Two common causes:

- The app registration still issues v1 tokens (`requestedAccessTokenVersion` unset or
  `1`), which can't have an HTTPS Application ID URI at all.
- `RESOURCE_URL` has a trailing slash or other character difference from the
  registered Application ID URI — the match must be exact.

## See Also

- [CONFIGURATION.md](CONFIGURATION.md#rfc-9728-protected-resource-metadata) - `RESOURCE_URL` / `RESOURCE_METADATA_PATH` configuration reference
- [WSO2.md](WSO2.md) - WSO2 API Manager integration with custom JWT format
- [AZURE.md](AZURE.md) - Azure-specific authentication details
- [Open Policy Agent Documentation](https://www.openpolicyagent.org/docs/latest/)
- [Rego Language Reference](https://www.openpolicyagent.org/docs/latest/policy-language/)
- [JWT.io Token Debugger](https://jwt.io) - Decode and inspect JWT tokens
- [Secure a Model Context Protocol (MCP) server with Microsoft Entra ID](https://learn.microsoft.com/en-us/entra/agent-id/secure-mcp-server-with-entra-id) - Entra ID app registration and token-validation guidance for MCP servers

---

*For detailed WSO2 API Manager integration with custom claims and headers, see [WSO2.md](WSO2.md).*
