# proton-moresync

Standalone Go CLI that backs up Proton contacts and calendar to a local, git-versioned tree.
Plays the "mbsync role" — fetch + decrypt + emit standard files; ingestion into a downstream corpus tool is out of scope here.

See `ARCHITECTURE.md` for design decisions (with rationale + rejected alternatives) and
`ROADMAP.md` for the executor task queue. `TODO.md` carries only a summary line.

## Commands

```sh
go build -o backup ./cmd/backup   # build the CLI
go test ./...                      # run the test suite (pure-logic helpers; no network)
go vet ./...                       # static checks
make build                         # = go build -o backup ./cmd/backup
make install                       # install systemd user units + env file
make enable                        # start the daily backup timer
```

The only built binary is `./backup` (from `cmd/backup`). `cmd/spike` is the original
decrypt proof-of-concept, kept for reference; it is not part of the shipped product.

## Testing

Tests are plain Go (`go test ./...`), stdlib + testify-free. They cover the pure-logic
helpers that need no Proton account, keyring, or network:

- `sanitize` (filename safety — path traversal, reserved chars, empty UID)
- `vcardUID` / `icsUID` (UID extraction with fallback)
- `wrapVCalendar` (VCALENDAR envelope idempotence + VEVENT wrapping)

Network/crypto/keyring paths (`connect`, `unlockKeys`, `backupContacts`,
`backupCalendars`, `secrets.go`) are NOT unit-tested — they require a live Proton
account and OS keyring. Verify those manually per `docs/proton-backup-runbook.md`
and the `@manual` scenarios in BDD (see `features/`).

## Gotchas (hard-won; do not rediscover)

- **`appVersion` must be `Other_<SemVer>`** (`cmd/backup/auth.go`). The go-proton-api
  default `"go"` is rejected by Proton's API; `Other` renders as "unknown" in Proton
  security notifications — a Proton-side limitation, not a client bug.
- **`CalendarEventPart.Decode` is broken upstream** (value receiver discards the
  decrypted data). Always use the local `decryptPart` in `cmd/backup/calendar.go`.
- **Contact decryption needs ALL address keyrings**, not just the primary — a contact
  may be encrypted to any address key. `contactKR` combines the user key (decrypt) +
  every address key (verify); see `main.go`.
- **`gopenpgp/v2`, not v3** — the Proton fork pins v2 APIs; do not bump to v3.
- **resty replace directive** in `go.mod` points at `github.com/ProtonMail/resty/v2`;
  removing it breaks the build.
- **Session 9101 (locked scope)** after refresh is handled by purge + fresh-login retry
  in `main.go`; do not treat it as fatal.
- **OS / tooling**: Manjaro — `pamac`, never `pacman -S`. Go toolchain is system-wide
  (no `uv`; `uv` is for Python repos only). ISO-8601 dates, 24h, SI, de_CH context.

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
.meta/skipped.json                    # partial-backup manifest (present only when a run skipped objects)
```

- Canonical tree: `~/proton-backup` (its own git repo, pushed to `<backup-host>:src/proton-backup.git` daily)
- Runner script: `proton-backup-sync.sh` (repo root) fired by `systemd/proton-backup.timer`
- `.vcf` and `.ics` files must be parseable by any standard-compliant client (no Proton fields inline)
- `.meta/` sidecars are the only place Proton-specific data lives
  - contacts: `proton_id`, `cards` (raw encrypted/signed card data — `[]{Type,Data,Signature}`), `version`
  - events: `proton_id`, `calendar_id`, `shared_key_packet`, `calendar_key_packet`, `shared_events`, `calendar_events`, `version`
- UIDs are the standard vCard/iCal UID fields, used as filenames
- Skips (decrypt/verify failures) are non-fatal and recorded in `.meta/skipped.json`
  (`{total, entries:[{kind,id,reason}]}`), cleared on a clean run. `--max-skip-rate F`
  (default 1.0 = advisory) makes a run exit non-zero when the skip fraction exceeds F
  (see `skiplog.go`, id:0dfb)

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

## Relay contract <!-- fables-executor contract v2 -->

This repo is managed by a reviewer/executor relay. Load the `fables-executor` skill
(`/fables-executor`) before working on any item, then follow its rules exactly.
