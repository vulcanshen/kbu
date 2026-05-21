package k8s

import "testing"

func TestResourceType_KubectlName(t *testing.T) {
	tests := []struct {
		rt   ResourceType
		want string
	}{
		{ResourceNamespaces, "namespace"},
		{ResourceNodes, "node"},
		{ResourcePods, "pod"},
		{ResourceDeployments, "deployment"},
		{ResourceDaemonSets, "daemonset"},
		{ResourceStatefulSets, "statefulset"},
		{ResourceJobs, "job"},
		{ResourceCronJobs, "cronjob"},
		{ResourceServices, "service"},
		{ResourceIngresses, "ingress"},
		{ResourceConfigMaps, "configmap"},
		{ResourceSecrets, "secret"},
		{ResourceEvents, "event"},
		{ResourceClusterRoles, "clusterrole"},
		{ResourceClusterRoleBindings, "clusterrolebinding"},
		{ResourceRoles, "role"},
		{ResourceRoleBindings, "rolebinding"},
		{ResourceServiceAccounts, "serviceaccount"},
		{ResourcePersistentVolumes, "persistentvolume"},
		{ResourcePersistentVolumeClaims, "persistentvolumeclaim"},
		{ResourceStorageClasses, "storageclass"},
		{ResourceHorizontalPodAutoscalers, "horizontalpodautoscaler"},
		{ResourcePodDisruptionBudgets, "poddisruptionbudget"},
	}

	for _, tt := range tests {
		t.Run(tt.rt.String(), func(t *testing.T) {
			got := tt.rt.KubectlName()
			if got != tt.want {
				t.Errorf("KubectlName() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestResourceType_KubectlName_Unknown(t *testing.T) {
	rt := ResourceType("nonexistent")
	if got := rt.KubectlName(); got != "nonexistent" {
		t.Errorf("KubectlName() for unknown type = %q, want 'nonexistent'", got)
	}
}
