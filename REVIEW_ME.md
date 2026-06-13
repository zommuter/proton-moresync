# Human review queue <!-- budget: 15 min -->

Judgment calls encoded in red tests — confirm or correct the interpretation.
Max ~10 open boxes; the reviewer prunes resolved ones each review turn.

- [x] cmd/backup/sanitize_test.go::TestSanitizeRejectsPathTraversal (roadmap:0ad0) —
  CONFIRMED by user 2026-06-13: **neutralize-but-keep** (replace dots/separators so the
  UID stays a usable filename), NOT reject-to-unknown and NOT hash. Current test
  interpretation stands.

- [x] cmd/backup/sanitize_test.go::TestSanitizeEmptyFallsBackToUnknown (roadmap:0ad0) —
  CONFIRMED by user 2026-06-13: dot-only / whitespace-only UIDs collapse to `unknown`
  (treated as empty, not preserved as a literal filename).
