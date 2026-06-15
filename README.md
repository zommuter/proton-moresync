# proton-moresync

A standalone Go CLI that backs up your Proton contacts and calendar to a local,
git-versioned tree of standard files.

It plays the same role as `mbsync` for mail: fetch, decrypt, and write standard
`.vcf`/`.ics` files that any calendar/contacts client can read. Downstream
ingestion or sync is out of scope here.

## What it backs up

| Data type | Output | Status |
|-----------|--------|--------|
| Contacts  | `contacts/<uid>.vcf` (RFC 6350) | ✅ Phase 1 done |
| Calendar  | `calendar/<cal-id>/<uid>.ics` (RFC 5545) | ✅ Phase 1 done |

Proton-specific metadata (object IDs, ciphertext, version) is stored in
out-of-band sidecars under `.meta/` — the `.vcf`/`.ics` files are plain
RFC-compliant and readable by any standard client.

## Build

```sh
go build -o backup ./cmd/backup
```

Requires Go 1.21+. No CGO.

## First-run authentication

Proton uses SRP + 2FA + a browser-based CAPTCHA on first login from a new
client. After that, the session is persisted in the OS keyring and subsequent
runs are fully unattended.

```sh
# First time only — will open a browser window for CAPTCHA, prompt for
# your mailbox password, and store everything in the keyring.
PROTON_USER=you@proton.me ./backup --output-dir ~/proton-backup
```

After the first run, just invoke `./backup --output-dir ~/proton-backup` (or
the systemd timer) — no prompts.

## Output layout

```
~/proton-backup/
  contacts/<uid>.vcf
  calendar/<cal-id>/<uid>.ics
  .meta/contacts/<uid>.json        # Proton object ID + raw card data
  .meta/calendar/<cal-id>/<uid>.json
```

The backup tree is its own git repo. Re-running is idempotent: only real
changes produce new commits.

## Scheduled daily backups

A systemd user timer is included under `systemd/`. Install with:

```sh
make install
```

This copies the `.service` and `.timer` units to `~/.config/systemd/user/`,
installs the `EnvironmentFile` template to `~/.config/proton-moresync/env`
(edit it to set `PROTON_USER=you@proton.me`), and enables the timer.

Monitor with:

```sh
journalctl --user -u proton-backup.service -n 50
systemctl --user list-timers | grep proton-backup
```

See `docs/proton-backup-runbook.md` for the full one-time setup and a
coverage table for all Proton data types.

## Phases

| Phase | Status | Description |
|-------|--------|-------------|
| Spike | done | End-to-end decrypt verified (contacts + calendar). |
| 1 | done | Read-only backup — full fetch+decrypt to git-versioned tree. |
| 2 | in progress | Read-only live view via Radicale (CardDAV+CalDAV); collection adapter done. See [runbook](docs/proton-backup-runbook.md#phase-2--read-only-live-view-via-radicale-carddav--caldav). |
| 3 | north star | Two-way sync; gated on rehearsal round-trip against test account. |

## Dependencies

- [`github.com/ProtonMail/go-proton-api`](https://github.com/ProtonMail/go-proton-api) — official Proton API client (MIT)
- [`github.com/ProtonMail/gopenpgp/v2`](https://github.com/ProtonMail/gopenpgp) — OpenPGP crypto primitives (MIT)
- [`github.com/emersion/hydroxide`](https://github.com/emersion/hydroxide) — contact E2E decryption reference (MIT)

## License

MIT — see [LICENSE](LICENSE).
