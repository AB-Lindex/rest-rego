# Basic Auth Example

This example shows how to deploy rest-rego as a sidecar using HTTP Basic Auth for authentication. Credentials are stored in an htpasswd file mounted from a Kubernetes Secret.

## Files

| File              | Description                                                 |
|-------------------|-------------------------------------------------------------|
| `deployment.yaml` | Deployment with rest-rego sidecar and htpasswd volume mount |
| `secret.yaml`     | Kubernetes Secret containing the htpasswd file              |
| `request.rego`    | Rego policy that uses `input.request.auth.user`             |

## Prerequisites

- Kubernetes cluster with `kubectl` configured
- `htpasswd` tool (`apache2-utils` on Debian/Ubuntu, `httpd-tools` on RHEL/Fedora)

## Quick Start

### 1. Create the namespace

```bash
kubectl create namespace demo
```

### 2. Generate credentials

Replace the example hashes in `secret.yaml` with your own:

```bash
# Generate bcrypt hashes (cost 12 recommended)
htpasswd -B -C 12 -c users.htpasswd alice
htpasswd -B -C 12 users.htpasswd bob

# Display the file contents to copy into secret.yaml
cat users.htpasswd
```

Update the `users.htpasswd` value in `secret.yaml` with the output.

Alternatively, create the Secret directly from the file:

```bash
htpasswd -B -C 12 -c users.htpasswd alice
htpasswd -B -C 12 users.htpasswd bob

kubectl create secret generic demo-htpasswd \
  --from-file=users.htpasswd \
  --namespace=demo
```

### 3. Deploy

```bash
# Apply all manifests (excluding secret if created above)
kubectl apply -f secret.yaml -f deployment.yaml -n demo

# Or apply everything at once
kubectl apply -f . -n demo
```

### 4. Verify

```bash
# Check pods are running
kubectl get pods -n demo

# Check sidecar logs for successful credential loading
kubectl logs -n demo -l k8s-app=demo -c sidecar | grep basicauth

# Test with valid credentials
curl -u alice:yourpassword http://<CLUSTER-IP>:8181/

# Test without credentials (should return 401)
curl http://<CLUSTER-IP>:8181/
```

## Policy Overview

The included `request.rego` policy demonstrates three rules:

| Rule                   | Description                                        |
|------------------------|----------------------------------------------------|
| Any authenticated user | Allows requests from any user in the htpasswd file |
| Public paths           | Allows unauthenticated access to `/public/*`       |
| Admin paths            | Restricts `/admin/*` to the `alice` account only   |

`input.request.auth.user` contains the authenticated username. `input.request.auth.password` is always an empty string — passwords are cleared before policy evaluation.

## Updating Credentials

Update the htpasswd file and replace the Secret:

```bash
# Re-generate the htpasswd file
htpasswd -B -C 12 -c users.htpasswd alice
htpasswd -B -C 12 users.htpasswd bob

# Replace the Secret
kubectl create secret generic demo-htpasswd \
  --from-file=users.htpasswd \
  --namespace=demo \
  --dry-run=client -o yaml | kubectl apply -f -
```

rest-rego detects the file change via `fsnotify` and reloads credentials automatically — no pod restart required.

## See Also

- [BASIC-AUTH.md](../../../docs/BASIC-AUTH.md) — Complete Basic Auth documentation
- [CONFIGURATION.md](../../../docs/CONFIGURATION.md) — All configuration options
- [POLICY.md](../../../docs/POLICY.md) — Rego policy reference
