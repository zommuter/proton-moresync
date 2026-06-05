# 2026-06-05 — Make proton-moresync GitHub-publishable

**Started:** 2026-06-05 10:59
**Session:** eb7711db-b097-4f37-935b-1730dfc1b88d
**Mode:** Class 2 planning record (no meeting was held — plan-mode output)
**Topic:** Audit and prepare the repo for public GitHub publishing.

## Context

Phase 1 is complete and the repo is working. The only remaining Class 2 TODO
was Phase 2/3 north-star design (deferred). The user requested a GitHub-publishing
pass instead: add missing publishing infra, close the one real security risk
(untracked real data in `tmp/`), and scrub personal infra topology from current
working-tree files.

## Plan

1. **Harden `.gitignore`** — add `/tmp/`, `session.json`, `*.env`, `.anon-map`.
2. **Create local anonymization map** (`.anon-map`, gitignored) — reversible map
   of machine names → placeholders, kept locally for reproducibility.
3. **Add MIT LICENSE** — consistent with all three dependencies (go-proton-api,
   gopenpgp/v2, hydroxide are all MIT).
4. **Scrub working-tree files** — replace machine names (`fievel`/`zomni`/
   `cartmanjaro`) and private-path refs (`~/src/zkm`, `~/src/claude-diary`,
   `~/src/zomni/mail`) with generic placeholders in CLAUDE.md, TODO.md,
   runbook, and four meeting notes. History kept as-is (user confirmed no
   severe data exposed).
5. **Genericize runner script** — replace the `~/src/claude-diary/git-lock-push.sh`
   private dependency with a plain `git push`; the script already has its own
   `flock` guard.
6. **Add README.md** — public-audience equivalent of CLAUDE.md: what it does,
   build, first-run auth, output layout, scheduling, phases, dependencies, license.

## Implementation findings

- All three key dependencies (go-proton-api, gopenpgp/v2, hydroxide) confirmed MIT.
- `tmp/` held 446 real decrypted `.vcf`/`.ics` files on disk but was NEVER committed
  (clean history). Risk closed by gitignore.
- No committed secrets, no real email addresses, no literal `/home/tobias`. Only
  SCRUB-level exposure (machine names, sibling-project names).
- `proton-backup-sync.sh` external lock hook was redundant (script already has its
  own `flock -n 9` guard at lines 20–21).
- `go build ./cmd/backup` clean after all edits.

## Decisions

- **History kept as-is** — no squash, no filter-repo. User confirmed nothing severe
  exposed in history; old machine names in historical commits are acceptable.
- **MIT license** — matches all dependencies, simplest for a Go CLI.
- **Scrub current working tree only** — machine names replaced with angle-bracket
  placeholders (`<backup-host>`, `<workstation>`, `<gateway>`).
- **Private runner dependency removed** — `git push origin main` replaces the
  claude-diary hook; self-contained for any user.
- **Anonymization map generalization deferred to zelegator** — `.anon-map` is a
  local artifact here; the reusable tool belongs in the zelegator project.
- **Remote not touched** — user adds `github` remote and pushes when ready
  (`git remote add github git@github.com:Zommuter/proton-moresync.git && git push github main`).

## Action items

- [ ] Add `github` remote and push to make the repo public. <!-- id:f8fb -->
