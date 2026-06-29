package main

import (
	libk8s "github.com/ckotzbauer/libk8soci/pkg/kubernetes"
	"github.com/ckotzbauer/libk8soci/pkg/oci"
	"github.com/l3montree-dev/devguard-k8s-image-inventory/kubernetes"
)

type TargetContext struct {
	Image     *oci.RegistryImage
	Container *libk8s.ContainerInfo
	Pod       *kubernetes.PodInfo
	Sbom      string
}

type Target interface {
	ProcessSbom(ctx *TargetContext) error
	LoadImages() ([]kubernetes.ImageInNamespace, error)
	Remove(images []kubernetes.ImageInNamespace) error
}

func NewContext(sbom string, image *oci.RegistryImage, container *libk8s.ContainerInfo, pod *kubernetes.PodInfo) *TargetContext {
	return &TargetContext{image, container, pod, sbom}
}
