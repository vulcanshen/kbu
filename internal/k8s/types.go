package k8s

// ClusterInfo holds metadata about the currently connected Kubernetes cluster.
type ClusterInfo struct {
	ContextName string
	ClusterName string
	ServerURL   string
	Namespace   string
}

// ResourceType identifies a Kubernetes resource kind supported by km8.
type ResourceType int

const (
	ResourceNamespaces ResourceType = iota
	ResourceNodes
	ResourcePods
	ResourceDeployments
	ResourceDaemonSets
	ResourceStatefulSets
	ResourceJobs
	ResourceCronJobs
	ResourceServices
	ResourceIngresses
	ResourceConfigMaps
	ResourceSecrets
	ResourceEvents
	ResourceServiceAccounts
	ResourceClusterRoles
	ResourceClusterRoleBindings
	ResourceRoles
	ResourceRoleBindings
)

// String returns the human-readable name of the resource type.
func (r ResourceType) String() string {
	switch r {
	case ResourceNamespaces:
		return "Namespaces"
	case ResourceNodes:
		return "Nodes"
	case ResourcePods:
		return "Pods"
	case ResourceDeployments:
		return "Deployments"
	case ResourceDaemonSets:
		return "DaemonSets"
	case ResourceStatefulSets:
		return "StatefulSets"
	case ResourceJobs:
		return "Jobs"
	case ResourceCronJobs:
		return "CronJobs"
	case ResourceServices:
		return "Services"
	case ResourceIngresses:
		return "Ingresses"
	case ResourceConfigMaps:
		return "ConfigMaps"
	case ResourceSecrets:
		return "Secrets"
	case ResourceEvents:
		return "Events"
	case ResourceServiceAccounts:
		return "ServiceAccounts"
	case ResourceClusterRoles:
		return "ClusterRoles"
	case ResourceClusterRoleBindings:
		return "ClusterRoleBindings"
	case ResourceRoles:
		return "Roles"
	case ResourceRoleBindings:
		return "RoleBindings"
	default:
		return "Unknown"
	}
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
	switch r {
	case ResourceDeployments, ResourceDaemonSets, ResourceStatefulSets,
		ResourceJobs, ResourceCronJobs, ResourcePods:
		return true
	}
	return false
}

// ChildResourceType returns what resource type the drill-down shows.
func (r ResourceType) ChildResourceType() ResourceType {
	switch r {
	case ResourceDeployments, ResourceDaemonSets, ResourceStatefulSets, ResourceJobs:
		return ResourcePods
	case ResourceCronJobs:
		return ResourceJobs
	}
	return -1
}

// KubectlName returns the kubectl resource name (e.g. "pod", "deployment").
func (r ResourceType) KubectlName() string {
	switch r {
	case ResourceNamespaces:
		return "namespace"
	case ResourceNodes:
		return "node"
	case ResourcePods:
		return "pod"
	case ResourceDeployments:
		return "deployment"
	case ResourceDaemonSets:
		return "daemonset"
	case ResourceStatefulSets:
		return "statefulset"
	case ResourceJobs:
		return "job"
	case ResourceCronJobs:
		return "cronjob"
	case ResourceServices:
		return "service"
	case ResourceIngresses:
		return "ingress"
	case ResourceConfigMaps:
		return "configmap"
	case ResourceSecrets:
		return "secret"
	case ResourceEvents:
		return "event"
	case ResourceServiceAccounts:
		return "serviceaccount"
	case ResourceClusterRoles:
		return "clusterrole"
	case ResourceClusterRoleBindings:
		return "clusterrolebinding"
	case ResourceRoles:
		return "role"
	case ResourceRoleBindings:
		return "rolebinding"
	default:
		return "unknown"
	}
}

// AllResourceTypes returns all supported resource types in display order.
func AllResourceTypes() []ResourceType {
	return []ResourceType{
		ResourceNamespaces,
		ResourceNodes,
		ResourcePods,
		ResourceDeployments,
		ResourceDaemonSets,
		ResourceStatefulSets,
		ResourceJobs,
		ResourceCronJobs,
		ResourceServices,
		ResourceIngresses,
		ResourceConfigMaps,
		ResourceSecrets,
		ResourceEvents,
		ResourceServiceAccounts,
		ResourceClusterRoles,
		ResourceClusterRoleBindings,
		ResourceRoles,
		ResourceRoleBindings,
	}
}
