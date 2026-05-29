# proton-moresync

Standalone Go CLI that backs up Proton contacts and calendar to a local, git-versioned tree.
Plays the "mbsync role" — fetch + decrypt + emit standard files; ingestion into zkm is downstream.

## Architecture

- **Language:** Go, using `github.com/ProtonMail/go-proton-api` (official Bridge library) + hydroxide's `protonmail` package for contact decryption
- **Auth:** SRP + 2FA via go-proton-api
- **Calendar decryption:** uncharted — the spike (`docs/meeting-notes/2026-05-29-1313-proton-moresync-scope-codereuse.md`) must prove this end-to-end before Phase 1 build

## Output contract

```
contacts/<uid>.vcf          # vanilla RFC 6350 — no Proton-specific extensions
calendar/<cal-id>/<uid>.ics # vanilla RFC 5545
.meta/<uid>.json            # Proton-specifics: object ID, ciphertext, version/key handle
```

- `.vcf` and `.ics` files must be parseable by any standard-compliant client (no Proton fields inline)
- `.meta/<uid>.json` is the only place Proton-specific data lives; keys: `proton_id`, `ciphertext`, `version`
- UIDs are the standard vCard/iCal UID fields, used as filenames

## Phases

| Phase | Status | Description |
|-------|--------|-------------|
| Spike | **next** | Decrypt one contact + one calendar event end-to-end |
| 1 | planned | Read-only backup — full fetch+decrypt to versioned tree |
| 2 | deferred | Read-only live view via Radicale/vdirsyncer/DAVx5 |
| 3 | north star | Two-way sync; gated on rehearsal round-trip against test account; contacts-write before calendar-write |

## Key dependencies

- `github.com/ProtonMail/go-proton-api` — official Proton API client (auth, contacts, calendar endpoints)
- `github.com/emersion/hydroxide/protonmail` — reference for contact E2E decryption
- `github.com/ProtonMail/gopenpgp/v3` — E2E crypto primitives

## Scope

**In scope:** contacts + calendar only.
**Out of scope:** mail (covered by protonmail-bridge + zkm/mbsync), Drive (rclone), Pass (manual export), zkm ingestion plugins (live in `~/src/zkm`).

## Related projects

- `~/src/zkm` — corpus search; future `zkm-vcard` / `zkm-calendar` plugins will ingest this tree
- `~/src/claude-diary` — diary; see `diary` branch
- Backup runbook: TODO (will reference this project + zkm + rclone for full Proton coverage)

## Founding meeting

`docs/meeting-notes/2026-05-29-1313-proton-moresync-scope-codereuse.md`
