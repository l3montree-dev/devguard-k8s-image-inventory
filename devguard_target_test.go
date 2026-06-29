package main

import (
	"testing"

	libk8s "github.com/ckotzbauer/libk8soci/pkg/kubernetes"
	"github.com/ckotzbauer/libk8soci/pkg/oci"
	"github.com/l3montree-dev/devguard-k8s-image-inventory/kubernetes"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func ptr(s string) *string { return &s }

func makePodInfo(namespace, ownerKind, ownerName string) *kubernetes.PodInfo {
	return &kubernetes.PodInfo{
		PodInfo:         libk8s.PodInfo{PodNamespace: namespace},
		OwnerReferences: metav1.OwnerReference{Kind: ownerKind, Name: ownerName},
	}
}

func TestBuildImageNameFromArtifact(t *testing.T) {
	tests := map[string]struct {
		input    string
		expected string
	}{
		"packageURL": {
			input:    "pkg:oci/myimage@sha256:abc123?repository_url=ghcr.io/org",
			expected: "ghcr.io/org:sha256:abc123",
		},
		"plainImageRef": {
			input:    "ghcr.io/org/myimage:latest",
			expected: "ghcr.io/org/myimage:latest",
		},
		"missingAt": {
			input:    "pkg:oci/myimage",
			expected: "pkg:oci/myimage",
		},
		"missingRepositoryURL": {
			input:    "pkg:oci/myimage@sha256:abc123",
			expected: "pkg:oci/myimage@sha256:abc123",
		},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			assert.Equal(t, tc.expected, buildImageNameFromArtifact(tc.input))
		})
	}
}

func TestBuildArtifactName(t *testing.T) {
	tests := map[string]struct {
		image    string
		expected string
	}{
		"alreadyPackageURL": {
			image:    "pkg:oci/myimage@sha256:abc?repository_url=ghcr.io",
			expected: "pkg:oci/myimage@sha256:abc?repository_url=ghcr.io",
		},
		"taggedImage": {
			image:    "ghcr.io/org/myimage:v1.2.3",
			expected: "pkg:oci/myimage@v1.2.3?repository_url=ghcr.io/org/myimage",
		},
		"noTagDefaultsToLatest": {
			image:    "ghcr.io/org/myimage",
			expected: "pkg:oci/myimage@latest?repository_url=ghcr.io/org/myimage",
		},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			assert.Equal(t, tc.expected, buildArtifactName(&oci.RegistryImage{Image: tc.image}))
		})
	}
}

func TestIsWorkloadOwner(t *testing.T) {
	tests := map[string]struct {
		kind     string
		expected bool
	}{
		"deployment":  {kind: "Deployment", expected: true},
		"daemonSet":   {kind: "DaemonSet", expected: true},
		"statefulSet": {kind: "StatefulSet", expected: true},
		"job":         {kind: "Job", expected: false},
		"cronJob":     {kind: "CronJob", expected: false},
		"empty":       {kind: "", expected: false},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			assert.Equal(t, tc.expected, isWorkloadOwner(tc.kind))
		})
	}
}

func TestBuildSbomPayload(t *testing.T) {
	image := &oci.RegistryImage{Image: "ghcr.io/org/app:v1", ImageID: "ghcr.io/org/app:v1"}

	t.Run("workloadOwner", func(t *testing.T) {
		ctx := &TargetContext{
			Image:     image,
			Container: &libk8s.ContainerInfo{Name: "app"},
			Pod:       makePodInfo("production", "Deployment", "my-deployment"),
			Sbom:      `{"bomFormat":"CycloneDX"}`,
		}
		p := buildSbomPayload(ctx)
		assert.Equal(t, DevGuardRequest{
			Verb:                       "update",
			ProjectExternalEntityID:    "production",
			ProjectName:                "production",
			ProjectDescription:         "Namespace",
			SubProjectExternalEntityID: "my-deployment",
			SubProjectName:             "my-deployment",
			SubProjectDescription:      "Deployment",
			AssetExternalEntityID:      "app",
			AssetName:                  "app",
			AssetDescription:           "container",
			AssetVersionName:           "latest",
			Artifact:                   "pkg:oci/app@v1?repository_url=ghcr.io/org/app",
			Sbom:                       []byte(`{"bomFormat":"CycloneDX"}`),
		}, p)
	})

	t.Run("nonWorkloadOwner", func(t *testing.T) {
		ctx := &TargetContext{
			Image:     image,
			Container: &libk8s.ContainerInfo{Name: "app"},
			Pod:       makePodInfo("production", "Job", "my-job"),
			Sbom:      `{"bomFormat":"CycloneDX"}`,
		}
		p := buildSbomPayload(ctx)
		assert.Equal(t, DevGuardRequest{
			Verb:                    "update",
			ProjectExternalEntityID: "production",
			ProjectName:             "production",
			ProjectDescription:      "Namespace",
			AssetExternalEntityID:   "app",
			AssetName:               "app",
			AssetDescription:        "container controlled by Job my-job",
			AssetVersionName:        "latest",
			Artifact:                "pkg:oci/app@v1?repository_url=ghcr.io/org/app",
			Sbom:                    []byte(`{"bomFormat":"CycloneDX"}`),
		}, p)
	})
}

func TestBuildDeletePayload(t *testing.T) {
	t.Run("withController", func(t *testing.T) {
		img := kubernetes.ImageInNamespace{
			Namespace:      "staging",
			ControllerName: ptr("my-deployment"),
			ContainerName:  "app",
			Image:          &oci.RegistryImage{Image: "ghcr.io/org/app:v1"},
		}
		p := buildDeletePayload(img)
		assert.Equal(t, DevGuardRequest{
			Verb:                       "delete",
			ProjectExternalEntityID:    "staging",
			ProjectName:                "staging",
			SubProjectExternalEntityID: "my-deployment",
			AssetExternalEntityID:      "app",
			AssetName:                  "app",
			AssetVersionName:           "latest",
			Artifact:                   "pkg:oci/app@v1?repository_url=ghcr.io/org/app",
		}, p)
	})

	t.Run("noController", func(t *testing.T) {
		img := kubernetes.ImageInNamespace{
			Namespace:     "staging",
			ContainerName: "app",
			Image:         &oci.RegistryImage{Image: "ghcr.io/org/app:v1"},
		}
		p := buildDeletePayload(img)
		assert.Equal(t, DevGuardRequest{
			Verb:                    "delete",
			ProjectExternalEntityID: "staging",
			ProjectName:             "staging",
			AssetExternalEntityID:   "app",
			AssetName:               "app",
			AssetVersionName:        "latest",
			Artifact:                "pkg:oci/app@v1?repository_url=ghcr.io/org/app",
		}, p)
	})
}

func TestFlattenProjectAssets(t *testing.T) {
	t.Run("empty", func(t *testing.T) {
		assert.Empty(t, flattenProjectAssets(nil))
		assert.Empty(t, flattenProjectAssets([]projectAssetsResponse{}))
	})

	t.Run("topLevelAssets", func(t *testing.T) {
		assets := []projectAssetsResponse{
			{
				ProjectExternalEntityID: "ns-a",
				Assets: []struct {
					AssetExternalEntityID string `json:"assetExternalEntityId"`
					AssetName             string `json:"assetName"`
					AssetVersions         []struct {
						AssetVersionName string   `json:"assetVersionName"`
						Artifacts        []string `json:"artifacts"`
					} `json:"assetVersions,omitempty"`
				}{
					{
						AssetExternalEntityID: "container-a",
						AssetVersions: []struct {
							AssetVersionName string   `json:"assetVersionName"`
							Artifacts        []string `json:"artifacts"`
						}{
							{Artifacts: []string{"pkg:oci/app@sha256:abc?repository_url=ghcr.io/org"}},
						},
					},
				},
			},
		}
		result := flattenProjectAssets(assets)
		assert.Equal(t, []kubernetes.ImageInNamespace{
			{
				Namespace:     "ns-a",
				ContainerName: "container-a",
				Image: &oci.RegistryImage{
					Image:   "ghcr.io/org:sha256:abc",
					ImageID: "ghcr.io/org:sha256:abc",
				},
			},
		}, result)
	})

	t.Run("subProjectAssets", func(t *testing.T) {
		assets := []projectAssetsResponse{
			{
				ProjectExternalEntityID: "ns-b",
				SubProjects: []struct {
					SubProjectExternalEntityID string `json:"subProjectExternalEntityId,omitempty"`
					SubProjectName             string `json:"subProjectName,omitempty"`
					SubProjectDescription      string `json:"subProjectDescription,omitempty"`
					Assets                     []struct {
						AssetExternalEntityID string `json:"assetExternalEntityId"`
						AssetName             string `json:"assetName"`
						AssetVersions         []struct {
							AssetVersionName string   `json:"assetVersionName"`
							Artifacts        []string `json:"artifacts"`
						} `json:"assetVersions,omitempty"`
					} `json:"assets"`
				}{
					{
						SubProjectExternalEntityID: "my-deploy",
						Assets: []struct {
							AssetExternalEntityID string `json:"assetExternalEntityId"`
							AssetName             string `json:"assetName"`
							AssetVersions         []struct {
								AssetVersionName string   `json:"assetVersionName"`
								Artifacts        []string `json:"artifacts"`
							} `json:"assetVersions,omitempty"`
						}{
							{
								AssetExternalEntityID: "sidecar",
								AssetVersions: []struct {
									AssetVersionName string   `json:"assetVersionName"`
									Artifacts        []string `json:"artifacts"`
								}{
									{Artifacts: []string{"pkg:oci/sidecar@sha256:def?repository_url=ghcr.io/org"}},
								},
							},
						},
					},
				},
			},
		}
		result := flattenProjectAssets(assets)
		assert.Len(t, result, 1)
		assert.Equal(t, "ns-b", result[0].Namespace)
		assert.Equal(t, "sidecar", result[0].ContainerName)
		assert.Equal(t, ptr("my-deploy"), result[0].ControllerName)
		assert.Equal(t, &oci.RegistryImage{
			Image:   "ghcr.io/org:sha256:def",
			ImageID: "ghcr.io/org:sha256:def",
		}, result[0].Image)
	})
}
