package k8s

import (
	"context"
	"fmt"
	"sort"
	"sync"

	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
)

// Column defines a table column with a title and minimum width.
type Column struct {
	Title    string
	MinWidth int
}

// ResourceFetcher fetches resources of a given type.
type ResourceFetcher func(ctx context.Context, clientset kubernetes.Interface, namespace string) ([]ResourceItem, error)

// ResourceDetailer extracts detail from a ResourceItem.
type ResourceDetailer func(item ResourceItem) ResourceDetail

// WatchStarterFunc starts a watch for a resource type.
type WatchStarterFunc func(ctx context.Context, clientset kubernetes.Interface, namespace string) (watch.Interface, error)

// ChildFetcher fetches child resources for drill-down.
type ChildFetcher func(ctx context.Context, clientset kubernetes.Interface, item ResourceItem) ([]ResourceItem, error)

// DrillDownConfig defines drill-down behavior for a resource type.
type DrillDownConfig struct {
	ChildType     ResourceType
	FetchChildren ChildFetcher
}

// ResourceDefinition contains all behavior for a resource type.
type ResourceDefinition struct {
	Type            ResourceType
	DisplayName     string
	KubectlName     string
	Category        string
	CategoryOrder   int // lower = higher in sidebar
	OrderInCategory int // position within category
	ClusterScoped   bool
	Columns         []Column
	Fetcher         ResourceFetcher
	Detailer        ResourceDetailer
	WatchStarter    WatchStarterFunc
	DrillDown       *DrillDownConfig
	HasLogs         bool
	Dynamic         bool // true for CRD resources
}

// Registry holds all registered resource definitions.
type Registry struct {
	mu   sync.RWMutex
	defs map[ResourceType]*ResourceDefinition
}

// NewRegistry creates an empty registry.
func NewRegistry() *Registry {
	return &Registry{
		defs: make(map[ResourceType]*ResourceDefinition),
	}
}

// Register adds a resource definition to the registry.
func (r *Registry) Register(def *ResourceDefinition) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.defs[def.Type] = def
}

// Unregister removes a resource definition from the registry.
func (r *Registry) Unregister(rt ResourceType) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.defs, rt)
}

// Get returns the definition for a resource type, or nil if not found.
func (r *Registry) Get(rt ResourceType) *ResourceDefinition {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.defs[rt]
}

// CategoryGroup represents a sidebar category with its resources.
type CategoryGroup struct {
	Label     string
	Order     int
	Resources []ResourceDefinition
}

// SidebarCategories returns resource definitions grouped by category, sorted.
func (r *Registry) SidebarCategories() []CategoryGroup {
	r.mu.RLock()
	defer r.mu.RUnlock()

	catMap := make(map[string]*CategoryGroup)
	for _, def := range r.defs {
		cg, ok := catMap[def.Category]
		if !ok {
			cg = &CategoryGroup{Label: def.Category, Order: def.CategoryOrder}
			catMap[def.Category] = cg
		}
		cg.Resources = append(cg.Resources, *def)
	}

	// Sort categories by order, then resources within each category by order.
	cats := make([]CategoryGroup, 0, len(catMap))
	for _, cg := range catMap {
		sort.Slice(cg.Resources, func(i, j int) bool {
			return cg.Resources[i].OrderInCategory < cg.Resources[j].OrderInCategory
		})
		cats = append(cats, *cg)
	}
	sort.Slice(cats, func(i, j int) bool {
		return cats[i].Order < cats[j].Order
	})

	return cats
}

// AllTypes returns all registered resource types in sidebar display order.
func (r *Registry) AllTypes() []ResourceType {
	cats := r.SidebarCategories()
	var types []ResourceType
	for _, cat := range cats {
		for _, def := range cat.Resources {
			types = append(types, def.Type)
		}
	}
	return types
}

// FetchResources fetches resources using the registered fetcher.
func (r *Registry) FetchResources(ctx context.Context, clientset kubernetes.Interface, rt ResourceType, namespace string) ([]ResourceItem, error) {
	def := r.Get(rt)
	if def == nil {
		return nil, fmt.Errorf("unsupported resource type: %s", rt)
	}
	return def.Fetcher(ctx, clientset, namespace)
}

// GetResourceDetail extracts detail using the registered detailer.
func (r *Registry) GetResourceDetail(rt ResourceType, item ResourceItem) ResourceDetail {
	def := r.Get(rt)
	if def == nil {
		return ResourceDetail{Name: item.Name, Namespace: item.Namespace, Kind: string(rt), UID: item.UID}
	}
	return def.Detailer(item)
}

// StartWatch starts a watch using the registered WatchStarter.
func (r *Registry) StartWatch(ctx context.Context, clientset kubernetes.Interface, rt ResourceType, namespace string) (watch.Interface, error) {
	def := r.Get(rt)
	if def == nil {
		return nil, fmt.Errorf("unsupported resource type: %s", rt)
	}
	return def.WatchStarter(ctx, clientset, namespace)
}

// ColumnsFor returns the column definitions for a resource type.
func (r *Registry) ColumnsFor(rt ResourceType) []Column {
	def := r.Get(rt)
	if def == nil {
		return []Column{{Title: "Name", MinWidth: 20}}
	}
	return def.Columns
}

// ClearDynamic removes all dynamically registered (CRD) resource definitions.
func (r *Registry) ClearDynamic() {
	r.mu.Lock()
	defer r.mu.Unlock()
	for rt, def := range r.defs {
		if def.Dynamic {
			delete(r.defs, rt)
		}
	}
}

// DefaultRegistry is the global registry initialized with built-in resources.
var DefaultRegistry = NewRegistry()
