// roadmap:6aad
package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// buildFakeBackupTree creates a minimal backup tree under root:
//
//	contacts/a.vcf, contacts/b.vcf
//	calendar/cal1/ev1.ics, calendar/cal1/ev2.ics
//	calendar/cal2/ev3.ics
//
// It returns the contact mtimes and calendar file mtimes so the caller can
// assert they are unmodified after the generator runs.
func buildFakeBackupTree(t *testing.T, root string) {
	t.Helper()
	for _, f := range []string{
		"contacts/a.vcf",
		"contacts/b.vcf",
		"calendar/cal1/ev1.ics",
		"calendar/cal1/ev2.ics",
		"calendar/cal2/ev3.ics",
	} {
		p := filepath.Join(root, f)
		if err := os.MkdirAll(filepath.Dir(p), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte("dummy"), 0644); err != nil {
			t.Fatal(err)
		}
	}
}

// fileMtimes returns a map of relative path → mtime for every file under dir.
func fileMtimes(t *testing.T, dir string) map[string]int64 {
	t.Helper()
	m := make(map[string]int64)
	err := filepath.Walk(dir, func(p string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return err
		}
		rel, _ := filepath.Rel(dir, p)
		m[rel] = info.ModTime().UnixNano()
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return m
}

// fileContents returns a map of relative path → content for every file under dir.
func fileContents(t *testing.T, dir string) map[string]string {
	t.Helper()
	m := make(map[string]string)
	err := filepath.Walk(dir, func(p string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return err
		}
		data, rerr := os.ReadFile(p)
		if rerr != nil {
			return rerr
		}
		rel, _ := filepath.Rel(dir, p)
		m[rel] = string(data)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return m
}

// radicaleProps represents the JSON content of a .Radicale.props file.
type radicaleProps struct {
	Tag string `json:"tag"`
}

// TestRadicaleGeneratorAddressBook asserts that the generator emits a single
// address-book collection for contacts/ with component type VADDRESSBOOK.
func TestRadicaleGeneratorAddressBook(t *testing.T) {
	backupRoot := t.TempDir()
	collRoot := t.TempDir()
	buildFakeBackupTree(t, backupRoot)

	if err := generateRadicaleCollections(backupRoot, collRoot); err != nil {
		t.Fatalf("generateRadicaleCollections: %v", err)
	}

	// There should be exactly one address-book collection directory.
	propsPath := filepath.Join(collRoot, "contacts", ".Radicale.props")
	data, err := os.ReadFile(propsPath)
	if err != nil {
		t.Fatalf("missing .Radicale.props for contacts collection at %s: %v", propsPath, err)
	}

	var props radicaleProps
	if err := json.Unmarshal(data, &props); err != nil {
		t.Fatalf("cannot parse .Radicale.props: %v", err)
	}
	if props.Tag != "VADDRESSBOOK" {
		t.Errorf(".Radicale.props tag = %q, want VADDRESSBOOK", props.Tag)
	}
}

// TestRadicaleGeneratorCalendarCollections asserts that the generator emits one
// calendar collection per calendar/<cal-id>/ directory, each tagged VCALENDAR.
func TestRadicaleGeneratorCalendarCollections(t *testing.T) {
	backupRoot := t.TempDir()
	collRoot := t.TempDir()
	buildFakeBackupTree(t, backupRoot)

	if err := generateRadicaleCollections(backupRoot, collRoot); err != nil {
		t.Fatalf("generateRadicaleCollections: %v", err)
	}

	// Expect one calendar collection per calendar subdirectory.
	calDirs, err := os.ReadDir(filepath.Join(backupRoot, "calendar"))
	if err != nil {
		t.Fatalf("reading calendar dir: %v", err)
	}

	for _, calDir := range calDirs {
		if !calDir.IsDir() {
			continue
		}
		propsPath := filepath.Join(collRoot, "calendar", calDir.Name(), ".Radicale.props")
		data, err := os.ReadFile(propsPath)
		if err != nil {
			t.Errorf("missing .Radicale.props for calendar %q at %s: %v", calDir.Name(), propsPath, err)
			continue
		}
		var props radicaleProps
		if err := json.Unmarshal(data, &props); err != nil {
			t.Errorf("cannot parse .Radicale.props for calendar %q: %v", calDir.Name(), err)
			continue
		}
		if props.Tag != "VCALENDAR" {
			t.Errorf("calendar %q: .Radicale.props tag = %q, want VCALENDAR", calDir.Name(), props.Tag)
		}
	}

	if len(calDirs) == 0 {
		t.Error("no calendar directories found in backup tree — test setup broken")
	}
}

// TestRadicaleGeneratorBackupTreeUnchanged asserts that running the generator
// leaves every file in the canonical backup tree byte-unchanged (same content,
// same mtime — the generator must not touch the backup tree at all).
func TestRadicaleGeneratorBackupTreeUnchanged(t *testing.T) {
	backupRoot := t.TempDir()
	collRoot := t.TempDir()
	buildFakeBackupTree(t, backupRoot)

	beforeMtimes := fileMtimes(t, backupRoot)
	beforeContents := fileContents(t, backupRoot)

	if err := generateRadicaleCollections(backupRoot, collRoot); err != nil {
		t.Fatalf("generateRadicaleCollections: %v", err)
	}

	afterMtimes := fileMtimes(t, backupRoot)
	afterContents := fileContents(t, backupRoot)

	for rel, mtime := range beforeMtimes {
		if afterMtimes[rel] != mtime {
			t.Errorf("mtime changed for %s: %d → %d", rel, mtime, afterMtimes[rel])
		}
	}
	for rel, content := range beforeContents {
		if afterContents[rel] != content {
			t.Errorf("content changed for %s", rel)
		}
	}
	// No new files should have appeared in the backup root.
	for rel := range afterContents {
		if _, ok := beforeContents[rel]; !ok {
			t.Errorf("unexpected new file in backup tree: %s", rel)
		}
	}
}

// TestRadicaleGeneratorCollectionRootSeparate asserts that the .Radicale.props
// markers are written to the collection root only, NEVER to the backup tree.
func TestRadicaleGeneratorCollectionRootSeparate(t *testing.T) {
	backupRoot := t.TempDir()
	collRoot := t.TempDir()
	buildFakeBackupTree(t, backupRoot)

	if err := generateRadicaleCollections(backupRoot, collRoot); err != nil {
		t.Fatalf("generateRadicaleCollections: %v", err)
	}

	// Walk the backup tree: no .Radicale.props should exist there.
	_ = filepath.Walk(backupRoot, func(p string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return err
		}
		if strings.HasSuffix(p, ".Radicale.props") {
			t.Errorf(".Radicale.props found inside backup tree at %s — must live in collection root only", p)
		}
		return nil
	})
}
