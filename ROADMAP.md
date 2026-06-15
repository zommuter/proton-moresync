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

- [x] Add a CI-grade `make test` target and document it [ROUTINE] <!-- id:3c7c --> — done 2026-06-13 (C5; green)
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

- [x] Phase 2 design: read-only live view (Radicale / vdirsyncer / DAVx5) [HARD — strong model] <!-- id:d407 --> — done 2026-06-15 (design; Radicale picked, vdirsyncer deferred to P3, DAVx5 = consumer client; spawned id:6aad)
  - **Why HARD**: open-ended architecture choice across three external tools with
    different trust/deployment models; needs a design meeting, not a code change.
  - **Acceptance**: a meeting note under `docs/meeting-notes/` that picks ONE
    serving path (or explicitly defers with a trigger), records rationale + rejected
    alternatives, and either spawns sized `[ROUTINE]` follow-ups or sets a reopen
    trigger. No production code before the decision.
  - **Decision**: `docs/meeting-notes/2026-06-15-1623-phase2-readonly-live-view.md` —
    serving path = **Radicale** (CalDAV+CardDAV, read-from-disk); vdirsyncer is a
    sync *client*, reclassified to Phase 3; DAVx5/Thunderbird/Apple are consumer
    clients (no decision). Canonical `.vcf`/`.ics` tree stays untouched. Spawned the
    sized ROUTINE id:6aad below.

- [x] Phase 2 Radicale collection adapter + config + runbook [ROUTINE] <!-- id:6aad --> — done 2026-06-15 (C5; green)
  - **Acceptance**: a generator (Go in `cmd/backup` or a sibling, taking the backup
    root + a Radicale collection root) materialises a **separate** Radicale collection
    root that *references* the backup tree — a single address-book collection wrapping
    `contacts/*.vcf`, and one calendar collection per `calendar/<cal-id>/` — emitting
    the per-collection `.Radicale.props` markers (UUID + component-type: VADDRESSBOOK
    vs VCALENDAR) **into the collection root only, NEVER into the git-versioned backup
    tree**. Ship a documented read-only-localhost `radicale.conf` snippet and a runbook
    section. Hard gates: (a) the canonical backup tree is byte-unchanged after running
    the generator; (b) no network port is opened and no daemon is auto-started by the
    backup timer / `proton-backup.service`; (c) the DAV server, if run, lives in its
    own service (e.g. `proton-moresync-dav.service`) or a manual invocation, never
    chained off the backup unit.
  - **Tests**: a Go test (`# roadmap:6aad`) over a temp backup tree asserting: the
    generated collection root contains one address-book collection + one calendar
    collection per calendar dir; each carries a `.Radicale.props` with the correct
    component type; the original `contacts/` and `calendar/` files are unmodified
    (mtime/content) by the generator. Pure offline — no live account, no network, no
    crypto. (currently RED — no generator exists yet)
  - **Done-check**: `go test ./cmd/backup/ -run TestRadicale -v`
  - **Context**: `docs/meeting-notes/2026-06-15-1623-phase2-readonly-live-view.md`
    carries the full rationale and the rejected alternatives (vdirsyncer-as-server,
    bare static HTTP, embedding a DAV server in the binary, re-laying-out the canonical
    tree). Reference (symlink or copy-on-change) the backup files from the collection
    root; do not duplicate or rewrite them. Read-only only — no write-back (that is
    Phase 3, gated on the rehearsal harness, id:56c9).

- [x] Phase 3 design: two-way sync rehearsal harness [HARD — strong model] <!-- id:56c9 --> — done 2026-06-15 (design; 5-step round-trip protocol, semantic baseline-(b) gate + allow-changed list + decrypt/verify clause, re-encrypt-not-replay, throwaway account under distinct keyring service, contacts-before-calendar ordering justified by key-model complexity, modify-only scope fence; write path build-gated on a throwaway-account reopen trigger)
  - **Why HARD**: write-back can corrupt a live Proton account; the rehearsal
    round-trip protocol and the contacts-before-calendar ordering are safety-critical
    judgment calls.
  - **Acceptance**: a design note defining the test-account round-trip protocol
    (read → modify → write → read-back-identical) and its pass/fail gate, with the
    contacts-write-first ordering justified. No write path is implemented until the
    rehearsal harness exists and is gated green.
  - **Decision**: `docs/meeting-notes/2026-06-15-1707-phase3-rehearsal-harness.md` —
    5-step per-object round-trip (backup→modify-one-field→write-back-via-re-encryption→
    re-backup→assert); pass/fail = semantic field-set equality on the object under test
    (modulo a reviewed allow-changed list for server bookkeeping) + byte-identical
    bystander sidecars + re-fetched object still decrypts & verifies; write step must
    exercise the encrypt/sign path, never replay stored ciphertext; throwaway account
    under a distinct `proton-moresync-rehearsal` keyring service, live round-trip
    `@manual`, gate logic offline-testable; contacts-write-first justified by key-model
    complexity; modify-only scope (create/delete fenced out). Write path build-gated —
    reopen when a disposable test account exists and Phase 3 is wanted; no ids minted yet.

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
