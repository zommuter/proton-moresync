# Roadmap <!-- fables-turn roadmap v1 -->

Executor-facing task spec. Each item is sized for ONE Sonnet session. Items are
the single source of truth — TODO.md carries only a summary line. Executors tick
checkboxes; only the reviewer adds, removes, or re-scopes items.

Tests live in `cmd/backup/*_test.go` (package `main`). Run with `go test ./...`.
The `[ROUTINE]` items below already have FAILING tests committed (C3) — an executor
is done when its item's tests go green plus a refactor pass, nothing else.

## Items

- [ ] [INPUT — decision] QR-login / session-fork probe (adoption-triggered) — 🚧 GATED (auto, id:3801; route:human): Adoption/human-gated: needs live Proton forks API probe (borrowed ChildClientID), gated on first external user (TODO id:96d7); not pool/executor-dispatchable (id:2d20). — needs /relay human <!-- id:5cc5 -->
  - **Why HARD**: requires probing Proton's `auth/v4/sessions/forks/{selector}`
    behaviour with a borrowed/generic `ChildClientID` — undocumented API behaviour,
    must yield a documented yes/no before any code.
  - **Acceptance**: a documented yes/no on `ChildClientID` acceptance for an
    `Independent` fork, plus AES-GCM payload decrypt under `sk` → `keyPassword`
    verified or refuted. No production code before the probe result. Gated on a
    first external user (TODO id:96d7).

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
