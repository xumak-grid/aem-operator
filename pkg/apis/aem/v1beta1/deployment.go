package v1beta1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// AEMDeployment  represents an AEM deployment.
type AEMDeployment struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata"`
	Spec              AEMDeploymentSpec   `json:"spec"`
	Status            AEMDeploymentStatus `json:"status"`
}

// AsOwnerReference gets an owner reference pointing to the deployment.
func (d *AEMDeployment) AsOwnerReference() *metav1.OwnerReference {
	isController := true
	return &metav1.OwnerReference{
		APIVersion: SchemeGroupVersion.String(),
		Kind:       ResourceKind,
		Name:       d.Name,
		UID:        d.UID,
		Controller: &isController,
	}
}

// InstanceSpec represents the specification for an instance type
// including type: `small, medium, large` and number of replicas.
type InstanceSpec struct {
	Type     string `json:"type"`
	Replicas int    `json:"replicas"`
}

// AEMDeploymentSpec represents the deployment specification.
type AEMDeploymentSpec struct {
	// selector is a label query over pods that should match the replica count.
	// If empty, defaulted to labels on the meta.
	// +optional
	Selector *metav1.LabelSelector `json:"selector,omitempty" protobuf:"bytes,2,opt,name=selector"`
	// Authors is the expected number of Adobe AEM authoring instances in the
	// deployment. It is currently not possible to have more than 1 authoring
	// environment in a deployment.
	//
	// The operator will eventually make the size of the running deployment equal
	// to the expected size.
	//
	// Options: "1"
	// Default: "1"
	Authors InstanceSpec `json:"authors"`

	// Publishers is the expected number of Adobe AEM publishing instances in the
	// deployment.
	//
	// The operator will eventually make the size of the running deployment equal
	// to the expected size.
	//
	// Options: "1", "2", "3", "4"
	// Default: "2"
	Publishers InstanceSpec `json:"publishers"`

	// Dispatchers is the expected number of Apache Webserver + Adobe Dispatcher
	// instances in the deployment.
	//
	// The operator will eventually make the size of the running deployment equal
	// to the expected size.
	//
	// Options: "1", "2", "3", "4"
	// Default: "2"
	Dispatchers InstanceSpec `json:"dispatchers"`

	// Version is the expected version of Adobe AEM for the deployment.
	//
	// The operator will eventually make the deployment version equal to the
	// expected version.
	//
	// Options: "6.1", "6.2", "6.3"
	// Default: "6.2"
	Version string `json:"version"`

	// DispatcherVersion is the expected version of Adobe Dispatcher for the
	// deployment.
	//
	// The operator will eventually make the deployment version equal to the
	// expected version.
	//
	// Default: "4.1.12"
	DispatcherVersion string `json:"dispatcherVersion"`

	// Paused is to pause control of the deployment by the operator.
	Paused bool `json:"paused,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// AEMDeploymentList is a list of Deployments.
type AEMDeploymentList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`
	Items           []AEMDeployment `json:"items"`
}
