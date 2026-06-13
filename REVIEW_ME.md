# Human review queue <!-- budget: 15 min -->

Judgment calls encoded in red tests — confirm or correct the interpretation.
Max ~10 open boxes; the reviewer prunes resolved ones each review turn.

- [ ] cmd/backup/sanitize_test.go::TestSanitizeRejectsPathTraversal (roadmap:0ad0) —
  ambiguity: how should `sanitize` neutralize a `..` / `.` / separator-bearing UID?
  The test encodes the interpretation "the result must not contain a path separator,
  must not start with `..`, and must not equal `.`/`..`" — i.e. neutralize-but-keep
  (e.g. replace dots/separators with `_`), NOT hash or reject. Confirm this is the
  desired safety behaviour vs. a stricter "reject and fall back to `unknown`" rule.

- [ ] cmd/backup/sanitize_test.go::TestSanitizeEmptyFallsBackToUnknown (roadmap:0ad0) —
  ambiguity: a UID of `"..."` (dots only) currently sanitizes to `"..."`. The test
  encodes "dot-only / whitespace-only UIDs collapse to `unknown`". Confirm dot-only
  should be treated as empty rather than preserved as a literal filename.
