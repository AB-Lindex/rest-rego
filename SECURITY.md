# Security Policy

## Reporting Security Vulnerabilities

**DO NOT** open public GitHub issues for security vulnerabilities. Instead:
- Email maintainers privately (see GitHub repository for contact info)
- Include detailed reproduction steps
- Allow reasonable time for a fix before public disclosure

## Supported Versions

We recommend always using the latest release. Only the latest version receives security updates.

## Threat Model and Security Boundaries

### What rest-rego Protects Against ✅

- Request authentication (JWT/Azure validation)
- Policy-based authorization enforcement
- Input validation and secure defaults
- Fail-closed behavior on errors
- Connection security and timeouts

### What You Must Protect ⚠️

#### 1. Policy Integrity and Deployment Security

**⚠️ CRITICAL: rest-rego assumes policies are trusted and secured by your deployment.**

If an attacker can modify policies, they've already compromised your authorization system (they can set `allow := true` to bypass everything).

**Your Responsibilities**:
- 🔒 Secure policy files with read-only filesystem permissions
- 🔒 Use version control (Git) and code review for policy changes
- 🔒 Test policies before deployment (OPA test, CI/CD checks)
- 🔒 Use immutable containers or read-only ConfigMaps in Kubernetes

**Example: Kubernetes Read-Only Policy Mount**

```yaml
apiVersion: v1
kind: Pod
spec:
  containers:
  - name: rest-rego
    volumeMounts:
    - name: policies
      mountPath: /policies
      readOnly: true  # ✅ Read-only mount
    securityContext:
      readOnlyRootFilesystem: true
      runAsNonRoot: true
      allowPrivilegeEscalation: false
  volumes:
  - name: policies
    configMap:
      name: rest-rego-policies
      defaultMode: 0444  # ✅ Read-only permissions
```

#### 2. Infrastructure Security

rest-rego does **not** provide:
- TLS/SSL termination (use ingress/load balancer)
- Rate limiting or DDoS protection (use ingress/CDN)
- Host/container security (keep images updated, apply patches)
- Network segmentation (use network policies)

#### 3. Backend Service Security

rest-rego is a Policy Enforcement Point (PEP), not a complete security solution. Your backend must still:
- Validate inputs independently (defense in depth)
- Protect against injection attacks
- Implement its own security controls

#### 4. Secrets Management

rest-rego does **not** manage secrets. Use:
- Kubernetes Secrets, HashiCorp Vault, or cloud provider secrets management
- Azure Workload Identity or similar (avoid hardcoded credentials)
- Environment variables (never hardcode secrets)

#### 5. Observability and Monitoring

rest-rego provides basic metrics and logs. You must provide:
- Log aggregation and analysis (SIEM integration)
- Alerting on suspicious patterns
- Audit trail retention and compliance reporting

## Security Best Practices

### Policy Development

```rego
# ✅ GOOD: Explicit deny-by-default
package policies
import future.keywords.if

default allow := false  # Always default deny

allow if {
    input.jwt.appid == "expected-app-id"
    input.request.method == "GET"
    startswith(input.request.path[0], "public")
}

# ❌ BAD: Never do this!
# allow := true
```

### Deployment Security

**Test policies before deployment**:
```bash
opa test policies/ -v
```

**Use minimal privileges** (Kubernetes example):
```yaml
securityContext:
  runAsNonRoot: true
  readOnlyRootFilesystem: true
  allowPrivilegeEscalation: false
  capabilities:
    drop: [ALL]
```

**Keep updated**: Subscribe to release notifications and test updates in staging first.

## Security Features

rest-rego is designed with security by default:

- **Fail-Closed**: All errors result in access denial (never fail-open)
- **Input Validation**: JWT/Azure tokens validated before policy evaluation
- **Secure Defaults**: Deny unless explicitly allowed in policy
- **Timeout Protection**: Connection and read/write timeouts prevent resource exhaustion

## Known Limitations

rest-rego does **not** provide:
- TLS termination (use ingress/load balancer)
- Rate limiting or WAF (use ingress/CDN)
- SIEM or advanced audit logging (collect logs externally)
- Policy signing (implement in your deployment pipeline)

## Additional Resources

- Review the [README.md](README.md) for deployment and configuration guidance
- Check GitHub issues for non-sensitive security discussions
- Contact maintainers privately for vulnerability reports

---

**Remember**: Security is a shared responsibility. rest-rego enforces authorization policies, but secure deployment requires proper policy management, infrastructure security, and operational practices.
