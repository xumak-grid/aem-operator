package v1beta1

// DeploymentPhase represents the current phase in which a deployment may be.
type DeploymentPhase string

// Deployment Phases
const (
	DeploymentPhaseNone     DeploymentPhase = ""
	DeploymentPhaseCreating                 = "Creating"
	DeploymentPhaseResizing                 = "Resizing"
	DeploymentPhaseRunning                  = "Running"
	DeploymentPhaseFailed                   = "Failed"
)

// AEMDeploymentStatus represents the status of a deployment.
type AEMDeploymentStatus struct {
	Phase DeploymentPhase `json:"phase"`
	// ControlPuased indicates the operator pauses the control of the cluster.
	ControlPaused bool `json:"controlPaused,omitempty"`
	// Current AEM Version
	Version string `json:"version"`
	// Current Dispatcher Version
	DispatcherVersion string `json:"dispatcherVersion"`
	// Represents the latest available observations of a deployment's current state.
	Conditions []DeploymentCondition `json:"conditions,omitempty"`
}

// DeploymentConditionType is the type of condition of the deployment.
type DeploymentConditionType string

// DeploymentCondition describes the state of a deployment at a certain point.
type DeploymentCondition struct {
	Type           DeploymentConditionType `json:"type"`
	Reason         string                  `json:"reason"`
	TransitionTime string                  `json:"transitionTime"`
}

// Deployment conditions.
const (
	DeploymentConditionReady              = "Ready"
	DeploymentConditionRemovingDeadMember = "RemovingDeadMember"
	DeploymentConditionRecovering         = "Recovering"
	DeploymentConditionScalingUp          = "ScalingUp"
	DeploymentConditionScalingDown        = "ScalingDown"
	DeploymentConditionGarbageCollecting  = "DataStoreGarbageCollecting"
	DeploymentConditionUpgrading          = "Upgrading"
)
