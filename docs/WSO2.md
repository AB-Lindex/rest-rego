# WSO2 API Manager Integration

This document describes how to configure rest-rego with WSO2 API Manager for on-premise JWT validation and authorization.

## Overview

WSO2 API Manager uses a custom JWT format with non-standard claims and header names. rest-rego supports these customizations through configuration options, allowing policy-based authorization of API requests secured by WSO2.

**Key Differences from Standard OIDC/JWT:**
- JWT is sent in custom header `X-Jwt-Assertion` instead of `Authorization: Bearer`
- Audience claim uses a custom key `http://wso2.org/claims/apiname` instead of standard `aud`
- Application identity requires checking both application name and service account name
- Claims use fully qualified URIs (e.g., `http://wso2.org/claims/*`)

## Architecture

```
┌──────────────┐      ┌──────────────────┐      ┌──────────────┐
│ API Consumer │─────>│ WSO2 API Manager │─────>│  rest-rego   │
│              │      │                  │      │   Sidecar    │
└──────────────┘      └──────────────────┘      └──────┬───────┘
                                                       │
                      Issues JWT in                    │
                      X-Jwt-Assertion                  │
                      header                           ▼
                                             ┌──────────────────┐
                                             │ Your Application │
                                             └──────────────────┘
```

## Setup

### WSO2 API Manager Configuration

1. **Publish your API** in WSO2 API Manager
2. **Subscribe an application** to your API
3. **Configure JWT Generation** to include required claims
4. **Note the API name** from the API configuration (used as audience)

### rest-rego Configuration

Add the following environment variables to your rest-rego deployment:

```bash
# OIDC Well-Known Configuration URL
WELLKNOWN_OIDC=https://api-manager.example.com/oauth2/token/.well-known/openid-configuration

# Custom audience claim key (WSO2-specific)
JWT_AUDIENCE_KEY=http://wso2.org/claims/apiname

# Your WSO2 API name (matches the published API name)
JWT_AUDIENCES=YourAPIName

# Custom JWT header name
AUTH_HEADER=X-Jwt-Assertion

# Empty auth kind (no "Bearer" prefix)
AUTH_KIND=
```

#### Configuration Details

| Setting            | Purpose | Example Value |
|--------------------|---------|---------------|
| `WELLKNOWN_OIDC`   | WSO2 OIDC discovery endpoint | `https://api-manager.example.com/oauth2/token/.well-known/openid-configuration` |
| `JWT_AUDIENCE_KEY` | Custom claim key for audience validation | `http://wso2.org/claims/apiname` |
| `JWT_AUDIENCES`    | Expected API name(s) in JWT | `YourAPIName` (comma-separated for multiple) |
| `AUTH_HEADER`      | Header containing JWT | `X-Jwt-Assertion` |
| `AUTH_KIND`        | Token prefix | *(empty)* - no prefix like "Bearer" |

## Token Acquisition

API consumers obtain access tokens from WSO2 API Manager using OAuth2 client credentials flow:

```bash
curl --request POST \
  --url https://api-manager.example.com/oauth2/token \
  --header 'Authorization: Basic <BASE64_CONSUMER_KEY:CONSUMER_SECRET>' \
  --header 'Content-Type: application/x-www-form-urlencoded' \
  --data 'grant_type=client_credentials'
```

**Response:**
```json
{
  "access_token": "eyJhbGc...",
  "scope": "default",
  "token_type": "Bearer",
  "expires_in": 3600
}
```

## WSO2 JWT Structure

### Request Flow

When WSO2 API Manager forwards a request to your protected endpoint:

1. WSO2 validates the access token
2. WSO2 generates a JWT with API context and claims
3. JWT is sent in `X-Jwt-Assertion` header to rest-rego
4. rest-rego validates the JWT and evaluates policies
5. Authorized requests are forwarded to your application

### JWT Claims

WSO2 JWTs contain custom claims with fully qualified URIs:

```json
{
  "iss": "wso2.org/products/am",
  "sub": "service-account-name",
  "exp": 1728417989,
  "iat": 1728414389,
  "jti": "5380c12e-001a-4fd5-ae01-895319686a1b",
  
  "scope": "default",
  
  "http://wso2.org/claims/apiname": "YourAPIName",
  "http://wso2.org/claims/apicontext": "/your/api/v1",
  "http://wso2.org/claims/version": "v1",
  "http://wso2.org/claims/tier": "Unlimited",
  
  "http://wso2.org/claims/applicationname": "ClientApplication",
  "http://wso2.org/claims/applicationid": "26",
  "http://wso2.org/claims/applicationUUId": "WSO2_GENERATED_APPLICATION_UUID",
  "http://wso2.org/claims/applicationtier": "Unlimited",
  
  "http://wso2.org/claims/subscriber": "domain/service-account",
  "http://wso2.org/claims/enduser": "domain/service-account@carbon.super",
  "http://wso2.org/claims/enduserTenantId": "-1234",
  "http://wso2.org/claims/usertype": "Application_User",
  "http://wso2.org/claims/keytype": "PRODUCTION"
}
```

#### Key Claims

| Claim                                    | Description                   | Use in Policy                   |
|------------------------------------------|-------------------------------|---------------------------------|
| `http://wso2.org/claims/apiname`         | Published API name            | Audience validation (automatic) |
| `http://wso2.org/claims/applicationname` | Subscribing application name  | Authorization (manual)          |
| `sub`                                    | Service account username      | Authorization (manual)          |
| `http://wso2.org/claims/apicontext`      | API path context              | Optional path validation        |
| `http://wso2.org/claims/keytype`         | Key type (PRODUCTION/SANDBOX) | Optional environment validation |

## Policy Input Structure

rest-rego provides the following input to Rego policies for WSO2 requests:

```json
{
  "request": {
    "method": "GET",
    "path": ["product", "123"],
    "headers": {
      "X-Jwt-Assertion": "<JWT_TOKEN>",
      "X-Forwarded-For": "10.4.20.156",
      ...
    },
    "auth": {
      "token": "<JWT_TOKEN>"
    },
    "size": 0
  },
  "jwt": {
    "sub": "service-account-name",
    "http://wso2.org/claims/apiname": "YourAPIName",
    "http://wso2.org/claims/applicationname": "ClientApplication",
    "http://wso2.org/claims/apicontext": "/your/api/v1",
    "http://wso2.org/claims/subscriber": "domain/service-account",
    ...
  }
}
```

## Authorization Policies

### Application Authorization with Header Forwarding

Authorize based on application name and service account (recommended approach). By assigning the `appname` and `appuser` variables, rest-rego automatically forwards these values as custom headers to your backend:

```rego
package request.rego

# Deny by default
default allow := false

# Extract application identity from JWT claims and assign as headers
appname := input.jwt["http://wso2.org/claims/applicationname"]
appuser := input.jwt.sub

# Allow authorized application + service account combinations
allow if {
    valid_apps := {
        ["ClientApp1", "svc-app1-account"],
        ["ClientApp2", "svc-app2-account"],
        ["ClientApp3", "svc-app3-account"],
    }
    [appname, appuser] in valid_apps
}
```

**Why check both application and user?**
- WSO2 requires a service account to subscribe an application to an API
- Same service account might be used by multiple applications
- Checking both provides stronger authorization guarantees

**Headers forwarded to your backend:**
- `X-Restrego-Appname`: Application name (e.g., "ClientApp1")
- `X-Restrego-Appuser`: Service account name (e.g., "svc-app1-account")

Your backend application can use these headers to identify the calling application and service account without parsing the JWT.

## Troubleshooting

### Token Validation Failures

**Symptom:** 401 Unauthorized responses

**Common Causes:**
1. **Incorrect `WELLKNOWN_OIDC` URL**
   - Verify WSO2 OIDC discovery endpoint is accessible
   - Check rest-rego logs for "loaded jwks" message with keys count

2. **Wrong `AUTH_HEADER` configuration**
   - WSO2 uses `X-Jwt-Assertion`, not `Authorization`
   - Verify header name matches WSO2 configuration

3. **JWT expired or invalid**
   - Check token expiration time
   - Verify signature with WSO2 public key

### Policy Failures

**Symptom:** 403 Forbidden responses with valid tokens

**Debugging Steps:**

1. **Enable debug logging:**
   ```bash
   # Add to rest-rego arguments
   --debug --verbose
   ```

2. **Check policy input:**
   Debug logs show the exact input passed to policies:
   ```json
   {
     "jwt": {
       "http://wso2.org/claims/applicationname": "actual-app-name",
       "sub": "actual-service-account"
     }
   }
   ```

3. **Verify claim names:**
   - Claims use full URIs: `http://wso2.org/claims/applicationname`
   - Access with brackets: `input.jwt["http://wso2.org/claims/applicationname"]`
   - Cannot use dot notation for URI-based keys

4. **Check tuple matching:**
   ```rego
   # Correct: tuple matching
   [appname, appuser] in valid_apps
   
   # Incorrect: individual matching
   appname in valid_apps  # Won't work for tuples
   ```

### Audience Validation Errors

**Symptom:** Logs show "audience validation failed"

**Resolution:**
1. Verify `JWT_AUDIENCES` matches your WSO2 API name exactly
2. Check `JWT_AUDIENCE_KEY` is set to `http://wso2.org/claims/apiname`
3. Inspect JWT claims to confirm API name claim value

### Common Configuration Mistakes

| Issue | Symptom | Fix |
|-------|---------|-----|
| Missing `AUTH_KIND=` | 401 errors, token not parsed | Set `AUTH_KIND` to empty string |
| Wrong `JWT_AUDIENCE_KEY` | Audience validation fails | Use `http://wso2.org/claims/apiname` |
| Missing brackets in claim access | Policy errors | Use `input.jwt["http://..."]` not `input.jwt.http://...` |
| Only checking `appname` | Weak authorization | Check tuple `[appname, appuser]` |

## Best Practices

### Security

1. **Always check both application and service account** - Prevents unauthorized access if accounts are shared
2. **Deny by default** - Start policies with `default allow := false`
3. **Validate key type** - Separate production and sandbox authorization if needed
4. **Log authorization decisions** - Monitor denied requests for security auditing
5. **Rotate consumer secrets** - WSO2 consumer secrets should be rotated regularly

### Policy Management

1. **Comment authorized applications** - Document which service owns each application
2. **Use descriptive application names** - Makes policies easier to audit
3. **Group related applications** - Organize by team or service for maintainability
4. **Test in sandbox first** - Validate policy changes with WSO2 sandbox keys
5. **Version control policies** - Track all policy changes in git

### Operations

1. **Monitor JWKS refresh** - rest-rego automatically refreshes keys every 24 hours
2. **Set resource limits** - Prevent sidecar resource exhaustion
3. **Configure health checks** - Enable Kubernetes self-healing
4. **Alert on authorization failures** - High failure rates indicate misconfiguration
5. **Document subscriber applications** - Maintain registry of authorized WSO2 applications

## Integration Examples

### Kubernetes Deployment

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
              value: "https://api-manager.example.com/oauth2/token/.well-known/openid-configuration"
            - name: JWT_AUDIENCE_KEY
              value: "http://wso2.org/claims/apiname"
            - name: JWT_AUDIENCES
              value: "YourAPIName"
            - name: AUTH_HEADER
              value: "X-Jwt-Assertion"
            - name: AUTH_KIND
              value: ""
          ports:
            - containerPort: 8181
              name: http
          volumeMounts:
            - name: policies
              mountPath: /policies
      
      volumes:
        - name: policies
          configMap:
            name: your-service-policies
```

### ConfigMap with Policy

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: your-service-policies
data:
  request.rego: |
    package request.rego
    
    default allow := false
    
    appname := input.jwt["http://wso2.org/claims/applicationname"]
    appuser := input.jwt.sub
    
    allow if {
        valid_apps := {
            ["ClientApp1", "svc-app1-account"],
            ["ClientApp2", "svc-app2-account"],
        }
        [appname, appuser] in valid_apps
    }
```

## Comparison with Azure AD

| Aspect              | WSO2 API Manager                     | Azure (Entra ID)                   |
|---------------------|--------------------------------------|------------------------------------|
| **JWT Header**      | `X-Jwt-Assertion` (custom)           | `Authorization: Bearer` (standard) |
| **Audience Claim**  | `http://wso2.org/claims/apiname`     | `aud` (standard)                   |
| **Audience Config** | `JWT_AUDIENCE_KEY` + `JWT_AUDIENCES` | `JWT_AUDIENCES` only               |
| **Auth Kind**       | Empty string                         | `Bearer` (default)                 |
| **App Identifier**  | Application name + service account   | `appid` (GUID)                     |
| **Authorization**   | Tuple `[appname, user]`              | Single `appid`                     |
| **Claims Format**   | Full URIs                            | Short names                        |
| **Token Issuer**    | WSO2 API Manager                     | Microsoft identity platform        |

## See Also

- [JWT Verification (Azure)](JWT.md) - Standard OIDC/JWT configuration
- [AZURE.md](AZURE.md) - Azure-specific authentication
- [Open Policy Agent Documentation](https://www.openpolicyagent.org/docs/latest/)
- [WSO2 API Manager JWT Documentation](https://apim.docs.wso2.com/)

---

*For questions or issues with WSO2 integration, check rest-rego logs with `--debug --verbose` flags for detailed JWT claim information.*
