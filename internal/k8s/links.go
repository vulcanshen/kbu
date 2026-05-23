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

// ---------------------------------------------------------------------------
// Static builders (no API call)
// ---------------------------------------------------------------------------

// buildEventLinks: drill to the involved object when km8 recognizes the kind.
// Empty when the object's kind isn't in kindToResourceType (e.g. CRDs).
func buildEventLinks(e *corev1.Event) []LinkSection {
	if e.InvolvedObject.Name == "" {
		return nil
	}
	rt, ok := kindToResourceType(e.InvolvedObject.Kind)
	if !ok {
		return nil
	}
	return []LinkSection{{
		Entries: []LinkRow{{
			Label: "Object",
			Value: fmt.Sprintf("%s/%s", e.InvolvedObject.Kind, e.InvolvedObject.Name),
			Ref:   &RefTarget{Type: rt, Name: e.InvolvedObject.Name, Namespace: e.InvolvedObject.Namespace},
		}},
	}}
}

// buildPVLinks: ClaimRef (PVC) + StorageClass. ClaimRef carries its own
// namespace; StorageClass is cluster-scoped.
func buildPVLinks(pv *corev1.PersistentVolume) []LinkSection {
	var entries []LinkRow
	if pv.Spec.ClaimRef != nil && pv.Spec.ClaimRef.Name != "" {
		entries = append(entries, LinkRow{
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
		entries = append(entries, LinkRow{
			Label: "StorageClass",
			Value: pv.Spec.StorageClassName,
			Ref:   &RefTarget{Type: ResourceStorageClasses, Name: pv.Spec.StorageClassName},
		})
	}
	if len(entries) == 0 {
		return nil
	}
	return []LinkSection{{Entries: entries}}
}

// buildEndpointSliceLinks: owning Service (via the well-known label) plus
// the per-endpoint Pod targets (deduped).
func buildEndpointSliceLinks(es *discoveryv1.EndpointSlice) []LinkSection {
	var sections []LinkSection
	if svc := es.Labels["kubernetes.io/service-name"]; svc != "" {
		sections = append(sections, LinkSection{
			Entries: []LinkRow{{
				Label: "Service",
				Value: svc,
				Ref:   &RefTarget{Type: ResourceServices, Name: svc, Namespace: es.Namespace},
			}},
		})
	}
	seen := make(map[string]bool)
	var podEntries []LinkRow
	for _, ep := range es.Endpoints {
		if ep.TargetRef == nil || ep.TargetRef.Kind != "Pod" || ep.TargetRef.Name == "" {
			continue
		}
		key := ep.TargetRef.Namespace + "/" + ep.TargetRef.Name
		if seen[key] {
			continue
		}
		seen[key] = true
		podEntries = append(podEntries, LinkRow{
			Label: "  " + ep.TargetRef.Name,
			Value: "pod",
			Ref:   &RefTarget{Type: ResourcePods, Name: ep.TargetRef.Name, Namespace: ep.TargetRef.Namespace},
		})
	}
	if len(podEntries) > 0 {
		sections = append(sections, LinkSection{
			Title:   fmt.Sprintf("Endpoints (%d)", len(podEntries)),
			Entries: podEntries,
		})
	}
	return sections
}

// buildClusterRoleBindingLinks / buildRoleBindingLinks: RoleRef + Subjects.
// Subjects with Kind=ServiceAccount become drillable; Users/Groups remain
// info-only since they're not km8 resources.
func buildClusterRoleBindingLinks(crb *rbacv1.ClusterRoleBinding) []LinkSection {
	sections := roleRefSection(crb.RoleRef, "")
	sections = appendSubjects(sections, crb.Subjects)
	return sections
}

func buildRoleBindingLinks(rb *rbacv1.RoleBinding) []LinkSection {
	// Role RoleRefs resolve to the RoleBinding's own namespace; ClusterRole
	// RoleRefs are cluster-scoped.
	sections := roleRefSection(rb.RoleRef, rb.Namespace)
	sections = appendSubjects(sections, rb.Subjects)
	return sections
}

func roleRefSection(ref rbacv1.RoleRef, bindingNS string) []LinkSection {
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
	return []LinkSection{{
		Entries: []LinkRow{{
			Label: "RoleRef",
			Value: fmt.Sprintf("%s/%s", ref.Kind, ref.Name),
			Ref:   &RefTarget{Type: rt, Name: ref.Name, Namespace: ns},
		}},
	}}
}

func appendSubjects(sections []LinkSection, subjects []rbacv1.Subject) []LinkSection {
	if len(subjects) == 0 {
		return sections
	}
	entries := make([]LinkRow, 0, len(subjects))
	for _, s := range subjects {
		entries = append(entries, subjectToLinkRow(s))
	}
	return append(sections, LinkSection{
		Title:   fmt.Sprintf("Subjects (%d)", len(entries)),
		Entries: entries,
	})
}

func subjectToLinkRow(s rbacv1.Subject) LinkRow {
	value := s.Kind
	if s.Namespace != "" {
		value = fmt.Sprintf("%s @ %s", s.Kind, s.Namespace)
	}
	var ref *RefTarget
	if s.Kind == "ServiceAccount" && s.Namespace != "" && s.Name != "" {
		ref = &RefTarget{Type: ResourceServiceAccounts, Name: s.Name, Namespace: s.Namespace}
	}
	return LinkRow{Label: "  " + s.Name, Value: value, Ref: ref}
}

// buildServiceAccountStaticLinks: the secrets explicitly attached to the SA
// + imagePullSecrets. Pods using this SA come from enrichServiceAccountConsumers.
func buildServiceAccountStaticLinks(sa *corev1.ServiceAccount) []LinkSection {
	var sections []LinkSection
	if len(sa.Secrets) > 0 {
		entries := make([]LinkRow, 0, len(sa.Secrets))
		for _, ref := range sa.Secrets {
			entries = append(entries, LinkRow{
				Label: "  " + ref.Name,
				Value: "Secret",
				Ref:   &RefTarget{Type: ResourceSecrets, Name: ref.Name, Namespace: sa.Namespace},
			})
		}
		sections = append(sections, LinkSection{
			Title:   fmt.Sprintf("Secrets (%d)", len(entries)),
			Entries: entries,
		})
	}
	if len(sa.ImagePullSecrets) > 0 {
		entries := make([]LinkRow, 0, len(sa.ImagePullSecrets))
		for _, ref := range sa.ImagePullSecrets {
			entries = append(entries, LinkRow{
				Label: "  " + ref.Name,
				Value: "Secret",
				Ref:   &RefTarget{Type: ResourceSecrets, Name: ref.Name, Namespace: sa.Namespace},
			})
		}
		sections = append(sections, LinkSection{
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
	var entries []LinkRow
	for i := range list.Items {
		p := &list.Items[i]
		if p.Spec.NodeName != node.Name {
			continue
		}
		entries = append(entries, LinkRow{
			Label: "  " + p.Namespace + "/" + p.Name,
			Value: string(p.Status.Phase),
			Ref:   &RefTarget{Type: ResourcePods, Name: p.Name, Namespace: p.Namespace},
		})
	}
	if len(entries) == 0 {
		return
	}
	detail.Links = append(detail.Links, LinkSection{
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
	var entries []LinkRow
	for i := range list.Items {
		p := &list.Items[i]
		podSA := p.Spec.ServiceAccountName
		if podSA == "" {
			podSA = "default"
		}
		if podSA != sa.Name {
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
	var entries []LinkRow
	for i := range list.Items {
		rb := &list.Items[i]
		if rb.RoleRef.Kind != "Role" || rb.RoleRef.Name != role.Name {
			continue
		}
		entries = append(entries, LinkRow{
			Label: "  " + rb.Name,
			Value: "RoleBinding",
			Ref:   &RefTarget{Type: ResourceRoleBindings, Name: rb.Name, Namespace: rb.Namespace},
		})
	}
	if len(entries) == 0 {
		return
	}
	detail.Links = append(detail.Links, LinkSection{
		Title:   fmt.Sprintf("Bound by (%d)", len(entries)),
		Entries: entries,
	})
}

// appendSelectorPodSection lists pods matching `selector` in `ns` and adds
// a section titled `title` to detail.Links. No-op when the list is empty.
func appendSelectorPodSection(ctx context.Context, cs kubernetes.Interface, ns, selector, title string, detail *ResourceDetail) {
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
		Title:   fmt.Sprintf("%s (%d)", title, len(entries)),
		Entries: entries,
	})
}
