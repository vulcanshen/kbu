package k8s

import (
	"time"

	appsv1 "k8s.io/api/apps/v1"
	autoscalingv2 "k8s.io/api/autoscaling/v2"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
)

// ExtractConditions returns the resource's .status.conditions as a flat
// []ConditionItem ready for display. Returns nil for kinds without
// conditions or when item.Raw doesn't match a supported type.
//
// Supported kinds: Pod, Node, PersistentVolumeClaim, Deployment, StatefulSet,
// DaemonSet, ReplicaSet, Job, HorizontalPodAutoscaler, Ingress.
//
// All other kinds (ConfigMap, Secret, Service, ServiceAccount, Role, ...)
// return nil and the Conditions tab will not appear for them.
func ExtractConditions(item ResourceItem) []ConditionItem {
	switch raw := item.Raw.(type) {
	case *corev1.Pod:
		out := make([]ConditionItem, 0, len(raw.Status.Conditions))
		for _, c := range raw.Status.Conditions {
			out = append(out, ConditionItem{
				Type:    string(c.Type),
				Status:  string(c.Status),
				Reason:  c.Reason,
				Message: c.Message,
				Age:     ageSince(c.LastTransitionTime.Time),
			})
		}
		return out
	case *corev1.Node:
		out := make([]ConditionItem, 0, len(raw.Status.Conditions))
		for _, c := range raw.Status.Conditions {
			out = append(out, ConditionItem{
				Type:    string(c.Type),
				Status:  string(c.Status),
				Reason:  c.Reason,
				Message: c.Message,
				Age:     ageSince(c.LastTransitionTime.Time),
			})
		}
		return out
	case *corev1.PersistentVolumeClaim:
		out := make([]ConditionItem, 0, len(raw.Status.Conditions))
		for _, c := range raw.Status.Conditions {
			out = append(out, ConditionItem{
				Type:    string(c.Type),
				Status:  string(c.Status),
				Reason:  c.Reason,
				Message: c.Message,
				Age:     ageSince(c.LastTransitionTime.Time),
			})
		}
		return out
	case *appsv1.Deployment:
		out := make([]ConditionItem, 0, len(raw.Status.Conditions))
		for _, c := range raw.Status.Conditions {
			out = append(out, ConditionItem{
				Type:    string(c.Type),
				Status:  string(c.Status),
				Reason:  c.Reason,
				Message: c.Message,
				Age:     ageSince(c.LastTransitionTime.Time),
			})
		}
		return out
	case *appsv1.StatefulSet:
		out := make([]ConditionItem, 0, len(raw.Status.Conditions))
		for _, c := range raw.Status.Conditions {
			out = append(out, ConditionItem{
				Type:    string(c.Type),
				Status:  string(c.Status),
				Reason:  c.Reason,
				Message: c.Message,
				Age:     ageSince(c.LastTransitionTime.Time),
			})
		}
		return out
	case *appsv1.DaemonSet:
		out := make([]ConditionItem, 0, len(raw.Status.Conditions))
		for _, c := range raw.Status.Conditions {
			out = append(out, ConditionItem{
				Type:    string(c.Type),
				Status:  string(c.Status),
				Reason:  c.Reason,
				Message: c.Message,
				Age:     ageSince(c.LastTransitionTime.Time),
			})
		}
		return out
	case *appsv1.ReplicaSet:
		out := make([]ConditionItem, 0, len(raw.Status.Conditions))
		for _, c := range raw.Status.Conditions {
			out = append(out, ConditionItem{
				Type:    string(c.Type),
				Status:  string(c.Status),
				Reason:  c.Reason,
				Message: c.Message,
				Age:     ageSince(c.LastTransitionTime.Time),
			})
		}
		return out
	case *batchv1.Job:
		out := make([]ConditionItem, 0, len(raw.Status.Conditions))
		for _, c := range raw.Status.Conditions {
			out = append(out, ConditionItem{
				Type:    string(c.Type),
				Status:  string(c.Status),
				Reason:  c.Reason,
				Message: c.Message,
				Age:     ageSince(c.LastTransitionTime.Time),
			})
		}
		return out
	case *autoscalingv2.HorizontalPodAutoscaler:
		out := make([]ConditionItem, 0, len(raw.Status.Conditions))
		for _, c := range raw.Status.Conditions {
			out = append(out, ConditionItem{
				Type:    string(c.Type),
				Status:  string(c.Status),
				Reason:  c.Reason,
				Message: c.Message,
				Age:     ageSince(c.LastTransitionTime.Time),
			})
		}
		return out
	case *networkingv1.Ingress:
		// Ingress conditions are nested under LoadBalancer status entries.
		// Most controllers don't populate them — return empty rather than
		// guess at a structure.
		return nil
	}
	return nil
}

// ageSince formats a duration since t as the same short string used elsewhere
// (s/m/h/d). Returns empty for zero-time inputs so the Age column reads "—"
// in display when a condition has never transitioned.
func ageSince(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return formatAge(t)
}
