package kubernetes_test

import (
	"testing"

	libk8s "github.com/ckotzbauer/libk8soci/pkg/kubernetes"
	"github.com/l3montree-dev/devguard-k8s-image-inventory/kubernetes"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Kubernetes 1.36 + containerd >= 2.3 report only the config digest in
// status.containerStatuses[].image. The image name must come from the pod spec.
func TestExtractPodInfos_DigestOnlyStatusImageUsesSpecImage(t *testing.T) {
	pod := corev1.Pod{
		ObjectMeta: meta.ObjectMeta{Name: "p", Namespace: "opendesk"},
		Spec: corev1.PodSpec{
			InitContainers: []corev1.Container{{Name: "init", Image: "docker.io/library/busybox@sha256:aaa"}},
			Containers:     []corev1.Container{{Name: "app", Image: "registry-1.docker.io/clamav/clamav@sha256:bbb"}},
		},
		Status: corev1.PodStatus{
			InitContainerStatuses: []corev1.ContainerStatus{{
				Name: "init", Image: "sha256:111", ImageID: "docker.io/library/busybox@sha256:aaa",
			}},
			ContainerStatuses: []corev1.ContainerStatus{{
				Name: "app", Image: "sha256:247f41546a757f83fcee048f1eb6168a070ccd289ce02ce27924c7111fdb5095",
				ImageID: "registry-1.docker.io/clamav/clamav@sha256:85dbdcae",
			}},
		},
	}

	client := &kubernetes.KubeClient{Client: &libk8s.KubeClient{}}
	info := client.ExtractPodInfos(pod)

	images := map[string]string{}
	for _, c := range info.Containers {
		images[c.Name] = c.Image.Image
		assert.NotContains(t, c.Image.Image, "sha256:1", "image name must not be a bare config digest")
	}
	assert.Equal(t, "registry-1.docker.io/clamav/clamav@sha256:bbb", images["app"])
	assert.Equal(t, "docker.io/library/busybox@sha256:aaa", images["init"])
}
