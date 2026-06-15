# Proton Backup Runbook

Full coverage of all Proton data types and which tool/process backs each up.
This repo owns **contacts + calendar only**; all other types are covered elsewhere.

## Data types and coverage

| Type | Tool | Status | Notes |
|------|------|--------|-------|
| **Mail** | `protonmail-bridge` + `mbsync` | ✅ covered | mbsync pulls into maildir; downstream corpus tool ingests |
| **Contacts** | `proton-moresync` (this repo) | ✅ Phase 1 done | `contacts/<uid>.vcf` + `.meta/contacts/<uid>.json` |
| **Calendar** | `proton-moresync` (this repo) | ✅ Phase 1 done | `calendar/<cal-id>/<uid>.ics` + `.meta/calendar/<cal-id>/<uid>.json` |
| **Drive** | `rclone` (protondrive backend) | ⚠️ beta/unmaintained | Use rclone if available; not a hard dependency |
| **Pass** | Proton Pass manual JSON/CSV export | ✅ covered | Export from Proton Pass settings; store encrypted offline |
| **Notes** | Not covered | 🚫 deferred | No free API; manual export only if needed |
| **Wallet** | Not covered | 🚫 out of scope | Deferred indefinitely |

## Running a full backup

### 1. Contacts + Calendar (this repo)

Canonical backup tree: `~/proton-backup` (its own git repo, pushed to `<backup-host>:src/proton-backup.git`).

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
# ingest into your downstream corpus tool
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
# a) Create bare repo on <backup-host>
ssh <backup-host> git init --bare src/proton-backup.git

# b) Initialise the local versioned tree
git init ~/proton-backup
git -C ~/proton-backup remote add origin <backup-host>:src/proton-backup.git
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

## Phase 2 — read-only live view via Radicale (CardDAV + CalDAV)

Radicale is a minimal CalDAV+CardDAV server that can serve the backup tree
read-only to any DAV client (phone, Thunderbird, Apple Contacts / Calendar).
The backup timer and the DAV server are **completely independent**: a stopped or
crashed Radicale never affects backup runs, and vice versa.

### How it works

The `gen-radicale-collections` sub-command (see below) materialises a separate
**Radicale collection root** — a directory that Radicale will serve — by:

1. Creating a `contacts/` collection directory with a `.Radicale.props` marker
   (`"tag":"VADDRESSBOOK"`).
2. Creating a `calendar/<cal-id>/` collection directory per Proton calendar, each
   with a `.Radicale.props` marker (`"tag":"VCALENDAR"`).
3. Symlinking the canonical `.vcf` / `.ics` files into those collection directories.

The **canonical backup tree** (`~/proton-backup`) is **never modified**. The
`.Radicale.props` markers and symlinks live in a separate collection root
(e.g. `~/proton-radicale-coll`) and are never committed to git.

### 1. Generate the collection root

After each backup run (or on demand):

```bash
# Build the generator (included in the main binary as a sub-command).
cd ~/src/proton-moresync && go build -o backup ./cmd/backup

# Generate (or regenerate) the Radicale collection root.
# Re-running is safe and idempotent.
./backup gen-radicale-collections \
  --backup-dir ~/proton-backup \
  --collection-root ~/proton-radicale-coll
```

### 2. Install Radicale

```bash
# Install as a Python user package — no system packages, no sudo.
pip install --user radicale
# or: uv pip install radicale
```

### 3. radicale.conf — read-only localhost snippet

Save as `~/.config/radicale/config` (or pass with `--config`):

```ini
[server]
# Bind to loopback only — not exposed to the network.
hosts = 127.0.0.1:5232

[auth]
# No authentication — localhost-only, read access only.
type = none

[storage]
# Point at the generated collection root (NOT the git backup tree).
filesystem_folder = ~/proton-radicale-coll

[rights]
# Read-only access for all principals.
type = owner_only
```

> **Security note**: this config serves on localhost only with no auth.
> Do NOT change `hosts` to `0.0.0.0` without adding TLS + authentication.

### 4. Start Radicale manually

```bash
python -m radicale --config ~/.config/radicale/config
```

Radicale will print the server URL:
```
Radicale server starting — http://127.0.0.1:5232/
```

### 5. Configure your DAV client

| Client | Server URL |
|--------|-----------|
| DAVx5 (Android) | `http://127.0.0.1:5232/` (or LAN IP for phone access) |
| Thunderbird | `http://127.0.0.1:5232/contacts/` and `http://127.0.0.1:5232/calendar/<cal-id>/` |
| Apple Contacts / Calendar (macOS) | `http://127.0.0.1:5232/` |

Leave username/password blank (no auth configured).

### 6. Optional: long-running systemd user service

To keep Radicale running as a user service — **separate from** `proton-backup.service`:

```bash
# Save as ~/.config/systemd/user/proton-moresync-dav.service
cat > ~/.config/systemd/user/proton-moresync-dav.service <<'EOF'
[Unit]
Description=Radicale DAV server (proton-moresync read-only live view)
After=network.target

[Service]
ExecStart=%h/.local/bin/python -m radicale --config %h/.config/radicale/config
Restart=on-failure

[Install]
WantedBy=default.target
EOF

systemctl --user daemon-reload
systemctl --user enable --now proton-moresync-dav.service
journalctl --user -u proton-moresync-dav.service -f
```

The service is **opt-in** and is never chained off `proton-backup.service`.

### Constraints (enforced by id:6aad)

- The canonical backup tree (`~/proton-backup`) is byte-unchanged by the generator.
- `.Radicale.props` markers are written to the collection root only — never inside
  the git-versioned backup tree.
- No network port is opened and no daemon is auto-started by the backup timer or
  `proton-backup.service`.
- Radicale is read-only (no write-back path exists until Phase 3).

## Related projects

- Downstream corpus search tool — `vcard` / `calendar` ingestion plugins will ingest this tree
- `rclone` — Drive backup
