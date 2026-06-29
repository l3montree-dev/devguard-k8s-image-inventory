# devguard-k8s-image-inventory

> Kubernetes operator that continuously inventories all container images running in a cluster and reports them to [DevGuard](https://devguard.org).

[![CI](https://github.com/l3montree-dev/devguard-k8s-image-inventory/actions/workflows/devguard.yml/badge.svg)](https://github.com/l3montree-dev/devguard-k8s-image-inventory/actions/workflows/devguard.yml)

## How it works

The operator watches all pods across namespaces and generates a Software Bill of Materials (SBOM) for each container image using [Trivy](https://github.com/aquasecurity/trivy). SBOMs are uploaded to DevGuard, where images are organized by namespace, workload controller, and container name.

When images disappear from the cluster they are automatically removed from DevGuard.

Two modes are available:

- **Real-time** (default): a pod informer triggers analysis immediately when pods change, plus a full sync on startup.
- **Cron**: a scheduled interval scans all pods periodically.

## Installation

### Prerequisites

- A DevGuard account and a project URL + API token
- A Kubernetes cluster (1.29+)

### Create the token secret

```bash
kubectl create namespace devguard

kubectl create secret generic devguard-k8s-image-inventory \
  --namespace devguard \
  --from-literal=devguard-token=<your-devguard-token>
```

### Apply the manifests

```bash
kubectl apply -f deploy/rbac.yaml
kubectl apply -f deploy/deployment.yaml
```

Edit `deploy/deployment.yaml` beforehand to set your `DEVGUARD_PROJECT_URL`:

```yaml
env:
  - name: DEVGUARD_PROJECT_URL
    value: "https://api.main.devguard.org/api/v1/organizations/<org>/projects/<project>/external/{provider-id}"
```

You can set any provider-id you would like. 

## Configuration

All configuration is read from environment variables or a YAML config file. The file path can be set via the standard config flag.

| Environment variable        | Default | Description |
|-----------------------------|---------|-------------|
| `DEVGUARD_TOKEN_FILE`       | —       | Path to a file containing the DevGuard API token (recommended for Kubernetes) |
| `DEVGUARD_PROJECT_URL`      | —       | DevGuard project URL (required) |
| `SBOM_CRON`                 | `""`    | Cron expression for scheduled mode. When empty, real-time mode is used. |
| `SBOM_POD_LABEL_SELECTOR`   | `""`    | Kubernetes label selector to restrict which pods are scanned |
| `SBOM_NAMESPACE_LABEL_SELECTOR` | `""` | Kubernetes label selector to restrict which namespaces are scanned |
| `SBOM_IGNORE_ANNOTATIONS`   | `false` | When `true`, re-scans all images even if already processed |
| `SBOM_FALLBACK_PULL_SECRET` | `""`    | Name of a pull secret (in the operator namespace) used as fallback for private registries |
| `SBOM_REGISTRY_PROXY`       | `[]`    | Registry proxy mappings, e.g. `docker.io=my-mirror.example.com`. Repeatable. |
| `SBOM_JOB_TIMEOUT`          | `3600`  | Maximum seconds a scan job may run |
| `SBOM_VERBOSITY`            | `info`  | Log level (`debug`, `info`, `warn`, `error`) |

## RBAC

The operator requires cluster-wide read access to pods, namespaces, secrets, and ReplicaSets. The manifests in `deploy/rbac.yaml` create a `ServiceAccount`, `ClusterRole`, and `ClusterRoleBinding` scoped to the minimum required permissions.

## Security

- Runs as non-root user (`UID 53111`) on a read-only root filesystem
- The DevGuard token is mounted as a file from a Kubernetes secret — never passed as an environment variable
- All capabilities are dropped; privilege escalation is disabled
- Seccomp profile set to `RuntimeDefault`
- Base image: `registry.opencode.de/oci-community/images/zendis/static` (distroless-equivalent, pinned by digest)

## Development

```bash
go build ./...
go test ./...
```

## License

[Apache 2.0](LICENSE)