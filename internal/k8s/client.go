package k8s

import (
	"fmt"
	"sync"

	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/clientcmd/api"
)

// Client wraps a Kubernetes clientset and tracks the current context and
// namespace filter. It is the primary entry point for all cluster operations
// in km8.
type Client struct {
	clientset     kubernetes.Interface
	dynamicClient dynamic.Interface
	restConfig    *rest.Config
	kubeConfig    api.Config
	contextName   string
	registry      *Registry

	mu        sync.RWMutex
	namespace string // "" means all namespaces
}

// NewClient creates a Client for the given context name. If contextName is
// empty, the current-context from the kubeconfig is used. The kubeconfig
// location is resolved via $KUBECONFIG, falling back to ~/.kube/config.
func NewClient(contextName string) (*Client, error) {
	loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()

	rawConfig, err := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
		loadingRules, &clientcmd.ConfigOverrides{},
	).RawConfig()
	if err != nil {
		return nil, fmt.Errorf("loading kubeconfig: %w", err)
	}

	if contextName == "" {
		contextName = rawConfig.CurrentContext
	}
	if contextName == "" {
		return nil, fmt.Errorf("no context specified and no current-context set in kubeconfig")
	}

	// Validate that the context exists.
	if _, ok := rawConfig.Contexts[contextName]; !ok {
		return nil, fmt.Errorf("context %q not found in kubeconfig", contextName)
	}

	overrides := &clientcmd.ConfigOverrides{
		CurrentContext: contextName,
	}
	clientConfig := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
		loadingRules, overrides,
	)

	restConfig, err := clientConfig.ClientConfig()
	if err != nil {
		return nil, fmt.Errorf("building rest config for context %q: %w", contextName, err)
	}

	restConfig.QPS = 50
	restConfig.Burst = 100

	clientset, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		return nil, fmt.Errorf("creating kubernetes clientset: %w", err)
	}

	dynClient, err := dynamic.NewForConfig(restConfig)
	if err != nil {
		return nil, fmt.Errorf("creating dynamic client: %w", err)
	}

	return &Client{
		clientset:     clientset,
		dynamicClient: dynClient,
		restConfig:    restConfig,
		kubeConfig:    rawConfig,
		contextName:   contextName,
		registry:      DefaultRegistry,
		namespace:     "",
	}, nil
}

// GetClusterInfo returns metadata about the currently connected cluster.
func (c *Client) GetClusterInfo() ClusterInfo {
	ctx, ok := c.kubeConfig.Contexts[c.contextName]
	if !ok {
		return ClusterInfo{ContextName: c.contextName}
	}

	var serverURL string
	if cluster, exists := c.kubeConfig.Clusters[ctx.Cluster]; exists {
		serverURL = cluster.Server
	}

	ns := ctx.Namespace
	if ns == "" {
		ns = "default"
	}

	return ClusterInfo{
		ContextName: c.contextName,
		ClusterName: ctx.Cluster,
		ServerURL:   serverURL,
		Namespace:   ns,
	}
}

// ListContexts returns all context names available in the kubeconfig.
func (c *Client) ListContexts() []string {
	contexts := make([]string, 0, len(c.kubeConfig.Contexts))
	for name := range c.kubeConfig.Contexts {
		contexts = append(contexts, name)
	}
	return contexts
}

// SetNamespace sets the namespace filter. An empty string means all namespaces.
func (c *Client) SetNamespace(ns string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.namespace = ns
}

// GetNamespace returns the current namespace filter. An empty string means
// all namespaces.
func (c *Client) GetNamespace() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.namespace
}

// Clientset returns the underlying Kubernetes clientset.
func (c *Client) Clientset() kubernetes.Interface {
	return c.clientset
}

// DynamicClient returns the dynamic Kubernetes client for CRD access.
func (c *Client) DynamicClient() dynamic.Interface {
	return c.dynamicClient
}

// RestConfig returns the REST config used to connect to the cluster.
func (c *Client) RestConfig() *rest.Config {
	return c.restConfig
}

// Registry returns the resource registry associated with this client.
func (c *Client) Registry() *Registry {
	return c.registry
}

// ContextName returns the name of the active kubeconfig context.
func (c *Client) ContextName() string {
	return c.contextName
}
