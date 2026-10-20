package main

import (
	"errors"
	"testing"

	libk8s "github.com/ckotzbauer/libk8soci/pkg/kubernetes"
	"github.com/ckotzbauer/libk8soci/pkg/oci"
	"github.com/l3montree-dev/devguard-k8s-image-inventory/kubernetes"
	"github.com/stretchr/testify/assert"
)

type fakeScanner struct {
	err   error
	calls int
}

func (f *fakeScanner) ExecuteTrivy(img *oci.RegistryImage) (string, error) {
	f.calls++
	return "sbom", f.err
}

type fakeTarget struct {
	err   error
	calls int
}

func (f *fakeTarget) ProcessSbom(ctx *TargetContext) error { f.calls++; return f.err }
func (f *fakeTarget) LoadImages() ([]kubernetes.ImageInNamespace, error) {
	return nil, nil
}
func (f *fakeTarget) Remove(images []kubernetes.ImageInNamespace) error { return nil }

func podWithImage(image string) kubernetes.PodInfo {
	return kubernetes.PodInfo{PodInfo: libk8s.PodInfo{
		PodName:      "p",
		PodNamespace: "opendesk",
		Containers: []*libk8s.ContainerInfo{
			{Name: "app", Image: &oci.RegistryImage{Image: image, ImageID: image}},
		},
	}}
}

func newTestProcessor(s *fakeScanner, t *fakeTarget) *Processor {
	return &Processor{
		K8s:      &kubernetes.KubeClient{Client: &libk8s.KubeClient{}},
		trivy:    s,
		Targets:  []Target{t},
		imageMap: make(map[string]bool),
	}
}

func TestScanPodFailedScanIsRetried(t *testing.T) {
	scanner := &fakeScanner{err: errors.New("401 unauthorized")}
	target := &fakeTarget{}
	p := newTestProcessor(scanner, target)
	pod := podWithImage("registry-1.docker.io/clamav/clamav@sha256:aaa")

	p.scanPod(pod)
	p.scanPod(pod)

	assert.Equal(t, 2, scanner.calls, "a failed scan must not mark the image as scanned")
	assert.Empty(t, p.imageMap)
	assert.Equal(t, 0, target.calls)
}

func TestScanPodFailedTargetIsRetried(t *testing.T) {
	scanner := &fakeScanner{}
	target := &fakeTarget{err: errors.New("devguard down")}
	p := newTestProcessor(scanner, target)
	pod := podWithImage("quay.io/foo/bar:v1")

	p.scanPod(pod)
	p.scanPod(pod)

	assert.Equal(t, 2, scanner.calls, "an image whose sbom could not be delivered must not be marked as scanned")
	assert.Empty(t, p.imageMap)
}
