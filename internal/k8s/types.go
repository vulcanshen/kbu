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
	default:
		return "Unknown"
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
	}
}
