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
	rt := ResourceType(999)
	if got := rt.KubectlName(); got != "unknown" {
		t.Errorf("KubectlName() for unknown type = %q, want 'unknown'", got)
	}
}
