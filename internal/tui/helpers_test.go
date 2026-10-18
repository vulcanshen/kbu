package tui

import (
	"reflect"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	terminu "github.com/vulcanshen/terminu-go"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	"github.com/vulcanshen/kbu/internal/config"
	"github.com/vulcanshen/kbu/internal/k8s"
)

// newTestModel is kbu without a cluster: nothing is fetched, the lists are
// fed by hand. The settings go to a temporary directory, so a test that
// saves never touches the real ones; kubectl and helm are never run.
func newTestModel(t *testing.T) *Model {
	t.Helper()
	t.Setenv("KBU__CONFIG", t.TempDir())
	t.Setenv("KBU__STATE", t.TempDir())
	terminu.SetIconWidth(1, 1)
	m := New(Options{Config: config.DefaultConfig(), State: config.DefaultState()})
	m.run = func(name string, args ...string) (string, error) {
		t.Fatalf("a test ran %s %s", name, strings.Join(args, " "))
		return "", nil
	}
	m.size(120, 40)
	m.drive(m.app.Init())
	return m
}

func (m *Model) size(w, h int) {
	_, cmd := m.app.Update(tea.WindowSizeMsg{Width: w, Height: h})
	m.drive(cmd)
}

// drive runs what a command asks for and feeds its messages back, as the
// program would: an animation's frames, a debounce's tick. A command that
// waits longer than a debounce is dropped.
func (m *Model) drive(cmd tea.Cmd) {
	queue := []tea.Cmd{cmd}
	for steps := 0; len(queue) > 0 && steps < 400; steps++ {
		c := queue[0]
		queue = queue[1:]
		if c == nil {
			continue
		}
		done := make(chan tea.Msg, 1)
		go func() { done <- c() }()
		var msg tea.Msg
		select {
		case msg = <-done:
		case <-time.After(500 * time.Millisecond):
			continue
		}
		if msg == nil {
			continue
		}
		if _, quit := msg.(tea.QuitMsg); quit {
			continue
		}
		// A batch, or a sequence (an unexported []tea.Cmd): run each.
		if v := reflect.ValueOf(msg); v.Kind() == reflect.Slice && v.Type().Elem() == reflect.TypeOf(tea.Cmd(nil)) {
			for i := range v.Len() {
				queue = append(queue, v.Index(i).Interface().(tea.Cmd))
			}
			continue
		}
		_, next := m.app.Update(msg)
		queue = append(queue, next)
	}
}

// press sends keys by their names (tdp's: "Enter", "Esc", "Alt-S", "j").
func (m *Model) press(keys ...string) {
	for _, k := range keys {
		_, cmd := m.app.Update(keyMsg(k))
		m.drive(cmd)
	}
}

func keyMsg(name string) tea.KeyMsg {
	special := map[string]tea.KeyType{
		"Enter": tea.KeyEnter, "Esc": tea.KeyEsc, "Tab": tea.KeyTab, "Shift-Tab": tea.KeyShiftTab,
		"Space": tea.KeySpace, "Backspace": tea.KeyBackspace, "Ctrl-C": tea.KeyCtrlC,
		"↑": tea.KeyUp, "↓": tea.KeyDown, "←": tea.KeyLeft, "→": tea.KeyRight,
		"PgUp": tea.KeyPgUp, "PgDn": tea.KeyPgDown,
	}
	if t, ok := special[name]; ok {
		if t == tea.KeySpace {
			return tea.KeyMsg{Type: t, Runes: []rune{' '}}
		}
		return tea.KeyMsg{Type: t}
	}
	if rest, ok := strings.CutPrefix(name, "Alt-"); ok && len([]rune(rest)) == 1 {
		return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(rest), Alt: true}
	}
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(name)}
}

// screen is the whole frame without its colours.
func (m *Model) screen_() string { return terminu.StripStyles(strings.Join(m.app.Frame(), "\n")) }

// feed gives panel 2 a list of the kind it shows, as the watcher would.
func (m *Model) feed(items ...k8s.ResourceItem) {
	m.drive(m.onMsg(watchDataMsg{kind: m.current, items: items}))
}

// pod is a pod for the lists: its row as the registry draws one.
func pod(ns, name string) k8s.ResourceItem {
	p := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: ns, UID: types.UID("uid-" + ns + "-" + name),
		CreationTimestamp: metav1.NewTime(time.Now().Add(-time.Hour))},
		Spec:   corev1.PodSpec{Containers: []corev1.Container{{Name: "app"}}},
		Status: corev1.PodStatus{Phase: corev1.PodRunning}}
	return k8s.ResourceItem{Name: name, Namespace: ns, UID: string(p.UID), Raw: p,
		Row: []string{name, "1/1", "Running", "0", "1h", "10.0.0.1", "node-1"}}
}
