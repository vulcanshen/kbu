package k8s

import (
	"os"
	"testing"
)

func skipUnlessK8s(t *testing.T) {
	t.Helper()
	if os.Getenv("KBU_TEST_K8S") == "" {
		t.Skip("set KBU_TEST_K8S=1 to run k8s integration tests")
	}
}

func TestNewClient_DefaultContext(t *testing.T) {
	skipUnlessK8s(t)

	c, err := NewClient("")
	if err != nil {
		t.Fatalf("NewClient with empty context: %v", err)
	}
	if c.ContextName() == "" {
		t.Error("expected non-empty context name when using default context")
	}
}

func TestNewClient_InvalidContext(t *testing.T) {
	skipUnlessK8s(t)

	_, err := NewClient("this-context-does-not-exist-99999")
	if err == nil {
		t.Error("expected error for non-existent context, got nil")
	}
}

func TestListContexts(t *testing.T) {
	skipUnlessK8s(t)

	c, err := NewClient("")
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}

	contexts := c.ListContexts()
	if len(contexts) == 0 {
		t.Error("expected at least one context from kubeconfig")
	}

	// The current context should be in the list.
	found := false
	for _, ctx := range contexts {
		if ctx == c.ContextName() {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("current context %q not found in ListContexts result %v", c.ContextName(), contexts)
	}
}

func TestGetClusterInfo(t *testing.T) {
	skipUnlessK8s(t)

	c, err := NewClient("")
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}

	info := c.GetClusterInfo()

	if info.ContextName == "" {
		t.Error("ClusterInfo.ContextName is empty")
	}
	if info.ClusterName == "" {
		t.Error("ClusterInfo.ClusterName is empty")
	}
	if info.ServerURL == "" {
		t.Error("ClusterInfo.ServerURL is empty")
	}
	if info.Namespace == "" {
		t.Error("ClusterInfo.Namespace is empty")
	}
}

func TestSetGetNamespace(t *testing.T) {
	skipUnlessK8s(t)

	c, err := NewClient("")
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}

	// Default should be all namespaces (empty string).
	if ns := c.GetNamespace(); ns != "" {
		t.Errorf("expected empty namespace (all namespaces), got %q", ns)
	}

	// Set a specific namespace.
	c.SetNamespace("kube-system")
	if ns := c.GetNamespace(); ns != "kube-system" {
		t.Errorf("expected namespace %q, got %q", "kube-system", ns)
	}

	// Reset to all namespaces.
	c.SetNamespace("")
	if ns := c.GetNamespace(); ns != "" {
		t.Errorf("expected empty namespace after reset, got %q", ns)
	}
}
