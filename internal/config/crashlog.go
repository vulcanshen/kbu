package config

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"time"
)

// LogDir returns the path to the km8 logs directory.
func LogDir() string {
	return filepath.Join(ConfigDir(), "logs")
}

// WriteCrashLog writes a panic message and stack trace to a crash log file.
// Returns the file path on success, empty string on failure.
func WriteCrashLog(panicVal interface{}) string {
	dir := LogDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return ""
	}

	ts := time.Now().Format("2006-01-02_15-04-05")
	path := filepath.Join(dir, fmt.Sprintf("crash-%s.log", ts))

	buf := make([]byte, 8192)
	n := runtime.Stack(buf, false)

	content := fmt.Sprintf("km8 crash at %s\n\npanic: %v\n\n%s\n",
		time.Now().Format(time.RFC3339), panicVal, buf[:n])

	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		return ""
	}
	return path
}
