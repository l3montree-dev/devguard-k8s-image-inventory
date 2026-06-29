package kubernetes

import (
	"context"
	"fmt"
	"log/slog"
	"os"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/cache"

	libk8s "github.com/ckotzbauer/libk8soci/pkg/kubernetes"
	"github.com/ckotzbauer/libk8soci/pkg/oci"
)

type KubeClient struct {
	Client             *libk8s.KubeClient
	ignoreAnnotations  bool
	fallbackPullSecret []*oci.KubeCreds
}

type PodInfo struct {
	libk8s.PodInfo
	OwnerReferences meta.OwnerReference
}

var (
	AnnotationTemplate = "devguard.org/%s"
	/* #nosec */
	jobSecretName       = "devguard-operator-job-config"
	JobName             = "devguard-operator-job"
	updatePodMaxRetries = 3
)

func NewClient(ignoreAnnotations bool, fallbackPullSecretName string) *KubeClient {
	client := libk8s.NewClient()

	sbomOperatorNamespace := os.Getenv("POD_NAMESPACE")
	fallbackPullSecret := loadFallbackPullSecret(client, sbomOperatorNamespace, fallbackPullSecretName)
	return &KubeClient{Client: client, ignoreAnnotations: ignoreAnnotations, fallbackPullSecret: fallbackPullSecret}
}

func (client *KubeClient) StartPodInformer(podLabelSelector string, handler cache.ResourceEventHandlerFuncs) (cache.SharedIndexInformer, error) {
	informer := client.Client.CreatePodInformer(podLabelSelector)
	_, err := informer.AddEventHandler(handler)
	if err != nil {
		return nil, err
	}

	err = informer.SetTransform(func(x interface{}) (interface{}, error) {
		pod := x.(*corev1.Pod).DeepCopy()

		return &corev1.Pod{
				ObjectMeta: meta.ObjectMeta{
					Name:            pod.Name,
					Namespace:       pod.Namespace,
					Annotations:     pod.Annotations,
					Labels:          pod.Labels,
					OwnerReferences: pod.OwnerReferences,
				},
				Status: corev1.PodStatus{
					InitContainerStatuses:      pod.Status.InitContainerStatuses,
					EphemeralContainerStatuses: pod.Status.EphemeralContainerStatuses,
					ContainerStatuses:          pod.Status.ContainerStatuses,
				},
				Spec: corev1.PodSpec{
					ImagePullSecrets: pod.Spec.ImagePullSecrets,
				},
			},
			nil
	})

	return informer, err
}

func loadFallbackPullSecret(client *libk8s.KubeClient, namespace, name string) []*oci.KubeCreds {
	var fallbackPullSecret []*oci.KubeCreds

	if name != "" {
		if namespace == "" {
			slog.Debug("please specify the environment variable 'POD_NAMESPACE' in order to use the fallbackPullSecret")
		} else {
			fallbackPullSecret = client.LoadSecrets(namespace, []corev1.LocalObjectReference{{Name: name}})
		}
	}

	return fallbackPullSecret
}

func (client *KubeClient) ExtractPodInfos(pod corev1.Pod) PodInfo {
	owner, err := client.getOwner(pod.Namespace, pod.OwnerReferences)
	if err != nil {
		slog.Warn("failed to get owner reference for pod, proceeding without owner reference", "namespace", pod.Namespace, "pod", pod.Name, "err", err)
		owner = v1.OwnerReference{
			Kind: "Pod",
			Name: pod.Name,
		}
	}
	return PodInfo{
		PodInfo:         client.Client.ExtractPodInfos(pod),
		OwnerReferences: owner,
	}
}

func (client *KubeClient) InjectPullSecrets(pod libk8s.PodInfo) {
	for _, container := range pod.Containers {
		container.Image.PullSecrets = client.Client.LoadSecrets(pod.PodNamespace, pod.PullSecretNames)

		if client.fallbackPullSecret != nil {
			container.Image.PullSecrets = append(container.Image.PullSecrets, client.fallbackPullSecret...)
		}
	}
}

func (client *KubeClient) getOwner(namespace string, refs []v1.OwnerReference) (v1.OwnerReference, error) {
	for _, ref := range refs {
		if ref.Controller != nil && *ref.Controller {

			if ref.Kind == "Deployment" || ref.Kind == "StatefulSet" || ref.Kind == "DaemonSet" {
				return ref, nil
			}

			switch ref.Kind {
			case "ReplicaSet":
				// list the owner of the replicaset
				rs, err := client.Client.Client.AppsV1().ReplicaSets(namespace).Get(context.Background(), ref.Name, v1.GetOptions{})
				if err != nil {
					return v1.OwnerReference{}, err
				}

				return client.getOwner(namespace, rs.OwnerReferences)
			default:
				return ref, nil
			}

		} else {
			continue
		}

	}
	return v1.OwnerReference{}, fmt.Errorf("no controller owner reference found")
}

func (client *KubeClient) LoadImageInfos(namespaces []corev1.Namespace, podLabelSelector string) ([]PodInfo, []ImageInNamespace) {
	podInfos := make([]PodInfo, 0)
	allImages := make([]ImageInNamespace, 0)

	for _, ns := range namespaces {
		pods, err := client.Client.Client.CoreV1().Pods(ns.Name).List(context.Background(), meta.ListOptions{LabelSelector: podLabelSelector})
		if err != nil {
			slog.Warn("failed to list pods", "namespace", ns.Name, "err", err)
			continue
		}

		for _, pod := range pods.Items {
			info := client.ExtractPodInfos(pod)
			podInfos = append(podInfos, info)
			ownerName := info.OwnerReferences.Name
			for _, container := range info.Containers {
				allImages = append(allImages, ImageInNamespace{Namespace: pod.Namespace, Image: container.Image, ContainerName: container.Name, ControllerName: &ownerName})
			}
		}
	}

	return podInfos, allImages
}

func (client *KubeClient) UpdatePodAnnotation(pod libk8s.PodInfo) {
	for i := 0; i < updatePodMaxRetries; i++ {
		err := client.updatePodAnnotation(pod)
		if err == nil {
			break
		}

		slog.Warn("Failed to update annotation", "namespace", pod.PodNamespace, "name", pod.PodName)
	}
}

func (client *KubeClient) updatePodAnnotation(pod libk8s.PodInfo) error {
	newPod, err := client.Client.Client.CoreV1().Pods(pod.PodNamespace).Get(context.Background(), pod.PodName, meta.GetOptions{})
	if err != nil {
		if !errors.IsNotFound(err) {
			return fmt.Errorf("pod could not be fetched: %w", err)
		}

		return nil
	}

	ann := newPod.Annotations
	if ann == nil {
		ann = make(map[string]string)
	}

	for _, c := range newPod.Status.ContainerStatuses {
		ann[fmt.Sprintf(AnnotationTemplate, c.Name)] = c.ImageID
	}

	for _, c := range newPod.Status.InitContainerStatuses {
		ann[fmt.Sprintf(AnnotationTemplate, c.Name)] = c.ImageID
	}

	for _, c := range newPod.Status.EphemeralContainerStatuses {
		ann[fmt.Sprintf(AnnotationTemplate, c.Name)] = c.ImageID
	}

	newPod.Annotations = ann

	_, err = client.Client.Client.CoreV1().Pods(newPod.Namespace).Update(context.Background(), newPod, meta.UpdateOptions{})
	if err != nil {
		return fmt.Errorf("pod could not be updated: %w", err)
	}

	return nil
}

func (client *KubeClient) HasAnnotation(annotations map[string]string, container *libk8s.ContainerInfo) bool {
	if annotations == nil || client.ignoreAnnotations {
		return false
	}

	if val, ok := annotations[fmt.Sprintf(AnnotationTemplate, container.Name)]; ok {
		return val == container.Image.ImageID
	}

	return false
}

func (client *KubeClient) CreateJobSecret(namespace, suffix string, data []byte) error {
	m := make(map[string][]byte)
	m["image-config.json"] = data
	vTrue := true
	vFalse := false

	s := &corev1.Secret{
		ObjectMeta: meta.ObjectMeta{
			Namespace: namespace,
			Name:      fmt.Sprintf("%s-%s", jobSecretName, suffix),
			OwnerReferences: []meta.OwnerReference{
				{
					APIVersion:         "v1",
					Kind:               "Pod",
					Name:               os.Getenv("POD_NAME"),
					UID:                types.UID(os.Getenv("POD_UID")),
					BlockOwnerDeletion: &vTrue,
					Controller:         &vFalse,
				},
			},
		},
		Data: m,
	}

	_, err := client.Client.Client.CoreV1().Secrets(namespace).Create(context.Background(), s, meta.CreateOptions{})
	return err
}

func (client *KubeClient) CreateJob(namespace, suffix, image, pullSecrets string, timeout int64, envs map[string]string) (*batchv1.Job, error) {
	backoffLimit := int32(0)
	vTrue := true
	vFalse := false

	j := &batchv1.Job{
		ObjectMeta: meta.ObjectMeta{
			Namespace: namespace,
			Name:      fmt.Sprintf("%s-%s", JobName, suffix),
			OwnerReferences: []meta.OwnerReference{
				{
					APIVersion:         "v1",
					Kind:               "Pod",
					Name:               os.Getenv("POD_NAME"),
					UID:                types.UID(os.Getenv("POD_UID")),
					BlockOwnerDeletion: &vTrue,
					Controller:         &vFalse,
				},
			},
		},
		Spec: batchv1.JobSpec{
			BackoffLimit:          &backoffLimit,
			ActiveDeadlineSeconds: &timeout,
			Template: corev1.PodTemplateSpec{
				ObjectMeta: meta.ObjectMeta{
					Name: JobName,
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  "sbom",
							Image: image,
							Env:   mapToEnvVars(envs),
							SecurityContext: &corev1.SecurityContext{
								Privileged: &vTrue,
							},
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      "config",
									MountPath: "/sbom",
								},
							},
						},
					},
					RestartPolicy:    corev1.RestartPolicyNever,
					ImagePullSecrets: createPullSecrets(pullSecrets),
					Volumes: []corev1.Volume{
						{
							Name: "config",
							VolumeSource: corev1.VolumeSource{
								Secret: &corev1.SecretVolumeSource{
									SecretName: fmt.Sprintf("%s-%s", jobSecretName, suffix),
								},
							},
						},
					},
				},
			},
		},
	}

	return client.Client.Client.BatchV1().Jobs(namespace).Create(context.Background(), j, meta.CreateOptions{})
}

func createPullSecrets(name string) []corev1.LocalObjectReference {
	refs := make([]corev1.LocalObjectReference, 0)

	if name != "" {
		refs = append(refs, corev1.LocalObjectReference{Name: name})
	}

	return refs
}

func mapToEnvVars(m map[string]string) []corev1.EnvVar {
	vars := make([]corev1.EnvVar, 0)
	for k, v := range m {
		vars = append(vars, corev1.EnvVar{Name: k, Value: v})
	}

	return vars
}

func (client *KubeClient) CreateConfigMap(namespace, name, imageId string, data []byte) error {
	cm := corev1.ConfigMap{
		ObjectMeta: meta.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels: map[string]string{
				"devguard.org": "true",
			},
			Annotations: map[string]string{
				fmt.Sprintf(AnnotationTemplate, "image-id"): imageId,
			},
		},
		BinaryData: map[string][]byte{"sbom": data},
	}

	_, err := client.Client.Client.CoreV1().ConfigMaps(namespace).Create(context.Background(), &cm, meta.CreateOptions{})
	return err
}

func (client *KubeClient) ListConfigMaps() ([]corev1.ConfigMap, error) {
	list, err := client.Client.Client.CoreV1().ConfigMaps("").List(context.Background(), meta.ListOptions{LabelSelector: "devguard.org=true"})
	return list.Items, err
}

func (client *KubeClient) DeleteConfigMap(cm corev1.ConfigMap) error {
	return client.Client.Client.CoreV1().ConfigMaps(cm.Namespace).Delete(context.Background(), cm.Name, meta.DeleteOptions{})
}
