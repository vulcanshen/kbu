package k8s

// ClusterInfo holds metadata about the currently connected Kubernetes cluster.
type ClusterInfo struct {
	ContextName string
	ClusterName string
	ServerURL   string
	Namespace   string
}

// ResourceType identifies a Kubernetes resource kind supported by km8.
type ResourceType string

const (
	ResourceNamespaces          ResourceType = "namespaces"
	ResourceNodes               ResourceType = "nodes"
	ResourcePods                ResourceType = "pods"
	ResourceDeployments         ResourceType = "deployments"
	ResourceDaemonSets          ResourceType = "daemonsets"
	ResourceStatefulSets        ResourceType = "statefulsets"
	ResourceJobs                ResourceType = "jobs"
	ResourceCronJobs            ResourceType = "cronjobs"
	ResourceServices            ResourceType = "services"
	ResourceIngresses           ResourceType = "ingresses"
	ResourceConfigMaps          ResourceType = "configmaps"
	ResourceSecrets             ResourceType = "secrets"
	ResourceEvents              ResourceType = "events"
	ResourceClusterRoles        ResourceType = "clusterroles"
	ResourceClusterRoleBindings ResourceType = "clusterrolebindings"
	ResourceRoles               ResourceType = "roles"
	ResourceRoleBindings        ResourceType = "rolebindings"
	ResourceServiceAccounts     ResourceType = "serviceaccounts"

	ResourcePersistentVolumes      ResourceType = "persistentvolumes"
	ResourcePersistentVolumeClaims ResourceType = "persistentvolumeclaims"
	ResourceStorageClasses         ResourceType = "storageclasses"

	ResourceHorizontalPodAutoscalers ResourceType = "horizontalpodautoscalers"
	ResourcePodDisruptionBudgets     ResourceType = "poddisruptionbudgets"

	ResourceNetworkPolicies ResourceType = "networkpolicies"
	ResourceEndpointSlices  ResourceType = "endpointslices"
	ResourceIngressClasses  ResourceType = "ingressclasses"
)

// String returns the human-readable name of the resource type.
func (r ResourceType) String() string {
	if def := DefaultRegistry.Get(r); def != nil {
		return def.DisplayName
	}
	return string(r)
}

// ResourceItem holds a single resource's table row and metadata.
type ResourceItem struct {
	Row       []string
	Name      string
	Namespace string
	UID       string
	Raw       interface{}
}

// ResourceDetail holds structured detail for a resource.
//
// YAML, if non-empty, is the canonical serialized form of the resource — the
// detail panel renders it instead of the structured Fields/Containers when
// available. Structured fields are still used for synthetic detail views
// (e.g. container drill-down) that have no native YAML.
type ResourceDetail struct {
	Name        string
	Namespace   string
	Kind        string
	UID         string
	CreatedAt   string
	Labels      map[string]string
	Annotations map[string]string
	Fields      []DetailField
	Containers  []ContainerInfo
	YAML        string
}

// DetailField is a key-value pair for resource-specific detail.
type DetailField struct {
	Label string
	Value string
}

// ContainerInfo holds detail about a single container in a Pod.
type ContainerInfo struct {
	Name     string
	Image    string
	State    string
	Ready    bool
	Restarts int
	Ports    string
	Init     bool
}

// EventItem holds a single event for display.
type EventItem struct {
	Type    string
	Reason  string
	Object  string
	Message string
	Age     string
}

// SupportsDrillDown returns true if this resource type can drill down to children.
func (r ResourceType) SupportsDrillDown() bool {
	if def := DefaultRegistry.Get(r); def != nil {
		return def.DrillDown != nil
	}
	return false
}

// KubectlName returns the kubectl resource name (e.g. "pod", "deployment").
func (r ResourceType) KubectlName() string {
	if def := DefaultRegistry.Get(r); def != nil {
		return def.KubectlName
	}
	return string(r)
}

// AllResourceTypes returns all supported resource types in display order.
func AllResourceTypes() []ResourceType {
	return DefaultRegistry.AllTypes()
}
