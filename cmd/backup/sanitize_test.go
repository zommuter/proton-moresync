// roadmap:0ad0
package main

import (
	"strings"
	"testing"
)

// TestSanitizeRejectsPathTraversal — a UID of ".." or "." (or any value that, after
// sanitizing, equals a parent/current-dir reference) must not be returned as-is, so it
// cannot escape or alias the output tree when used as a filename component.
func TestSanitizeRejectsPathTraversal(t *testing.T) {
	for _, in := range []string{"..", ".", "../..", "../etc/passwd"} {
		got := sanitize(in)
		if got == "." || got == ".." {
			t.Errorf("sanitize(%q) = %q — must not be a dir-reference", in, got)
		}
		if strings.Contains(got, "/") || strings.Contains(got, "\\") {
			t.Errorf("sanitize(%q) = %q — must not contain a path separator", in, got)
		}
		// Must not be able to walk up: no leading ".." segment.
		if strings.HasPrefix(got, "..") {
			t.Errorf("sanitize(%q) = %q — must not start with ..", in, got)
		}
	}
}

// TestSanitizeStripsLeadingDots — leading dots and surrounding whitespace must be
// stripped/replaced so the result is not a hidden file or padded with spaces.
func TestSanitizeStripsLeadingDots(t *testing.T) {
	got := sanitize("  .hidden  ")
	if strings.HasPrefix(got, ".") {
		t.Errorf("sanitize(%q) = %q — must not start with a dot", "  .hidden  ", got)
	}
	if got != strings.TrimSpace(got) {
		t.Errorf("sanitize(%q) = %q — must not have leading/trailing whitespace", "  .hidden  ", got)
	}
}

// TestSanitizePreservesNormalUID — ordinary vCard/iCal UIDs round-trip essentially
// unchanged (no spurious mangling of safe characters).
func TestSanitizePreservesNormalUID(t *testing.T) {
	in := "proton-1a2b3c4d@proton.me"
	got := sanitize(in)
	if got != in {
		t.Errorf("sanitize(%q) = %q — normal UID must be preserved", in, got)
	}
}

// TestSanitizeEmptyFallsBackToUnknown — empty (or whitespace/dot-only collapsing to
// empty) UID falls back to "unknown".
func TestSanitizeEmptyFallsBackToUnknown(t *testing.T) {
	for _, in := range []string{"", "   ", "..."} {
		if got := sanitize(in); got != "unknown" {
			t.Errorf("sanitize(%q) = %q — want %q", in, got, "unknown")
		}
	}
}
