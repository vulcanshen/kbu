package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// auditLogPath is set once at session start and reused for all entries.
var auditLogPath = func() string {
	ts := time.Now().Format("2006-01-02_15-04-05")
	return filepath.Join(LogDir(), fmt.Sprintf("audit-%s.log", ts))
}()

// WriteAuditEntry appends one line to the session audit log.
// op is the operation (e.g. "edit", "delete"), resource is "kind/name",
// namespace is the resource namespace, and output is kubectl's combined output.
func WriteAuditEntry(op, resource, namespace, output string) error {
	if err := os.MkdirAll(LogDir(), 0o755); err != nil {
		return err
	}
	f, err := os.OpenFile(auditLogPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()

	ts := time.Now().Format("2006-01-02 15:04:05")
	out := strings.TrimSpace(output)
	if out == "" {
		out = "-"
	}
	_, err = fmt.Fprintf(f, "%s\t%s\t%s/%s\t%s\n", ts, op, namespace, resource, out)
	return err
}
