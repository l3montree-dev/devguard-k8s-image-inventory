package kubernetes_test

import (
	"fmt"
	"testing"

	"github.com/ckotzbauer/libk8soci/pkg/oci"
	"github.com/l3montree-dev/devguard-k8s-image-inventory/kubernetes"

	"github.com/stretchr/testify/assert"
)

type pullReferenceTestData struct {
	imageID  string
	image    string
	expected string
	ref      string
	dataMap  map[string]string
}

const testDigest = "sha256:0123456789012345678901234567890123456789012345678901234567890123"

func TestPullReference(t *testing.T) {
	tests := []pullReferenceTestData{
		{
			imageID:  "alpine:3.17",
			image:    "alpine:3.17",
			expected: "docker.io/library/alpine",
			ref:      "3.17",
			dataMap:  make(map[string]string),
		},
		{
			imageID:  "docker.io/alpine:3.17",
			image:    "docker.io/alpine:3.17",
			expected: "ghcr.io/library/alpine",
			ref:      "3.17",
			dataMap:  map[string]string{"docker.io": "ghcr.io"},
		},
		{
			// Pod spec without a registry host: ImageID is what makes the
			// proxy hit, and library/ has to survive for the mirror cache.
			imageID:  "docker.io/library/busybox:1.38.0",
			image:    "busybox:1.38.0",
			expected: "my.registry.com:5000/library/busybox",
			ref:      "1.38.0",
			dataMap:  map[string]string{"docker.io": "my.registry.com:5000"},
		},
		{
			// ImageID wins over Image and pins the digest the pod runs.
			imageID:  "docker.io/library/traefik@" + testDigest,
			image:    "traefik:1.2.3",
			expected: "ghcr.io/library/traefik",
			ref:      testDigest,
			dataMap:  map[string]string{"docker.io": "ghcr.io"},
		},
		{
			// Docker clients report registry-1.docker.io instead of docker.io.
			imageID:  "registry-1.docker.io/clamav/clamav@" + testDigest,
			image:    "clamav/clamav:latest",
			expected: "ghcr.io/clamav/clamav",
			ref:      testDigest,
			dataMap:  map[string]string{"docker.io": "ghcr.io"},
		},
		{
			imageID:  "index.docker.io/library/nginx:1.31-alpine",
			image:    "nginx:1.31-alpine",
			expected: "ghcr.io/library/nginx",
			ref:      "1.31-alpine",
			dataMap:  map[string]string{"docker.io": "ghcr.io"},
		},
		{
			// Unmapped registries pass through unchanged.
			imageID:  "ghcr.io/org/app:v1",
			image:    "ghcr.io/org/app:v1",
			expected: "ghcr.io/org/app",
			ref:      "v1",
			dataMap:  map[string]string{"docker.io": "ghcr.io"},
		},
		{
			imageID:  "alpine:3.17",
			image:    "alpine:3.17",
			expected: "docker.io/library/alpine",
			ref:      "3.17",
			dataMap:  map[string]string{"ghcr.io": "docker.io"},
		},
		{
			imageID:  "my.registry.com:5000/alpine:3.17",
			image:    "my.registry.com:5000/alpine:3.17",
			expected: "ghcr.io/alpine",
			ref:      "3.17",
			dataMap:  map[string]string{"my.registry.com:5000": "ghcr.io"},
		},
		{
			// An empty ImageID falls back to the pod spec.
			image:    "docker.io/alpine:3.17",
			expected: "ghcr.io/library/alpine",
			ref:      "3.17",
			dataMap:  map[string]string{"docker.io": "ghcr.io"},
		},
	}

	for i, v := range tests {
		t.Run(fmt.Sprintf("%02d_%s_%s", i, v.imageID, v.image), func(t *testing.T) {
			img := &oci.RegistryImage{ImageID: v.imageID, Image: v.image}
			repository, ref, err := kubernetes.PullReference(img, v.dataMap)
			assert.NoError(t, err)
			assert.Equal(t, v.expected, repository)
			assert.Equal(t, v.ref, ref)
			assert.Equal(t, v.imageID, img.ImageID, "ImageID must not be modified")
			assert.Equal(t, v.image, img.Image, "Image must not be modified")
		})
	}
}

func TestPullReferenceInvalidImage(t *testing.T) {
	img := &oci.RegistryImage{ImageID: "", Image: "not a valid image!!!"}
	_, _, err := kubernetes.PullReference(img, nil)
	assert.Error(t, err)
}

func TestImageInNamespaceString(t *testing.T) {
	tests := []struct {
		name     string
		image    string
		expected string
	}{
		{"plain with tag", "alpine:3.17", "ns/docker.io/library/alpine"},
		{"no tag", "alpine", "ns/docker.io/library/alpine"},
		{"registry with port", "my.registry.com:5000/team/app:1.0", "ns/my.registry.com:5000/team/app"},
		{"registry with port and no tag", "my.registry.com:5000/team/app", "ns/my.registry.com:5000/team/app"},
		{"digest only", "registry-1.docker.io/clamav/clamav@sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", "ns/registry-1.docker.io/clamav/clamav"},
		{"tag and digest", "busybox:1.37@sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", "ns/docker.io/library/busybox"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			i := kubernetes.ImageInNamespace{Namespace: "ns", Image: &oci.RegistryImage{Image: tt.image}}
			assert.Equal(t, tt.expected, i.String())
		})
	}
}
