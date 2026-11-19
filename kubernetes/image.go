package kubernetes

import (
	"fmt"
	"log/slog"

	"github.com/ckotzbauer/libk8soci/pkg/oci"
	parser "github.com/novln/docker-parser"
)

type ImageInNamespace struct {
	Namespace      string
	ControllerName *string
	ContainerName  string
	Image          *oci.RegistryImage
}

// dockerRegistryAliases maps the registry hosts Docker reports in image IDs to
// the canonical docker.io host used in registry proxy mappings.
var dockerRegistryAliases = map[string]string{
	"registry-1.docker.io": "docker.io",
	"index.docker.io":      "docker.io",
}

func (i ImageInNamespace) String() string {
	ref, err := parser.Parse(i.Image.Image)
	if err != nil {
		return i.Namespace + "/" + i.Image.Image
	}
	return i.Namespace + "/" + ref.Repository()
}

// PullReference returns the repository and reference to pull for img, with any
// configured registry proxy applied.
//
// ImageID is preferred over Image: the CRI reports it registry-qualified and
// digest-pinned, while Image is the pod-spec string (e.g. "busybox:1.38.0"),
// which usually has no registry host and may name a different tag than the one
// the pod actually runs. Neither field is modified - Image keeps naming the
// upstream image for DevGuard artifacts.
func PullReference(img *oci.RegistryImage, registryProxyMap map[string]string) (string, string, error) {
	imageRef, err := parser.Parse(img.ImageID)
	if err != nil {
		imageRef, err = parser.Parse(img.Image)
	}
	if err != nil {
		slog.Error("Could not parse image", "err", err, "imageID", img.ImageID, "image", img.Image)
		return "", "", err
	}

	registry := imageRef.Registry()
	if canonical, ok := dockerRegistryAliases[registry]; ok {
		registry = canonical
	}
	if proxyRegistry, ok := registryProxyMap[registry]; ok {
		registry = proxyRegistry
	}

	return fmt.Sprintf("%s/%s", registry, imageRef.ShortName()), imageRef.Tag(), nil
}
