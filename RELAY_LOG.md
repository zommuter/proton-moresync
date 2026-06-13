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

## 2026-06-13 13:59 — reviewer (claude-opus-4-8, fable-down pilot)

Handoff C1–C5 (Opus pilot, Fable down): refreshed CLAUDE.md + new ARCHITECTURE.md (7 decisions), 6-item ROADMAP (2 ROUTINE/4 HARD), first test suite (red specs + regression coverage for already-correct UID/wrap helpers), @manual Gherkin (live-Proton CLI), 2 REVIEW_ME entries. C5 executed id:0ad0: sanitize path-traversal hardening red→green→refactor. go vet/build clean; only open item 3c7c (make test target) remains red.

## 2026-06-13 14:18 — reviewer (claude-opus-4-8, fable-standin, opus-pilot)

Re-review marker retrofit: the 2026-06-13-1359 handoff was Opus standing in for Fable (Fable outage). Prior tag label said 'fable-down pilot' which the id:9821 detection grep (fable-standin) misses. This checkpoint carries the literal marker so an independent Fable session re-reviews the full C1–C5 handoff. No code change. INDEPENDENT FABLE RE-REVIEW PENDING.

## 2026-06-13 15:12 — reviewer (claude-opus-4-8, fable-standin, relay-loop)

review 20260613-1450: 1 commit audited clean (id:0ad0 sanitize verified green, genuine impl), only red test is EXPECTED-RED 3c7c, contract pointer v2 in sync, REVIEW_ME pruned
