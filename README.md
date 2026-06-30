# devguard-k8s-image-inventory

> A daemon that runs inside your Kubernetes cluster, watches all pods, generates a CycloneDX SBOM for each container image using Trivy, and reports it to [DevGuard](https://main.devguard.org).

[![CI](https://github.com/l3montree-dev/devguard-k8s-image-inventory/actions/workflows/devguard.yml/badge.svg)](https://github.com/l3montree-dev/devguard-k8s-image-inventory/actions/workflows/devguard.yml)

## Documentation

📖 **[https://docs.devguard.org/how-to-guides/integrations/k8s-image-inventory/](https://docs.devguard.org/how-to-guides/integrations/k8s-image-inventory/)**

Installation, configuration reference, and usage examples are maintained there.

## Trivy Configuration

The container image sets `TRIVY_CONFIG=/etc/devguard/trivy.yaml`. Mount a ConfigMap to that path to customize Trivy's behavior without changing any flags:

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: trivy-config
data:
  trivy.yaml: |
    severity:
      - HIGH
      - CRITICAL
    skip-update: true
```

Then mount it in the deployment:

```yaml
volumeMounts:
  - name: trivy-config
    mountPath: /etc/devguard
volumes:
  - name: trivy-config
    configMap:
      name: trivy-config
```

## Development

```bash
go build ./...
go test ./...
```

## License

[Apache 2.0](LICENSE)
