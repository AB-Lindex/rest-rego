# Examples

## Kubernetes

In the `kubernetes`-folder is a regular deployment with service, serviceaccount, ingress.

Our additions in the a sidecar in the deployment and a configmap with the policy-file.

Also, since the main container now isn't exposed, it should not declare any ports.

```yaml
# kustomization.yaml
...
configMapGenerator:
  - name: policies
    files:
      - request.rego
...
```

```yaml
# deployment.yaml
...
# spec.template.spec.containers

        # additional container
        - name: sidecar
          image: lindex/rest-rego
          imagePullPolicy: IfNotPresent

          # port to access main container
          env:
            - name: BACKEND_PORT
              value: '10000'
            - name: AZURE_TENANT
              value: '10101010-2020-3030-4040-505050505050'

          # rest-repo standard port
          ports:
            - containerPort: 8181
              name: http

          # probes to the internal port
          livenessProbe:
            httpGet:
              path: /healthz
              port: 8182
            initialDelaySeconds: 2
          readinessProbe:
            httpGet:
              path: /readyz
              port: 8182

          # mount config into default location
          volumeMounts:
            - name: policies
              mountPath: /policies
...
# spec.template.spec

      # add the configmap-volume
      # if using subpath or specific files the hot-reload won't work
      volumes:
        - name: policies
          configMap:
            name: policies

```