# proton-moresync

Standalone Go CLI that backs up Proton contacts and calendar to a local, git-versioned tree.
Plays the "mbsync role" — fetch + decrypt + emit standard files; ingestion into a downstream corpus tool is out of scope here.

## Architecture

- **Language:** Go, using `github.com/ProtonMail/go-proton-api` (official Bridge library) + hydroxide's `protonmail` package for contact decryption
- **Auth:** SRP + 2FA via go-proton-api
- **Calendar decryption:** TURNKEY via `GetCalendarPassphrase+Decrypt` → passphrase; `GetCalendarKeys+Unlock` → keyring; per-part inline decrypt (see `decryptPart` in `cmd/backup/calendar.go`)

## Output contract

```
contacts/<uid>.vcf                    # vanilla RFC 6350 — no Proton-specific extensions
calendar/<cal-id>/<uid>.ics           # vanilla RFC 5545
.meta/contacts/<uid>.json             # Proton contact sidecar
.meta/calendar/<cal-id>/<uid>.json    # Proton calendar event sidecar
```

- Canonical tree: `~/proton-backup` (its own git repo, pushed to `<backup-host>:src/proton-backup.git` daily)
- Runner script: `proton-backup-sync.sh` (repo root) fired by `systemd/proton-backup.timer`
- `.vcf` and `.ics` files must be parseable by any standard-compliant client (no Proton fields inline)
- `.meta/` sidecars are the only place Proton-specific data lives
  - contacts: `proton_id`, `cards` (raw encrypted/signed card data — `[]{Type,Data,Signature}`), `version`
  - events: `proton_id`, `calendar_id`, `shared_key_packet`, `calendar_key_packet`, `shared_events`, `calendar_events`, `version`
- UIDs are the standard vCard/iCal UID fields, used as filenames

## Phases

| Phase | Status | Description |
|-------|--------|-------------|
| Spike | **done** | Contact + calendar decrypt both TURNKEY. Calendar key-unwrap turnkey. Session persisted. |
| 1 | **done** | Read-only backup — full fetch+decrypt to versioned tree |
| 2 | deferred | Read-only live view via Radicale/vdirsyncer/DAVx5 |
| 3 | north star | Two-way sync; gated on rehearsal round-trip against test account; contacts-write before calendar-write |

## Key dependencies

- `github.com/ProtonMail/go-proton-api` — official Proton API client (auth, contacts, calendar endpoints)
- `github.com/emersion/hydroxide/protonmail` — reference for contact E2E decryption
- `github.com/ProtonMail/gopenpgp/v2` — E2E crypto primitives (v2, not v3)

## Scope

**In scope:** contacts + calendar only.
**Out of scope:** mail (covered by protonmail-bridge + mbsync), Drive (rclone), Pass (manual export), downstream corpus ingestion plugins.

## Spike findings (2026-06-04)

- **Contact decrypt:** `contact.Cards.Merge(kr)` is TURNKEY. Must iterate ALL `addrKRs` — contacts may be encrypted to any address key.
- **Calendar decrypt:** FULLY TURNKEY: `GetCalendarPassphrase+Decrypt` → passphrase; `GetCalendarKeys+Unlock` → keyring; per-part `Decrypt` → VEVENT. No hand-assembly needed.
- **`CalendarEventPart.Decode` bug:** value receiver discards decrypted data — must inline decrypt logic (see `decryptPart` in `cmd/spike/main.go`).
- **CAPTCHA:** composite token `hvToken:prefix+hcaptchaResponse` required. Bare hvToken rejected. Session persistence (`NewClientWithRefresh`) makes CAPTCHA one-time-only. Phase 1 automation: chromedp.
- **Session:** persisted at `~/.local/state/proton-moresync/session.json` (0600). Subsequent runs skip auth entirely.

## Related projects

- Downstream corpus search tool — future `vcard` / `calendar` ingestion plugins will ingest this tree
- Backup runbook: `docs/proton-backup-runbook.md` — references this project + mbsync + rclone for full Proton coverage

## Founding meeting

`docs/meeting-notes/2026-05-29-1313-proton-moresync-scope-codereuse.md`
