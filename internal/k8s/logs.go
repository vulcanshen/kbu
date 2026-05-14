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
	Container string
	Text      string
}

// LogStreamer streams logs from one or more containers in a pod.
// It integrates with Bubble Tea through a channel-based message pattern.
type LogStreamer struct {
	clientset kubernetes.Interface
	mu        sync.Mutex
	cancel    context.CancelFunc
	lines     chan LogLine
}

// NewLogStreamer creates a new LogStreamer for the given clientset.
func NewLogStreamer(clientset kubernetes.Interface) *LogStreamer {
	return &LogStreamer{
		clientset: clientset,
		lines:     make(chan LogLine, 64),
	}
}

// Start cancels any existing stream and starts streaming logs from the
// specified containers in the given pod. Each container gets its own
// goroutine that reads log lines and sends them to the shared channel.
func (ls *LogStreamer) Start(podName, namespace string, containers []string) {
	ls.Stop()

	ls.mu.Lock()
	defer ls.mu.Unlock()

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
	for _, c := range containers {
		wg.Add(1)
		go func(container string) {
			defer wg.Done()
			ls.streamContainer(ctx, podName, namespace, container)
		}(c)
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
		case ls.lines <- LogLine{Container: container, Text: "[error: " + err.Error() + "]"}:
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
		case ls.lines <- LogLine{Container: container, Text: scanner.Text()}:
		}
	}

	// If the scanner ended due to an error (not EOF / context cancel), report it.
	if err := scanner.Err(); err != nil && ctx.Err() == nil {
		select {
		case ls.lines <- LogLine{Container: container, Text: "[stream error: " + err.Error() + "]"}:
		case <-ctx.Done():
		}
	}

	// If the stream ended (container terminated) and context is still active,
	// send a marker so the user knows the stream ended.
	if ctx.Err() == nil {
		select {
		case ls.lines <- LogLine{Container: container, Text: "[stream ended]"}:
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
