package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// WriteAuditEntry appends one line to today's audit log.
// op is the operation (e.g. "edit", "delete"), resource is "kind/name",
// namespace is the resource namespace, and output is kubectl's combined output.
func WriteAuditEntry(op, resource, namespace, output string) error {
	if err := os.MkdirAll(LogDir(), 0o755); err != nil {
		return err
	}
	date := time.Now().Format("2006-01-02")
	path := filepath.Join(LogDir(), fmt.Sprintf("audit-%s.log", date))
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
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
