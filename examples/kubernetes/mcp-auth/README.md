# MCP Auth (RFC 9728) Example

This example shows how to deploy rest-rego as a sidecar in JWT (OIDC) auth mode with
[RFC 9728](https://www.rfc-editor.org/rfc/rfc9728) Protected Resource Metadata enabled,
so MCP-aware clients (e.g. VS Code's MCP client) can discover the real upstream
authorization server instead of assuming rest-rego itself issues tokens.

## Files

| File               | Description                                                          |
|--------------------|-----------------------------------------------------------------------|
| `deployment.yaml`  | Deployment with rest-rego sidecar configured for JWT + `RESOURCE_URL` |
| `ingress.yaml`      | Ingress routing both the resource path and the RFC 9728 well-known path |
| `request.rego`     | Rego policy requiring a validated JWT for `/mcp`                     |

## Prerequisites

- Kubernetes cluster with `kubectl` configured
- A reachable OIDC identity provider (e.g. Microsoft Entra ID) and its well-known
  discovery URL

### If your IdP is Microsoft Entra ID

Follow [Secure a Model Context Protocol (MCP) server with Microsoft Entra
ID](https://learn.microsoft.com/en-us/entra/agent-id/secure-mcp-server-with-entra-id)
to register the app that issues tokens for this resource. Two steps in that guide
are easy to miss and will otherwise cause token requests to fail:

- **Request v2 access tokens** — set `requestedAccessTokenVersion` to `2` in the
  app registration's manifest (`api` object). This must be done *before* the next
  step.
- **Set the Application ID URI to your exact `RESOURCE_URL`** — under
  **Expose an API**, set the Application ID URI to the same value you'll use for
  `RESOURCE_URL` below, character-for-character (no trailing slash). A mismatch
  here causes `AADSTS9010010` when clients request a token.

## Quick Start

### 1. Create the namespace

```bash
kubectl create namespace demo
```

### 2. Configure the identity provider and resource URL

Edit `deployment.yaml` and set:

- `WELLKNOWN_OIDC` — your IdP's OIDC discovery URL
- `JWT_AUDIENCES` — the expected audience(s) for tokens presented to this API
- `RESOURCE_URL` — the externally-reachable URL of this resource, including its path
  (e.g. `https://api.example.com/mcp`)
- `RESOURCE_SCOPES` — the app registration's exposed scope (e.g. `access_as_user`),
  expressed as a full scope URI matching `RESOURCE_URL`
  (e.g. `https://api.example.com/mcp/access_as_user`). Omitting it can cause
  `AADSTS9010010` for MCP clients that fall back to the IdP tenant's generic OIDC
  scopes instead of this resource's own scope

### 3. Deploy

```bash
kubectl apply -f . -n demo
```

### 4. Verify

```bash
# RFC 9728 metadata document (public, unauthenticated)
curl https://api.example.com/.well-known/oauth-protected-resource/mcp

# Anonymous request to the protected resource -> 401 + WWW-Authenticate
curl -i https://api.example.com/mcp

# Authenticated request -> proxied to backend (200/403 depending on policy)
curl -H "Authorization: Bearer $TOKEN" https://api.example.com/mcp
```

## Why two ingress paths?

rest-rego advertises the RFC 9728 well-known URL by inserting
`RESOURCE_METADATA_PATH` **before** the path component of `RESOURCE_URL` (RFC 8414
convention), e.g. `RESOURCE_URL=https://api.example.com/mcp` advertises
`https://api.example.com/.well-known/oauth-protected-resource/mcp`. rest-rego itself
only ever listens on the plain `/.well-known/oauth-protected-resource` path, so the
ingress must rewrite the advertised path-suffixed URL back to the plain one — see
`ingress.yaml`. If `RESOURCE_URL` has no path component, this rewrite is unnecessary.

## See Also

- [JWT.md](../../../docs/JWT.md#rfc-9728-protected-resource-metadata) — Full RFC 9728 flow documentation
- [CONFIGURATION.md](../../../docs/CONFIGURATION.md#rfc-9728-protected-resource-metadata) — `RESOURCE_URL` / `RESOURCE_METADATA_PATH` configuration reference
- [DEPLOYMENT.md](../../../docs/DEPLOYMENT.md#rfc-9728-ingress-rewrite) — Ingress rewrite requirement in depth
- [Secure a Model Context Protocol (MCP) server with Microsoft Entra ID](https://learn.microsoft.com/en-us/entra/agent-id/secure-mcp-server-with-entra-id) — Entra ID app registration and token-validation guidance this example follows
