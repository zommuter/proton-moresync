# Roadmap <!-- fables-turn roadmap v1 -->

Executor-facing task spec. Each item is sized for ONE Sonnet session. Items are
the single source of truth — TODO.md carries only a summary line. Executors tick
checkboxes; only the reviewer adds, removes, or re-scopes items.

Tests live in `cmd/backup/*_test.go` (package `main`). Run with `go test ./...`.
The `[ROUTINE]` items below already have FAILING tests committed (C3) — an executor
is done when its item's tests go green plus a refactor pass, nothing else.

## Items

- [x] Harden `sanitize` against path traversal and reserved names [ROUTINE] <!-- id:0ad0 --> — done 2026-06-13 (C5; green)
  - **Acceptance**: `sanitize` never returns a value that can escape the output
    tree or collide with a filesystem-special name. Specifically: a UID of `..`,
    `.`, or any value containing a path separator must not yield a usable parent-
    directory reference; leading/trailing dots and whitespace are stripped or
    replaced; the empty result still falls back to `"unknown"`. Existing safe UIDs
    (normal vCard/iCal UIDs) are returned essentially unchanged.
  - **Tests**: `cmd/backup/sanitize_test.go::TestSanitizeRejectsPathTraversal`,
    `::TestSanitizeStripsLeadingDots`, `::TestSanitizePreservesNormalUID`,
    `::TestSanitizeEmptyFallsBackToUnknown` (all marked `# roadmap:0ad0`) (currently RED)
  - **Done-check**: `go test ./cmd/backup/ -run TestSanitize -v`
  - **Context**: `cmd/backup/write.go` (`sanitize`). The current implementation only
    replaces a fixed set of reserved chars; `..` and leading-dot UIDs pass through.
    `sanitize` output is used as a filename component in `writeContact`/`writeEvent`.
    Keep the public behaviour (returns a string; `""` → `"unknown"`).

- [ ] Add a CI-grade `make test` target and document it [ROUTINE] <!-- id:3c7c -->
  - **Acceptance**: `make test` runs `go vet ./...` then `go test ./...` and exits
    non-zero if either fails. `make build` still works. The target is listed in
    `make help` if a help target exists, otherwise in the Makefile's `.PHONY` line
    and a one-line comment. README/CLAUDE.md already reference `go test ./...`; the
    Makefile target must match that command.
  - **Tests**: `cmd/backup/makefile_test.go::TestMakefileHasTestTarget` — a Go test
    that shells out to `grep` the repo `Makefile` for a `test:` target running
    `go test` (marked `# roadmap:3c7c`) (currently RED)
  - **Done-check**: `go test ./cmd/backup/ -run TestMakefileHasTestTarget -v && make test`
  - **Context**: `Makefile`. Add `test` to `.PHONY`. Keep it dependency-free
    (no golangci-lint install — vet only). The Go test locates the Makefile via
    `runtime.Caller` + walking up to the repo root, or via the `MORESYNC_ROOT`
    env var if set.

- [ ] Phase 2 design: read-only live view (Radicale / vdirsyncer / DAVx5) [HARD — strong model] <!-- id:d407 -->
  - **Why HARD**: open-ended architecture choice across three external tools with
    different trust/deployment models; needs a design meeting, not a code change.
  - **Acceptance**: a meeting note under `docs/meeting-notes/` that picks ONE
    serving path (or explicitly defers with a trigger), records rationale + rejected
    alternatives, and either spawns sized `[ROUTINE]` follow-ups or sets a reopen
    trigger. No production code before the decision.

- [ ] Phase 3 design: two-way sync rehearsal harness [HARD — strong model] <!-- id:56c9 -->
  - **Why HARD**: write-back can corrupt a live Proton account; the rehearsal
    round-trip protocol and the contacts-before-calendar ordering are safety-critical
    judgment calls.
  - **Acceptance**: a design note defining the test-account round-trip protocol
    (read → modify → write → read-back-identical) and its pass/fail gate, with the
    contacts-write-first ordering justified. No write path is implemented until the
    rehearsal harness exists and is gated green.

- [ ] QR-login / session-fork probe (adoption-triggered) [HARD — strong model] <!-- id:5cc5 -->
  - **Why HARD**: requires probing Proton's `auth/v4/sessions/forks/{selector}`
    behaviour with a borrowed/generic `ChildClientID` — undocumented API behaviour,
    must yield a documented yes/no before any code.
  - **Acceptance**: a documented yes/no on `ChildClientID` acceptance for an
    `Independent` fork, plus AES-GCM payload decrypt under `sk` → `keyPassword`
    verified or refuted. No production code before the probe result. Gated on a
    first external user (TODO id:96d7).

- [ ] Decryption-failure observability pass [HARD — strong model] <!-- id:0dfb -->
  - **Why HARD**: deciding what "acceptable skip rate" means and whether a partial
    backup should fail loudly is a policy judgment, not a mechanical change.
  - **Acceptance**: a design note + minimal implementation deciding how
    `backupContacts`/`backupCalendars` should surface skipped objects (currently
    just a count printed to stdout). Options to weigh: exit non-zero above a
    threshold, write a `.meta/skipped.json` manifest, or stay advisory. Decision
    recorded with rationale before any behaviour change.

## Regression coverage (no open item)

`cmd/backup/uid_test.go` and `cmd/backup/wrap_test.go` cover `vcardUID`, `icsUID`,
and `wrapVCalendar`, which were already correct at handoff. They are committed as
green regression tests, not as item specs. Tokens `5334` and `a077` were originally
earmarked for these and are now unused/free.

## Notes for executors

- Tests are `package main` in `cmd/backup/` — they can call unexported helpers
  directly. No build tags needed.
- Do NOT attempt to test network/crypto/keyring paths in unit tests; those need a
  live account and are covered by the `@manual` BDD scenarios in `features/`.
- After turning an item green, do a refactor pass (naming, dead code) and tick the
  checkbox here AND decrement the TODO.md summary count.
