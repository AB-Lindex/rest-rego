# File-Based JWKS - Kubernetes Example

This example demonstrates how to deploy rest-rego in Kubernetes using file-based JWKS authentication. This approach is ideal for:

- Testing and development environments
- Air-gapped deployments without external identity providers
- CI/CD pipelines requiring offline authentication
- Integration testing with controlled JWT validation

> **⚠️ Disclaimer**: This example is AI-generated and has not been verified on an actual Kubernetes cluster. While the configuration follows Kubernetes best practices and the rest-rego implementation has been tested, you should validate the example in your own environment before using it in production or critical workflows.

## Overview

The example includes:

- **ConfigMap** with embedded JWKS and OIDC well-known configuration
- **Deployment** showing volume mount patterns and environment configuration
- **Rego policy** for testing JWT validation
- Instructions for generating your own RSA key pairs

## Quick Start

### Prerequisites

- Kubernetes cluster (minikube, kind, or any K8s cluster)
- kubectl configured to access your cluster
- OpenSSL or similar tools for key generation (optional, for custom keys)

### Deploy the Example

Apply the configuration to your cluster:

```bash
# Create namespace
kubectl create namespace demo

# Apply ConfigMap with JWKS configuration
kubectl apply -f configmap-jwks.yaml

# Deploy the application
kubectl apply -f deployment.yaml
```

Check the deployment status:

```bash
# Check pods
kubectl get pods -n demo -l k8s-app=demo-file-jwks

# Check logs to verify file-based JWKS loading
kubectl logs -n demo -l k8s-app=demo-file-jwks -c sidecar
```

You should see log entries indicating file-based loading:

```
jwtsupport: loaded well-known from file url=file:///config/jwks/well-known.json
jwtsupport: loaded jwks from file url=file:///config/jwks/jwks.json keys=1
```

### Test the Deployment

Port-forward to access the service:

```bash
kubectl port-forward -n demo deployment/demo-file-jwks 8181:8181
```

Test with a valid JWT token (see [Generating Test Tokens](#generating-test-tokens) below):

```bash
curl -H "Authorization: Bearer YOUR_JWT_TOKEN" http://localhost:8181/api/test
```

## Generating Your Own Keys

To create your own RSA key pair and JWKS:

### Step 1: Generate RSA Private Key

```bash
# Generate 2048-bit RSA private key
openssl genrsa -out private-key.pem 2048
```

### Step 2: Extract Public Key

```bash
# Extract public key in PEM format
openssl rsa -in private-key.pem -pubout -out public-key.pem
```

### Step 3: Create JWKS from Public Key

Use a tool to convert the public key to JWKS format. Here's a simple Python script:

```python
#!/usr/bin/env python3
import json
import subprocess
import base64
from cryptography.hazmat.primitives import serialization
from cryptography.hazmat.backends import default_backend

# Read public key
with open('public-key.pem', 'rb') as f:
    public_key = serialization.load_pem_public_key(f.read(), backend=default_backend())

# Extract RSA components
public_numbers = public_key.public_numbers()
e = public_numbers.e
n = public_numbers.n

# Convert to base64url encoding
def int_to_base64url(num):
    num_bytes = num.to_bytes((num.bit_length() + 7) // 8, byteorder='big')
    return base64.urlsafe_b64encode(num_bytes).rstrip(b'=').decode('utf-8')

# Create JWKS
jwks = {
    "keys": [
        {
            "kty": "RSA",
            "use": "sig",
            "kid": "my-custom-key-2026",
            "e": int_to_base64url(e),
            "n": int_to_base64url(n),
            "alg": "RS256"
        }
    ]
}

# Write JWKS
with open('jwks.json', 'w') as f:
    json.dump(jwks, f, indent=2)

print("JWKS created: jwks.json")
```

Run the script:

```bash
python3 create-jwks.py
```

### Step 4: Create Well-Known Configuration

Create `well-known.json` with your issuer information:

```json
{
  "issuer": "https://your-issuer.example.com",
  "jwks_uri": "file:///config/jwks/jwks.json",
  "authorization_endpoint": "https://your-issuer.example.com/oauth2/authorize",
  "token_endpoint": "https://your-issuer.example.com/oauth2/token",
  "id_token_signing_alg_values_supported": ["RS256"],
  "response_types_supported": ["code", "token", "id_token"],
  "subject_types_supported": ["public"]
}
```

### Step 5: Update ConfigMap

Replace the `jwks.json` content in [configmap-jwks.yaml](configmap-jwks.yaml) with your generated JWKS, then reapply:

```bash
kubectl apply -f configmap-jwks.yaml
kubectl rollout restart deployment/demo-file-jwks -n demo
```

## Generating Test Tokens

To create valid JWT tokens for testing, use a tool like `jwt-cli` or create them programmatically.

### Using jwt-cli

Install jwt-cli:

```bash
cargo install jwt-cli
```

Create a token:

```bash
jwt encode \
  --alg RS256 \
  --kid "my-custom-key-2026" \
  --exp "+1h" \
  --iss "https://your-issuer.example.com" \
  --aud "https://example.com" \
  --sub "test-user" \
  --secret @private-key.pem
```

### Using Python

```python
#!/usr/bin/env python3
import jwt
import datetime

# Read private key
with open('private-key.pem', 'r') as f:
    private_key = f.read()

# Create token claims
claims = {
    'iss': 'https://your-issuer.example.com',
    'sub': 'test-user',
    'aud': 'https://example.com',
    'exp': datetime.datetime.utcnow() + datetime.timedelta(hours=1),
    'iat': datetime.datetime.utcnow(),
    'name': 'Test User',
    'email': 'test@example.com'
}

# Sign token
token = jwt.encode(
    claims,
    private_key,
    algorithm='RS256',
    headers={'kid': 'my-custom-key-2026'}
)

print(token)
```

## Configuration Details

### Environment Variables

The deployment configures rest-rego with:

| Variable | Value | Description |
|----------|-------|-------------|
| `WELLKNOWN_OIDC` | `file:///config/jwks/well-known.json` | File-based well-known configuration |
| `JWT_AUDIENCES` | `https://example.com` | Expected JWT audience claim |
| `BACKEND_PORT` | `10000` | Backend service port |
| `LOG_LEVEL` | `debug` | Enable debug logging to see file loading |

### Volume Mounts

The deployment mounts two ConfigMaps:

| Volume | ConfigMap | Mount Path | Purpose |
|--------|-----------|------------|---------|
| `jwks-config` | `file-based-jwks` | `/config/jwks` | Contains `well-known.json` and `jwks.json` |
| `policies` | `file-based-policies` | `/policies` | Contains Rego policy files |

### Policy Behavior

The included [request.rego](request.rego) policy allows:

- Requests with valid JWT signatures containing `iss`, `sub`, and `exp` claims
- Unauthenticated access to `/health` and `/ready` endpoints
- Unauthenticated access to `/public/*` paths

Customize the policy to match your authorization requirements.

## Troubleshooting

### Logs show "file not found" error

Check that the ConfigMap is properly mounted:

```bash
kubectl exec -n demo deployment/demo-file-jwks -c sidecar -- ls -la /config/jwks/
```

You should see `well-known.json` and `jwks.json` files.

### Logs show "source type mismatch" error

Ensure both `well-known.json` and the `jwks_uri` it contains use `file://` URLs. Mixing `file://` and `https://` within a single issuer is not allowed.

### JWT validation fails

Verify:

1. The JWT `iss` claim matches the `issuer` in `well-known.json`
2. The JWT `aud` claim matches the `JWT_AUDIENCES` environment variable
3. The JWT `kid` header matches a key ID in `jwks.json`
4. The token is signed with the private key corresponding to the public key in the JWKS
5. The token has not expired (check `exp` claim)

Enable debug logging to see detailed validation errors:

```bash
kubectl set env deployment/demo-file-jwks -n demo -c sidecar LOG_LEVEL=debug
```

### Pod fails to start

Check events and logs:

```bash
kubectl describe pod -n demo -l k8s-app=demo-file-jwks
kubectl logs -n demo -l k8s-app=demo-file-jwks -c sidecar --previous
```

Common issues:
- ServiceAccount does not exist (create with `kubectl create sa demo -n demo`)
- ConfigMap not found (apply `configmap-jwks.yaml` first)
- Image pull errors (check image name and registry access)

## Further Reading

- [File-Based JWKS Documentation](../../../docs/FILE-BASED-JWKS.md) - Comprehensive guide
- [JWT Authentication](../../../docs/JWT.md) - JWT authentication overview
- [Configuration Reference](../../../docs/CONFIGURATION.md) - All configuration options
- [Policy Writing Guide](../../../docs/POLICY.md) - Rego policy documentation

## Security Considerations

This example uses a sample public key for demonstration. In production:

- Generate unique key pairs for each environment
- Protect private keys using secure key management (Kubernetes secrets, vault, etc.)
- Use appropriate RBAC to restrict access to ConfigMaps containing JWKS
- Consider using proper OIDC providers for production workloads
- Regularly rotate signing keys
- Monitor for unauthorized access attempts

## License

This example is part of the rest-rego project and follows the same license.
