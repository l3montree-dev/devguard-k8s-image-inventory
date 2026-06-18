package main

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"sync"

	"github.com/l3montree-dev/devguard/pkg/devguard"
	parser "github.com/novln/docker-parser"

	libk8s "github.com/ckotzbauer/libk8soci/pkg/oci"
	"github.com/l3montree-dev/devguard-operator/kubernetes"
)

type DevGuardTarget struct {
	projectURL string
	token      string
	tags       []string
	client     devguard.HTTPClient
}

type DevGuardRequest struct {
	Verb                       string          `json:"verb"`
	ProjectExternalEntityID    string          `json:"projectExternalEntityId"`
	ProjectName                string          `json:"projectName"`
	ProjectDescription         string          `json:"projectDescription,omitempty"`
	SubProjectExternalEntityID string          `json:"subProjectExternalEntityId,omitempty"`
	SubProjectName             string          `json:"subProjectName,omitempty"`
	SubProjectDescription      string          `json:"subProjectDescription,omitempty"`
	AssetExternalEntityID      string          `json:"assetExternalEntityId"`
	AssetName                  string          `json:"assetName"`
	AssetDescription           string          `json:"assetDescription,omitempty"`
	AssetVersionName           string          `json:"assetVersionName,omitempty"`
	Artifact                   string          `json:"artifact,omitempty"`
	Sbom                       json.RawMessage `json:"sbom,omitempty"`
}

type projectAssetsResponse struct {
	ProjectExternalEntityID string `json:"projectExternalEntityId"`
	ProjectName             string `json:"projectName"`
	SubProjects             []struct {
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
	} `json:"subProjects,omitempty"`
	Assets []struct {
		AssetExternalEntityID string `json:"assetExternalEntityId"`
		AssetName             string `json:"assetName"`
		AssetVersions         []struct {
			AssetVersionName string   `json:"assetVersionName"`
			Artifacts        []string `json:"artifacts"`
		} `json:"assetVersions,omitempty"`
	} `json:"assets"`
}

func NewDevGuardTarget(token, projectURL string, tags []string) *DevGuardTarget {
	client := devguard.NewHTTPClient(token, projectURL)
	projectURL = projectURL + "/dn/devguard-operator"
	return &DevGuardTarget{
		projectURL: projectURL,
		token:      token,
		tags:       tags,
		client:     client,
	}
}

func (g *DevGuardTarget) LoadImages() ([]kubernetes.ImageInNamespace, error) {
	req, err := http.NewRequest("GET", g.projectURL, nil)
	if err != nil {
		return nil, err
	}

	resp, err := g.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, errors.New("failed to load images from DevGuard: " + resp.Status)
	}

	var assets []projectAssetsResponse
	if err := json.NewDecoder(resp.Body).Decode(&assets); err != nil {
		return nil, err
	}

	result := make([]kubernetes.ImageInNamespace, 0)
	for _, a := range assets {
		for _, asset := range a.Assets {
			for _, version := range asset.AssetVersions {
				for _, artifact := range version.Artifacts {
					result = append(result, kubernetes.ImageInNamespace{
						Namespace:     a.ProjectExternalEntityID,
						ContainerName: asset.AssetExternalEntityID,
						Image: &libk8s.RegistryImage{
							ImageID: g.buildImageNameFromArtifact(artifact),
							Image:   g.buildImageNameFromArtifact(artifact),
						},
					})
				}
			}
		}
		for _, sp := range a.SubProjects {
			for _, asset := range sp.Assets {
				for _, version := range asset.AssetVersions {
					for _, artifact := range version.Artifacts {
						result = append(result, kubernetes.ImageInNamespace{
							Namespace:      a.ProjectExternalEntityID,
							ControllerName: &sp.SubProjectExternalEntityID,
							ContainerName:  asset.AssetExternalEntityID,
							Image: &libk8s.RegistryImage{
								Image:   g.buildImageNameFromArtifact(artifact),
								ImageID: g.buildImageNameFromArtifact(artifact),
							},
						},
						)
					}
				}
			}
		}
	}

	return result, nil
}

func (g *DevGuardTarget) buildArtifactName(image *libk8s.RegistryImage) string {
	if strings.HasPrefix(image.Image, "pkg:oci/") {
		return image.Image
	}

	imageRepo, tag, shortName, err := getRepoWithVersion(image)
	if err != nil {
		slog.Error("Could not parse image!!!", "image", image.Image)
		return image.Image
	}
	if tag == "" {
		tag = "latest"
	}
	return "pkg:oci/" + shortName + "@" + tag + "?repository_url=" + imageRepo
}

func (g *DevGuardTarget) buildImageNameFromArtifact(artifact string) string {
	if !strings.HasPrefix(artifact, "pkg:oci/") {
		return artifact
	}
	parts := strings.SplitN(artifact, "@", 2)
	if len(parts) != 2 {
		return artifact
	}
	p := strings.SplitN(parts[1], "?repository_url=", 2)
	if len(p) != 2 {
		return artifact
	}
	digest := p[0]
	repo := p[1]
	return repo + ":" + digest
}

func (g *DevGuardTarget) ProcessSbom(ctx *TargetContext) error {

	if ctx.Sbom == "" {
		slog.Info("Empty SBOM - skip image", "image", ctx.Image.ImageID)
		return nil
	}

	payload := DevGuardRequest{
		Verb:                    "update",
		ProjectExternalEntityID: ctx.Pod.PodNamespace,
		ProjectName:             ctx.Pod.PodNamespace,
		ProjectDescription:      "Namespace",
		AssetExternalEntityID:   ctx.Container.Name,
		AssetName:               ctx.Container.Name,
		AssetVersionName:        "latest",
		Artifact:                g.buildArtifactName(ctx.Image),
		Sbom:                    json.RawMessage(ctx.Sbom),
	}
	if ctx.Pod.OwnerReferences.Kind == "Deployment" || ctx.Pod.OwnerReferences.Kind == "DaemonSet" || ctx.Pod.OwnerReferences.Kind == "StatefulSet" {
		payload.SubProjectExternalEntityID = ctx.Pod.OwnerReferences.Name
		payload.SubProjectName = ctx.Pod.OwnerReferences.Name
		payload.SubProjectDescription = ctx.Pod.OwnerReferences.Kind

		payload.AssetDescription = "container"
	} else {
		payload.AssetDescription = "container controlled by " + ctx.Pod.OwnerReferences.Kind + " " + ctx.Pod.OwnerReferences.Name
	}

	jsonBody, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	req, err := http.NewRequest("POST", g.projectURL, strings.NewReader(string(jsonBody)))
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", "application/json")

	slog.Info("Sending SBOM to DevGuard", "Namespace", ctx.Pod.PodNamespace, "Container", ctx.Container.Name)

	_, err = g.client.Do(req)
	if err != nil {
		slog.Error("Could not upload SBOM", "err", err)
		return err
	}

	slog.Info("Uploaded SBOM to DevGuard", "Namespace", ctx.Pod.PodNamespace, "Container", ctx.Container.Name)
	return nil
}

func (g *DevGuardTarget) Remove(images []kubernetes.ImageInNamespace) error {
	wg := sync.WaitGroup{}

	for _, img := range images {
		wg.Add(1)
		go func(img kubernetes.ImageInNamespace) {
			defer wg.Done()
			slog.Debug("Removing asset from DevGuard", "Namespace", img.Namespace, "Container", img.ContainerName)

			controllerName := ""
			if img.ControllerName != nil {
				controllerName = *img.ControllerName

			}

			payload := DevGuardRequest{
				Verb:                       "delete",
				ProjectExternalEntityID:    img.Namespace,
				ProjectName:                img.Namespace,
				SubProjectExternalEntityID: controllerName,
				AssetExternalEntityID:      img.ContainerName,
				AssetName:                  img.ContainerName,
				AssetVersionName:           "latest",
				Artifact:                   g.buildArtifactName(img.Image),
			}

			jsonBody, err := json.Marshal(payload)
			if err != nil {
				slog.Error("could not marshal delete request", "err", err)
				return
			}

			req, err := http.NewRequest("POST", g.projectURL, strings.NewReader(string(jsonBody)))
			if err != nil {
				slog.Error("could not create delete request", "err", err)
				return
			}

			req.Header.Set("Content-Type", "application/json")

			slog.Info("Deleting asset", "Namespace", img.Namespace, "Container", img.ContainerName)

			_, err = g.client.Do(req)
			if err != nil {
				slog.Error("could not delete asset", "err", err)
				return
			}
		}(img)
	}

	wg.Wait()
	return nil
}

func getRepoWithVersion(image *libk8s.RegistryImage) (string, string, string, error) {
	imageRef, err := parser.Parse(image.Image)
	if err != nil {
		slog.Error("Could not parse image", "image", image.Image)
		return "", "", "", err
	}

	return imageRef.Repository(), imageRef.Tag(), imageRef.ShortName(), nil
}
