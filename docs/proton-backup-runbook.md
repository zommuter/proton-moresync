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

Canonical backup tree: `~/proton-backup` (its own git repo, pushed to `fievel:src/proton-backup.git`).

**Daily scheduled run (normal path):** handled by the systemd timer — no manual steps needed.

**Manual run:**
```bash
~/src/proton-moresync/proton-backup-sync.sh
```

**First run / fresh machine:** run the binary directly once so it can prompt for credentials +
mailbox password (and CAPTCHA if the session is cold). Subsequent runs are unattended.
```bash
cd ~/src/proton-moresync && go build -o backup ./cmd/backup
PROTON_USER=you@proton.me ./backup --output-dir ~/proton-backup
```

Output tree (git-versioned at `~/proton-backup`):
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

`proton-moresync` follows the **mbsync-hook pattern**: runner script in repo, fired by a
systemd user timer, output captured in journald.

### One-time setup

```sh
# a) Create bare repo on fievel (mirrors ~/mail → fievel:src/mail.git)
ssh fievel git init --bare src/proton-backup.git

# b) Initialise the local versioned tree
git init ~/proton-backup
git -C ~/proton-backup remote add origin fievel:src/proton-backup.git
printf '.proton-backup-sync.lock\n' > ~/proton-backup/.gitignore
git -C ~/proton-backup add .gitignore
git -C ~/proton-backup commit -m "init"
git -C ~/proton-backup push -u origin main

# c) Seed keyring — one interactive run (prompts for password + CAPTCHA once)
cd ~/src/proton-moresync && go build -o backup ./cmd/backup
PROTON_USER=you@proton.me ./backup --output-dir ~/proton-backup
git -C ~/proton-backup add -A
git -C ~/proton-backup commit -m "initial proton backup"
git -C ~/proton-backup push

# d) Install and enable the timer
#    Edit PROTON_USER in the .service file first.
cp ~/src/proton-moresync/systemd/proton-backup.{service,timer} ~/.config/systemd/user/
#    (open ~/.config/systemd/user/proton-backup.service and set PROTON_USER=you@proton.me)
systemctl --user daemon-reload
systemctl --user enable --now proton-backup.timer
```

### Monitoring

```sh
# See last run logs
journalctl --user -u proton-backup.service -n 50

# Check next scheduled fire
systemctl --user list-timers | grep proton-backup

# Run immediately (bypass timer)
systemctl --user start proton-backup.service
```

## Recovery

To reconstruct contacts/calendar from scratch: delete the output tree and re-run `./backup`.
The backup is idempotent; re-running over an existing tree is safe (git will show only real diffs).

## Related projects

- `~/src/zkm` — corpus search; `zkm-vcard` / `zkm-calendar` plugins (V-prefix/C-prefix in zkm's TODO) will ingest this tree
- `rclone` — Drive backup
