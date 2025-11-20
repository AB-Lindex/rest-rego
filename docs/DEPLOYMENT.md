# Deployment Guide

Production-ready deployment patterns for rest-rego across different platforms.

## Table of Contents

- [Docker Deployment](#docker-deployment)
- [Kubernetes Deployment](#kubernetes-deployment)
- [Scaling](#scaling)
- [Security Best Practices](#security-best-practices)
- [High Availability](#high-availability)
- [Network Patterns](#network-patterns)

## Docker Deployment

### Basic Deployment

```bash
docker run -d \
  --name rest-rego \
  -p 8181:8181 \
  -p 8182:8182 \
  -e WELLKNOWN_OIDC="https://your-idp/.well-known/openid-configuration" \
  -e JWT_AUDIENCES="your-audience" \
  -e BACKEND_PORT="8080" \
  -v $(pwd)/policies:/policies:ro \
  lindex/rest-rego:latest
```

### Production Docker Deployment

```bash
docker run -d \
  --name rest-rego \
  --restart unless-stopped \
  --memory 256m \
  --cpus 0.5 \
  -p 8181:8181 \
  -p 8182:8182 \
  -e WELLKNOWN_OIDC="https://login.microsoftonline.com/TENANT/v2.0/.well-known/openid-configuration" \
  -e JWT_AUDIENCES="api://production-api" \
  -e BACKEND_HOST="backend" \
  -e BACKEND_PORT="8080" \
  -e READ_TIMEOUT="30s" \
  -e WRITE_TIMEOUT="90s" \
  -v $(pwd)/policies:/policies:ro \
  --health-cmd="wget -q -O- http://localhost:8182/healthz || exit 1" \
  --health-interval=10s \
  --health-timeout=2s \
  --health-retries=3 \
  lindex/rest-rego:latest
```

### Docker Compose

```yaml
version: '3.8'

services:
  rest-rego:
    image: lindex/rest-rego:latest
    container_name: rest-rego
    restart: unless-stopped
    ports:
      - "8181:8181"
      - "8182:8182"
    environment:
      WELLKNOWN_OIDC: "https://login.microsoftonline.com/${TENANT_ID}/v2.0/.well-known/openid-configuration"
      JWT_AUDIENCES: "${JWT_AUDIENCE}"
      BACKEND_HOST: "backend"
      BACKEND_PORT: "8080"
      DEBUG: "${DEBUG:-false}"
    volumes:
      - ./policies:/policies:ro
    healthcheck:
      test: ["CMD", "wget", "-q", "-O-", "http://localhost:8182/healthz"]
      interval: 10s
      timeout: 2s
      retries: 3
    depends_on:
      backend:
        condition: service_healthy
    networks:
      - app-network
    deploy:
      resources:
        limits:
          cpus: '0.5'
          memory: 256M
        reservations:
          cpus: '0.1'
          memory: 128M

  backend:
    image: your-backend:latest
    container_name: backend
    expose:
      - "8080"
    healthcheck:
      test: ["CMD", "wget", "-q", "-O-", "http://localhost:8080/health"]
      interval: 10s
      timeout: 2s
      retries: 3
    networks:
      - app-network

networks:
  app-network:
    driver: bridge
```

### Environment File

Create `.env` file:

```bash
# Authentication
TENANT_ID=your-tenant-id
JWT_AUDIENCE=api://your-api

# Debug (disable in production)
DEBUG=false

# Backend
BACKEND_HOST=backend
BACKEND_PORT=8080
```

Run with environment file:

```bash
docker-compose --env-file .env up -d
```

## Kubernetes Deployment

### Sidecar Pattern (Recommended)

Deploy rest-rego as a sidecar container alongside your application:

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: my-app
  namespace: production
spec:
  replicas: 3
  selector:
    matchLabels:
      app: my-app
  template:
    metadata:
      labels:
        app: my-app
    spec:
      serviceAccountName: my-app
      
      containers:
      # Main application
      - name: app
        image: my-app:v1.0.0
        ports:
        - containerPort: 8080
          name: http
        env:
        - name: PORT
          value: "8080"
        resources:
          requests:
            cpu: 200m
            memory: 256Mi
          limits:
            cpu: 1000m
            memory: 512Mi
        livenessProbe:
          httpGet:
            path: /health
            port: 8080
          initialDelaySeconds: 10
          periodSeconds: 10
        readinessProbe:
          httpGet:
            path: /ready
            port: 8080
          initialDelaySeconds: 5
          periodSeconds: 5
      
      # rest-rego sidecar
      - name: rest-rego
        image: lindex/rest-rego:latest
        ports:
        - containerPort: 8181
          name: proxy
        - containerPort: 8182
          name: metrics
        env:
        - name: BACKEND_PORT
          value: "8080"
        - name: WELLKNOWN_OIDC
          valueFrom:
            configMapKeyRef:
              name: rest-rego-config
              key: WELLKNOWN_OIDC
        - name: JWT_AUDIENCES
          valueFrom:
            configMapKeyRef:
              name: rest-rego-config
              key: JWT_AUDIENCES
        - name: READ_TIMEOUT
          value: "30s"
        - name: WRITE_TIMEOUT
          value: "90s"
        volumeMounts:
        - name: policies
          mountPath: /policies
          readOnly: true
        resources:
          requests:
            cpu: 100m
            memory: 128Mi
          limits:
            cpu: 500m
            memory: 256Mi
        livenessProbe:
          httpGet:
            path: /healthz
            port: 8182
          initialDelaySeconds: 2
          periodSeconds: 10
        readinessProbe:
          httpGet:
            path: /readyz
            port: 8182
          initialDelaySeconds: 2
          periodSeconds: 5
        securityContext:
          runAsNonRoot: true
          runAsUser: 1000
          readOnlyRootFilesystem: true
          allowPrivilegeEscalation: false
          capabilities:
            drop: [ALL]
      
      volumes:
      - name: policies
        configMap:
          name: my-app-policies
```

### Service Configuration

Expose rest-rego proxy port, not the application directly:

```yaml
apiVersion: v1
kind: Service
metadata:
  name: my-app
  namespace: production
  labels:
    app: my-app
spec:
  type: ClusterIP
  selector:
    app: my-app
  ports:
  - name: http
    port: 80
    targetPort: 8181  # Route to rest-rego, not app
    protocol: TCP
  - name: metrics
    port: 8182
    targetPort: 8182
    protocol: TCP
```

### Policy ConfigMap

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: my-app-policies
  namespace: production
data:
  request.rego: |
    package policies
    
    default allow := false
    
    # Allow specific applications
    allow if {
      input.jwt.appid in [
        "11112222-3333-4444-5555-666677778888",  # production-api
        "22223333-4444-5555-6666-777788889999",  # staging-api
      ]
    }
    
    # Allow admins
    allow if {
      "admin" in input.jwt.roles
    }
```

### Configuration ConfigMap

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: rest-rego-config
  namespace: production
data:
  WELLKNOWN_OIDC: "https://login.microsoftonline.com/YOUR-TENANT/v2.0/.well-known/openid-configuration"
  JWT_AUDIENCES: "api://production-api"
```

### Ingress Configuration

```yaml
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: my-app
  namespace: production
  annotations:
    nginx.ingress.kubernetes.io/ssl-redirect: "true"
    cert-manager.io/cluster-issuer: "letsencrypt-prod"
spec:
  ingressClassName: nginx
  tls:
  - hosts:
    - api.example.com
    secretName: my-app-tls
  rules:
  - host: api.example.com
    http:
      paths:
      - path: /
        pathType: Prefix
        backend:
          service:
            name: my-app
            port:
              number: 80
```

### ServiceMonitor (Prometheus Operator)

```yaml
apiVersion: monitoring.coreos.com/v1
kind: ServiceMonitor
metadata:
  name: rest-rego
  namespace: production
spec:
  selector:
    matchLabels:
      app: my-app
  endpoints:
  - port: metrics
    interval: 30s
    path: /metrics
```

## Scaling

### Horizontal Scaling

rest-rego is stateless and scales horizontally with your application.

#### HorizontalPodAutoscaler

```yaml
apiVersion: autoscaling/v2
kind: HorizontalPodAutoscaler
metadata:
  name: my-app-hpa
  namespace: production
spec:
  scaleTargetRef:
    apiVersion: apps/v1
    kind: Deployment
    name: my-app
  minReplicas: 3
  maxReplicas: 10
  metrics:
  # Scale on CPU
  - type: Resource
    resource:
      name: cpu
      target:
        type: Utilization
        averageUtilization: 70
  
  # Scale on memory
  - type: Resource
    resource:
      name: memory
      target:
        type: Utilization
        averageUtilization: 80
  
  # Scale on request rate
  - type: Pods
    pods:
      metric:
        name: restrego_requests_total
      target:
        type: AverageValue
        averageValue: "1000"  # 1000 req/s per pod
  
  # Scale on latency
  - type: Pods
    pods:
      metric:
        name: restrego_request_duration_seconds_p99
      target:
        type: AverageValue
        averageValue: "50m"  # 50ms P99
  
  behavior:
    scaleDown:
      stabilizationWindowSeconds: 300
      policies:
      - type: Percent
        value: 50
        periodSeconds: 60
    scaleUp:
      stabilizationWindowSeconds: 60
      policies:
      - type: Percent
        value: 100
        periodSeconds: 30
```

### Resource Recommendations

| Workload | Requests (CPU/Memory) | Limits (CPU/Memory) | Expected Throughput |
|----------|----------------------|---------------------|---------------------|
| **Low** | 100m / 128Mi | 500m / 256Mi | ~1000 req/s |
| **Medium** | 250m / 256Mi | 1000m / 512Mi | ~5000 req/s |
| **High** | 500m / 512Mi | 2000m / 1Gi | ~10000 req/s |

### Vertical Scaling

For CPU-intensive policies, increase CPU allocation:

```yaml
resources:
  requests:
    cpu: 500m
  limits:
    cpu: 2000m
```

### PodDisruptionBudget

Ensure availability during rolling updates:

```yaml
apiVersion: policy/v1
kind: PodDisruptionBudget
metadata:
  name: my-app-pdb
  namespace: production
spec:
  minAvailable: 2
  selector:
    matchLabels:
      app: my-app
```

## Security Best Practices

### Policy Integrity

```yaml
# Mount policies read-only
volumeMounts:
- name: policies
  mountPath: /policies
  readOnly: true

# Use ConfigMap with controlled access
volumes:
- name: policies
  configMap:
    name: my-app-policies
    defaultMode: 0444  # Read-only
```

### Container Security

```yaml
securityContext:
  # Pod-level
  runAsNonRoot: true
  runAsUser: 2000
  fsGroup: 2000
  seccompProfile:
    type: RuntimeDefault

containers:
- name: rest-rego
  securityContext:
    # Container-level
    runAsNonRoot: true
    runAsUser: 1000
    readOnlyRootFilesystem: true
    allowPrivilegeEscalation: false
    capabilities:
      drop: [ALL]
```

### Network Policies

Restrict traffic to rest-rego:

```yaml
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: rest-rego-netpol
  namespace: production
spec:
  podSelector:
    matchLabels:
      app: my-app
  policyTypes:
  - Ingress
  - Egress
  
  ingress:
  # Allow from ingress controller
  - from:
    - namespaceSelector:
        matchLabels:
          name: ingress-nginx
    ports:
    - protocol: TCP
      port: 8181
  
  # Allow from Prometheus
  - from:
    - namespaceSelector:
        matchLabels:
          name: monitoring
    ports:
    - protocol: TCP
      port: 8182
  
  egress:
  # Allow to backend (same pod)
  - to:
    - podSelector:
        matchLabels:
          app: my-app
    ports:
    - protocol: TCP
      port: 8080
  
  # Allow DNS
  - to:
    - namespaceSelector:
        matchLabels:
          name: kube-system
    ports:
    - protocol: UDP
      port: 53
  
  # Allow to OIDC provider
  - to:
    - namespaceSelector: {}
    ports:
    - protocol: TCP
      port: 443
```

### Secrets Management

Use Kubernetes Secrets for sensitive configuration:

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: rest-rego-secrets
  namespace: production
type: Opaque
stringData:
  AZURE_TENANT: "your-tenant-id"
---
apiVersion: apps/v1
kind: Deployment
spec:
  template:
    spec:
      containers:
      - name: rest-rego
        env:
        - name: AZURE_TENANT
          valueFrom:
            secretKeyRef:
              name: rest-rego-secrets
              key: AZURE_TENANT
```

### Azure Workload Identity

For Azure authentication without secrets:

```yaml
apiVersion: v1
kind: ServiceAccount
metadata:
  name: my-app
  namespace: production
  annotations:
    azure.workload.identity/client-id: "your-client-id"
---
apiVersion: apps/v1
kind: Deployment
spec:
  template:
    metadata:
      labels:
        azure.workload.identity/use: "true"
    spec:
      serviceAccountName: my-app
```

## High Availability

### Multi-Zone Deployment

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: my-app
spec:
  replicas: 6  # 2 per zone
  template:
    spec:
      topologySpreadConstraints:
      - maxSkew: 1
        topologyKey: topology.kubernetes.io/zone
        whenUnsatisfiable: DoNotSchedule
        labelSelector:
          matchLabels:
            app: my-app
      
      affinity:
        podAntiAffinity:
          preferredDuringSchedulingIgnoredDuringExecution:
          - weight: 100
            podAffinityTerm:
              labelSelector:
                matchLabels:
                  app: my-app
              topologyKey: kubernetes.io/hostname
```

### Circuit Breaker Pattern

Configure timeouts to fail fast:

```yaml
env:
- name: READ_TIMEOUT
  value: "10s"
- name: WRITE_TIMEOUT
  value: "30s"
- name: BACKEND_DIAL_TIMEOUT
  value: "5s"
- name: BACKEND_RESPONSE_TIMEOUT
  value: "20s"
```

### Health Check Tuning

```yaml
livenessProbe:
  httpGet:
    path: /healthz
    port: 8182
  initialDelaySeconds: 2
  periodSeconds: 10
  timeoutSeconds: 2
  failureThreshold: 3  # 30 seconds until restart

readinessProbe:
  httpGet:
    path: /readyz
    port: 8182
  initialDelaySeconds: 2
  periodSeconds: 5
  timeoutSeconds: 2
  failureThreshold: 2  # 10 seconds until removed from service
  successThreshold: 1
```

## Network Patterns

### Sidecar Pattern (Recommended)

```
┌─────────────────────────────────────┐
│           Kubernetes Pod            │
│                                     │
│  ┌──────────┐      ┌─────────────┐ │
│  │          │:8080 │             │ │
│  │   App    │◄─────┤  rest-rego  │ │
│  │          │      │             │ │
│  └──────────┘      └─────────────┘ │
│                       ▲             │
│                       │:8181        │
└───────────────────────┼─────────────┘
                        │
                    External
                    Traffic
```

**Benefits:**
- Minimal latency (localhost communication)
- Independent scaling
- Policies deployed with application
- Fault isolation per service

### Gateway Pattern

```
┌──────────────┐      ┌─────────────┐
│              │:8080 │             │
│  Backend 1   │◄─────┤             │
│              │      │             │
└──────────────┘      │             │
                      │  rest-rego  │
┌──────────────┐      │   Gateway   │
│              │:8080 │             │
│  Backend 2   │◄─────┤             │
│              │      │             │
└──────────────┘      └─────────────┘
                          ▲
                          │:8181
                      External
                      Traffic
```

**Benefits:**
- Centralized authorization point
- Easier policy management
- Suitable for legacy systems

**Drawbacks:**
- Single point of failure
- Network latency
- Harder to scale

### Service Mesh Integration

rest-rego can work alongside service meshes (Istio, Linkerd):

```yaml
# Use rest-rego for authorization, service mesh for mTLS
apiVersion: security.istio.io/v1beta1
kind: AuthorizationPolicy
metadata:
  name: allow-rest-rego
spec:
  selector:
    matchLabels:
      app: my-app
  rules:
  - to:
    - operation:
        ports: ["8181"]  # Only allow traffic to rest-rego
```

## Related Documentation

- [Configuration Reference](./CONFIGURATION.md) - All configuration options
- [Observability Guide](./OBSERVABILITY.md) - Monitoring and logging
- [Policy Development](./POLICY.md) - Writing policies
- [Troubleshooting](./TROUBLESHOOTING.md) - Common issues
- [Examples](../examples/kubernetes/) - Complete deployment examples
