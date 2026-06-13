// roadmap:3c7c
package main

import (
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"testing"
)

// repoRoot walks up from this test file (or MORESYNC_ROOT) until it finds go.mod.
func repoRoot(t *testing.T) string {
	t.Helper()
	if r := os.Getenv("MORESYNC_ROOT"); r != "" {
		return r
	}
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	dir := filepath.Dir(file)
	for i := 0; i < 8; i++ {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		dir = filepath.Dir(dir)
	}
	t.Fatal("could not locate repo root (go.mod)")
	return ""
}

// TestMakefileHasTestTarget — the Makefile must define a `test:` target that runs
// `go test`, so `make test` is the documented CI-grade entry point.
func TestMakefileHasTestTarget(t *testing.T) {
	data, err := os.ReadFile(filepath.Join(repoRoot(t), "Makefile"))
	if err != nil {
		t.Fatalf("read Makefile: %v", err)
	}
	mk := string(data)

	if !regexp.MustCompile(`(?m)^test:`).MatchString(mk) {
		t.Errorf("Makefile has no `test:` target")
	}
	// The test target's recipe must invoke `go test`.
	if !strings.Contains(mk, "go test") {
		t.Errorf("Makefile `test` target does not run `go test`")
	}
	if !regexp.MustCompile(`\.PHONY:.*\btest\b`).MatchString(mk) {
		t.Errorf("`test` is not declared .PHONY")
	}
}
