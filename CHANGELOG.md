# Changelog

All notable changes to this project will be documented in this file.

## [v1.0.2] - 2026-10-27

### Fixed

- CI workflow requests the `id-token: write` permission, which the DevGuard reusable workflow needs

## [v1.0.1] - 2026-10-27

Maintenance release. No functional changes.

### Changed

- Updated dependencies, including `devguard` to v1.17.1 and `libk8soci`
- Updated the Nix flake inputs and the vendor hashes of the Nix image build
- Pinned `k8s.io/kube-openapi` to the version required by the Kubernetes v0.37.1 libraries, since a newer version pulled in an incompatible `structured-merge-diff` major version and broke the build

## [v1.0.0] - 2026-10-20

First release of devguard-k8s-image-inventory. The project is based on [sbom-operator](https://github.com/ckotzbauer/sbom-operator) 0.36.0 and reworked to report image inventories to DevGuard.

### Added

- **DevGuard target** — SBOMs are reported to a DevGuard project, configured with a project URL. Sub-projects are supported and images are resolved from artifacts
- **Trivy instead of Syft** — SBOMs are generated with Trivy as CycloneDX JSON. Trivy is configured through `TRIVY_CONFIG`
- **Namespace label selector in real-time mode** — `SBOM_NAMESPACE_LABEL_SELECTOR` is now applied to the pod informer as well. Before, it only had an effect in cron mode while the informer watched all namespaces
- `config.example.yaml`
- OCI image built with Nix, based on a static base image

### Changed

- **Simplified configuration** — camelCase keys, unused flags removed
- HTTP requests to DevGuard use a timeout and the explicit Trivy config flag is gone
- Removed a lot of unused sbom-operator code and renamed the Go module
- Updated all dependencies (Kubernetes libraries to v0.37.1) and pinned GitHub Actions to commit SHAs

### Fixed

- **Digest-pinned images could not be scanned on Kubernetes 1.36 with containerd >= 2.3** — these runtimes report only the config digest (`sha256:...`) in `status.containerStatuses[].image`, which was parsed as the repository `docker.io/library/sha256` and failed with `401 unauthorized`. The image name now comes from the pod spec, the digest from `status.imageID` is kept
- **Images were skipped as "already scanned" after a failed scan** — an image is now only marked as scanned after the scan and the delivery of the SBOM succeeded, so failures are retried
- **Duplicate deduplication keys for images with a registry port or digest** — the key is built from the parsed repository instead of splitting at the first colon
- Wrong image name construction
- HTTP status is checked when loading images from DevGuard
- Trivy cache directory points to a writable path and the RBAC rules of the example deployment were corrected
- Nix image build works with the current devguard Trivy package
