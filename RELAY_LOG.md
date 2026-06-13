# Relay log <!-- merge=union; append-only — never edit or reorder past entries -->

## 2026-06-13 13:45 — reviewer (claude-opus-4-8, handoff)

Handoff turn on a Go CLI (Proton contacts + calendar backup) that was code-complete
through Phase 1 but had ZERO tests. C1: refreshed CLAUDE.md (added Commands/Testing/
Gotchas + the fables-executor contract v2 pointer) and wrote ARCHITECTURE.md with the
seven major decisions and their rejected alternatives (scope, output contract, keyring
auth, manual CAPTCHA, inline decryptPart, deferred QR-fork, phasing). C2: ROADMAP with
6 open items. C3: discovered that `vcardUID`/`icsUID`/`wrapVCalendar` were already
correct — their tests went green pre-implementation, so they are committed as
regression coverage (no open item, tokens 5334/a077 freed) rather than as ROUTINE
specs. Genuinely-red ROUTINE items: `sanitize` path-traversal hardening (id:0ad0) and
a `make test` target (id:3c7c). C4: `@manual` Gherkin (live Proton account needed, not
automatable) + 2 REVIEW_ME entries on the sanitize-hardening interpretation. C5:
executed id:0ad0 (sanitize hardening) full red→green→refactor — see below.
