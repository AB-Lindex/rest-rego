# Examples

## Kubernetes

In the `kubernetes` folder you'll find a complete example deployment with sidecar pattern.

### What's Included

- **deployment.yaml** - Main deployment with sidecar container
- **service.yaml** - Kubernetes service exposing the sidecar port
- **serviceaccount.yaml** - Service account for the deployment
- **ingress.yaml** - Ingress configuration
- **request.rego** - Authorization policy (mounted via ConfigMap)
- **kustomization.yaml** - Kustomize configuration

### Architecture

The sidecar pattern adds rest-rego as an additional container alongside your main application:

- **Main container** (e.g., your API) runs on internal port (not exposed)
- **Sidecar container** (rest-rego) exposes port 8181 for API traffic
- Sidecar validates all requests before forwarding to main container
- Policies are mounted from a ConfigMap for easy updates

### Configuration

#### 1. Create Policy ConfigMap

```yaml
# kustomization.yaml
configMapGenerator:
  - name: demo-policies
    files:
      - request.rego
```

#### 2. Configure Sidecar Container

**Recommended: JWT/OIDC Authentication**

```yaml
# deployment.yaml - spec.template.spec.containers
- name: sidecar
  image: lindex/rest-rego
  imagePullPolicy: IfNotPresent
  
  env:
    # Port of your main container
    - name: BACKEND_PORT
      value: '10000'
    
    # JWT/OIDC authentication (recommended)
    - name: WELLKNOWN_OIDC
      value: 'https://login.microsoftonline.com/YOUR-TENANT-ID/v2.0/.well-known/openid-configuration'
    - name: JWT_AUDIENCES
      value: 'api://your-guard-app-id'
  
  # rest-rego ports
  ports:
    - containerPort: 8181  # API proxy port
      name: http
  
  # Health checks on management port
  livenessProbe:
    httpGet:
      path: /healthz
      port: 8182
    initialDelaySeconds: 2
  readinessProbe:
    httpGet:
      path: /readyz
      port: 8182
  
  # Mount policies from ConfigMap
  volumeMounts:
    - name: policies
      mountPath: /policies

# Add ConfigMap volume (spec.template.spec.volumes)
volumes:
  - name: policies
    configMap:
      name: demo-policies
```

**Alternative: Azure Graph Authentication**

If you need Azure AD app metadata (less common), use:

```yaml
env:
  - name: BACKEND_PORT
    value: '10000'
  - name: AZURE_TENANT
    value: 'your-tenant-id'
```

And update your policy to use `input.user.appId` instead of `input.jwt.appid`.

### Policy Hot-Reload

**Important**: Mount the entire ConfigMap directory, not individual files, to enable hot-reload:

```yaml
# ✅ Correct - hot-reload works
volumeMounts:
  - name: policies
    mountPath: /policies

# ❌ Incorrect - hot-reload won't work
volumeMounts:
  - name: policies
    mountPath: /policies/request.rego
    subPath: request.rego
```

When you update the ConfigMap, Kubernetes will update the mounted files after a propagation delay (typically 30-60 seconds, depending on your cluster's kubelet sync period), and rest-rego will automatically reload the policies within ~1 second of the file system update.

### Applying the Configuration

```bash
# Navigate to the examples directory
cd examples/kubernetes

# Update YOUR-TENANT-ID and your-guard-app-id in deployment.yaml

# Apply with kubectl
kubectl apply -k .

# Or with kustomize
kustomize build . | kubectl apply -f -
```

### Testing

```bash
# Get a JWT token from Azure AD (or your OIDC provider)
TOKEN=$(curl -X POST \
  https://login.microsoftonline.com/YOUR-TENANT-ID/oauth2/v2.0/token \
  -d "scope=api://your-guard-app-id/.default" \
  -d "grant_type=client_credentials" \
  -d "client_id=YOUR-CLIENT-ID" \
  -d "client_secret=YOUR-CLIENT-SECRET" \
  | jq -r .access_token)

# Make a request through the sidecar
curl -H "Authorization: Bearer $TOKEN" http://your-ingress-host/api/endpoint
```

### See Also

- [JWT Authentication Setup](../docs/JWT.md)
- [Azure Graph Setup](../docs/AZURE.md)
- [WSO2 API Manager Setup](../docs/WSO2.md)
