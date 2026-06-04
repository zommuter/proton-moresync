# Proton Backup Runbook

Full coverage of all Proton data types and which tool/process backs each up.
This repo owns **contacts + calendar only**; all other types are covered elsewhere.

## Data types and coverage

| Type | Tool | Status | Notes |
|------|------|--------|-------|
| **Mail** | `protonmail-bridge` + `mbsync` → `~/src/zkm` | ✅ covered | mbsync pulls into maildir; zkm-eml ingests |
| **Contacts** | `proton-moresync` (this repo) | ✅ Phase 1 done | `contacts/<uid>.vcf` + `.meta/contacts/<uid>.json` |
| **Calendar** | `proton-moresync` (this repo) | ✅ Phase 1 done | `calendar/<cal-id>/<uid>.ics` + `.meta/calendar/<cal-id>/<uid>.json` |
| **Drive** | `rclone` (protondrive backend) | ⚠️ beta/unmaintained | Use rclone if available; not a hard dependency |
| **Pass** | Proton Pass manual JSON/CSV export | ✅ covered | Export from Proton Pass settings; store encrypted offline |
| **Notes** | Not covered | 🚫 deferred | No free API; manual export only if needed |
| **Wallet** | Not covered | 🚫 out of scope | Deferred indefinitely |

## Running a full backup

### 1. Contacts + Calendar (this repo)

```bash
cd ~/src/proton-moresync
./backup  # or: go run ./cmd/backup
```

Session is persisted at `~/.local/state/proton-moresync/session.json` (0600).
First run will prompt for Proton credentials + mailbox password + CAPTCHA (browser opens).
Subsequent runs reuse the session — no CAPTCHA.

Output tree (git-versioned, typically at a path you configure):
```
contacts/<uid>.vcf
calendar/<cal-id>/<uid>.ics
.meta/contacts/<uid>.json
.meta/calendar/<cal-id>/<uid>.json
```

### 2. Mail

```bash
mbsync -a          # sync all configured IMAP sources
zkm convert zkm-eml  # ingest into zkm corpus
```

### 3. Drive

```bash
rclone sync protondrive: ~/backup/proton-drive/
```

### 4. Pass

Manual: Proton Pass → Settings → Export → JSON (encrypted) or CSV.
Store the export encrypted (e.g. `age -r <your-public-key> < pass-export.json > pass-export.json.age`).

## Scheduling

`proton-moresync` follows the mbsync-hook pattern (see TODO.md `id:8af0`):
script in repo, trigger via `.git/hooks` or systemd timer, output piped through `systemd-cat`.
This item is not yet implemented — see TODO.md for status.

## Recovery

To reconstruct contacts/calendar from scratch: delete the output tree and re-run `./backup`.
The backup is idempotent; re-running over an existing tree is safe (git will show only real diffs).

## Related projects

- `~/src/zkm` — corpus search; `zkm-vcard` / `zkm-calendar` plugins (V-prefix/C-prefix in zkm's TODO) will ingest this tree
- `rclone` — Drive backup
