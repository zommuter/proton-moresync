# 2026-06-04 — Scheduling/trigger runner for proton-moresync

**Started:** 2026-06-04 14:07
**Session:** dbc0831e-02bb-4bf6-85d3-97916c05757e
**Mode:** Class 2 planning record (no meeting was held — plan-mode output)
**Topic:** Design and implement the mbsync-style scheduling runner for daily unattended Proton backup.

## Context

TODO item `id:8af0` (founding meeting `2026-05-29-1313-proton-moresync-scope-codereuse.md`) called
for "mbsync-hook pattern — script in repo, symlink into `.git/hooks` or systemd timer,
journald-queryable." Phase 1 backup is complete and runs unattended (keyring, 9101 auto-retry).
The only gap is a wired-up scheduler so daily backups run without manual invocation.

## Plan

**Approach selected:** mirror the existing `~/mail` backup pattern exactly.

- Reference: `~/src/zomni/mail/mail-sync.sh` + `~/.config/systemd/user/mbsync.{service,timer}`.
- `~/mail` pushes to `fievel:src/mail.git`; proton-backup follows the same model
  (`fievel:src/proton-backup.git`).
- The canonical versioned tree is `~/proton-backup` — a *separate* git repo, not in the
  public code repo (which holds decrypted personal data).
- cron was rejected: no session D-Bus → keyring locked → unattended run fails.
- `.git/hooks` symlink was rejected: a backup is the *producer* of commits; no upstream hook
  point exists.
- systemd user timer wins: lingering enabled, gnome-keyring already unlocked at login,
  `Persistent=true` catches missed runs.

## Implementation findings

- `~/src/proton-moresync/proton-backup-sync.sh` written (mirrors `mail-sync.sh`): flock → go
  build (incremental) → `./backup --output-dir ~/proton-backup </dev/null` → git add+commit →
  push if ssh-add reports a loaded key.
- `systemd/proton-backup.service` + `systemd/proton-backup.timer` written to `systemd/` in repo
  (versioned; install = copy + enable).
- `docs/proton-backup-runbook.md` updated with full one-time setup + monitoring commands.
- `CLAUDE.md` updated with canonical tree path and runner reference.

## Decisions

- **Canonical tree = `~/proton-backup`** (dedicated git repo, separate from public code repo).
  Decrypted personal data must not land in the code repo. *Out of scope: encryption-at-rest on
  remote; fievel is trusted private storage, same model as ~/mail.*
- **Trigger = systemd user timer** (`OnCalendar=daily`, `RandomizedDelaySec=1h`, `Persistent=true`).
  *Out of scope: cron (no D-Bus/keyring), git hooks (no hook point), hourly/weekly cadence.*
- **Runner commits locally + pushes to `fievel:src/proton-backup.git`** when SSH key is loaded.
  Push failure is silent (retry next run). *Out of scope: push encryption, encrypted remote.*
- **`PROTON_USER` set in `.service` unit** (session reuse + keyring cover the daily happy path;
  env var only used on fresh login after expiry). *Out of scope: passing credentials any other way.*
- **Units shipped in-repo under `systemd/`** (not written directly to `~/.config/systemd/user/`);
  install = `cp + systemctl enable`.

## Action items

- [ ] One-time setup: `ssh fievel git init --bare src/proton-backup.git`; `git init ~/proton-backup`; seed keyring with one interactive run; push; install + enable timer. — see `docs/proton-backup-runbook.md § Scheduling` for exact commands. <!-- id:4aae -->
