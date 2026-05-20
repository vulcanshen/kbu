package k8s

import (
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/scheme"
	sigsyaml "sigs.k8s.io/yaml"
)

// MarshalItemYAML returns a YAML representation of a resource item for the
// detail panel. ManagedFields are stripped for readability (matching kubectl's
// default behavior since 1.21). Returns an empty string when the item's Raw
// payload cannot be marshaled — callers should fall back to structured detail.
func MarshalItemYAML(item ResourceItem) string {
	if item.Raw == nil {
		return ""
	}
	obj, ok := item.Raw.(runtime.Object)
	if !ok {
		return ""
	}
	copied := obj.DeepCopyObject()
	// Restore apiVersion/kind on typed objects. client-go strips TypeMeta when
	// caching from list/watch because it is redundant at runtime; for YAML we
	// want it back. Unstructured objects (CRDs) keep their own GVK.
	if _, isUnstructured := copied.(*unstructured.Unstructured); !isUnstructured {
		if gvks, _, err := scheme.Scheme.ObjectKinds(copied); err == nil && len(gvks) > 0 {
			copied.GetObjectKind().SetGroupVersionKind(gvks[0])
		}
	}
	if accessor, err := meta.Accessor(copied); err == nil {
		accessor.SetManagedFields(nil)
	}
	out, err := sigsyaml.Marshal(copied)
	if err != nil {
		return ""
	}
	return string(out)
}

// MarshalContainerYAML extracts a single container's spec and status from the
// parent Pod (wrapped in item.Raw) and returns them as a YAML document.
// Returns empty string when item is not a Pod or the container is not found.
func MarshalContainerYAML(item ResourceItem, containerName string) string {
	if item.Raw == nil || containerName == "" {
		return ""
	}
	pod, ok := item.Raw.(*corev1.Pod)
	if !ok {
		return ""
	}
	type view struct {
		Spec   *corev1.Container       `json:"spec,omitempty"`
		Status *corev1.ContainerStatus `json:"status,omitempty"`
	}
	var v view
	for i := range pod.Spec.Containers {
		if pod.Spec.Containers[i].Name == containerName {
			v.Spec = &pod.Spec.Containers[i]
			break
		}
	}
	if v.Spec == nil {
		for i := range pod.Spec.InitContainers {
			if pod.Spec.InitContainers[i].Name == containerName {
				v.Spec = &pod.Spec.InitContainers[i]
				break
			}
		}
	}
	for i := range pod.Status.ContainerStatuses {
		if pod.Status.ContainerStatuses[i].Name == containerName {
			v.Status = &pod.Status.ContainerStatuses[i]
			break
		}
	}
	if v.Status == nil {
		for i := range pod.Status.InitContainerStatuses {
			if pod.Status.InitContainerStatuses[i].Name == containerName {
				v.Status = &pod.Status.InitContainerStatuses[i]
				break
			}
		}
	}
	if v.Spec == nil && v.Status == nil {
		return ""
	}
	out, err := sigsyaml.Marshal(v)
	if err != nil {
		return ""
	}
	return string(out)
}
