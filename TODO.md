# proton-moresync TODO

## Current

- [ ] **Spike: prove end-to-end decrypt.** Go program on go-proton-api that authenticates (SRP + 2FA) and decrypts exactly one contact and one calendar event; record whether calendar key-unwrap is turnkey or needs hand-assembly. Gates the rest. `cmd/spike/main.go` implemented — awaiting run against live account to record FINDING: lines. Note: `CalendarEventPart.Decode` has a value-receiver bug in go-proton-api v0.4.0 (decrypted data lost); workaround in `decryptPart` helper. — see docs/meeting-notes/2026-05-29-1313-proton-moresync-scope-codereuse.md <!-- id:4438 -->
- [ ] **Define the output contract.** Standard `.vcf` (RFC 6350) / `.ics` (RFC 5545), one object/file by UID; Proton-specifics in `.meta/<uid>.json` sidecar. — see docs/meeting-notes/2026-05-29-1313-proton-moresync-scope-codereuse.md <!-- id:d3bb -->
- [ ] **Phase 1 build: full read-only backup.** Fetch + decrypt all contacts + all calendar events; git-versioned tree + sidecars; crib hydroxide's protonmail package. — see docs/meeting-notes/2026-05-29-1313-proton-moresync-scope-codereuse.md <!-- id:5416 -->
- [ ] **Scheduling/trigger runner.** mbsync-hook pattern — script in repo, symlink into `.git/hooks` or systemd timer, journald-queryable. — see docs/meeting-notes/2026-05-29-1313-proton-moresync-scope-codereuse.md <!-- id:8af0 -->
- [ ] **Backup runbook.** Top-level doc referencing zkm (mail), rclone (drive), Pass export for full Proton coverage. — see docs/meeting-notes/2026-05-29-1313-proton-moresync-scope-codereuse.md <!-- id:22b4 -->
- [ ] **Phase 2/3 north star (design later).** P2 = Radicale/vdirsyncer live-view; P3 = two-way, gated on rehearsal round-trip, contacts-write before calendar-write. — see docs/meeting-notes/2026-05-29-1313-proton-moresync-scope-codereuse.md <!-- id:e436 -->
- [ ] **Cross-repo (zkm): generic `zkm-vcard` / `zkm-calendar` ingestion plugins.** Source-agnostic; one-liner forward-flag in zkm's TODO. — see docs/meeting-notes/2026-05-29-1313-proton-moresync-scope-codereuse.md <!-- id:eef8 -->
