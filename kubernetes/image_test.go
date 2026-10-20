package kubernetes_test

import (
	"testing"

	"github.com/ckotzbauer/libk8soci/pkg/oci"
	"github.com/l3montree-dev/devguard-k8s-image-inventory/kubernetes"

	"github.com/stretchr/testify/assert"
)

type mapTestData struct {
	input    string
	expected string
	dataMap  map[string]string
}

func TestApplyProxyRegistry(t *testing.T) {
	tests := []mapTestData{
		{
			input:    "alpine:3.17",
			expected: "alpine:3.17",
			dataMap:  make(map[string]string),
		},
		{
			input:    "docker.io/alpine:3.17",
			expected: "ghcr.io/alpine:3.17",
			dataMap:  map[string]string{"docker.io": "ghcr.io"},
		},
		{
			input:    "alpine:3.17",
			expected: "ghcr.io/alpine:3.17",
			dataMap:  map[string]string{"docker.io": "ghcr.io"},
		},
		{
			input:    "alpine:3.17",
			expected: "alpine:3.17",
			dataMap:  map[string]string{"ghcr.io": "docker.io"},
		},
		{
			input:    "alpine:3.17",
			expected: "my.registry.com:5000/alpine:3.17",
			dataMap:  map[string]string{"docker.io": "my.registry.com:5000"},
		},
		{
			input:    "my.registry.com:5000/alpine:3.17",
			expected: "ghcr.io/alpine:3.17",
			dataMap:  map[string]string{"my.registry.com:5000": "ghcr.io"},
		},
	}

	for _, v := range tests {
		t.Run(v.input, func(t *testing.T) {
			img := &oci.RegistryImage{ImageID: v.input, Image: v.input}
			err := kubernetes.ApplyProxyRegistry(img, false, v.dataMap)
			assert.NoError(t, err)
			assert.Equal(t, v.expected, img.ImageID)
		})
	}
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
