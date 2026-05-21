package k8s

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	autoscalingv2 "k8s.io/api/autoscaling/v2"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	storagev1 "k8s.io/api/storage/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// FetchResources lists resources of the given type via the DefaultRegistry.
func FetchResources(ctx context.Context, clientset kubernetes.Interface, rt ResourceType, namespace string) ([]ResourceItem, error) {
	return DefaultRegistry.FetchResources(ctx, clientset, rt, namespace)
}

// GetResourceDetail extracts structured detail via the DefaultRegistry.
func GetResourceDetail(rt ResourceType, item ResourceItem) ResourceDetail {
	return DefaultRegistry.GetResourceDetail(rt, item)
}

// FetchResourceEvents fetches events related to a specific resource by name,
// optionally filtering by namespace. Events are returned sorted by last
// timestamp, newest first.
func FetchResourceEvents(ctx context.Context, clientset kubernetes.Interface, name, namespace string) ([]EventItem, error) {
	selector := fmt.Sprintf("involvedObject.name=%s", name)
	opts := metav1.ListOptions{FieldSelector: selector}

	list, err := clientset.CoreV1().Events(namespace).List(ctx, opts)
	if err != nil {
		return nil, fmt.Errorf("listing events for %s: %w", name, err)
	}

	// Sort by last timestamp, newest first.
	sort.Slice(list.Items, func(i, j int) bool {
		ti := eventTime(list.Items[i])
		tj := eventTime(list.Items[j])
		return ti.After(tj)
	})

	items := make([]EventItem, 0, len(list.Items))
	for i := range list.Items {
		e := &list.Items[i]
		items = append(items, EventItem{
			Type:    e.Type,
			Reason:  e.Reason,
			Object:  fmt.Sprintf("%s/%s", e.InvolvedObject.Kind, e.InvolvedObject.Name),
			Message: e.Message,
			Age:     formatAge(eventTime(*e)),
		})
	}
	return items, nil
}

// FetchChildResources fetches child resources for a parent via the registry's DrillDown config.
func FetchChildResources(ctx context.Context, clientset kubernetes.Interface, parentType ResourceType, item ResourceItem) (ResourceType, []ResourceItem, error) {
	def := DefaultRegistry.Get(parentType)
	if def == nil || def.DrillDown == nil || def.DrillDown.FetchChildren == nil {
		return "", nil, fmt.Errorf("drill-down not supported for %s", parentType)
	}
	children, err := def.DrillDown.FetchChildren(ctx, clientset, item)
	return def.DrillDown.ChildType, children, err
}

func fetchPodsWithSelector(ctx context.Context, cs kubernetes.Interface, namespace, selector string) ([]ResourceItem, error) {
	list, err := cs.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{LabelSelector: selector})
	if err != nil {
		return nil, fmt.Errorf("listing pods: %w", err)
	}
	items := make([]ResourceItem, 0, len(list.Items))
	for i := range list.Items {
		p := &list.Items[i]
		ready, total := podReadyCounts(p)
		items = append(items, ResourceItem{
			Name:      p.Name,
			Namespace: p.Namespace,
			UID:       string(p.UID),
			Raw:       p,
			Row: []string{
				p.Name,
				fmt.Sprintf("%d/%d", ready, total),
				string(p.Status.Phase),
				fmt.Sprintf("%d", podRestarts(p)),
				formatAge(p.CreationTimestamp.Time),
				p.Spec.NodeName,
			},
		})
	}
	return items, nil
}

func fetchPodsForJob(ctx context.Context, cs kubernetes.Interface, item ResourceItem) ([]ResourceItem, error) {
	selector := fmt.Sprintf("job-name=%s", item.Name)
	return fetchPodsWithSelector(ctx, cs, item.Namespace, selector)
}

func fetchJobsForCronJob(ctx context.Context, cs kubernetes.Interface, item ResourceItem) ([]ResourceItem, error) {
	list, err := cs.BatchV1().Jobs(item.Namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("listing jobs: %w", err)
	}
	var items []ResourceItem
	for i := range list.Items {
		j := &list.Items[i]
		for _, ref := range j.OwnerReferences {
			if ref.Kind == "CronJob" && ref.Name == item.Name {
				completions := int32(0)
				if j.Spec.Completions != nil {
					completions = *j.Spec.Completions
				}
				duration := "<pending>"
				if j.Status.StartTime != nil {
					end := time.Now()
					if j.Status.CompletionTime != nil {
						end = j.Status.CompletionTime.Time
					}
					duration = end.Sub(j.Status.StartTime.Time).Truncate(time.Second).String()
				}
				items = append(items, ResourceItem{
					Name:      j.Name,
					Namespace: j.Namespace,
					UID:       string(j.UID),
					Raw:       j,
					Row: []string{
						j.Name,
						fmt.Sprintf("%d/%d", j.Status.Succeeded, completions),
						duration,
						formatAge(j.CreationTimestamp.Time),
					},
				})
				break
			}
		}
	}
	return items, nil
}

// ---------------------------------------------------------------------------
// formatAge
// ---------------------------------------------------------------------------

// formatAge converts a timestamp to a human-readable age string relative to
// now (e.g. "3d", "5h", "2m", "10s"). Returns "<unknown>" for zero times.
func formatAge(t time.Time) string {
	if t.IsZero() {
		return "<unknown>"
	}
	d := time.Since(t)
	if d < 0 {
		d = 0
	}
	switch {
	case d < time.Minute:
		return fmt.Sprintf("%ds", int(d.Seconds()))
	case d < time.Hour:
		return fmt.Sprintf("%dm", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh", int(d.Hours()))
	default:
		return fmt.Sprintf("%dd", int(d.Hours()/24))
	}
}

// ---------------------------------------------------------------------------
// eventTime helper
// ---------------------------------------------------------------------------

func eventTime(e corev1.Event) time.Time {
	if !e.LastTimestamp.IsZero() {
		return e.LastTimestamp.Time
	}
	if e.EventTime.Time.IsZero() {
		return e.CreationTimestamp.Time
	}
	return e.EventTime.Time
}

// ---------------------------------------------------------------------------
// Namespace
// ---------------------------------------------------------------------------

func fetchNamespaces(ctx context.Context, cs kubernetes.Interface) ([]ResourceItem, error) {
	list, err := cs.CoreV1().Namespaces().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("listing namespaces: %w", err)
	}
	items := make([]ResourceItem, 0, len(list.Items))
	for i := range list.Items {
		ns := &list.Items[i]
		items = append(items, ResourceItem{
			Name:      ns.Name,
			Namespace: "",
			UID:       string(ns.UID),
			Raw:       ns,
			Row: []string{
				ns.Name,
				string(ns.Status.Phase),
				formatAge(ns.CreationTimestamp.Time),
			},
		})
	}
	return items, nil
}

func detailNamespace(item ResourceItem) ResourceDetail {
	ns, _ := item.Raw.(*corev1.Namespace)
	d := baseDetail(item, "Namespace", ns.ObjectMeta)
	d.Fields = []DetailField{
		{Label: "Phase", Value: string(ns.Status.Phase)},
	}
	return d
}

// ---------------------------------------------------------------------------
// Node
// ---------------------------------------------------------------------------

func fetchNodes(ctx context.Context, cs kubernetes.Interface) ([]ResourceItem, error) {
	list, err := cs.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("listing nodes: %w", err)
	}
	items := make([]ResourceItem, 0, len(list.Items))
	for i := range list.Items {
		n := &list.Items[i]
		items = append(items, ResourceItem{
			Name:      n.Name,
			Namespace: "",
			UID:       string(n.UID),
			Raw:       n,
			Row: []string{
				n.Name,
				nodeStatus(n),
				nodeRoles(n),
				n.Status.NodeInfo.KubeletVersion,
				formatAge(n.CreationTimestamp.Time),
			},
		})
	}
	return items, nil
}

func nodeStatus(n *corev1.Node) string {
	for _, c := range n.Status.Conditions {
		if c.Type == corev1.NodeReady {
			if c.Status == corev1.ConditionTrue {
				return "Ready"
			}
			return "NotReady"
		}
	}
	return "Unknown"
}

func nodeRoles(n *corev1.Node) string {
	var roles []string
	for k := range n.Labels {
		const prefix = "node-role.kubernetes.io/"
		if strings.HasPrefix(k, prefix) {
			role := strings.TrimPrefix(k, prefix)
			if role == "" {
				role = "worker"
			}
			roles = append(roles, role)
		}
	}
	if len(roles) == 0 {
		return "<none>"
	}
	sort.Strings(roles)
	return strings.Join(roles, ",")
}

func detailNode(item ResourceItem) ResourceDetail {
	n, _ := item.Raw.(*corev1.Node)
	d := baseDetail(item, "Node", n.ObjectMeta)
	d.Fields = []DetailField{
		{Label: "Status", Value: nodeStatus(n)},
		{Label: "Roles", Value: nodeRoles(n)},
		{Label: "Kubelet Version", Value: n.Status.NodeInfo.KubeletVersion},
		{Label: "OS Image", Value: n.Status.NodeInfo.OSImage},
		{Label: "Kernel Version", Value: n.Status.NodeInfo.KernelVersion},
		{Label: "Container Runtime", Value: n.Status.NodeInfo.ContainerRuntimeVersion},
		{Label: "Architecture", Value: n.Status.NodeInfo.Architecture},
	}
	return d
}

// ---------------------------------------------------------------------------
// Pod
// ---------------------------------------------------------------------------

func fetchPods(ctx context.Context, cs kubernetes.Interface, ns string) ([]ResourceItem, error) {
	list, err := cs.CoreV1().Pods(ns).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("listing pods: %w", err)
	}
	items := make([]ResourceItem, 0, len(list.Items))
	for i := range list.Items {
		p := &list.Items[i]
		ready, total := podReadyCounts(p)
		restarts := podRestarts(p)
		items = append(items, ResourceItem{
			Name:      p.Name,
			Namespace: p.Namespace,
			UID:       string(p.UID),
			Raw:       p,
			Row: []string{
				p.Name,
				fmt.Sprintf("%d/%d", ready, total),
				string(p.Status.Phase),
				fmt.Sprintf("%d", restarts),
				formatAge(p.CreationTimestamp.Time),
				p.Spec.NodeName,
			},
		})
	}
	return items, nil
}

func podReadyCounts(p *corev1.Pod) (ready, total int) {
	total = len(p.Spec.Containers)
	for _, cs := range p.Status.ContainerStatuses {
		if cs.Ready {
			ready++
		}
	}
	return
}

func podRestarts(p *corev1.Pod) int32 {
	var total int32
	for _, cs := range p.Status.ContainerStatuses {
		total += cs.RestartCount
	}
	return total
}

func detailPod(item ResourceItem) ResourceDetail {
	p, _ := item.Raw.(*corev1.Pod)
	d := baseDetail(item, "Pod", p.ObjectMeta)
	ready, total := podReadyCounts(p)

	d.Fields = []DetailField{
		{Label: "Phase", Value: string(p.Status.Phase)},
		{Label: "Node", Value: p.Spec.NodeName},
		{Label: "Pod IP", Value: p.Status.PodIP},
		{Label: "Host IP", Value: p.Status.HostIP},
		{Label: "Ready", Value: fmt.Sprintf("%d/%d", ready, total)},
		{Label: "Restarts", Value: fmt.Sprintf("%d", podRestarts(p))},
		{Label: "Service Account", Value: p.Spec.ServiceAccountName},
	}
	if p.Spec.Priority != nil {
		d.Fields = append(d.Fields, DetailField{Label: "Priority", Value: fmt.Sprintf("%d", *p.Spec.Priority)})
	}

	statusMap := make(map[string]corev1.ContainerStatus)
	for i := range p.Status.ContainerStatuses {
		statusMap[p.Status.ContainerStatuses[i].Name] = p.Status.ContainerStatuses[i]
	}
	for i := range p.Status.InitContainerStatuses {
		statusMap[p.Status.InitContainerStatuses[i].Name] = p.Status.InitContainerStatuses[i]
	}

	for _, c := range p.Spec.InitContainers {
		d.Containers = append(d.Containers, containerDetail(c, statusMap, true))
	}
	for _, c := range p.Spec.Containers {
		d.Containers = append(d.Containers, containerDetail(c, statusMap, false))
	}

	return d
}

func containerDetail(c corev1.Container, statusMap map[string]corev1.ContainerStatus, isInit bool) ContainerInfo {
	info := ContainerInfo{
		Name:  c.Name,
		Image: c.Image,
		Init:  isInit,
	}

	var ports []string
	for _, p := range c.Ports {
		if p.Name != "" {
			ports = append(ports, fmt.Sprintf("%s:%d/%s", p.Name, p.ContainerPort, p.Protocol))
		} else {
			ports = append(ports, fmt.Sprintf("%d/%s", p.ContainerPort, p.Protocol))
		}
	}
	info.Ports = strings.Join(ports, ", ")

	if cs, ok := statusMap[c.Name]; ok {
		info.Ready = cs.Ready
		info.Restarts = int(cs.RestartCount)
		if cs.State.Running != nil {
			info.State = "Running"
		} else if cs.State.Waiting != nil {
			info.State = "Waiting: " + cs.State.Waiting.Reason
		} else if cs.State.Terminated != nil {
			info.State = "Terminated: " + cs.State.Terminated.Reason
		}
	}

	return info
}

// ---------------------------------------------------------------------------
// Deployment
// ---------------------------------------------------------------------------

func fetchDeployments(ctx context.Context, cs kubernetes.Interface, ns string) ([]ResourceItem, error) {
	list, err := cs.AppsV1().Deployments(ns).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("listing deployments: %w", err)
	}
	items := make([]ResourceItem, 0, len(list.Items))
	for i := range list.Items {
		dep := &list.Items[i]
		replicas := int32(0)
		if dep.Spec.Replicas != nil {
			replicas = *dep.Spec.Replicas
		}
		items = append(items, ResourceItem{
			Name:      dep.Name,
			Namespace: dep.Namespace,
			UID:       string(dep.UID),
			Raw:       dep,
			Row: []string{
				dep.Name,
				fmt.Sprintf("%d/%d", dep.Status.ReadyReplicas, replicas),
				fmt.Sprintf("%d", dep.Status.UpdatedReplicas),
				fmt.Sprintf("%d", dep.Status.AvailableReplicas),
				formatAge(dep.CreationTimestamp.Time),
			},
		})
	}
	return items, nil
}

func detailDeployment(item ResourceItem) ResourceDetail {
	dep, _ := item.Raw.(*appsv1.Deployment)
	d := baseDetail(item, "Deployment", dep.ObjectMeta)
	replicas := int32(0)
	if dep.Spec.Replicas != nil {
		replicas = *dep.Spec.Replicas
	}
	strategy := string(dep.Spec.Strategy.Type)
	d.Fields = []DetailField{
		{Label: "Strategy", Value: strategy},
		{Label: "Replicas", Value: fmt.Sprintf("%d desired | %d updated | %d available | %d ready",
			replicas, dep.Status.UpdatedReplicas, dep.Status.AvailableReplicas, dep.Status.ReadyReplicas)},
		{Label: "Min Ready Seconds", Value: fmt.Sprintf("%d", dep.Spec.MinReadySeconds)},
	}
	if dep.Spec.Strategy.RollingUpdate != nil {
		ru := dep.Spec.Strategy.RollingUpdate
		maxUnavail := "<nil>"
		maxSurge := "<nil>"
		if ru.MaxUnavailable != nil {
			maxUnavail = ru.MaxUnavailable.String()
		}
		if ru.MaxSurge != nil {
			maxSurge = ru.MaxSurge.String()
		}
		d.Fields = append(d.Fields,
			DetailField{Label: "Max Unavailable", Value: maxUnavail},
			DetailField{Label: "Max Surge", Value: maxSurge},
		)
	}
	return d
}

// ---------------------------------------------------------------------------
// DaemonSet
// ---------------------------------------------------------------------------

func fetchDaemonSets(ctx context.Context, cs kubernetes.Interface, ns string) ([]ResourceItem, error) {
	list, err := cs.AppsV1().DaemonSets(ns).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("listing daemonsets: %w", err)
	}
	items := make([]ResourceItem, 0, len(list.Items))
	for i := range list.Items {
		ds := &list.Items[i]
		items = append(items, ResourceItem{
			Name:      ds.Name,
			Namespace: ds.Namespace,
			UID:       string(ds.UID),
			Raw:       ds,
			Row: []string{
				ds.Name,
				fmt.Sprintf("%d", ds.Status.DesiredNumberScheduled),
				fmt.Sprintf("%d", ds.Status.CurrentNumberScheduled),
				fmt.Sprintf("%d", ds.Status.NumberReady),
				formatAge(ds.CreationTimestamp.Time),
			},
		})
	}
	return items, nil
}

func detailDaemonSet(item ResourceItem) ResourceDetail {
	ds, _ := item.Raw.(*appsv1.DaemonSet)
	d := baseDetail(item, "DaemonSet", ds.ObjectMeta)
	d.Fields = []DetailField{
		{Label: "Desired", Value: fmt.Sprintf("%d", ds.Status.DesiredNumberScheduled)},
		{Label: "Current", Value: fmt.Sprintf("%d", ds.Status.CurrentNumberScheduled)},
		{Label: "Ready", Value: fmt.Sprintf("%d", ds.Status.NumberReady)},
		{Label: "Up-to-date", Value: fmt.Sprintf("%d", ds.Status.UpdatedNumberScheduled)},
		{Label: "Available", Value: fmt.Sprintf("%d", ds.Status.NumberAvailable)},
		{Label: "Update Strategy", Value: string(ds.Spec.UpdateStrategy.Type)},
	}
	return d
}

// ---------------------------------------------------------------------------
// StatefulSet
// ---------------------------------------------------------------------------

func fetchStatefulSets(ctx context.Context, cs kubernetes.Interface, ns string) ([]ResourceItem, error) {
	list, err := cs.AppsV1().StatefulSets(ns).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("listing statefulsets: %w", err)
	}
	items := make([]ResourceItem, 0, len(list.Items))
	for i := range list.Items {
		ss := &list.Items[i]
		replicas := int32(0)
		if ss.Spec.Replicas != nil {
			replicas = *ss.Spec.Replicas
		}
		items = append(items, ResourceItem{
			Name:      ss.Name,
			Namespace: ss.Namespace,
			UID:       string(ss.UID),
			Raw:       ss,
			Row: []string{
				ss.Name,
				fmt.Sprintf("%d/%d", ss.Status.ReadyReplicas, replicas),
				formatAge(ss.CreationTimestamp.Time),
			},
		})
	}
	return items, nil
}

func detailStatefulSet(item ResourceItem) ResourceDetail {
	ss, _ := item.Raw.(*appsv1.StatefulSet)
	d := baseDetail(item, "StatefulSet", ss.ObjectMeta)
	replicas := int32(0)
	if ss.Spec.Replicas != nil {
		replicas = *ss.Spec.Replicas
	}
	d.Fields = []DetailField{
		{Label: "Replicas", Value: fmt.Sprintf("%d desired | %d ready | %d current",
			replicas, ss.Status.ReadyReplicas, ss.Status.CurrentReplicas)},
		{Label: "Update Strategy", Value: string(ss.Spec.UpdateStrategy.Type)},
		{Label: "Pod Management Policy", Value: string(ss.Spec.PodManagementPolicy)},
		{Label: "Service Name", Value: ss.Spec.ServiceName},
	}
	return d
}

// ---------------------------------------------------------------------------
// Job
// ---------------------------------------------------------------------------

func fetchJobs(ctx context.Context, cs kubernetes.Interface, ns string) ([]ResourceItem, error) {
	list, err := cs.BatchV1().Jobs(ns).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("listing jobs: %w", err)
	}
	items := make([]ResourceItem, 0, len(list.Items))
	for i := range list.Items {
		j := &list.Items[i]
		completions := int32(1)
		if j.Spec.Completions != nil {
			completions = *j.Spec.Completions
		}
		items = append(items, ResourceItem{
			Name:      j.Name,
			Namespace: j.Namespace,
			UID:       string(j.UID),
			Raw:       j,
			Row: []string{
				j.Name,
				fmt.Sprintf("%d/%d", j.Status.Succeeded, completions),
				jobDuration(j),
				formatAge(j.CreationTimestamp.Time),
			},
		})
	}
	return items, nil
}

func jobDuration(j *batchv1.Job) string {
	if j.Status.StartTime == nil {
		return "<pending>"
	}
	end := time.Now()
	if j.Status.CompletionTime != nil {
		end = j.Status.CompletionTime.Time
	}
	d := end.Sub(j.Status.StartTime.Time)
	if d < 0 {
		d = 0
	}
	return formatDuration(d)
}

func formatDuration(d time.Duration) string {
	switch {
	case d < time.Minute:
		return fmt.Sprintf("%ds", int(d.Seconds()))
	case d < time.Hour:
		return fmt.Sprintf("%dm%ds", int(d.Minutes()), int(d.Seconds())%60)
	default:
		return fmt.Sprintf("%dh%dm", int(d.Hours()), int(d.Minutes())%60)
	}
}

func detailJob(item ResourceItem) ResourceDetail {
	j, _ := item.Raw.(*batchv1.Job)
	d := baseDetail(item, "Job", j.ObjectMeta)
	completions := int32(1)
	if j.Spec.Completions != nil {
		completions = *j.Spec.Completions
	}
	parallelism := int32(1)
	if j.Spec.Parallelism != nil {
		parallelism = *j.Spec.Parallelism
	}
	d.Fields = []DetailField{
		{Label: "Completions", Value: fmt.Sprintf("%d/%d", j.Status.Succeeded, completions)},
		{Label: "Parallelism", Value: fmt.Sprintf("%d", parallelism)},
		{Label: "Duration", Value: jobDuration(j)},
		{Label: "Active", Value: fmt.Sprintf("%d", j.Status.Active)},
		{Label: "Failed", Value: fmt.Sprintf("%d", j.Status.Failed)},
	}
	if j.Spec.BackoffLimit != nil {
		d.Fields = append(d.Fields, DetailField{Label: "Backoff Limit", Value: fmt.Sprintf("%d", *j.Spec.BackoffLimit)})
	}
	return d
}

// ---------------------------------------------------------------------------
// CronJob
// ---------------------------------------------------------------------------

func fetchCronJobs(ctx context.Context, cs kubernetes.Interface, ns string) ([]ResourceItem, error) {
	list, err := cs.BatchV1().CronJobs(ns).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("listing cronjobs: %w", err)
	}
	items := make([]ResourceItem, 0, len(list.Items))
	for i := range list.Items {
		cj := &list.Items[i]
		suspend := "False"
		if cj.Spec.Suspend != nil && *cj.Spec.Suspend {
			suspend = "True"
		}
		lastSchedule := "<none>"
		if cj.Status.LastScheduleTime != nil {
			lastSchedule = formatAge(cj.Status.LastScheduleTime.Time)
		}
		items = append(items, ResourceItem{
			Name:      cj.Name,
			Namespace: cj.Namespace,
			UID:       string(cj.UID),
			Raw:       cj,
			Row: []string{
				cj.Name,
				cj.Spec.Schedule,
				suspend,
				fmt.Sprintf("%d", len(cj.Status.Active)),
				lastSchedule,
			},
		})
	}
	return items, nil
}

func detailCronJob(item ResourceItem) ResourceDetail {
	cj, _ := item.Raw.(*batchv1.CronJob)
	d := baseDetail(item, "CronJob", cj.ObjectMeta)
	suspend := "False"
	if cj.Spec.Suspend != nil && *cj.Spec.Suspend {
		suspend = "True"
	}
	lastSchedule := "<none>"
	if cj.Status.LastScheduleTime != nil {
		lastSchedule = cj.Status.LastScheduleTime.Time.Format(time.RFC3339)
	}
	d.Fields = []DetailField{
		{Label: "Schedule", Value: cj.Spec.Schedule},
		{Label: "Suspend", Value: suspend},
		{Label: "Active", Value: fmt.Sprintf("%d", len(cj.Status.Active))},
		{Label: "Last Schedule", Value: lastSchedule},
		{Label: "Concurrency Policy", Value: string(cj.Spec.ConcurrencyPolicy)},
	}
	if cj.Spec.SuccessfulJobsHistoryLimit != nil {
		d.Fields = append(d.Fields, DetailField{Label: "Success History Limit", Value: fmt.Sprintf("%d", *cj.Spec.SuccessfulJobsHistoryLimit)})
	}
	if cj.Spec.FailedJobsHistoryLimit != nil {
		d.Fields = append(d.Fields, DetailField{Label: "Failed History Limit", Value: fmt.Sprintf("%d", *cj.Spec.FailedJobsHistoryLimit)})
	}
	return d
}

// ---------------------------------------------------------------------------
// Service
// ---------------------------------------------------------------------------

func fetchServices(ctx context.Context, cs kubernetes.Interface, ns string) ([]ResourceItem, error) {
	list, err := cs.CoreV1().Services(ns).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("listing services: %w", err)
	}
	items := make([]ResourceItem, 0, len(list.Items))
	for i := range list.Items {
		svc := &list.Items[i]
		items = append(items, ResourceItem{
			Name:      svc.Name,
			Namespace: svc.Namespace,
			UID:       string(svc.UID),
			Raw:       svc,
			Row: []string{
				svc.Name,
				string(svc.Spec.Type),
				svc.Spec.ClusterIP,
				serviceExternalIPs(svc),
				servicePorts(svc),
				formatAge(svc.CreationTimestamp.Time),
			},
		})
	}
	return items, nil
}

func serviceExternalIPs(svc *corev1.Service) string {
	if len(svc.Status.LoadBalancer.Ingress) > 0 {
		var ips []string
		for _, ing := range svc.Status.LoadBalancer.Ingress {
			if ing.IP != "" {
				ips = append(ips, ing.IP)
			} else if ing.Hostname != "" {
				ips = append(ips, ing.Hostname)
			}
		}
		if len(ips) > 0 {
			return strings.Join(ips, ",")
		}
	}
	if len(svc.Spec.ExternalIPs) > 0 {
		return strings.Join(svc.Spec.ExternalIPs, ",")
	}
	return "<none>"
}

func servicePorts(svc *corev1.Service) string {
	if len(svc.Spec.Ports) == 0 {
		return "<none>"
	}
	var ports []string
	for _, p := range svc.Spec.Ports {
		s := fmt.Sprintf("%d/%s", p.Port, p.Protocol)
		ports = append(ports, s)
	}
	return strings.Join(ports, ",")
}

func detailService(item ResourceItem) ResourceDetail {
	svc, _ := item.Raw.(*corev1.Service)
	d := baseDetail(item, "Service", svc.ObjectMeta)

	var selectors []string
	for k, v := range svc.Spec.Selector {
		selectors = append(selectors, fmt.Sprintf("%s=%s", k, v))
	}
	sort.Strings(selectors)

	d.Fields = []DetailField{
		{Label: "Type", Value: string(svc.Spec.Type)},
		{Label: "Cluster IP", Value: svc.Spec.ClusterIP},
		{Label: "External IPs", Value: serviceExternalIPs(svc)},
		{Label: "Ports", Value: servicePorts(svc)},
		{Label: "Selector", Value: strings.Join(selectors, ", ")},
		{Label: "Session Affinity", Value: string(svc.Spec.SessionAffinity)},
	}
	return d
}

// ---------------------------------------------------------------------------
// Ingress
// ---------------------------------------------------------------------------

func fetchIngresses(ctx context.Context, cs kubernetes.Interface, ns string) ([]ResourceItem, error) {
	list, err := cs.NetworkingV1().Ingresses(ns).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("listing ingresses: %w", err)
	}
	items := make([]ResourceItem, 0, len(list.Items))
	for i := range list.Items {
		ing := &list.Items[i]
		items = append(items, ResourceItem{
			Name:      ing.Name,
			Namespace: ing.Namespace,
			UID:       string(ing.UID),
			Raw:       ing,
			Row: []string{
				ing.Name,
				ingressClass(ing),
				ingressHosts(ing),
				ingressPorts(ing),
				formatAge(ing.CreationTimestamp.Time),
			},
		})
	}
	return items, nil
}

func ingressClass(ing *networkingv1.Ingress) string {
	if ing.Spec.IngressClassName != nil {
		return *ing.Spec.IngressClassName
	}
	// Fall back to the deprecated annotation.
	if v, ok := ing.Annotations["kubernetes.io/ingress.class"]; ok {
		return v
	}
	return "<none>"
}

func ingressHosts(ing *networkingv1.Ingress) string {
	hostSet := make(map[string]struct{})
	for _, rule := range ing.Spec.Rules {
		h := rule.Host
		if h == "" {
			h = "*"
		}
		hostSet[h] = struct{}{}
	}
	if len(hostSet) == 0 {
		return "*"
	}
	hosts := make([]string, 0, len(hostSet))
	for h := range hostSet {
		hosts = append(hosts, h)
	}
	sort.Strings(hosts)
	return strings.Join(hosts, ",")
}

func ingressPorts(ing *networkingv1.Ingress) string {
	hasTLS := len(ing.Spec.TLS) > 0
	if hasTLS {
		return "80,443"
	}
	return "80"
}

func detailIngress(item ResourceItem) ResourceDetail {
	ing, _ := item.Raw.(*networkingv1.Ingress)
	d := baseDetail(item, "Ingress", ing.ObjectMeta)
	d.Fields = []DetailField{
		{Label: "Class", Value: ingressClass(ing)},
		{Label: "Hosts", Value: ingressHosts(ing)},
		{Label: "Ports", Value: ingressPorts(ing)},
	}
	// Add rules detail.
	for ri, rule := range ing.Spec.Rules {
		host := rule.Host
		if host == "" {
			host = "*"
		}
		if rule.HTTP != nil {
			for _, path := range rule.HTTP.Paths {
				pathStr := "/"
				if path.Path != "" {
					pathStr = path.Path
				}
				backend := fmt.Sprintf("%s:%v", path.Backend.Service.Name, path.Backend.Service.Port.Number)
				d.Fields = append(d.Fields, DetailField{
					Label: fmt.Sprintf("Rule %d", ri+1),
					Value: fmt.Sprintf("%s%s -> %s", host, pathStr, backend),
				})
			}
		}
	}
	return d
}

// ---------------------------------------------------------------------------
// ConfigMap
// ---------------------------------------------------------------------------

func fetchConfigMaps(ctx context.Context, cs kubernetes.Interface, ns string) ([]ResourceItem, error) {
	list, err := cs.CoreV1().ConfigMaps(ns).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("listing configmaps: %w", err)
	}
	items := make([]ResourceItem, 0, len(list.Items))
	for i := range list.Items {
		cm := &list.Items[i]
		items = append(items, ResourceItem{
			Name:      cm.Name,
			Namespace: cm.Namespace,
			UID:       string(cm.UID),
			Raw:       cm,
			Row: []string{
				cm.Name,
				fmt.Sprintf("%d", len(cm.Data)+len(cm.BinaryData)),
				formatAge(cm.CreationTimestamp.Time),
			},
		})
	}
	return items, nil
}

func detailConfigMap(item ResourceItem) ResourceDetail {
	cm, _ := item.Raw.(*corev1.ConfigMap)
	d := baseDetail(item, "ConfigMap", cm.ObjectMeta)
	d.Fields = []DetailField{
		{Label: "Data Keys", Value: fmt.Sprintf("%d", len(cm.Data))},
		{Label: "Binary Data Keys", Value: fmt.Sprintf("%d", len(cm.BinaryData))},
	}
	// List keys (not values for safety — could be large).
	var keys []string
	for k := range cm.Data {
		keys = append(keys, k)
	}
	for k := range cm.BinaryData {
		keys = append(keys, k+" (binary)")
	}
	sort.Strings(keys)
	if len(keys) > 0 {
		d.Fields = append(d.Fields, DetailField{Label: "Keys", Value: strings.Join(keys, ", ")})
	}
	return d
}

// ---------------------------------------------------------------------------
// Secret — metadata only, never show data content
// ---------------------------------------------------------------------------

func fetchSecrets(ctx context.Context, cs kubernetes.Interface, ns string) ([]ResourceItem, error) {
	list, err := cs.CoreV1().Secrets(ns).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("listing secrets: %w", err)
	}
	items := make([]ResourceItem, 0, len(list.Items))
	for i := range list.Items {
		s := &list.Items[i]
		items = append(items, ResourceItem{
			Name:      s.Name,
			Namespace: s.Namespace,
			UID:       string(s.UID),
			Raw:       s,
			Row: []string{
				s.Name,
				string(s.Type),
				fmt.Sprintf("%d", len(s.Data)),
				formatAge(s.CreationTimestamp.Time),
			},
		})
	}
	return items, nil
}

func detailSecret(item ResourceItem) ResourceDetail {
	s, _ := item.Raw.(*corev1.Secret)
	d := baseDetail(item, "Secret", s.ObjectMeta)
	d.Fields = []DetailField{
		{Label: "Type", Value: string(s.Type)},
		{Label: "Data Keys", Value: fmt.Sprintf("%d", len(s.Data))},
	}
	// List key names only — never show secret data content.
	var keys []string
	for k := range s.Data {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	if len(keys) > 0 {
		d.Fields = append(d.Fields, DetailField{Label: "Keys", Value: strings.Join(keys, ", ")})
	}
	return d
}

// ---------------------------------------------------------------------------
// Events
// ---------------------------------------------------------------------------

func fetchEvents(ctx context.Context, cs kubernetes.Interface, ns string) ([]ResourceItem, error) {
	list, err := cs.CoreV1().Events(ns).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("listing events: %w", err)
	}

	// Sort newest first.
	sort.Slice(list.Items, func(i, j int) bool {
		return eventTime(list.Items[i]).After(eventTime(list.Items[j]))
	})

	items := make([]ResourceItem, 0, len(list.Items))
	for i := range list.Items {
		e := &list.Items[i]
		items = append(items, ResourceItem{
			Name:      e.Name,
			Namespace: e.Namespace,
			UID:       string(e.UID),
			Raw:       e,
			Row: []string{
				e.Type,
				e.Reason,
				fmt.Sprintf("%s/%s", e.InvolvedObject.Kind, e.InvolvedObject.Name),
				e.Message,
				formatAge(eventTime(*e)),
			},
		})
	}
	return items, nil
}

func detailEvent(item ResourceItem) ResourceDetail {
	e, _ := item.Raw.(*corev1.Event)
	d := baseDetail(item, "Event", e.ObjectMeta)
	d.Fields = []DetailField{
		{Label: "Type", Value: e.Type},
		{Label: "Reason", Value: e.Reason},
		{Label: "Object", Value: fmt.Sprintf("%s/%s", e.InvolvedObject.Kind, e.InvolvedObject.Name)},
		{Label: "Message", Value: e.Message},
		{Label: "Source", Value: fmt.Sprintf("%s, %s", e.Source.Component, e.Source.Host)},
		{Label: "Count", Value: fmt.Sprintf("%d", e.Count)},
		{Label: "First Seen", Value: formatAge(e.FirstTimestamp.Time)},
		{Label: "Last Seen", Value: formatAge(eventTime(*e))},
	}
	return d
}

// ---------------------------------------------------------------------------
// ClusterRole
// ---------------------------------------------------------------------------

func fetchClusterRoles(ctx context.Context, cs kubernetes.Interface) ([]ResourceItem, error) {
	list, err := cs.RbacV1().ClusterRoles().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("listing clusterroles: %w", err)
	}
	items := make([]ResourceItem, 0, len(list.Items))
	for i := range list.Items {
		cr := &list.Items[i]
		items = append(items, ResourceItem{
			Name:      cr.Name,
			Namespace: "",
			UID:       string(cr.UID),
			Raw:       cr,
			Row: []string{
				cr.Name,
				formatAge(cr.CreationTimestamp.Time),
			},
		})
	}
	return items, nil
}

func detailClusterRole(item ResourceItem) ResourceDetail {
	cr, _ := item.Raw.(*rbacv1.ClusterRole)
	d := baseDetail(item, "ClusterRole", cr.ObjectMeta)
	d.Fields = []DetailField{
		{Label: "Rules", Value: fmt.Sprintf("%d", len(cr.Rules))},
	}
	return d
}

// ---------------------------------------------------------------------------
// ClusterRoleBinding
// ---------------------------------------------------------------------------

func fetchClusterRoleBindings(ctx context.Context, cs kubernetes.Interface) ([]ResourceItem, error) {
	list, err := cs.RbacV1().ClusterRoleBindings().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("listing clusterrolebindings: %w", err)
	}
	items := make([]ResourceItem, 0, len(list.Items))
	for i := range list.Items {
		crb := &list.Items[i]
		items = append(items, ResourceItem{
			Name:      crb.Name,
			Namespace: "",
			UID:       string(crb.UID),
			Raw:       crb,
			Row: []string{
				crb.Name,
				fmt.Sprintf("%s/%s", crb.RoleRef.Kind, crb.RoleRef.Name),
				formatAge(crb.CreationTimestamp.Time),
			},
		})
	}
	return items, nil
}

func detailClusterRoleBinding(item ResourceItem) ResourceDetail {
	crb, _ := item.Raw.(*rbacv1.ClusterRoleBinding)
	d := baseDetail(item, "ClusterRoleBinding", crb.ObjectMeta)
	d.Fields = []DetailField{
		{Label: "RoleRef", Value: fmt.Sprintf("%s/%s", crb.RoleRef.Kind, crb.RoleRef.Name)},
	}
	for i, s := range crb.Subjects {
		d.Fields = append(d.Fields, DetailField{
			Label: fmt.Sprintf("Subject %d", i+1),
			Value: formatSubject(s),
		})
	}
	return d
}

// ---------------------------------------------------------------------------
// Role
// ---------------------------------------------------------------------------

func fetchRoles(ctx context.Context, cs kubernetes.Interface, ns string) ([]ResourceItem, error) {
	list, err := cs.RbacV1().Roles(ns).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("listing roles: %w", err)
	}
	items := make([]ResourceItem, 0, len(list.Items))
	for i := range list.Items {
		r := &list.Items[i]
		items = append(items, ResourceItem{
			Name:      r.Name,
			Namespace: r.Namespace,
			UID:       string(r.UID),
			Raw:       r,
			Row: []string{
				r.Name,
				r.Namespace,
				formatAge(r.CreationTimestamp.Time),
			},
		})
	}
	return items, nil
}

func detailRole(item ResourceItem) ResourceDetail {
	r, _ := item.Raw.(*rbacv1.Role)
	d := baseDetail(item, "Role", r.ObjectMeta)
	d.Fields = []DetailField{
		{Label: "Rules", Value: fmt.Sprintf("%d", len(r.Rules))},
	}
	return d
}

// ---------------------------------------------------------------------------
// RoleBinding
// ---------------------------------------------------------------------------

func fetchRoleBindings(ctx context.Context, cs kubernetes.Interface, ns string) ([]ResourceItem, error) {
	list, err := cs.RbacV1().RoleBindings(ns).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("listing rolebindings: %w", err)
	}
	items := make([]ResourceItem, 0, len(list.Items))
	for i := range list.Items {
		rb := &list.Items[i]
		items = append(items, ResourceItem{
			Name:      rb.Name,
			Namespace: rb.Namespace,
			UID:       string(rb.UID),
			Raw:       rb,
			Row: []string{
				rb.Name,
				rb.Namespace,
				fmt.Sprintf("%s/%s", rb.RoleRef.Kind, rb.RoleRef.Name),
				formatAge(rb.CreationTimestamp.Time),
			},
		})
	}
	return items, nil
}

func detailRoleBinding(item ResourceItem) ResourceDetail {
	rb, _ := item.Raw.(*rbacv1.RoleBinding)
	d := baseDetail(item, "RoleBinding", rb.ObjectMeta)
	d.Fields = []DetailField{
		{Label: "RoleRef", Value: fmt.Sprintf("%s/%s", rb.RoleRef.Kind, rb.RoleRef.Name)},
	}
	for i, s := range rb.Subjects {
		d.Fields = append(d.Fields, DetailField{
			Label: fmt.Sprintf("Subject %d", i+1),
			Value: formatSubject(s),
		})
	}
	return d
}

// formatSubject formats a RBAC subject as "kind:namespace/name".
func formatSubject(s rbacv1.Subject) string {
	if s.Namespace != "" {
		return fmt.Sprintf("%s:%s/%s", s.Kind, s.Namespace, s.Name)
	}
	return fmt.Sprintf("%s:%s", s.Kind, s.Name)
}

// ---------------------------------------------------------------------------
// PersistentVolume / PersistentVolumeClaim / StorageClass
// ---------------------------------------------------------------------------

// shortAccessModes formats PV/PVC access modes using kubectl shorthand
// (RWO / ROX / RWX / RWOP), comma-joined.
func shortAccessModes(modes []corev1.PersistentVolumeAccessMode) string {
	short := make([]string, 0, len(modes))
	for _, m := range modes {
		switch m {
		case corev1.ReadWriteOnce:
			short = append(short, "RWO")
		case corev1.ReadOnlyMany:
			short = append(short, "ROX")
		case corev1.ReadWriteMany:
			short = append(short, "RWX")
		case corev1.ReadWriteOncePod:
			short = append(short, "RWOP")
		}
	}
	return strings.Join(short, ",")
}

func fetchPersistentVolumes(ctx context.Context, cs kubernetes.Interface) ([]ResourceItem, error) {
	list, err := cs.CoreV1().PersistentVolumes().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("listing persistentvolumes: %w", err)
	}
	items := make([]ResourceItem, 0, len(list.Items))
	for i := range list.Items {
		pv := &list.Items[i]
		capacity := ""
		if q, ok := pv.Spec.Capacity[corev1.ResourceStorage]; ok {
			capacity = q.String()
		}
		claim := ""
		if pv.Spec.ClaimRef != nil {
			claim = fmt.Sprintf("%s/%s", pv.Spec.ClaimRef.Namespace, pv.Spec.ClaimRef.Name)
		}
		items = append(items, ResourceItem{
			Name:      pv.Name,
			Namespace: "",
			UID:       string(pv.UID),
			Raw:       pv,
			Row: []string{
				pv.Name,
				capacity,
				shortAccessModes(pv.Spec.AccessModes),
				string(pv.Status.Phase),
				claim,
				pv.Spec.StorageClassName,
				formatAge(pv.CreationTimestamp.Time),
			},
		})
	}
	return items, nil
}

func detailPersistentVolume(item ResourceItem) ResourceDetail {
	pv, _ := item.Raw.(*corev1.PersistentVolume)
	d := baseDetail(item, "PersistentVolume", pv.ObjectMeta)
	capacity := ""
	if q, ok := pv.Spec.Capacity[corev1.ResourceStorage]; ok {
		capacity = q.String()
	}
	claim := "<unbound>"
	if pv.Spec.ClaimRef != nil {
		claim = fmt.Sprintf("%s/%s", pv.Spec.ClaimRef.Namespace, pv.Spec.ClaimRef.Name)
	}
	d.Fields = []DetailField{
		{Label: "Capacity", Value: capacity},
		{Label: "AccessModes", Value: shortAccessModes(pv.Spec.AccessModes)},
		{Label: "ReclaimPolicy", Value: string(pv.Spec.PersistentVolumeReclaimPolicy)},
		{Label: "Status", Value: string(pv.Status.Phase)},
		{Label: "Claim", Value: claim},
		{Label: "StorageClass", Value: pv.Spec.StorageClassName},
	}
	if pv.Status.Reason != "" {
		d.Fields = append(d.Fields, DetailField{Label: "Reason", Value: pv.Status.Reason})
	}
	return d
}

func fetchPersistentVolumeClaims(ctx context.Context, cs kubernetes.Interface, ns string) ([]ResourceItem, error) {
	list, err := cs.CoreV1().PersistentVolumeClaims(ns).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("listing persistentvolumeclaims: %w", err)
	}
	items := make([]ResourceItem, 0, len(list.Items))
	for i := range list.Items {
		pvc := &list.Items[i]
		capacity := ""
		if q, ok := pvc.Status.Capacity[corev1.ResourceStorage]; ok {
			capacity = q.String()
		}
		sc := ""
		if pvc.Spec.StorageClassName != nil {
			sc = *pvc.Spec.StorageClassName
		}
		items = append(items, ResourceItem{
			Name:      pvc.Name,
			Namespace: pvc.Namespace,
			UID:       string(pvc.UID),
			Raw:       pvc,
			Row: []string{
				pvc.Name,
				string(pvc.Status.Phase),
				pvc.Spec.VolumeName,
				capacity,
				shortAccessModes(pvc.Status.AccessModes),
				sc,
				formatAge(pvc.CreationTimestamp.Time),
			},
		})
	}
	return items, nil
}

func detailPersistentVolumeClaim(item ResourceItem) ResourceDetail {
	pvc, _ := item.Raw.(*corev1.PersistentVolumeClaim)
	d := baseDetail(item, "PersistentVolumeClaim", pvc.ObjectMeta)
	requestCap := ""
	if q, ok := pvc.Spec.Resources.Requests[corev1.ResourceStorage]; ok {
		requestCap = q.String()
	}
	statusCap := ""
	if q, ok := pvc.Status.Capacity[corev1.ResourceStorage]; ok {
		statusCap = q.String()
	}
	sc := "<default>"
	if pvc.Spec.StorageClassName != nil {
		sc = *pvc.Spec.StorageClassName
	}
	d.Fields = []DetailField{
		{Label: "Status", Value: string(pvc.Status.Phase)},
		{Label: "Volume", Value: pvc.Spec.VolumeName},
		{Label: "RequestedCapacity", Value: requestCap},
		{Label: "BoundCapacity", Value: statusCap},
		{Label: "AccessModes", Value: shortAccessModes(pvc.Status.AccessModes)},
		{Label: "StorageClass", Value: sc},
	}
	return d
}

// fetchPodsForPVC returns Pods in the PVC's namespace that mount it via
// spec.volumes[].persistentVolumeClaim.claimName.
func fetchPodsForPVC(ctx context.Context, cs kubernetes.Interface, item ResourceItem) ([]ResourceItem, error) {
	list, err := cs.CoreV1().Pods(item.Namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("listing pods: %w", err)
	}
	items := make([]ResourceItem, 0)
	for i := range list.Items {
		p := &list.Items[i]
		uses := false
		for _, vol := range p.Spec.Volumes {
			if vol.PersistentVolumeClaim != nil && vol.PersistentVolumeClaim.ClaimName == item.Name {
				uses = true
				break
			}
		}
		if !uses {
			continue
		}
		ready, total := podReadyCounts(p)
		items = append(items, ResourceItem{
			Name:      p.Name,
			Namespace: p.Namespace,
			UID:       string(p.UID),
			Raw:       p,
			Row: []string{
				p.Name,
				fmt.Sprintf("%d/%d", ready, total),
				string(p.Status.Phase),
				fmt.Sprintf("%d", podRestarts(p)),
				formatAge(p.CreationTimestamp.Time),
				p.Spec.NodeName,
			},
		})
	}
	return items, nil
}

func fetchStorageClasses(ctx context.Context, cs kubernetes.Interface) ([]ResourceItem, error) {
	list, err := cs.StorageV1().StorageClasses().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("listing storageclasses: %w", err)
	}
	items := make([]ResourceItem, 0, len(list.Items))
	for i := range list.Items {
		sc := &list.Items[i]
		reclaim := ""
		if sc.ReclaimPolicy != nil {
			reclaim = string(*sc.ReclaimPolicy)
		}
		binding := ""
		if sc.VolumeBindingMode != nil {
			binding = string(*sc.VolumeBindingMode)
		}
		items = append(items, ResourceItem{
			Name:      sc.Name,
			Namespace: "",
			UID:       string(sc.UID),
			Raw:       sc,
			Row: []string{
				sc.Name,
				sc.Provisioner,
				reclaim,
				binding,
				formatAge(sc.CreationTimestamp.Time),
			},
		})
	}
	return items, nil
}

func detailStorageClass(item ResourceItem) ResourceDetail {
	sc, _ := item.Raw.(*storagev1.StorageClass)
	d := baseDetail(item, "StorageClass", sc.ObjectMeta)
	reclaim := "<default>"
	if sc.ReclaimPolicy != nil {
		reclaim = string(*sc.ReclaimPolicy)
	}
	binding := "<default>"
	if sc.VolumeBindingMode != nil {
		binding = string(*sc.VolumeBindingMode)
	}
	allowExpand := "false"
	if sc.AllowVolumeExpansion != nil && *sc.AllowVolumeExpansion {
		allowExpand = "true"
	}
	d.Fields = []DetailField{
		{Label: "Provisioner", Value: sc.Provisioner},
		{Label: "ReclaimPolicy", Value: reclaim},
		{Label: "VolumeBindingMode", Value: binding},
		{Label: "AllowVolumeExpansion", Value: allowExpand},
		{Label: "Parameters", Value: fmt.Sprintf("%d", len(sc.Parameters))},
	}
	return d
}

// ---------------------------------------------------------------------------
// HorizontalPodAutoscaler
// ---------------------------------------------------------------------------

// hpaTargetSummary returns a short "current/target" string for the first
// Resource-type metric, or "<n metrics>" when multiple non-Resource metrics
// exist. Empty when no metrics are reported.
func hpaTargetSummary(hpa *autoscalingv2.HorizontalPodAutoscaler) string {
	if len(hpa.Spec.Metrics) == 0 {
		return ""
	}
	if len(hpa.Spec.Metrics) > 1 {
		return fmt.Sprintf("<%d metrics>", len(hpa.Spec.Metrics))
	}
	m := hpa.Spec.Metrics[0]
	if m.Resource == nil || m.Resource.Target.AverageUtilization == nil {
		return string(m.Type)
	}
	target := *m.Resource.Target.AverageUtilization
	current := "<unknown>"
	for _, cm := range hpa.Status.CurrentMetrics {
		if cm.Resource != nil && cm.Resource.Name == m.Resource.Name && cm.Resource.Current.AverageUtilization != nil {
			current = fmt.Sprintf("%d%%", *cm.Resource.Current.AverageUtilization)
			break
		}
	}
	return fmt.Sprintf("%s/%d%%", current, target)
}

func fetchHorizontalPodAutoscalers(ctx context.Context, cs kubernetes.Interface, ns string) ([]ResourceItem, error) {
	list, err := cs.AutoscalingV2().HorizontalPodAutoscalers(ns).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("listing horizontalpodautoscalers: %w", err)
	}
	items := make([]ResourceItem, 0, len(list.Items))
	for i := range list.Items {
		hpa := &list.Items[i]
		ref := fmt.Sprintf("%s/%s", hpa.Spec.ScaleTargetRef.Kind, hpa.Spec.ScaleTargetRef.Name)
		minPods := int32(1)
		if hpa.Spec.MinReplicas != nil {
			minPods = *hpa.Spec.MinReplicas
		}
		items = append(items, ResourceItem{
			Name:      hpa.Name,
			Namespace: hpa.Namespace,
			UID:       string(hpa.UID),
			Raw:       hpa,
			Row: []string{
				hpa.Name,
				ref,
				fmt.Sprintf("%d", minPods),
				fmt.Sprintf("%d", hpa.Spec.MaxReplicas),
				fmt.Sprintf("%d", hpa.Status.CurrentReplicas),
				hpaTargetSummary(hpa),
				formatAge(hpa.CreationTimestamp.Time),
			},
		})
	}
	return items, nil
}

func detailHorizontalPodAutoscaler(item ResourceItem) ResourceDetail {
	hpa, _ := item.Raw.(*autoscalingv2.HorizontalPodAutoscaler)
	d := baseDetail(item, "HorizontalPodAutoscaler", hpa.ObjectMeta)
	minPods := int32(1)
	if hpa.Spec.MinReplicas != nil {
		minPods = *hpa.Spec.MinReplicas
	}
	d.Fields = []DetailField{
		{Label: "Reference", Value: fmt.Sprintf("%s/%s", hpa.Spec.ScaleTargetRef.Kind, hpa.Spec.ScaleTargetRef.Name)},
		{Label: "MinReplicas", Value: fmt.Sprintf("%d", minPods)},
		{Label: "MaxReplicas", Value: fmt.Sprintf("%d", hpa.Spec.MaxReplicas)},
		{Label: "CurrentReplicas", Value: fmt.Sprintf("%d", hpa.Status.CurrentReplicas)},
		{Label: "DesiredReplicas", Value: fmt.Sprintf("%d", hpa.Status.DesiredReplicas)},
		{Label: "Targets", Value: hpaTargetSummary(hpa)},
		{Label: "Metrics", Value: fmt.Sprintf("%d", len(hpa.Spec.Metrics))},
	}
	return d
}

// fetchPodsForHPA resolves the HPA's scaleTargetRef and returns the target
// workload's pods. Supports Deployment / StatefulSet / ReplicaSet / DaemonSet
// targets; other kinds return an empty list (no error).
func fetchPodsForHPA(ctx context.Context, cs kubernetes.Interface, item ResourceItem) ([]ResourceItem, error) {
	hpa, ok := item.Raw.(*autoscalingv2.HorizontalPodAutoscaler)
	if !ok {
		return nil, fmt.Errorf("HPA item missing typed Raw")
	}
	ref := hpa.Spec.ScaleTargetRef
	var selector string
	switch ref.Kind {
	case "Deployment":
		dep, err := cs.AppsV1().Deployments(item.Namespace).Get(ctx, ref.Name, metav1.GetOptions{})
		if err != nil {
			return nil, fmt.Errorf("resolving deployment %s: %w", ref.Name, err)
		}
		sel, _ := metav1.LabelSelectorAsSelector(dep.Spec.Selector)
		selector = sel.String()
	case "StatefulSet":
		ss, err := cs.AppsV1().StatefulSets(item.Namespace).Get(ctx, ref.Name, metav1.GetOptions{})
		if err != nil {
			return nil, fmt.Errorf("resolving statefulset %s: %w", ref.Name, err)
		}
		sel, _ := metav1.LabelSelectorAsSelector(ss.Spec.Selector)
		selector = sel.String()
	case "ReplicaSet":
		rs, err := cs.AppsV1().ReplicaSets(item.Namespace).Get(ctx, ref.Name, metav1.GetOptions{})
		if err != nil {
			return nil, fmt.Errorf("resolving replicaset %s: %w", ref.Name, err)
		}
		sel, _ := metav1.LabelSelectorAsSelector(rs.Spec.Selector)
		selector = sel.String()
	case "DaemonSet":
		ds, err := cs.AppsV1().DaemonSets(item.Namespace).Get(ctx, ref.Name, metav1.GetOptions{})
		if err != nil {
			return nil, fmt.Errorf("resolving daemonset %s: %w", ref.Name, err)
		}
		sel, _ := metav1.LabelSelectorAsSelector(ds.Spec.Selector)
		selector = sel.String()
	default:
		return []ResourceItem{}, nil
	}
	return fetchPodsWithSelector(ctx, cs, item.Namespace, selector)
}

// ---------------------------------------------------------------------------
// ServiceAccount
// ---------------------------------------------------------------------------

func fetchServiceAccounts(ctx context.Context, cs kubernetes.Interface, ns string) ([]ResourceItem, error) {
	list, err := cs.CoreV1().ServiceAccounts(ns).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("listing serviceaccounts: %w", err)
	}
	items := make([]ResourceItem, 0, len(list.Items))
	for i := range list.Items {
		sa := &list.Items[i]
		items = append(items, ResourceItem{
			Name:      sa.Name,
			Namespace: sa.Namespace,
			UID:       string(sa.UID),
			Raw:       sa,
			Row: []string{
				sa.Name,
				fmt.Sprintf("%d", len(sa.Secrets)),
				formatAge(sa.CreationTimestamp.Time),
			},
		})
	}
	return items, nil
}

func detailServiceAccount(item ResourceItem) ResourceDetail {
	sa, _ := item.Raw.(*corev1.ServiceAccount)
	d := baseDetail(item, "ServiceAccount", sa.ObjectMeta)
	automount := "true"
	if sa.AutomountServiceAccountToken != nil && !*sa.AutomountServiceAccountToken {
		automount = "false"
	}
	d.Fields = []DetailField{
		{Label: "Secrets", Value: fmt.Sprintf("%d", len(sa.Secrets))},
		{Label: "ImagePullSecrets", Value: fmt.Sprintf("%d", len(sa.ImagePullSecrets))},
		{Label: "AutomountToken", Value: automount},
	}
	return d
}

// ---------------------------------------------------------------------------
// Common helpers
// ---------------------------------------------------------------------------

func baseDetail(item ResourceItem, kind string, meta metav1.ObjectMeta) ResourceDetail {
	return ResourceDetail{
		Name:        item.Name,
		Namespace:   item.Namespace,
		Kind:        kind,
		UID:         item.UID,
		CreatedAt:   formatAge(meta.CreationTimestamp.Time),
		Labels:      meta.Labels,
		Annotations: meta.Annotations,
	}
}
