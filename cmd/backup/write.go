package main

import (
	"os"
	"path/filepath"
	"strings"
)

const pageSize = 100

func writeFile(path string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

// sanitize replaces characters unsafe for cross-platform filenames.
func sanitize(uid string) string {
	r := strings.NewReplacer(
		"/", "_", "\\", "_", ":", "_",
		"*", "_", "?", "_", "\"", "_",
		"<", "_", ">", "_", "|", "_",
	)
	s := r.Replace(uid)
	if s == "" {
		return "unknown"
	}
	return s
}
