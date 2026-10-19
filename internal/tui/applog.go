package tui

import (
	"fmt"
	"strings"
	"time"

	terminu "github.com/vulcanshen/terminu-go"
)

// The App Log (!): what kbu has done and what failed, newest last. It is a
// viewer (components/dialog/note); while it is open new entries keep
// arriving in it, as they do in a log.

type logLevel int

const (
	logInfo logLevel = iota
	logWarn
	logError
	logSuccess
)

// logEntry is one line of the App Log.
type logEntry struct {
	at    time.Time
	level logLevel
	text  string
}

// maxLogEntries and maxEntryChars keep the App Log from growing without end:
// a crash-looping watcher would otherwise add a line every few seconds for
// as long as kbu runs.
const (
	maxLogEntries = 1000
	maxEntryChars = 300
)

// appLog holds the entries and counts the errors and warnings the user has
// not seen yet, for the statusbar.
type appLog struct {
	entries      []logEntry
	unseenErrors int
	unseenWarns  int
	lastSuccess  string
	viewer       *terminu.Viewer // the open App Log, fed as entries come
}

func (l *appLog) add(level logLevel, text string) {
	if r := []rune(text); len(r) > maxEntryChars {
		text = string(r[:maxEntryChars]) + "…"
	}
	l.entries = append(l.entries, logEntry{at: time.Now(), level: level, text: text})
	if len(l.entries) > maxLogEntries {
		l.entries = l.entries[len(l.entries)-maxLogEntries:]
	}
	switch level {
	case logError:
		l.unseenErrors++
	case logWarn:
		l.unseenWarns++
	case logSuccess:
		l.lastSuccess = text
	}
	if l.viewer != nil {
		l.viewer.Lines = append(l.viewer.Lines, l.entries[len(l.entries)-1].line())
	}
}

func (l *appLog) info(text string)    { l.add(logInfo, text) }
func (l *appLog) warn(text string)    { l.add(logWarn, text) }
func (l *appLog) err(text string)     { l.add(logError, text) }
func (l *appLog) success(text string) { l.add(logSuccess, text) }

// seen is the user opening the App Log: the statusbar's counts go.
func (l *appLog) seen() {
	l.unseenErrors, l.unseenWarns, l.lastSuccess = 0, 0, ""
}

// lines are the entries as the viewer shows them.
func (l *appLog) lines() []string {
	out := make([]string, len(l.entries))
	for i, e := range l.entries {
		out[i] = e.line()
	}
	return out
}

// line is one entry: its time, its level coloured as a log colours it (the
// app's content, tdp P4), then what happened.
func (e logEntry) line() string {
	level, color := "INFO", terminu.Text
	switch e.level {
	case logWarn:
		level, color = "WARN", terminu.Peach
	case logError:
		level, color = "ERR", terminu.Red
	case logSuccess:
		level, color = "OK", terminu.Green
	}
	text := strings.ReplaceAll(e.text, "\n", " ")
	return terminu.Fg(terminu.Overlay0).Render(e.at.Format("15:04:05")) + " " +
		terminu.Fg(color).Render(fmt.Sprintf("%-4s", level)) + " " + text
}
