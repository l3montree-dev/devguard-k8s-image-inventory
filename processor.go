package main

import (
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	libk8s "github.com/ckotzbauer/libk8soci/pkg/kubernetes"
	"github.com/ckotzbauer/libk8soci/pkg/oci"
	"github.com/l3montree-dev/devguard-k8s-image-inventory/kubernetes"
	parser "github.com/novln/docker-parser"

	"k8s.io/client-go/tools/cache"

	corev1 "k8s.io/api/core/v1"
)

type sbomScanner interface {
	ExecuteTrivy(img *oci.RegistryImage) (string, error)
}

type Processor struct {
	K8s      *kubernetes.KubeClient
	trivy    sbomScanner
	Targets  []Target
	imageMap map[string]bool
}

func NewProcessor(k8s *kubernetes.KubeClient, triv *Trivy) *Processor {
	targets := initTargets()

	return &Processor{K8s: k8s, trivy: triv, Targets: targets, imageMap: make(map[string]bool)}
}

func (p *Processor) ListenForPods() {
	namespaces, err := p.K8s.NewNamespaceFilter(ProvidedConfig.NamespaceLabelSelector)
	if err != nil {
		slog.Error("Invalid namespace label selector.", "err", err)
		return
	}

	var informer cache.SharedIndexInformer
	informer, err = p.K8s.StartPodInformer(ProvidedConfig.PodLabelSelector, namespaces, cache.ResourceEventHandlerFuncs{
		UpdateFunc: func(old, new any) {
			oldPod := old.(*corev1.Pod)
			newPod := new.(*corev1.Pod)
			oldInfo := p.K8s.ExtractPodInfos(*oldPod)
			newInfo := p.K8s.ExtractPodInfos(*newPod)

			var removedContainers []*libk8s.ContainerInfo
			newInfo.Containers, removedContainers = getChangedContainers(oldInfo.PodInfo, newInfo.PodInfo)
			p.scanPod(newInfo)

			p.cleanupImagesIfNeeded(newInfo, removedContainers, informer.GetStore().List())
		},
		DeleteFunc: func(obj any) {
			pod := obj.(*corev1.Pod)
			info := p.K8s.ExtractPodInfos(*pod)

			p.cleanupImagesIfNeeded(info, info.Containers, informer.GetStore().List())
		},
	})

	if err != nil {
		slog.Error("Can't listen for pod-changes.", "err", err)
		return
	}

	p.runInformerAsync(informer, namespaces)
}

func (p *Processor) ProcessAllPods(pods []kubernetes.PodInfo, allImages []kubernetes.ImageInNamespace) {
	p.executeScans(pods, allImages)
}

func (p *Processor) scanPod(pod kubernetes.PodInfo) {
	errOccurred := false
	p.K8s.InjectPullSecrets(pod.PodInfo)

	for _, container := range pod.Containers {
		key := kubernetes.ImageInNamespace{Namespace: pod.PodNamespace, Image: container.Image}.String()
		if /*p.K8s.HasAnnotation(pod.Annotations, container) ||*/ p.imageMap[key] {
			slog.Debug("Image already scanned", "image", container.Image.Image)
			continue
		}

		sbom, err := p.trivy.ExecuteTrivy(container.Image)
		if err != nil {
			slog.Error("scan failed", "err", err, "image", container.Image.Image)
			errOccurred = true
			continue
		}

		delivered := true
		for _, t := range p.Targets {
			err = t.ProcessSbom(NewContext(sbom, container.Image, container, &pod))
			delivered = delivered && err == nil
		}

		if !delivered {
			errOccurred = true
			continue
		}

		// only skip the image next time if the scan and the delivery succeeded
		p.imageMap[key] = true
	}

	if !errOccurred && len(pod.Containers) > 0 {
		p.K8s.UpdatePodAnnotation(pod.PodInfo)
	}
}

func initTargets() []Target {
	targets := make([]Target, 0)

	if ProvidedConfig.DevGuardToken == "" || ProvidedConfig.DevGuardProjectURL == "" {
		panic("DevGuard token and project URL must be provided in the configuration.")
	}

	t := NewDevGuardTarget(ProvidedConfig.DevGuardToken, ProvidedConfig.DevGuardProjectURL, nil)
	targets = append(targets, t)

	return targets
}

func (p *Processor) executeScans(pods []kubernetes.PodInfo, allImages []kubernetes.ImageInNamespace) {
	for _, pod := range pods {
		p.scanPod(pod)
	}

	for _, t := range p.Targets {
		targetImages, err := t.LoadImages()

		if err != nil {
			slog.Error("Failed to load images from target", "err", err)
			continue
		}

		removableImages := make([]kubernetes.ImageInNamespace, 0)
		for _, t := range targetImages {
			//	slog.Debug("Checking image for removal", "image", t.String())
			if !containsImage(allImages, t) {
				removableImages = append(removableImages, t)
				delete(p.imageMap, t.String())

				slog.Debug("Image marked for removal", "image", t.String())
			}
		}

		if len(removableImages) > 0 {
			t.Remove(removableImages)
		}
	}
}

func getChangedContainers(oldPod, newPod libk8s.PodInfo) ([]*libk8s.ContainerInfo, []*libk8s.ContainerInfo) {
	addedContainers := make([]*libk8s.ContainerInfo, 0)
	removedContainers := make([]*libk8s.ContainerInfo, 0)
	for _, c := range newPod.Containers {
		if !containsContainerImage(oldPod.Containers, c.Image.ImageID) {
			addedContainers = append(addedContainers, c)
		}
	}

	for _, c := range oldPod.Containers {
		if !containsContainerImage(newPod.Containers, c.Image.ImageID) {
			removedContainers = append(removedContainers, c)
		}
	}

	return addedContainers, removedContainers
}

func containsImage(images []kubernetes.ImageInNamespace, target kubernetes.ImageInNamespace) bool {
	targetParsed, err := parser.Parse(target.Image.Image)
	if err != nil {
		return false
	}

	for _, candidate := range images {
		if candidate.Namespace != target.Namespace || candidate.ContainerName != target.ContainerName || (candidate.ControllerName != nil && target.ControllerName != nil && *candidate.ControllerName != *target.ControllerName) {
			continue
		}
		candidateParsed, err := parser.Parse(candidate.Image.Image)
		if err != nil {
			continue
		}
		if candidate.Namespace == target.Namespace && candidateParsed.Remote() == targetParsed.Remote() {
			return true
		}
	}

	return false
}

func containsContainerImage(containers []*libk8s.ContainerInfo, image string) bool {
	for _, c := range containers {
		if c.Image.ImageID == image {
			return true
		}
	}

	return false
}

func (p *Processor) cleanupImagesIfNeeded(info kubernetes.PodInfo, removedContainers []*libk8s.ContainerInfo, allPods []any) {
	images := make([]kubernetes.ImageInNamespace, 0)

	for _, c := range removedContainers {
		found := false
		for _, po := range allPods {
			pod := po.(*corev1.Pod)
			info := p.K8s.ExtractPodInfos(*pod)
			found = found || containsContainerImage(info.Containers, c.Image.ImageID)
		}

		if !found {
			imageWithNamespace := kubernetes.ImageInNamespace{Namespace: info.PodNamespace, Image: c.Image, ContainerName: c.Name, ControllerName: &info.OwnerReferences.Name}
			images = append(images, imageWithNamespace)
			delete(p.imageMap, imageWithNamespace.String())

			slog.Debug("Image marked for removal", "image", imageWithNamespace.String())

		}
	}

	if len(images) > 0 {
		for _, t := range p.Targets {
			t.Remove(images)
		}
	}
}

func (p *Processor) runInformerAsync(informer cache.SharedIndexInformer, namespaces *kubernetes.NamespaceFilter) {
	stop := make(chan struct{})
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		run := true
		for run {
			sig := <-sigs
			switch sig {
			case syscall.SIGTERM, syscall.SIGINT:
				slog.Info("Received signal to stop", "signal", sig)
				close(stop)
				run = false
			}
		}
	}()

	go func() {

		slog.Info("Start pod-informer")
		informer.Run(stop)
		slog.Info("Pod-informer has stopped")
		os.Exit(0)
	}()

	go func() {
		slog.Info("Wait for cache to be synced")
		if !cache.WaitForCacheSync(stop, informer.HasSynced) {
			slog.Error("Timed out waiting for the cache to sync")
			panic("Timed out waiting for the cache to sync")
		}

		slog.Info("Finished cache sync")
		pods := informer.GetStore().List()
		missingPods := make([]kubernetes.PodInfo, 0)
		allImages := make([]kubernetes.ImageInNamespace, 0)

		for _, t := range p.Targets {
			targetImages, err := t.LoadImages()

			if err != nil {
				slog.Error("Failed to load images from target", "err", err)
				continue
			}

			for _, po := range pods {
				pod := po.(*corev1.Pod)
				if !namespaces.Selected(pod.Namespace) {
					continue
				}

				slog.Debug("Pod found", "pod", pod.Name, "namespace", pod.Namespace)
				info := p.K8s.ExtractPodInfos(*pod)
				for _, c := range info.Containers {
					allImages = append(allImages, kubernetes.ImageInNamespace{Namespace: info.PodNamespace, Image: c.Image, ContainerName: c.Name, ControllerName: &info.OwnerReferences.Name})

					if !containsImage(targetImages, kubernetes.ImageInNamespace{
						Image:          c.Image,
						Namespace:      info.PodNamespace,
						ContainerName:  c.Name,
						ControllerName: &info.OwnerReferences.Name,
					}) {
						missingPods = append(missingPods, info)
						slog.Debug("Pod needs to be analyzed", "pod", info.PodName, "namespace", info.PodNamespace)
						break
					}
				}
			}

			// check for images which need to be deleted
			removableImages := make([]kubernetes.ImageInNamespace, 0)
			for _, t := range targetImages {
				slog.Debug("Checking image for removal", "image", t.String())
				if !containsImage(allImages, t) {
					removableImages = append(removableImages, t)
					delete(p.imageMap, t.String())

					slog.Debug("Image marked for removal", "image", t.String())
				}
			}

			if len(removableImages) > 0 {
				slog.Info("Remove images from target", "amount", len(removableImages))
				t.Remove(removableImages)
			}
		}

		if len(missingPods) > 0 {
			slog.Info("Execute initial scans on missing pods", "amount", len(missingPods))
			p.executeScans(missingPods, allImages)
		}

		slog.Info("Finished initial scan")
	}()
}
