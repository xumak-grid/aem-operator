package k8s

import (
	"fmt"
	"strconv"
	"strings"

	aemv1beta1 "github.com/xumak-grid/aem-operator/pkg/apis/aem/v1beta1"
	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

// Pod Constants
const (
	VendorAdobe                 = "adobe"
	AppAEM                      = "aem"
	AEMCRXVolumeName            = "crx"
	AEMRunmodePublish           = "publish"
	AEMRunmodeAuthor            = "author"
	AEMRunmodeDispatcher        = "dispatcher"
	AEMHealtcheckReadinessURL   = "/system/health?tags=shallow"
	AEMHealtcheckLivenessURL    = "/system/health?tags=shallow"
	DispatcherSideCarLiveness   = "/check/liveness"
	AEMDispatcherHealtcheckURL  = "/"
	EnvCQPort                   = "CQ_PORT"
	EnvCQRunmode                = "CQ_RUNMODE"
	gridRegistry                = ""
	aemContainerImage           = "grid/aem-danta:6.3-1.0.5-jdk8"
	aemDispatcherContainerImage = "grid/dispatcher:4.2.2"
	sideCarDispatcherImage      = "grid/sidecar-check-state:0.0.1"
	ConfigVolumeKeySites        = "config-volume-sites"
	ConfigVolumeKeyFarm         = "config-volume-farm"
)

// NewPod creates a new AEMPod.
func NewPod(name, runmode string, deployment *aemv1beta1.AEMDeployment) *v1.Pod {
	labels := map[string]string{
		"vendor":     VendorAdobe,
		"app":        AppAEM,
		"runmode":    runmode,
		"deployment": deployment.Name,
		"name":       name,
	}
	var (
		containers []v1.Container
		volumes    []v1.Volume
	)

	switch runmode {
	case AEMRunmodeAuthor, AEMRunmodePublish:
		containers = append(containers, aemContainer(runmode))
		volumes = []v1.Volume{
			{
				Name: AEMCRXVolumeName,
				VolumeSource: v1.VolumeSource{
					PersistentVolumeClaim: &v1.PersistentVolumeClaimVolumeSource{
						ClaimName: MakePVCName(name),
					},
				},
			},
		}
	case AEMRunmodeDispatcher:
		containers = append(containers, dispatcherContainer(name, deployment.Name, deployment.Namespace))
		containers = append(containers, dispatcherSideCar(deployment.Name))
		volumes = []v1.Volume{
			{
				Name: ConfigVolumeKeySites,
				VolumeSource: v1.VolumeSource{
					ConfigMap: &v1.ConfigMapVolumeSource{
						LocalObjectReference: v1.LocalObjectReference{
							Name: makeVolumeKey(deployment.Name, "dispatcher"),
						},
						Items: []v1.KeyToPath{
							v1.KeyToPath{
								// the key of the configMap
								Key: DispatcherVirtualHostConfigKey,
								// the path where the value of the key will be added
								Path: DispatcherVirtualHostConfigKey,
							},
						},
					},
				},
			},
			{
				Name: ConfigVolumeKeyFarm,
				VolumeSource: v1.VolumeSource{
					ConfigMap: &v1.ConfigMapVolumeSource{
						LocalObjectReference: v1.LocalObjectReference{
							Name: makeVolumeKey(deployment.Name, "dispatcher"),
						},
						Items: []v1.KeyToPath{
							v1.KeyToPath{
								// the key of the configMap
								Key: DispatcherPublishConfigKey,
								// the path where the value of the key will be added
								Path: DispatcherPublishConfigKey,
							},
						},
					},
				},
			},
		}
	}
	automountServiceAccount := false
	pod := v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:        name,
			Labels:      labels,
			Annotations: map[string]string{},
		},
		Spec: v1.PodSpec{
			Containers:                   containers,
			Volumes:                      volumes,
			Hostname:                     name,
			Subdomain:                    deployment.Name,
			AutomountServiceAccountToken: &automountServiceAccount,
		},
	}
	if deployment.AsOwnerReference() != nil {
		pod.OwnerReferences = append(pod.OwnerReferences, *deployment.AsOwnerReference())
	}
	return &pod
}

func aemContainer(runmode string) v1.Container {
	p := 4502
	jmxPort := 9010
	if runmode == AEMRunmodePublish {
		p = 4503
	}
	container := v1.Container{
		Name:            fmt.Sprintf("adobe-aem-%s", runmode),
		Image:           getFullImageURL(aemContainerImage),
		ImagePullPolicy: v1.PullIfNotPresent,
		Ports: []v1.ContainerPort{
			{
				Name:          "aem",
				ContainerPort: int32(p),
			},
			{
				Name:          "jmx",
				ContainerPort: int32(jmxPort),
				Protocol:      v1.ProtocolTCP,
			},
		},
		ReadinessProbe: &v1.Probe{
			Handler: v1.Handler{
				HTTPGet: &v1.HTTPGetAction{
					Path: AEMHealtcheckReadinessURL,
					Port: intstr.FromInt(p),
				},
			},
			InitialDelaySeconds: 100,
			PeriodSeconds:       10,
		},
		// LivenessProbe: &v1.Probe{
		// 	Handler: v1.Handler{
		// 		HTTPGet: &v1.HTTPGetAction{
		// 			Path: AEMHealtcheckLivenessURL,
		// 			Port: intstr.FromInt(p),
		// 		},
		// 	},
		// 	InitialDelaySeconds: 100,
		// 	PeriodSeconds:       360,
		// },
		Env: []v1.EnvVar{
			v1.EnvVar{
				Name:  EnvCQRunmode,
				Value: runmode,
			},
			v1.EnvVar{
				Name:  EnvCQPort,
				Value: strconv.Itoa(p),
			},
		},
		VolumeMounts: []v1.VolumeMount{
			v1.VolumeMount{
				Name:      AEMCRXVolumeName,
				MountPath: "/bin/crx-quickstart",
			},
		},
		Resources: v1.ResourceRequirements{
			Requests: v1.ResourceList{
				v1.ResourceMemory: resource.MustParse("2Gi"),
			},
			Limits: v1.ResourceList{
				v1.ResourceMemory: resource.MustParse("4Gi"),
			},
		},
	}
	return container
}

func dispatcherContainer(podName, deploymentName, ns string) v1.Container {
	httpPort := 80
	httpsPort := 443
	publishHost := matchPublishHost(podName, deploymentName, ns)
	container := v1.Container{
		Name:            makeVolumeKey(deploymentName, "dispatcher"),
		Image:           getFullImageURL(aemDispatcherContainerImage),
		ImagePullPolicy: v1.PullIfNotPresent,
		Ports: []v1.ContainerPort{
			{
				Name:          "http",
				ContainerPort: int32(httpPort),
				Protocol:      v1.ProtocolTCP,
			},
			{
				Name:          "https",
				ContainerPort: int32(httpsPort),
				Protocol:      v1.ProtocolTCP,
			},
		},
		ReadinessProbe: &v1.Probe{
			Handler: v1.Handler{
				HTTPGet: &v1.HTTPGetAction{
					Path: AEMDispatcherHealtcheckURL,
					Port: intstr.FromInt(httpPort),
				},
			},
			InitialDelaySeconds: 5,
			PeriodSeconds:       1,
		},
		Env: []v1.EnvVar{
			v1.EnvVar{
				Name:  "PUBLISH_IP",
				Value: publishHost,
			},
			v1.EnvVar{
				Name:  "PUBLISH_PORT",
				Value: "4503",
			},
		},
		VolumeMounts: []v1.VolumeMount{
			v1.VolumeMount{
				Name:      ConfigVolumeKeyFarm,
				MountPath: "/usr/local/apache2/conf/farms.d",
			},
			v1.VolumeMount{
				Name:      ConfigVolumeKeySites,
				MountPath: "/usr/local/apache2/sites-enabled",
			},
		},
	}
	return container
}

// dispatcherSideCar a container for the disptacher pod
func dispatcherSideCar(deploymentName string) v1.Container {
	httpPort := 9090
	container := v1.Container{
		Name:            makeVolumeKey(deploymentName, "sidecar"),
		Image:           getFullImageURL(sideCarDispatcherImage),
		ImagePullPolicy: v1.PullAlways,
		Ports: []v1.ContainerPort{
			{
				Name:          "http",
				ContainerPort: int32(httpPort),
				Protocol:      v1.ProtocolTCP,
			},
		},
		LivenessProbe: &v1.Probe{
			Handler: v1.Handler{
				HTTPGet: &v1.HTTPGetAction{
					Path: DispatcherSideCarLiveness,
					Port: intstr.FromInt(httpPort),
				},
			},
			InitialDelaySeconds: 100,
			PeriodSeconds:       10,
			FailureThreshold:    1,
		},
		Env: []v1.EnvVar{
			v1.EnvVar{
				// files that are cached inside de sidecar container
				Name:  "SIDE_CAR_CONFIG_FILES",
				Value: "/usr/local/apache2/conf/farms.d/publish_dispatcher.any,/usr/local/apache2/sites-enabled/bedrock.conf",
			},
		},
		VolumeMounts: []v1.VolumeMount{
			v1.VolumeMount{
				Name:      ConfigVolumeKeyFarm,
				MountPath: "/usr/local/apache2/conf/farms.d",
			},
			v1.VolumeMount{
				Name:      ConfigVolumeKeySites,
				MountPath: "/usr/local/apache2/sites-enabled",
			},
		},
	}
	return container
}

// matchPublishHost returns the matched publish host base on the dispacher name
// for example: adobe-example-aem-dispatcher-001 -> adobe-example-aem-publish-001.example-aem.demo.svc.cluster.local
// if the dispatcherName is "" returns localhost
func matchPublishHost(dispatcherName, deploymentName, ns string) string {
	publishHost := "localhost"
	segs := strings.Split(dispatcherName, "-")
	if len(segs) == 1 {
		return publishHost
	}
	id := segs[len(segs)-1]
	publishName := MakePodName(deploymentName, "publish", id)
	publishHost = GetPodHost(publishName, deploymentName, ns)
	return publishHost
}

// MakePodName returns a desired name of a pod
// example: adobe-example-aem-dispatcher-001
func MakePodName(deploymentName, runmode, id string) string {
	return fmt.Sprintf("%s-%s-%s", deploymentName, runmode, id)
}

// GetPodHost returns the pod host
// example: adobe-example-aem-publish-001.example-aem.demo
func GetPodHost(podName, serviceName, ns string) string {
	return fmt.Sprintf("%s.%s.%s", podName, serviceName, ns)
}

// getFullImageURL returns the full URL for the given image e.g. registry/image
func getFullImageURL(image string) string {
	return fmt.Sprintf("%v/%v", gridRegistry, image)
}
