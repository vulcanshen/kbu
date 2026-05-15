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

// ChildResourceType returns what resource type the drill-down shows.
func (r ResourceType) ChildResourceType() ResourceType {
	if def := DefaultRegistry.Get(r); def != nil && def.DrillDown != nil {
		return def.DrillDown.ChildType
	}
	return ""
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
