package k8s

import (
	"bufio"
	"context"
	"sync"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
)

// LogLine represents a single log line from a container.
type LogLine struct {
	Pod       string
	Container string
	Text      string
}

// PodTarget identifies a pod whose containers should be streamed. Used by
// LogStreamer.StartMulti to aggregate logs across multiple pods (e.g. all
// pods of a Deployment's current ReplicaSet).
type PodTarget struct {
	Name       string
	Namespace  string
	Containers []string
}

// LogStreamer streams logs from one or more containers in a pod.
// It integrates with Bubble Tea through a channel-based message pattern.
//
// aggregate distinguishes single-pod streams (Start: Pod identity is
// implicit — the user is on the Pod's detail panel) from multi-pod
// aggregate streams (StartMulti: pods come from a workload's selector and
// every line needs a Pod tag so the consumer can colour-code by source).
// When aggregate=false, emitted LogLine.Pod is left empty so the renderer
// skips the `<pod-hash>@` prefix segment.
type LogStreamer struct {
	clientset kubernetes.Interface
	mu        sync.Mutex
	cancel    context.CancelFunc
	lines     chan LogLine
	aggregate bool
}

// NewLogStreamer creates a new LogStreamer for the given clientset.
func NewLogStreamer(clientset kubernetes.Interface) *LogStreamer {
	return &LogStreamer{
		clientset: clientset,
		lines:     make(chan LogLine, 64),
	}
}

// Start streams the named containers from a single pod. LogLine.Pod is left
// empty on emitted lines so the renderer drops the `<pod-hash>@` prefix
// segment (pod identity is implicit when the user is on Pod detail).
func (ls *LogStreamer) Start(podName, namespace string, containers []string) {
	ls.start(false, []PodTarget{{Name: podName, Namespace: namespace, Containers: containers}})
}

// StartMulti cancels any existing stream and starts streaming logs from
// every (pod, container) pair in `targets`. All lines flow through the
// shared Lines() channel; each LogLine carries Pod / Container / Text so
// the consumer can multiplex by either dimension.
func (ls *LogStreamer) StartMulti(targets []PodTarget) {
	ls.start(true, targets)
}

func (ls *LogStreamer) start(aggregate bool, targets []PodTarget) {
	ls.Stop()

	ls.mu.Lock()
	defer ls.mu.Unlock()

	ls.aggregate = aggregate
	ctx, cancel := context.WithCancel(context.Background())
	ls.cancel = cancel

	// Drain any leftover lines from a previous stream.
	for {
		select {
		case <-ls.lines:
		default:
			goto drained
		}
	}
drained:

	var wg sync.WaitGroup
	for _, t := range targets {
		for _, c := range t.Containers {
			wg.Add(1)
			go func(podName, namespace, container string) {
				defer wg.Done()
				ls.streamContainer(ctx, podName, namespace, container)
			}(t.Name, t.Namespace, c)
		}
	}

	// Close-sentinel goroutine: when all container goroutines finish
	// (e.g. because ctx was cancelled), we don't close the channel
	// because it may be reused by a subsequent Start call.
	go func() {
		wg.Wait()
	}()
}

// Stop cancels the current log stream.
func (ls *LogStreamer) Stop() {
	ls.mu.Lock()
	defer ls.mu.Unlock()
	if ls.cancel != nil {
		ls.cancel()
		ls.cancel = nil
	}
}

// Lines returns the channel for receiving log lines.
func (ls *LogStreamer) Lines() <-chan LogLine {
	return ls.lines
}

// podTag returns the Pod field value to emit on each LogLine — empty in
// single-pod mode (implicit identity), populated in aggregate mode.
func (ls *LogStreamer) podTag(podName string) string {
	if ls.aggregate {
		return podName
	}
	return ""
}

func (ls *LogStreamer) streamContainer(ctx context.Context, podName, namespace, container string) {
	tailLines := int64(100)
	opts := &corev1.PodLogOptions{
		Container: container,
		Follow:    true,
		TailLines: &tailLines,
	}

	req := ls.clientset.CoreV1().Pods(namespace).GetLogs(podName, opts)
	stream, err := req.Stream(ctx)
	if err != nil {
		// Send an error line so the user sees something.
		select {
		case ls.lines <- LogLine{Pod: ls.podTag(podName), Container: container, Text: "[error: " + err.Error() + "]"}:
		case <-ctx.Done():
		}
		return
	}
	defer func() {
		_ = stream.Close()
	}()

	scanner := bufio.NewScanner(stream)
	// Increase the scanner buffer for potentially long log lines.
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	for scanner.Scan() {
		select {
		case <-ctx.Done():
			return
		case ls.lines <- LogLine{Pod: ls.podTag(podName), Container: container, Text: scanner.Text()}:
		}
	}

	// If the scanner ended due to an error (not EOF / context cancel), report it.
	if err := scanner.Err(); err != nil && ctx.Err() == nil {
		select {
		case ls.lines <- LogLine{Pod: ls.podTag(podName), Container: container, Text: "[stream error: " + err.Error() + "]"}:
		case <-ctx.Done():
		}
	}

	// If the stream ended (container terminated) and context is still active,
	// send a marker so the user knows the stream ended.
	if ctx.Err() == nil {
		select {
		case ls.lines <- LogLine{Pod: ls.podTag(podName), Container: container, Text: "[stream ended]"}:
		case <-ctx.Done():
		}
	}
}

// ContainerNames extracts container names (init + regular) from a Pod's Raw field.
// Returns nil if the Raw field is not a *corev1.Pod.
func ContainerNames(raw interface{}) []string {
	pod, ok := raw.(*corev1.Pod)
	if !ok || pod == nil {
		return nil
	}

	var names []string
	for _, c := range pod.Spec.InitContainers {
		names = append(names, c.Name)
	}
	for _, c := range pod.Spec.Containers {
		names = append(names, c.Name)
	}
	return names
}

// ReadLogLine is a helper for reading a single LogLine from the channel,
// suitable for use in non-blocking selects. It returns the read line and
// a boolean indicating whether a line was available.
func ReadLogLine(ch <-chan LogLine) (LogLine, bool) {
	select {
	case line, ok := <-ch:
		return line, ok
	default:
		return LogLine{}, false
	}
}
