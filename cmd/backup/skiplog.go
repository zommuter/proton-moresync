package main

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// skipManifest is the structure written to .meta/skipped.json. It makes a
// partial backup auditable: instead of a count vanishing into stdout, every
// skipped object is recorded with its kind, Proton ID, and the failure reason
// (usually a decrypt/verify error). A downstream watcher (or a human) can diff
// it across runs to see whether a skip is transient or a persistent corruption.
type skipManifest struct {
	Total   int         `json:"total"`
	Entries []skipEntry `json:"entries"`
}

type skipEntry struct {
	Kind   string `json:"kind"`   // "contact" or "event"
	ID     string `json:"id"`     // Proton object ID
	Reason string `json:"reason"` // the error that caused the skip
}

// skipLog accumulates skipped objects during a backup run. The zero value is
// ready to use.
type skipLog struct {
	entries []skipEntry
}

func (s *skipLog) add(kind, id, reason string) {
	s.entries = append(s.entries, skipEntry{Kind: kind, ID: id, Reason: reason})
}

func (s *skipLog) count() int { return len(s.entries) }

// writeManifest writes .meta/skipped.json when anything was skipped, and removes
// a stale manifest from a prior run when the current run was clean — so the
// presence of the file is itself a reliable "this backup is partial" signal.
func (s *skipLog) writeManifest(outDir string) error {
	path := filepath.Join(outDir, ".meta", "skipped.json")
	if len(s.entries) == 0 {
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			return err
		}
		return nil
	}
	data, err := json.MarshalIndent(skipManifest{
		Total:   len(s.entries),
		Entries: s.entries,
	}, "", "  ")
	if err != nil {
		return err
	}
	return writeFile(path, append(data, '\n'))
}

// exceedsRate reports whether the skip fraction strictly exceeds max. max is the
// caller's "acceptable skip rate"; the default (1.0) is advisory and never trips,
// preserving Phase-1 behaviour. A run that attempted nothing (skipped+written==0)
// never trips — an empty account is not a failure.
func exceedsRate(skipped, written int, max float64) bool {
	attempted := skipped + written
	if attempted == 0 {
		return false
	}
	return float64(skipped)/float64(attempted) > max
}
