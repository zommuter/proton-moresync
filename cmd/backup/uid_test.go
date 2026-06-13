// regression coverage — no open roadmap item (vcardUID/icsUID already correct as of
// handoff 2026-06-13; these tests guard against regressions during refactors).
package main

import (
	"testing"

	vcard "github.com/emersion/go-vcard"
)

// TestVcardUIDPresent — when the card carries a non-empty UID field, it is returned.
func TestVcardUIDPresent(t *testing.T) {
	c := vcard.Card{vcard.FieldUID: []*vcard.Field{{Value: "card-uid-123"}}}
	if got := vcardUID(c, "fallback"); got != "card-uid-123" {
		t.Errorf("vcardUID = %q, want %q", got, "card-uid-123")
	}
}

// TestVcardUIDFallback — empty or missing UID field falls back.
func TestVcardUIDFallback(t *testing.T) {
	if got := vcardUID(vcard.Card{}, "fb"); got != "fb" {
		t.Errorf("vcardUID(empty) = %q, want %q", got, "fb")
	}
	c := vcard.Card{vcard.FieldUID: []*vcard.Field{{Value: ""}}}
	if got := vcardUID(c, "fb"); got != "fb" {
		t.Errorf("vcardUID(blank) = %q, want %q", got, "fb")
	}
}

// TestICSUIDFromContent — the value of the first UID: line is returned.
func TestICSUIDFromContent(t *testing.T) {
	content := "BEGIN:VEVENT\nUID:event-abc\nSUMMARY:x\nEND:VEVENT\n"
	if got := icsUID(content, "fb"); got != "event-abc" {
		t.Errorf("icsUID = %q, want %q", got, "event-abc")
	}
}

// TestICSUIDCRLFTolerant — a UID line terminated by CRLF must be returned WITHOUT the
// trailing carriage return. (JUDGMENT CALL — see REVIEW_ME.md.)
func TestICSUIDCRLFTolerant(t *testing.T) {
	content := "BEGIN:VEVENT\r\nUID:event-crlf\r\nEND:VEVENT\r\n"
	if got := icsUID(content, "fb"); got != "event-crlf" {
		t.Errorf("icsUID(crlf) = %q, want %q (trailing CR must be stripped)", got, "event-crlf")
	}
}

// TestICSUIDFallback — content with no UID line falls back.
func TestICSUIDFallback(t *testing.T) {
	if got := icsUID("BEGIN:VEVENT\nSUMMARY:x\nEND:VEVENT\n", "fb"); got != "fb" {
		t.Errorf("icsUID(no-uid) = %q, want %q", got, "fb")
	}
}
