package k8s

import (
	"context"
	"fmt"
	"sort"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
)

// DiscoverCRDs queries the API server for CustomResourceDefinitions and
// registers each one into the client's registry under the "Custom Resources"
// category. Only CRDs with a served+storage version are registered.
func DiscoverCRDs(ctx context.Context, client *Client) (int, error) {
	crdGVR := schema.GroupVersionResource{
		Group:    "apiextensions.k8s.io",
		Version:  "v1",
		Resource: "customresourcedefinitions",
	}

	list, err := client.DynamicClient().Resource(crdGVR).List(ctx, metav1.ListOptions{})
	if err != nil {
		return 0, fmt.Errorf("listing CRDs: %w", err)
	}

	reg := client.Registry()
	count := 0
	for i := range list.Items {
		crd := &list.Items[i]
		def := crdToDefinition(crd, client.DynamicClient(), count)
		if def != nil {
			reg.Register(def)
			count++
		}
	}

	return count, nil
}

func crdToDefinition(crd *unstructured.Unstructured, dynClient dynamic.Interface, index int) *ResourceDefinition {
	spec, ok := crd.Object["spec"].(map[string]interface{})
	if !ok {
		return nil
	}

	names, ok := spec["names"].(map[string]interface{})
	if !ok {
		return nil
	}

	plural, _ := names["plural"].(string)
	kind, _ := names["kind"].(string)
	group, _ := spec["group"].(string)
	scope, _ := spec["scope"].(string)

	if plural == "" || kind == "" || group == "" {
		return nil
	}

	version, printerColumns := findServedVersion(spec)
	if version == "" {
		return nil
	}

	gvr := schema.GroupVersionResource{
		Group:    group,
		Version:  version,
		Resource: plural,
	}

	clusterScoped := scope == "Cluster"
	columns := buildCRDColumns(printerColumns)
	rtKey := ResourceType(plural + "." + group)

	return &ResourceDefinition{
		Type:            rtKey,
		DisplayName:     kind,
		KubectlName:     plural + "." + group,
		Category:        "Custom Resources",
		CategoryOrder:   100,
		OrderInCategory: index,
		ClusterScoped:   clusterScoped,
		Dynamic:         true,
		HasLogs:         false,
		Columns:         columns,
		Fetcher:         dynamicFetcher(dynClient, gvr, columns, clusterScoped),
		Detailer:        dynamicDetailer(kind),
		WatchStarter:    dynamicWatchStarter(dynClient, gvr, clusterScoped),
	}
}

func findServedVersion(spec map[string]interface{}) (string, []interface{}) {
	versions, ok := spec["versions"].([]interface{})
	if !ok {
		return "", nil
	}

	for _, v := range versions {
		ver, ok := v.(map[string]interface{})
		if !ok {
			continue
		}
		served, _ := ver["served"].(bool)
		if !served {
			continue
		}
		name, _ := ver["name"].(string)
		if name == "" {
			continue
		}
		cols, _ := ver["additionalPrinterColumns"].([]interface{})
		return name, cols
	}

	return "", nil
}

func buildCRDColumns(printerColumns []interface{}) []Column {
	cols := []Column{
		{Title: "Name", MinWidth: 20},
	}

	for _, pc := range printerColumns {
		col, ok := pc.(map[string]interface{})
		if !ok {
			continue
		}
		name, _ := col["name"].(string)
		if name == "" || name == "Name" {
			continue
		}
		// Skip Age column — we add our own at the end
		if name == "Age" {
			continue
		}
		minWidth := 12
		if len(name) > minWidth {
			minWidth = len(name) + 2
		}
		cols = append(cols, Column{Title: name, MinWidth: minWidth})
	}

	cols = append(cols, Column{Title: "Age", MinWidth: 8})
	return cols
}

func dynamicFetcher(dynClient dynamic.Interface, gvr schema.GroupVersionResource, columns []Column, clusterScoped bool) ResourceFetcher {
	return func(ctx context.Context, _ kubernetes.Interface, namespace string) ([]ResourceItem, error) {
		var list *unstructured.UnstructuredList
		var err error

		if clusterScoped {
			list, err = dynClient.Resource(gvr).List(ctx, metav1.ListOptions{})
		} else {
			list, err = dynClient.Resource(gvr).Namespace(namespace).List(ctx, metav1.ListOptions{})
		}
		if err != nil {
			return nil, fmt.Errorf("listing %s: %w", gvr.Resource, err)
		}

		items := make([]ResourceItem, 0, len(list.Items))
		for i := range list.Items {
			obj := &list.Items[i]
			items = append(items, unstructuredToItem(obj, columns))
		}
		return items, nil
	}
}

func unstructuredToItem(obj *unstructured.Unstructured, columns []Column) ResourceItem {
	row := make([]string, len(columns))
	row[0] = obj.GetName()

	metadata := obj.Object
	for i := 1; i < len(columns); i++ {
		title := columns[i].Title
		if title == "Age" {
			ct := obj.GetCreationTimestamp()
			if !ct.IsZero() {
				row[i] = formatAge(ct.Time)
			} else {
				row[i] = "<unknown>"
			}
			continue
		}
		row[i] = extractPrinterColumnValue(metadata, title)
	}

	return ResourceItem{
		Name:      obj.GetName(),
		Namespace: obj.GetNamespace(),
		UID:       string(obj.GetUID()),
		Raw:       obj,
		Row:       row,
	}
}

func extractPrinterColumnValue(obj map[string]interface{}, columnName string) string {
	normalizedName := strings.ToLower(strings.ReplaceAll(columnName, " ", ""))

	status, _ := obj["status"].(map[string]interface{})
	spec, _ := obj["spec"].(map[string]interface{})

	for _, section := range []map[string]interface{}{status, spec, obj} {
		if section == nil {
			continue
		}
		for k, v := range section {
			if strings.ToLower(k) == normalizedName {
				return fmt.Sprintf("%v", v)
			}
		}
	}

	return ""
}

func dynamicDetailer(kind string) ResourceDetailer {
	return func(item ResourceItem) ResourceDetail {
		obj, ok := item.Raw.(*unstructured.Unstructured)
		if !ok {
			return ResourceDetail{Name: item.Name, Namespace: item.Namespace, Kind: kind, UID: item.UID}
		}

		d := ResourceDetail{
			Name:        item.Name,
			Namespace:   item.Namespace,
			Kind:        kind,
			UID:         item.UID,
			Labels:      obj.GetLabels(),
			Annotations: obj.GetAnnotations(),
		}

		ct := obj.GetCreationTimestamp()
		if !ct.IsZero() {
			d.CreatedAt = formatAge(ct.Time)
		}

		spec, _ := obj.Object["spec"].(map[string]interface{})
		status, _ := obj.Object["status"].(map[string]interface{})

		if spec != nil {
			keys := sortedKeys(spec)
			for _, k := range keys {
				v := spec[k]
				if isSimpleValue(v) {
					d.Fields = append(d.Fields, DetailField{Label: k, Value: fmt.Sprintf("%v", v)})
				}
			}
		}

		if status != nil {
			keys := sortedKeys(status)
			for _, k := range keys {
				v := status[k]
				if isSimpleValue(v) {
					d.Fields = append(d.Fields, DetailField{Label: "status." + k, Value: fmt.Sprintf("%v", v)})
				}
			}
		}

		return d
	}
}

func dynamicWatchStarter(dynClient dynamic.Interface, gvr schema.GroupVersionResource, clusterScoped bool) WatchStarterFunc {
	return func(ctx context.Context, _ kubernetes.Interface, namespace string) (watch.Interface, error) {
		if clusterScoped {
			return dynClient.Resource(gvr).Watch(ctx, metav1.ListOptions{})
		}
		return dynClient.Resource(gvr).Namespace(namespace).Watch(ctx, metav1.ListOptions{})
	}
}

func sortedKeys(m map[string]interface{}) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func isSimpleValue(v interface{}) bool {
	switch v.(type) {
	case string, float64, int64, bool:
		return true
	}
	return false
}
