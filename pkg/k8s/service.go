package k8s

import (
	"fmt"
	"os"

	aemv1beta1 "github.com/xumak-grid/aem-operator/pkg/apis/aem/v1beta1"
	"k8s.io/api/core/v1"
	v1beta1 "k8s.io/api/extensions/v1beta1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/kubernetes"
)

// CreateServices handles service creation for deployment
func CreateServices(client kubernetes.Interface, deployment *aemv1beta1.AEMDeployment) error {
	return initialService(client, deployment)
}

// initialService creates a new DNS-based service discovery to access all the pods
// for more info see Kubernetes DNS-Based Service Discovery documentation
// the entrypoints that match with the service are based on app: aem and deployment:<deployment-name>
func initialService(client kubernetes.Interface, deployment *aemv1beta1.AEMDeployment) error {
	labels := map[string]string{
		"vendor":     VendorAdobe,
		"app":        AppAEM,
		"deployment": deployment.Name,
	}
	svc := &v1.Service{
		Spec: v1.ServiceSpec{
			Selector: map[string]string{
				"app":        "aem",
				"deployment": deployment.Name,
			},
			ClusterIP:                "None",
			PublishNotReadyAddresses: true,
			Ports: []v1.ServicePort{
				// This port is not necessary for the discovery, but is required when create a new service
				v1.ServicePort{
					Name: "http",
					Port: 80,
					TargetPort: intstr.IntOrString{
						IntVal: 80,
					},
				},
			},
		},
	}
	svc.Name = deployment.Name
	svc.Namespace = deployment.Namespace
	svc.Labels = labels
	if deployment.AsOwnerReference() != nil {
		svc.OwnerReferences = append(svc.OwnerReferences, *deployment.AsOwnerReference())
	}
	_, err := client.CoreV1().Services(deployment.Namespace).Create(svc)
	if err != nil && !errors.IsAlreadyExists(err) {
		return err
	}
	return nil
}

// CreateExternalEndpoint exposes a single instance by creating a service and a ingress.
func CreateExternalEndpoint(client kubernetes.Interface, instanceName, runmode string, deployment *aemv1beta1.AEMDeployment) error {
	port := 4502
	switch runmode {
	case "publish":
		port = 4503
	case "dispatcher":
		port = 80
	}

	servicePort := 80
	svcName := MakeServiceName(instanceName)
	selector := map[string]string{
		"vendor":     VendorAdobe,
		"app":        AppAEM,
		"runmode":    runmode,
		"deployment": deployment.Name,
		"name":       instanceName,
	}

	svc := &v1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      svcName,
			Namespace: deployment.Namespace,
		},
		Spec: v1.ServiceSpec{
			Selector: selector,
			Type:     v1.ServiceTypeClusterIP,
			Ports: []v1.ServicePort{
				v1.ServicePort{
					Name: "http",
					Port: int32(servicePort),
					TargetPort: intstr.IntOrString{
						IntVal: int32(port),
					},
				},
			},
		},
	}
	if deployment.AsOwnerReference() != nil {
		svc.OwnerReferences = append(svc.OwnerReferences, *deployment.AsOwnerReference())
	}
	_, err := client.CoreV1().Services(deployment.Namespace).Create(svc)
	if err != nil && !errors.IsAlreadyExists(err) {
		return err
	}

	ingressHost := fmt.Sprintf("%s-%s.%s", instanceName, deployment.Namespace, os.Getenv("GRID_EXTERNAL_DOMAIN"))
	ingress := &v1beta1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Name:      MakeIngressName(instanceName),
			Namespace: deployment.Namespace,
			Annotations: map[string]string{
				"ingress.kubernetes.io/force-ssl-redirect": "true",
				"kubernetes.io/ingress.class":              "contour",
			},
		},
		Spec: v1beta1.IngressSpec{
			Rules: []v1beta1.IngressRule{
				v1beta1.IngressRule{
					Host: ingressHost,
					IngressRuleValue: v1beta1.IngressRuleValue{
						HTTP: &v1beta1.HTTPIngressRuleValue{
							Paths: []v1beta1.HTTPIngressPath{
								v1beta1.HTTPIngressPath{
									Path: "/",
									Backend: v1beta1.IngressBackend{
										ServiceName: svcName,
										ServicePort: intstr.FromInt(servicePort),
									},
								},
							},
						},
					},
				},
			},
			TLS: []v1beta1.IngressTLS{
				v1beta1.IngressTLS{
					Hosts: []string{
						ingressHost,
					},
					SecretName: deployment.Namespace + "-public-tls",
				},
			},
		},
	}
	if deployment.AsOwnerReference() != nil {
		ingress.OwnerReferences = append(ingress.OwnerReferences, *deployment.AsOwnerReference())
	}
	_, err = client.ExtensionsV1beta1().Ingresses(deployment.Namespace).Create(ingress)
	if err != nil && !errors.IsAlreadyExists(err) {
		return err
	}
	return nil
}

// MakeServiceName returns a desired name of a service
func MakeServiceName(podName string) string {
	return fmt.Sprintf("%s-controller-svc", podName)
}

// MakeIngressName returns a desired name of an ingress
func MakeIngressName(podName string) string {
	return fmt.Sprintf("%s-ingress", podName)
}
