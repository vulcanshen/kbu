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
//
// PodLinks is populated only for Pod details and carries the per-kind
// navigable references the Links tab needs (owner, node, SA, ...) so the
// ui package can render drillable rows without re-parsing the Pod object.
type ResourceDetail struct {
	Name         string
	Namespace    string
	Kind         string
	UID          string
	CreatedAt    string
	Labels       map[string]string
	Annotations  map[string]string
	Fields       []DetailField
	Containers   []ContainerInfo
	YAML         string
	PodLinks     *PodLinksData
	ServiceLinks *ServiceLinksData
}

// RefTarget identifies another Kubernetes resource that the Links tab can
// drill into. Type is the km8 ResourceType (Pod, Secret, ConfigMap, Node, ...);
// Namespace is empty for cluster-scoped kinds.
type RefTarget struct {
	Type      ResourceType
	Name      string
	Namespace string
}

// PodLinksData is the structured "Links" content for a Pod.
//
// Owner is the immediate K8s owner reference (ReplicaSet / DaemonSet /
// StatefulSet / Job). Drilling further (RS → Deployment) is left to follow-up
// commits — for MVP we surface the one-hop owner only.
//
// Volumes lists the Pod's spec.volumes with their source kind and an optional
// drill ref (ConfigMap / Secret / PVC sources are drillable; emptyDir /
// hostPath / projected / downwardAPI are informational).
//
// Images carries the rendered image strings (e.g. "nginx:1.27.1") for
// informational display — there's no K8s resource to drill into for an image.
type PodLinksData struct {
	Owner          *RefTarget
	Node           *RefTarget // cluster-scoped
	ServiceAccount *RefTarget
	Volumes        []VolumeRef
	Images         []string
	InitImages     []string
}

// VolumeRef describes a Pod volume's source. Ref is non-nil when the source
// is another K8s resource the user can drill into (ConfigMap, Secret, PVC).
type VolumeRef struct {
	Name string // volume name in spec.volumes
	Kind string // "configMap" / "secret" / "persistentVolumeClaim" / "emptyDir" / "hostPath" / "projected" / "downwardAPI" / "other"
	Ref  *RefTarget
}

// ServiceLinksData is the navigable-refs payload for a Service detail.
// Pods is the workload selected by the Service's label selector — each one
// is a drillable RefTarget so the user can answer "which pods does this
// Service route to?" in one keystroke.
//
// Populated by EnrichLinks at fetch time (it issues a CoreV1().Pods().List
// against the selector); the synchronous detailService can't do this
// because it has no clientset.
type ServiceLinksData struct {
	Pods []RefTarget
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
