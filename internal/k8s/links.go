package k8s

import (
	"context"
	"fmt"

	appsv1 "k8s.io/api/apps/v1"
	autoscalingv2 "k8s.io/api/autoscaling/v2"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// buildWorkloadStaticLinks builds the API-free portion of a workload's Links:
// owner ref (when present) and a non-default ServiceAccount. Dynamic content
// (pods matched by selector) is appended by EnrichLinks.
func buildWorkloadStaticLinks(owners []metav1.OwnerReference, podSA, namespace string) []LinkSection {
	var entries []LinkRow
	for _, ref := range owners {
		if rt, ok := kindToResourceType(ref.Kind); ok {
			entries = append(entries, LinkRow{
				Label: "Owner",
				Value: fmt.Sprintf("%s/%s", ref.Kind, ref.Name),
				Ref:   &RefTarget{Type: rt, Name: ref.Name, Namespace: namespace},
			})
			break
		}
	}
	if podSA != "" && podSA != "default" {
		entries = append(entries, LinkRow{
			Label: "ServiceAccount",
			Value: podSA,
			Ref:   &RefTarget{Type: ResourceServiceAccounts, Name: podSA, Namespace: namespace},
		})
	}
	if len(entries) == 0 {
		return nil
	}
	return []LinkSection{{Entries: entries}}
}

// buildIngressLinks: IngressClass + backend Services (deduped) + TLS Secrets.
// Spec-only — no API call needed.
func buildIngressLinks(ing *networkingv1.Ingress) []LinkSection {
	var sections []LinkSection

	var classEntries []LinkRow
	if ing.Spec.IngressClassName != nil && *ing.Spec.IngressClassName != "" {
		name := *ing.Spec.IngressClassName
		classEntries = append(classEntries, LinkRow{
			Label: "IngressClass",
			Value: name,
			Ref:   &RefTarget{Type: ResourceIngressClasses, Name: name},
		})
	}
	if len(classEntries) > 0 {
		sections = append(sections, LinkSection{Entries: classEntries})
	}

	seenSvc := make(map[string]bool)
	var svcEntries []LinkRow
	for _, rule := range ing.Spec.Rules {
		if rule.HTTP == nil {
			continue
		}
		host := rule.Host
		if host == "" {
			host = "*"
		}
		for _, path := range rule.HTTP.Paths {
			if path.Backend.Service == nil {
				continue
			}
			name := path.Backend.Service.Name
			key := name
			if seenSvc[key] {
				continue
			}
			seenSvc[key] = true
			route := host + path.Path
			if path.Path == "" {
				route = host + "/"
			}
			svcEntries = append(svcEntries, LinkRow{
				Label: "  " + name,
				Value: route,
				Ref:   &RefTarget{Type: ResourceServices, Name: name, Namespace: ing.Namespace},
			})
		}
	}
	if len(svcEntries) > 0 {
		sections = append(sections, LinkSection{
			Title:   fmt.Sprintf("Backend Services (%d)", len(svcEntries)),
			Entries: svcEntries,
		})
	}

	seenSec := make(map[string]bool)
	var tlsEntries []LinkRow
	for _, tls := range ing.Spec.TLS {
		if tls.SecretName == "" || seenSec[tls.SecretName] {
			continue
		}
		seenSec[tls.SecretName] = true
		desc := fmt.Sprintf("%d host(s)", len(tls.Hosts))
		tlsEntries = append(tlsEntries, LinkRow{
			Label: "  " + tls.SecretName,
			Value: desc,
			Ref:   &RefTarget{Type: ResourceSecrets, Name: tls.SecretName, Namespace: ing.Namespace},
		})
	}
	if len(tlsEntries) > 0 {
		sections = append(sections, LinkSection{
			Title:   fmt.Sprintf("TLS Secrets (%d)", len(tlsEntries)),
			Entries: tlsEntries,
		})
	}

	return sections
}

// buildHPALinks: scaleTargetRef → Deployment / StatefulSet / DaemonSet.
// Unsupported kinds (ReplicaSet etc.) render no link.
func buildHPALinks(hpa *autoscalingv2.HorizontalPodAutoscaler) []LinkSection {
	ref := hpa.Spec.ScaleTargetRef
	rt, ok := kindToResourceType(ref.Kind)
	if !ok {
		return nil
	}
	return []LinkSection{{
		Entries: []LinkRow{{
			Label: "ScaleTarget",
			Value: fmt.Sprintf("%s/%s", ref.Kind, ref.Name),
			Ref:   &RefTarget{Type: rt, Name: ref.Name, Namespace: hpa.Namespace},
		}},
	}}
}

// buildPVCLinks: bound PV + StorageClass. Both refs are cluster-scoped.
func buildPVCLinks(pvc *corev1.PersistentVolumeClaim) []LinkSection {
	var entries []LinkRow
	if pvc.Spec.VolumeName != "" {
		entries = append(entries, LinkRow{
			Label: "Volume",
			Value: pvc.Spec.VolumeName,
			Ref:   &RefTarget{Type: ResourcePersistentVolumes, Name: pvc.Spec.VolumeName},
		})
	}
	if pvc.Spec.StorageClassName != nil && *pvc.Spec.StorageClassName != "" {
		name := *pvc.Spec.StorageClassName
		entries = append(entries, LinkRow{
			Label: "StorageClass",
			Value: name,
			Ref:   &RefTarget{Type: ResourceStorageClasses, Name: name},
		})
	}
	if len(entries) == 0 {
		return nil
	}
	return []LinkSection{{Entries: entries}}
}

// buildCronJobLinks: active Jobs spawned by the CronJob (read off
// status.active so no extra API call is needed at detail time).
func buildCronJobLinks(cj *batchv1.CronJob) []LinkSection {
	if len(cj.Status.Active) == 0 {
		return nil
	}
	entries := make([]LinkRow, 0, len(cj.Status.Active))
	for _, ref := range cj.Status.Active {
		entries = append(entries, LinkRow{
			Label: "  " + ref.Name,
			Value: "Job",
			Ref:   &RefTarget{Type: ResourceJobs, Name: ref.Name, Namespace: ref.Namespace},
		})
	}
	return []LinkSection{{
		Title:   fmt.Sprintf("Active Jobs (%d)", len(entries)),
		Entries: entries,
	}}
}

// enrichWorkloadPods appends the "Pods (n)" section by listing pods that
// match the workload's selector. Uses PodsForWorkload's current-RS logic for
// Deployments; raw selector for STS / DS / Job. Quiet on error.
func enrichWorkloadPods(ctx context.Context, cs kubernetes.Interface, item ResourceItem, detail *ResourceDetail) {
	selector, ns := workloadSelectorAndNS(item)
	if selector == "" {
		return
	}

	// Deployments: prefer current-RS selector when available, to mirror the
	// aggregate-logs view of "live generation only".
	if dep, ok := item.Raw.(*appsv1.Deployment); ok {
		if rs, ok := currentRSSelector(ctx, cs, dep); ok {
			selector = rs
		}
	}

	list, err := cs.CoreV1().Pods(ns).List(ctx, metav1.ListOptions{LabelSelector: selector})
	if err != nil || len(list.Items) == 0 {
		return
	}
	entries := make([]LinkRow, 0, len(list.Items))
	for i := range list.Items {
		p := &list.Items[i]
		entries = append(entries, LinkRow{
			Label: "  " + p.Name,
			Value: string(p.Status.Phase),
			Ref:   &RefTarget{Type: ResourcePods, Name: p.Name, Namespace: p.Namespace},
		})
	}
	detail.Links = append(detail.Links, LinkSection{
		Title:   fmt.Sprintf("Pods (%d)", len(entries)),
		Entries: entries,
	})
}

// workloadSelectorAndNS returns the label-selector string and namespace for
// a workload item, or ("", "") when the kind isn't a recognized workload.
func workloadSelectorAndNS(item ResourceItem) (string, string) {
	switch raw := item.Raw.(type) {
	case *appsv1.Deployment:
		return formatSelector(raw.Spec.Selector), raw.Namespace
	case *appsv1.StatefulSet:
		return formatSelector(raw.Spec.Selector), raw.Namespace
	case *appsv1.DaemonSet:
		return formatSelector(raw.Spec.Selector), raw.Namespace
	case *batchv1.Job:
		return formatSelector(raw.Spec.Selector), raw.Namespace
	}
	return "", ""
}

func formatSelector(ls *metav1.LabelSelector) string {
	if ls == nil {
		return ""
	}
	sel, err := metav1.LabelSelectorAsSelector(ls)
	if err != nil {
		return ""
	}
	return sel.String()
}

// enrichPVCConsumers appends "Mounted by (n)" with pods that mount this PVC
// via spec.volumes[].persistentVolumeClaim.claimName. Namespace-scoped.
func enrichPVCConsumers(ctx context.Context, cs kubernetes.Interface, item ResourceItem, detail *ResourceDetail) {
	pvc, ok := item.Raw.(*corev1.PersistentVolumeClaim)
	if !ok {
		return
	}
	list, err := cs.CoreV1().Pods(pvc.Namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return
	}
	var entries []LinkRow
	for i := range list.Items {
		p := &list.Items[i]
		uses := false
		for _, v := range p.Spec.Volumes {
			if v.PersistentVolumeClaim != nil && v.PersistentVolumeClaim.ClaimName == pvc.Name {
				uses = true
				break
			}
		}
		if !uses {
			continue
		}
		entries = append(entries, LinkRow{
			Label: "  " + p.Name,
			Value: string(p.Status.Phase),
			Ref:   &RefTarget{Type: ResourcePods, Name: p.Name, Namespace: p.Namespace},
		})
	}
	if len(entries) == 0 {
		return
	}
	detail.Links = append(detail.Links, LinkSection{
		Title:   fmt.Sprintf("Mounted by (%d)", len(entries)),
		Entries: entries,
	})
}

// enrichConfigMapConsumers / enrichSecretConsumers scan all pods in the
// resource's namespace and surface ones that reference it via volumes,
// envFrom, or env.valueFrom. Pod listing is namespace-scoped — expensive
// for large namespaces but unavoidable for true reverse refs.
func enrichConfigMapConsumers(ctx context.Context, cs kubernetes.Interface, item ResourceItem, detail *ResourceDetail) {
	cm, ok := item.Raw.(*corev1.ConfigMap)
	if !ok {
		return
	}
	list, err := cs.CoreV1().Pods(cm.Namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return
	}
	var entries []LinkRow
	for i := range list.Items {
		p := &list.Items[i]
		if !podUsesConfigMap(p, cm.Name) {
			continue
		}
		entries = append(entries, LinkRow{
			Label: "  " + p.Name,
			Value: string(p.Status.Phase),
			Ref:   &RefTarget{Type: ResourcePods, Name: p.Name, Namespace: p.Namespace},
		})
	}
	if len(entries) == 0 {
		return
	}
	detail.Links = append(detail.Links, LinkSection{
		Title:   fmt.Sprintf("Used by Pods (%d)", len(entries)),
		Entries: entries,
	})
}

func enrichSecretConsumers(ctx context.Context, cs kubernetes.Interface, item ResourceItem, detail *ResourceDetail) {
	s, ok := item.Raw.(*corev1.Secret)
	if !ok {
		return
	}
	list, err := cs.CoreV1().Pods(s.Namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return
	}
	var entries []LinkRow
	for i := range list.Items {
		p := &list.Items[i]
		if !podUsesSecret(p, s.Name) {
			continue
		}
		entries = append(entries, LinkRow{
			Label: "  " + p.Name,
			Value: string(p.Status.Phase),
			Ref:   &RefTarget{Type: ResourcePods, Name: p.Name, Namespace: p.Namespace},
		})
	}
	if len(entries) == 0 {
		return
	}
	detail.Links = append(detail.Links, LinkSection{
		Title:   fmt.Sprintf("Used by Pods (%d)", len(entries)),
		Entries: entries,
	})
}

func podUsesConfigMap(p *corev1.Pod, name string) bool {
	for _, v := range p.Spec.Volumes {
		if v.ConfigMap != nil && v.ConfigMap.Name == name {
			return true
		}
		if v.Projected != nil {
			for _, src := range v.Projected.Sources {
				if src.ConfigMap != nil && src.ConfigMap.Name == name {
					return true
				}
			}
		}
	}
	if containerUsesConfigMap(p.Spec.InitContainers, name) {
		return true
	}
	return containerUsesConfigMap(p.Spec.Containers, name)
}

func containerUsesConfigMap(cs []corev1.Container, name string) bool {
	for _, c := range cs {
		for _, ef := range c.EnvFrom {
			if ef.ConfigMapRef != nil && ef.ConfigMapRef.Name == name {
				return true
			}
		}
		for _, e := range c.Env {
			if e.ValueFrom != nil && e.ValueFrom.ConfigMapKeyRef != nil && e.ValueFrom.ConfigMapKeyRef.Name == name {
				return true
			}
		}
	}
	return false
}

func podUsesSecret(p *corev1.Pod, name string) bool {
	for _, ips := range p.Spec.ImagePullSecrets {
		if ips.Name == name {
			return true
		}
	}
	for _, v := range p.Spec.Volumes {
		if v.Secret != nil && v.Secret.SecretName == name {
			return true
		}
		if v.Projected != nil {
			for _, src := range v.Projected.Sources {
				if src.Secret != nil && src.Secret.Name == name {
					return true
				}
			}
		}
	}
	if containerUsesSecret(p.Spec.InitContainers, name) {
		return true
	}
	return containerUsesSecret(p.Spec.Containers, name)
}

func containerUsesSecret(cs []corev1.Container, name string) bool {
	for _, c := range cs {
		for _, ef := range c.EnvFrom {
			if ef.SecretRef != nil && ef.SecretRef.Name == name {
				return true
			}
		}
		for _, e := range c.Env {
			if e.ValueFrom != nil && e.ValueFrom.SecretKeyRef != nil && e.ValueFrom.SecretKeyRef.Name == name {
				return true
			}
		}
	}
	return false
}
