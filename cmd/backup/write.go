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

// sanitize turns an arbitrary UID into a safe single-path-component filename.
//
// It (1) trims surrounding whitespace, (2) replaces path separators and
// cross-platform-reserved characters with "_", (3) strips leading dots so the
// result is never a hidden file or a "."/".." directory reference (defends the
// output tree against path traversal — a UID is attacker-influenced data from the
// remote account), and (4) falls back to "unknown" when nothing usable remains.
func sanitize(uid string) string {
	s := strings.TrimSpace(uid)

	r := strings.NewReplacer(
		"/", "_", "\\", "_", ":", "_",
		"*", "_", "?", "_", "\"", "_",
		"<", "_", ">", "_", "|", "_",
	)
	s = r.Replace(s)

	// Strip leading dots: prevents ".", "..", and hidden/dotfile names from a UID
	// like ".." or "...". A name that is only dots collapses to empty → "unknown".
	s = strings.TrimLeft(s, ".")
	s = strings.TrimSpace(s)

	if s == "" {
		return "unknown"
	}
	return s
}
