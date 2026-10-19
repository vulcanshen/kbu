package config

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// DataDir is where kbu keeps what it writes down for the user, its logs
// (tdp E3: a log deleted is gone for good, so it is data): $KBU__DATA,
// else ~/.kbu (on Windows %USERPROFILE%\.kbu).
func DataDir() string {
	if d := strings.TrimSpace(os.Getenv("KBU__DATA")); d != "" {
		return d
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, "."+appName)
}

// LogDir is the logs directory: the audit log and the crash logs.
func LogDir() string {
	return filepath.Join(DataDir(), "logs")
}

// WriteCrashLog writes a panic message and stack trace to a crash log file.
// Returns the file path on success, empty string on failure.
func WriteCrashLog(panicVal interface{}) string {
	dir := LogDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return ""
	}

	date := time.Now().Format("2006-01-02")
	path := filepath.Join(dir, fmt.Sprintf("crash-%s.log", date))

	buf := make([]byte, 8192)
	n := runtime.Stack(buf, false)

	content := fmt.Sprintf("---\nkbu crash at %s\n\npanic: %v\n\n%s\n",
		time.Now().Format(time.RFC3339), panicVal, buf[:n])

	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return ""
	}
	defer f.Close()
	if _, err := fmt.Fprint(f, content); err != nil {
		return ""
	}
	return path
}
