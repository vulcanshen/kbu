package k8s

import (
	"context"
	"fmt"

	appsv1 "k8s.io/api/apps/v1"
	autoscalingv2 "k8s.io/api/autoscaling/v2"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	discoveryv1 "k8s.io/api/discovery/v1"
	networkingv1 "k8s.io/api/networking/v1"
	policyv1 "k8s.io/api/policy/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	storagev1 "k8s.io/api/storage/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// buildWorkloadStaticRelatives builds the API-free portion of a workload's Relatives:
// owner ref (when present) and a non-default ServiceAccount. Dynamic content
// (pods matched by selector) is appended by EnrichRelatives.
func buildWorkloadStaticRelatives(owners []metav1.OwnerReference, podSA, namespace string) []RelativeSection {
	var entries []RelativeRow
	for _, ref := range owners {
		if rt, ok := kindToResourceType(ref.Kind); ok {
			entries = append(entries, RelativeRow{
				Label: "Owner",
				Value: fmt.Sprintf("%s/%s", ref.Kind, ref.Name),
				Ref:   &RefTarget{Type: rt, Name: ref.Name, Namespace: namespace},
			})
			break
		}
	}
	if podSA != "" && podSA != "default" {
		entries = append(entries, RelativeRow{
			Label: "ServiceAccount",
			Value: podSA,
			Ref:   &RefTarget{Type: ResourceServiceAccounts, Name: podSA, Namespace: namespace},
		})
	}
	if len(entries) == 0 {
		return nil
	}
	return []RelativeSection{{Entries: entries}}
}

// buildIngressRelatives: IngressClass + backend Services (deduped) + TLS Secrets.
// Spec-only — no API call needed.
func buildIngressRelatives(ing *networkingv1.Ingress) []RelativeSection {
	var sections []RelativeSection

	var classEntries []RelativeRow
	if ing.Spec.IngressClassName != nil && *ing.Spec.IngressClassName != "" {
		name := *ing.Spec.IngressClassName
		classEntries = append(classEntries, RelativeRow{
			Label: "IngressClass",
			Value: name,
			Ref:   &RefTarget{Type: ResourceIngressClasses, Name: name},
		})
	}
	if len(classEntries) > 0 {
		sections = append(sections, RelativeSection{Entries: classEntries})
	}

	seenSvc := make(map[string]bool)
	var svcEntries []RelativeRow
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
			svcEntries = append(svcEntries, RelativeRow{
				Label: "  " + name,
				Value: route,
				Ref:   &RefTarget{Type: ResourceServices, Name: name, Namespace: ing.Namespace},
			})
		}
	}
	if len(svcEntries) > 0 {
		sections = append(sections, RelativeSection{
			Title:   fmt.Sprintf("Backend Services (%d)", len(svcEntries)),
			Entries: svcEntries,
		})
	}

	seenSec := make(map[string]bool)
	var tlsEntries []RelativeRow
	for _, tls := range ing.Spec.TLS {
		if tls.SecretName == "" || seenSec[tls.SecretName] {
			continue
		}
		seenSec[tls.SecretName] = true
		desc := fmt.Sprintf("%d host(s)", len(tls.Hosts))
		tlsEntries = append(tlsEntries, RelativeRow{
			Label: "  " + tls.SecretName,
			Value: desc,
			Ref:   &RefTarget{Type: ResourceSecrets, Name: tls.SecretName, Namespace: ing.Namespace},
		})
	}
	if len(tlsEntries) > 0 {
		sections = append(sections, RelativeSection{
			Title:   fmt.Sprintf("TLS Secrets (%d)", len(tlsEntries)),
			Entries: tlsEntries,
		})
	}

	return sections
}

// buildHPARelatives: scaleTargetRef → Deployment / StatefulSet / DaemonSet.
// Unsupported kinds (ReplicaSet etc.) render no link.
func buildHPARelatives(hpa *autoscalingv2.HorizontalPodAutoscaler) []RelativeSection {
	ref := hpa.Spec.ScaleTargetRef
	rt, ok := kindToResourceType(ref.Kind)
	if !ok {
		return nil
	}
	return []RelativeSection{{
		Entries: []RelativeRow{{
			Label: "ScaleTarget",
			Value: fmt.Sprintf("%s/%s", ref.Kind, ref.Name),
			Ref:   &RefTarget{Type: rt, Name: ref.Name, Namespace: hpa.Namespace},
		}},
	}}
}

// buildPVCRelatives: bound PV + StorageClass. Both refs are cluster-scoped.
func buildPVCRelatives(pvc *corev1.PersistentVolumeClaim) []RelativeSection {
	var entries []RelativeRow
	if pvc.Spec.VolumeName != "" {
		entries = append(entries, RelativeRow{
			Label: "Volume",
			Value: pvc.Spec.VolumeName,
			Ref:   &RefTarget{Type: ResourcePersistentVolumes, Name: pvc.Spec.VolumeName},
		})
	}
	if pvc.Spec.StorageClassName != nil && *pvc.Spec.StorageClassName != "" {
		name := *pvc.Spec.StorageClassName
		entries = append(entries, RelativeRow{
			Label: "StorageClass",
			Value: name,
			Ref:   &RefTarget{Type: ResourceStorageClasses, Name: name},
		})
	}
	if len(entries) == 0 {
		return nil
	}
	return []RelativeSection{{Entries: entries}}
}

// buildCronJobRelatives: active Jobs spawned by the CronJob (read off
// status.active so no extra API call is needed at detail time).
func buildCronJobRelatives(cj *batchv1.CronJob) []RelativeSection {
	if len(cj.Status.Active) == 0 {
		return nil
	}
	entries := make([]RelativeRow, 0, len(cj.Status.Active))
	for _, ref := range cj.Status.Active {
		entries = append(entries, RelativeRow{
			Label: "  " + ref.Name,
			Value: "Job",
			Ref:   &RefTarget{Type: ResourceJobs, Name: ref.Name, Namespace: ref.Namespace},
		})
	}
	return []RelativeSection{{
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
	entries := make([]RelativeRow, 0, len(list.Items))
	for i := range list.Items {
		p := &list.Items[i]
		entries = append(entries, RelativeRow{
			Label: "  " + p.Name,
			Value: string(p.Status.Phase),
			Ref:   &RefTarget{Type: ResourcePods, Name: p.Name, Namespace: p.Namespace},
		})
	}
	detail.Relatives = append(detail.Relatives, RelativeSection{
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
	var entries []RelativeRow
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
		entries = append(entries, RelativeRow{
			Label: "  " + p.Name,
			Value: string(p.Status.Phase),
			Ref:   &RefTarget{Type: ResourcePods, Name: p.Name, Namespace: p.Namespace},
		})
	}
	if len(entries) == 0 {
		return
	}
	detail.Relatives = append(detail.Relatives, RelativeSection{
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
	var entries []RelativeRow
	for i := range list.Items {
		p := &list.Items[i]
		if !podUsesConfigMap(p, cm.Name) {
			continue
		}
		entries = append(entries, RelativeRow{
			Label: "  " + p.Name,
			Value: string(p.Status.Phase),
			Ref:   &RefTarget{Type: ResourcePods, Name: p.Name, Namespace: p.Namespace},
		})
	}
	if len(entries) == 0 {
		return
	}
	detail.Relatives = append(detail.Relatives, RelativeSection{
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
	var entries []RelativeRow
	for i := range list.Items {
		p := &list.Items[i]
		if !podUsesSecret(p, s.Name) {
			continue
		}
		entries = append(entries, RelativeRow{
			Label: "  " + p.Name,
			Value: string(p.Status.Phase),
			Ref:   &RefTarget{Type: ResourcePods, Name: p.Name, Namespace: p.Namespace},
		})
	}
	if len(entries) == 0 {
		return
	}
	detail.Relatives = append(detail.Relatives, RelativeSection{
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

// ---------------------------------------------------------------------------
// Static builders (no API call)
// ---------------------------------------------------------------------------

// buildEventRelatives: drill to the involved object when km8 recognizes the kind.
// Empty when the object's kind isn't in kindToResourceType (e.g. CRDs).
func buildEventRelatives(e *corev1.Event) []RelativeSection {
	if e.InvolvedObject.Name == "" {
		return nil
	}
	rt, ok := kindToResourceType(e.InvolvedObject.Kind)
	if !ok {
		return nil
	}
	return []RelativeSection{{
		Entries: []RelativeRow{{
			Label: "Object",
			Value: fmt.Sprintf("%s/%s", e.InvolvedObject.Kind, e.InvolvedObject.Name),
			Ref:   &RefTarget{Type: rt, Name: e.InvolvedObject.Name, Namespace: e.InvolvedObject.Namespace},
		}},
	}}
}

// buildPVRelatives: ClaimRef (PVC) + StorageClass. ClaimRef carries its own
// namespace; StorageClass is cluster-scoped.
func buildPVRelatives(pv *corev1.PersistentVolume) []RelativeSection {
	var entries []RelativeRow
	if pv.Spec.ClaimRef != nil && pv.Spec.ClaimRef.Name != "" {
		entries = append(entries, RelativeRow{
			Label: "Claim",
			Value: fmt.Sprintf("%s/%s", pv.Spec.ClaimRef.Namespace, pv.Spec.ClaimRef.Name),
			Ref: &RefTarget{
				Type:      ResourcePersistentVolumeClaims,
				Name:      pv.Spec.ClaimRef.Name,
				Namespace: pv.Spec.ClaimRef.Namespace,
			},
		})
	}
	if pv.Spec.StorageClassName != "" {
		entries = append(entries, RelativeRow{
			Label: "StorageClass",
			Value: pv.Spec.StorageClassName,
			Ref:   &RefTarget{Type: ResourceStorageClasses, Name: pv.Spec.StorageClassName},
		})
	}
	if len(entries) == 0 {
		return nil
	}
	return []RelativeSection{{Entries: entries}}
}

// buildEndpointSliceRelatives: owning Service (via the well-known label) plus
// the per-endpoint Pod targets (deduped).
func buildEndpointSliceRelatives(es *discoveryv1.EndpointSlice) []RelativeSection {
	var sections []RelativeSection
	if svc := es.Labels["kubernetes.io/service-name"]; svc != "" {
		sections = append(sections, RelativeSection{
			Entries: []RelativeRow{{
				Label: "Service",
				Value: svc,
				Ref:   &RefTarget{Type: ResourceServices, Name: svc, Namespace: es.Namespace},
			}},
		})
	}
	seen := make(map[string]bool)
	var podEntries []RelativeRow
	for _, ep := range es.Endpoints {
		if ep.TargetRef == nil || ep.TargetRef.Kind != "Pod" || ep.TargetRef.Name == "" {
			continue
		}
		key := ep.TargetRef.Namespace + "/" + ep.TargetRef.Name
		if seen[key] {
			continue
		}
		seen[key] = true
		podEntries = append(podEntries, RelativeRow{
			Label: "  " + ep.TargetRef.Name,
			Value: "pod",
			Ref:   &RefTarget{Type: ResourcePods, Name: ep.TargetRef.Name, Namespace: ep.TargetRef.Namespace},
		})
	}
	if len(podEntries) > 0 {
		sections = append(sections, RelativeSection{
			Title:   fmt.Sprintf("Endpoints (%d)", len(podEntries)),
			Entries: podEntries,
		})
	}
	return sections
}

// buildClusterRoleBindingRelatives / buildRoleBindingRelatives: RoleRef + Subjects.
// Subjects with Kind=ServiceAccount become drillable; Users/Groups remain
// info-only since they're not km8 resources.
func buildClusterRoleBindingRelatives(crb *rbacv1.ClusterRoleBinding) []RelativeSection {
	sections := roleRefSection(crb.RoleRef, "")
	sections = appendSubjects(sections, crb.Subjects)
	return sections
}

func buildRoleBindingRelatives(rb *rbacv1.RoleBinding) []RelativeSection {
	// Role RoleRefs resolve to the RoleBinding's own namespace; ClusterRole
	// RoleRefs are cluster-scoped.
	sections := roleRefSection(rb.RoleRef, rb.Namespace)
	sections = appendSubjects(sections, rb.Subjects)
	return sections
}

func roleRefSection(ref rbacv1.RoleRef, bindingNS string) []RelativeSection {
	if ref.Name == "" {
		return nil
	}
	var rt ResourceType
	ns := ""
	switch ref.Kind {
	case "ClusterRole":
		rt = ResourceClusterRoles
	case "Role":
		rt = ResourceRoles
		ns = bindingNS
	default:
		return nil
	}
	return []RelativeSection{{
		Entries: []RelativeRow{{
			Label: "RoleRef",
			Value: fmt.Sprintf("%s/%s", ref.Kind, ref.Name),
			Ref:   &RefTarget{Type: rt, Name: ref.Name, Namespace: ns},
		}},
	}}
}

func appendSubjects(sections []RelativeSection, subjects []rbacv1.Subject) []RelativeSection {
	if len(subjects) == 0 {
		return sections
	}
	entries := make([]RelativeRow, 0, len(subjects))
	for _, s := range subjects {
		entries = append(entries, subjectToRelativeRow(s))
	}
	return append(sections, RelativeSection{
		Title:   fmt.Sprintf("Subjects (%d)", len(entries)),
		Entries: entries,
	})
}

func subjectToRelativeRow(s rbacv1.Subject) RelativeRow {
	value := s.Kind
	if s.Namespace != "" {
		value = fmt.Sprintf("%s @ %s", s.Kind, s.Namespace)
	}
	var ref *RefTarget
	if s.Kind == "ServiceAccount" && s.Namespace != "" && s.Name != "" {
		ref = &RefTarget{Type: ResourceServiceAccounts, Name: s.Name, Namespace: s.Namespace}
	}
	return RelativeRow{Label: "  " + s.Name, Value: value, Ref: ref}
}

// buildServiceAccountStaticRelatives: the secrets explicitly attached to the SA
// + imagePullSecrets. Pods using this SA come from enrichServiceAccountConsumers.
func buildServiceAccountStaticRelatives(sa *corev1.ServiceAccount) []RelativeSection {
	var sections []RelativeSection
	if len(sa.Secrets) > 0 {
		entries := make([]RelativeRow, 0, len(sa.Secrets))
		for _, ref := range sa.Secrets {
			entries = append(entries, RelativeRow{
				Label: "  " + ref.Name,
				Value: "Secret",
				Ref:   &RefTarget{Type: ResourceSecrets, Name: ref.Name, Namespace: sa.Namespace},
			})
		}
		sections = append(sections, RelativeSection{
			Title:   fmt.Sprintf("Secrets (%d)", len(entries)),
			Entries: entries,
		})
	}
	if len(sa.ImagePullSecrets) > 0 {
		entries := make([]RelativeRow, 0, len(sa.ImagePullSecrets))
		for _, ref := range sa.ImagePullSecrets {
			entries = append(entries, RelativeRow{
				Label: "  " + ref.Name,
				Value: "Secret",
				Ref:   &RefTarget{Type: ResourceSecrets, Name: ref.Name, Namespace: sa.Namespace},
			})
		}
		sections = append(sections, RelativeSection{
			Title:   fmt.Sprintf("ImagePullSecrets (%d)", len(entries)),
			Entries: entries,
		})
	}
	return sections
}

// ---------------------------------------------------------------------------
// Enrichers (one API call)
// ---------------------------------------------------------------------------

// enrichNodePods lists pods on this node. The real API server honors the
// spec.nodeName fieldSelector and filters server-side; we re-check on the
// client too so fake/testing clientsets (which ignore field selectors) and
// hypothetical buggy proxies still produce the correct list.
func enrichNodePods(ctx context.Context, cs kubernetes.Interface, item ResourceItem, detail *ResourceDetail) {
	node, ok := item.Raw.(*corev1.Node)
	if !ok {
		return
	}
	list, err := cs.CoreV1().Pods("").List(ctx, metav1.ListOptions{
		FieldSelector: "spec.nodeName=" + node.Name,
	})
	if err != nil {
		return
	}
	var entries []RelativeRow
	for i := range list.Items {
		p := &list.Items[i]
		if p.Spec.NodeName != node.Name {
			continue
		}
		entries = append(entries, RelativeRow{
			Label: "  " + p.Namespace + "/" + p.Name,
			Value: string(p.Status.Phase),
			Ref:   &RefTarget{Type: ResourcePods, Name: p.Name, Namespace: p.Namespace},
		})
	}
	if len(entries) == 0 {
		return
	}
	detail.Relatives = append(detail.Relatives, RelativeSection{
		Title:   fmt.Sprintf("Pods (%d)", len(entries)),
		Entries: entries,
	})
}

// enrichServiceAccountConsumers: pods in the SA's namespace whose
// spec.serviceAccountName matches. SA defaulting (empty name → "default")
// is handled here so the default SA doesn't pick up every pod that omitted
// the field.
func enrichServiceAccountConsumers(ctx context.Context, cs kubernetes.Interface, item ResourceItem, detail *ResourceDetail) {
	sa, ok := item.Raw.(*corev1.ServiceAccount)
	if !ok {
		return
	}
	list, err := cs.CoreV1().Pods(sa.Namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return
	}
	var entries []RelativeRow
	for i := range list.Items {
		p := &list.Items[i]
		podSA := p.Spec.ServiceAccountName
		if podSA == "" {
			podSA = "default"
		}
		if podSA != sa.Name {
			continue
		}
		entries = append(entries, RelativeRow{
			Label: "  " + p.Name,
			Value: string(p.Status.Phase),
			Ref:   &RefTarget{Type: ResourcePods, Name: p.Name, Namespace: p.Namespace},
		})
	}
	if len(entries) == 0 {
		return
	}
	detail.Relatives = append(detail.Relatives, RelativeSection{
		Title:   fmt.Sprintf("Used by Pods (%d)", len(entries)),
		Entries: entries,
	})
}

// enrichServiceAccountBindings: RoleBindings (in the SA's namespace) and
// ClusterRoleBindings (cluster-wide) that name this SA as a subject —
// i.e., what permissions the SA actually has. Two API calls; the CRB
// pass is cluster-wide (tier 🔴 — slow on large clusters) but RBAC
// queries this way are how you'd debug "why can / can't this SA do X".
func enrichServiceAccountBindings(ctx context.Context, cs kubernetes.Interface, item ResourceItem, detail *ResourceDetail) {
	sa, ok := item.Raw.(*corev1.ServiceAccount)
	if !ok {
		return
	}
	if rbList, err := cs.RbacV1().RoleBindings(sa.Namespace).List(ctx, metav1.ListOptions{}); err == nil {
		var entries []RelativeRow
		for i := range rbList.Items {
			rb := &rbList.Items[i]
			if !bindingHasSASubject(rb.Subjects, sa.Name, sa.Namespace) {
				continue
			}
			entries = append(entries, RelativeRow{
				Label: "  " + rb.Name,
				Value: "RoleBinding",
				Ref:   &RefTarget{Type: ResourceRoleBindings, Name: rb.Name, Namespace: rb.Namespace},
			})
		}
		if len(entries) > 0 {
			detail.Relatives = append(detail.Relatives, RelativeSection{
				Title:   fmt.Sprintf("RoleBindings (%d)", len(entries)),
				Entries: entries,
			})
		}
	}
	if crbList, err := cs.RbacV1().ClusterRoleBindings().List(ctx, metav1.ListOptions{}); err == nil {
		var entries []RelativeRow
		for i := range crbList.Items {
			crb := &crbList.Items[i]
			if !bindingHasSASubject(crb.Subjects, sa.Name, sa.Namespace) {
				continue
			}
			entries = append(entries, RelativeRow{
				Label: "  " + crb.Name,
				Value: "ClusterRoleBinding",
				Ref:   &RefTarget{Type: ResourceClusterRoleBindings, Name: crb.Name},
			})
		}
		if len(entries) > 0 {
			detail.Relatives = append(detail.Relatives, RelativeSection{
				Title:   fmt.Sprintf("ClusterRoleBindings (%d)", len(entries)),
				Entries: entries,
			})
		}
	}
}

func bindingHasSASubject(subjects []rbacv1.Subject, saName, saNamespace string) bool {
	for _, s := range subjects {
		if s.Kind == "ServiceAccount" && s.Name == saName && s.Namespace == saNamespace {
			return true
		}
	}
	return false
}

// enrichServiceAccountTokenSecrets: Secrets in the SA's namespace whose
// `kubernetes.io/service-account.name` annotation points back at this SA.
// Catches the legacy SA-token Secrets that k8s used to auto-create and
// any manually-bound token-type Secret — sa.Secrets only carries the
// SA's own explicit reference list, which is usually empty on modern
// (>=1.24) clusters.
func enrichServiceAccountTokenSecrets(ctx context.Context, cs kubernetes.Interface, item ResourceItem, detail *ResourceDetail) {
	sa, ok := item.Raw.(*corev1.ServiceAccount)
	if !ok {
		return
	}
	list, err := cs.CoreV1().Secrets(sa.Namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return
	}
	var entries []RelativeRow
	for i := range list.Items {
		s := &list.Items[i]
		if s.Annotations[saTokenAnnotationName] != sa.Name {
			continue
		}
		entries = append(entries, RelativeRow{
			Label: "  " + s.Name,
			Value: string(s.Type),
			Ref:   &RefTarget{Type: ResourceSecrets, Name: s.Name, Namespace: s.Namespace},
		})
	}
	if len(entries) == 0 {
		return
	}
	detail.Relatives = append(detail.Relatives, RelativeSection{
		Title:   fmt.Sprintf("Token Secrets (%d)", len(entries)),
		Entries: entries,
	})
}

// enrichSecretServiceAccount: reverse direction of the SA→Secret link.
// If the Secret carries the `kubernetes.io/service-account.name`
// annotation, surface the named SA as a drillable section. Pure
// annotation read, no API call — included in the enricher dispatch so
// the section appears alongside Used by Pods.
func enrichSecretServiceAccount(ctx context.Context, cs kubernetes.Interface, item ResourceItem, detail *ResourceDetail) {
	s, ok := item.Raw.(*corev1.Secret)
	if !ok {
		return
	}
	saName := s.Annotations[saTokenAnnotationName]
	if saName == "" {
		return
	}
	detail.Relatives = append(detail.Relatives, RelativeSection{
		Title: "ServiceAccount",
		Entries: []RelativeRow{{
			Label: "  " + saName,
			Value: "ServiceAccount",
			Ref:   &RefTarget{Type: ResourceServiceAccounts, Name: saName, Namespace: s.Namespace},
		}},
	})
}

// saTokenAnnotationName is the well-known annotation key kubelet /
// kube-controller-manager set on SA-token Secrets back-referencing the
// owning ServiceAccount. The companion `kubernetes.io/service-account.uid`
// annotation also exists but UID isn't useful for navigation — name +
// namespace fully identify the SA.
const saTokenAnnotationName = "kubernetes.io/service-account.name"

// enrichPDBPods: pods matched by the PDB's selector — the ones it protects.
func enrichPDBPods(ctx context.Context, cs kubernetes.Interface, item ResourceItem, detail *ResourceDetail) {
	pdb, ok := item.Raw.(*policyv1.PodDisruptionBudget)
	if !ok || pdb.Spec.Selector == nil {
		return
	}
	appendSelectorPodSection(ctx, cs, pdb.Namespace, formatSelector(pdb.Spec.Selector), "Selected Pods", detail)
}

// enrichNetworkPolicyPods: pods matched by spec.podSelector — the targets
// the policy applies to. Empty selector matches all pods in namespace.
func enrichNetworkPolicyPods(ctx context.Context, cs kubernetes.Interface, item ResourceItem, detail *ResourceDetail) {
	np, ok := item.Raw.(*networkingv1.NetworkPolicy)
	if !ok {
		return
	}
	appendSelectorPodSection(ctx, cs, np.Namespace, formatSelector(&np.Spec.PodSelector), "Selected Pods", detail)
}

// enrichRoleBindings: RoleBindings in the Role's namespace that reference it.
func enrichRoleBindings(ctx context.Context, cs kubernetes.Interface, item ResourceItem, detail *ResourceDetail) {
	role, ok := item.Raw.(*rbacv1.Role)
	if !ok {
		return
	}
	list, err := cs.RbacV1().RoleBindings(role.Namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return
	}
	var entries []RelativeRow
	for i := range list.Items {
		rb := &list.Items[i]
		if rb.RoleRef.Kind != "Role" || rb.RoleRef.Name != role.Name {
			continue
		}
		entries = append(entries, RelativeRow{
			Label: "  " + rb.Name,
			Value: "RoleBinding",
			Ref:   &RefTarget{Type: ResourceRoleBindings, Name: rb.Name, Namespace: rb.Namespace},
		})
	}
	if len(entries) == 0 {
		return
	}
	detail.Relatives = append(detail.Relatives, RelativeSection{
		Title:   fmt.Sprintf("Bound by (%d)", len(entries)),
		Entries: entries,
	})
}

// enrichPodOwner resolves a Pod's owner past the ReplicaSet
// implementation-detail layer. The Pod's OwnerReference points to a
// ReplicaSet (K8s auto-creates one per Deployment revision), but
// buildPodRelatives already mapped Type to ResourceDeployments — leaving
// Name pointing at the RS, which doesn't exist as a Deployment. Result:
// drill into Owner errors with "deployment not found".
//
// Fix: look up the RS, find its owning Deployment, replace the Name.
// No-op when the Pod's owner is a direct-workload kind (DaemonSet,
// StatefulSet, Job — those already carry the right name) or when the
// RS lookup fails (RBAC, deleted mid-rollout, ...).
func enrichPodOwner(ctx context.Context, cs kubernetes.Interface, item ResourceItem, detail *ResourceDetail) {
	p, ok := item.Raw.(*corev1.Pod)
	if !ok || detail.PodRelatives == nil || detail.PodRelatives.Owner == nil {
		return
	}
	if len(p.OwnerReferences) == 0 {
		return
	}
	if p.OwnerReferences[0].Kind != "ReplicaSet" {
		return
	}
	rsName := p.OwnerReferences[0].Name
	rs, err := cs.AppsV1().ReplicaSets(p.Namespace).Get(ctx, rsName, metav1.GetOptions{})
	if err != nil {
		return
	}
	for _, ref := range rs.OwnerReferences {
		if ref.Kind == "Deployment" {
			detail.PodRelatives.Owner = &RefTarget{
				Type:      ResourceDeployments,
				Name:      ref.Name,
				Namespace: p.Namespace,
			}
			return
		}
	}
}

// enrichClusterRoleBindings: list ClusterRoleBindings + cluster-wide
// RoleBindings whose RoleRef points back at this ClusterRole. Two API
// calls; runs cluster-wide because RoleBindings can live in any namespace.
func enrichClusterRoleBindings(ctx context.Context, cs kubernetes.Interface, item ResourceItem, detail *ResourceDetail) {
	cr, ok := item.Raw.(*rbacv1.ClusterRole)
	if !ok {
		return
	}

	crbList, err := cs.RbacV1().ClusterRoleBindings().List(ctx, metav1.ListOptions{})
	if err == nil {
		var entries []RelativeRow
		for i := range crbList.Items {
			b := &crbList.Items[i]
			if b.RoleRef.Kind != "ClusterRole" || b.RoleRef.Name != cr.Name {
				continue
			}
			entries = append(entries, RelativeRow{
				Label: "  " + b.Name,
				Value: "ClusterRoleBinding",
				Ref:   &RefTarget{Type: ResourceClusterRoleBindings, Name: b.Name},
			})
		}
		if len(entries) > 0 {
			detail.Relatives = append(detail.Relatives, RelativeSection{
				Title:   fmt.Sprintf("ClusterRoleBindings (%d)", len(entries)),
				Entries: entries,
			})
		}
	}

	rbList, err := cs.RbacV1().RoleBindings("").List(ctx, metav1.ListOptions{})
	if err == nil {
		var entries []RelativeRow
		for i := range rbList.Items {
			b := &rbList.Items[i]
			if b.RoleRef.Kind != "ClusterRole" || b.RoleRef.Name != cr.Name {
				continue
			}
			entries = append(entries, RelativeRow{
				Label: "  " + b.Namespace + "/" + b.Name,
				Value: "RoleBinding",
				Ref:   &RefTarget{Type: ResourceRoleBindings, Name: b.Name, Namespace: b.Namespace},
			})
		}
		if len(entries) > 0 {
			detail.Relatives = append(detail.Relatives, RelativeSection{
				Title:   fmt.Sprintf("RoleBindings (%d)", len(entries)),
				Entries: entries,
			})
		}
	}
}

// enrichStorageClassPVCs: cluster-wide PVC list filtered to those whose
// spec.storageClassName matches. Doesn't try to resolve "default class"
// semantics — only explicit references count, so a PVC that relies on the
// default-class annotation won't show up here unless it set the name.
func enrichStorageClassPVCs(ctx context.Context, cs kubernetes.Interface, item ResourceItem, detail *ResourceDetail) {
	sc, ok := item.Raw.(*storagev1.StorageClass)
	if !ok {
		return
	}
	list, err := cs.CoreV1().PersistentVolumeClaims("").List(ctx, metav1.ListOptions{})
	if err != nil {
		return
	}
	var entries []RelativeRow
	for i := range list.Items {
		pvc := &list.Items[i]
		name := ""
		if pvc.Spec.StorageClassName != nil {
			name = *pvc.Spec.StorageClassName
		}
		if name != sc.Name {
			continue
		}
		entries = append(entries, RelativeRow{
			Label: "  " + pvc.Namespace + "/" + pvc.Name,
			Value: string(pvc.Status.Phase),
			Ref: &RefTarget{
				Type:      ResourcePersistentVolumeClaims,
				Name:      pvc.Name,
				Namespace: pvc.Namespace,
			},
		})
	}
	if len(entries) == 0 {
		return
	}
	detail.Relatives = append(detail.Relatives, RelativeSection{
		Title:   fmt.Sprintf("PVCs (%d)", len(entries)),
		Entries: entries,
	})
}

// enrichIngressClassIngresses: cluster-wide Ingress list filtered by
// spec.ingressClassName == this class. Only the modern field is checked;
// the deprecated `kubernetes.io/ingress.class` annotation is intentionally
// ignored — clusters still on it can update the annotation to the field
// or skip this Relatives entry.
func enrichIngressClassIngresses(ctx context.Context, cs kubernetes.Interface, item ResourceItem, detail *ResourceDetail) {
	ic, ok := item.Raw.(*networkingv1.IngressClass)
	if !ok {
		return
	}
	list, err := cs.NetworkingV1().Ingresses("").List(ctx, metav1.ListOptions{})
	if err != nil {
		return
	}
	var entries []RelativeRow
	for i := range list.Items {
		ing := &list.Items[i]
		cls := ""
		if ing.Spec.IngressClassName != nil {
			cls = *ing.Spec.IngressClassName
		}
		if cls != ic.Name {
			continue
		}
		entries = append(entries, RelativeRow{
			Label: "  " + ing.Namespace + "/" + ing.Name,
			Value: "Ingress",
			Ref: &RefTarget{
				Type:      ResourceIngresses,
				Name:      ing.Name,
				Namespace: ing.Namespace,
			},
		})
	}
	if len(entries) == 0 {
		return
	}
	detail.Relatives = append(detail.Relatives, RelativeSection{
		Title:   fmt.Sprintf("Ingresses (%d)", len(entries)),
		Entries: entries,
	})
}

// appendSelectorPodSection lists pods matching `selector` in `ns` and adds
// a section titled `title` to detail.Relatives. No-op when the list is empty.
func appendSelectorPodSection(ctx context.Context, cs kubernetes.Interface, ns, selector, title string, detail *ResourceDetail) {
	list, err := cs.CoreV1().Pods(ns).List(ctx, metav1.ListOptions{LabelSelector: selector})
	if err != nil || len(list.Items) == 0 {
		return
	}
	entries := make([]RelativeRow, 0, len(list.Items))
	for i := range list.Items {
		p := &list.Items[i]
		entries = append(entries, RelativeRow{
			Label: "  " + p.Name,
			Value: string(p.Status.Phase),
			Ref:   &RefTarget{Type: ResourcePods, Name: p.Name, Namespace: p.Namespace},
		})
	}
	detail.Relatives = append(detail.Relatives, RelativeSection{
		Title:   fmt.Sprintf("%s (%d)", title, len(entries)),
		Entries: entries,
	})
}
