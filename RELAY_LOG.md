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

## 2026-06-13 — executor (sonnet)

Worked id:3c7c — added `test:` target to Makefile running `go vet ./...` then `go test ./...`; declared `.PHONY`; `TestMakefileHasTestTarget` went red→green; full `go test ./...` stays green; ticked checkbox in ROADMAP.md and decremented TODO.md ROADMAP count 5→4.
Friction: none — well-scoped item, straightforward Makefile addition.

## 2026-06-13 15:29 — executor (sonnet, relay-loop)

executor: id:3c7c make test target — go vet + go test, all tests green

## 2026-06-14 09:42 — reviewer (claude-opus-4-8, relay-loop)

review 20260614-0942: 3 docs commits since fable-ckpt-1529 audited CLEAN — e67fde5 (WIP overstatement) → 1137ee5 (correct: Phase 1 done+verified, @verified annotation on unattended-re-run scenario in backup.feature) → 52b3c1a (drop CLAUDE.md Status block). Net diff = only features/backup.feature @manual→@manual @verified. User's 2026-06-14 memory feedback (feedback-relay-findings-flow-back) explicitly endorses BOTH 1137ee5 (@verified flow-back) and 52b3c1a (Status-block revert) by commit hash — this lands exactly where directed, not gaming. No test files touched/deleted; go test + make test green; contract pointer v2 in sync; README Phase-1-done consistent. ROADMAP unchanged: 2 ROUTINE closed, 4 HARD open (d407/56c9/5cc5/0dfb). routine_open=0.

## 2026-06-14 09:47 — reviewer (claude-opus-4-8, fable-standin, relay-loop)

review 20260614-0942: 3 docs commits since fable-ckpt-1529 audited clean (Status add+revert + @verified flow-back), user-endorsed by hash, no test changes, go test + make test green, contract v2 in sync, routine_open=0

## 2026-06-15 16:27 — strong-execute (claude-opus-4-8, fable-standin, relay-loop)

relay(hard): C5 d407 — Phase 2 read-only live-view design picks Radicale (vdirsyncer→P3, DAVx5=client); spawned ROUTINE id:6aad
