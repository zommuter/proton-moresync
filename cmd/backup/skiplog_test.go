// roadmap:0dfb — decryption-failure observability pass.
package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestSkipLogRecordsAndCounts(t *testing.T) {
	var sl skipLog
	if sl.count() != 0 {
		t.Fatalf("fresh skipLog count = %d, want 0", sl.count())
	}
	sl.add("contact", "AAA", "decrypt: bad key")
	sl.add("event", "BBB", "no SharedEvents decrypted")
	if sl.count() != 2 {
		t.Fatalf("count = %d, want 2", sl.count())
	}
}

func TestSkipLogWritesManifestWhenNonEmpty(t *testing.T) {
	dir := t.TempDir()
	var sl skipLog
	sl.add("contact", "AAA", "decrypt: bad key")

	if err := sl.writeManifest(dir); err != nil {
		t.Fatalf("writeManifest: %v", err)
	}
	path := filepath.Join(dir, ".meta", "skipped.json")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read manifest: %v", err)
	}
	var m skipManifest
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("unmarshal manifest: %v", err)
	}
	if m.Total != 1 || len(m.Entries) != 1 {
		t.Fatalf("manifest total=%d entries=%d, want 1/1", m.Total, len(m.Entries))
	}
	if m.Entries[0].Kind != "contact" || m.Entries[0].ID != "AAA" || m.Entries[0].Reason == "" {
		t.Fatalf("entry = %+v, want kind=contact id=AAA reason!=\"\"", m.Entries[0])
	}
}

// An empty skipLog must REMOVE any stale manifest from a prior run so a clean
// backup never leaves a misleading skipped.json behind.
func TestSkipLogEmptyRemovesStaleManifest(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".meta", "skipped.json")
	if err := writeFile(path, []byte(`{"total":3}`)); err != nil {
		t.Fatalf("seed stale manifest: %v", err)
	}
	var sl skipLog // empty
	if err := sl.writeManifest(dir); err != nil {
		t.Fatalf("writeManifest (empty): %v", err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("stale manifest still present (err=%v), want removed", err)
	}
}

// exceedsRate compares the skip fraction against a threshold. The default
// threshold (1.0) is advisory — it never trips, preserving Phase-1 behaviour.
func TestSkipLogExceedsRate(t *testing.T) {
	cases := []struct {
		skipped, written int
		max              float64
		want             bool
	}{
		{0, 10, 1.0, false},  // clean
		{10, 0, 1.0, false},  // advisory default never trips
		{1, 9, 0.05, true},   // 10% > 5%
		{1, 99, 0.05, false}, // 1% <= 5%
		{0, 0, 0.5, false},   // nothing attempted → never trips
		{5, 5, 0.5, false},   // exactly 50% == threshold, not strictly greater
	}
	for _, c := range cases {
		got := exceedsRate(c.skipped, c.written, c.max)
		if got != c.want {
			t.Errorf("exceedsRate(skipped=%d, written=%d, max=%g) = %v, want %v",
				c.skipped, c.written, c.max, got, c.want)
		}
	}
}
