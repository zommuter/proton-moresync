# proton-moresync TODO

## Current

- [ ] **Spike: prove end-to-end decrypt.** Go program on go-proton-api that authenticates (SRP + 2FA) and decrypts exactly one contact and one calendar event; record whether calendar key-unwrap is turnkey or needs hand-assembly. Gates the rest. `cmd/spike/main.go` implemented + CAPTCHA solve wired (`cmd/spike/captcha.go`). Run: `PROTON_USER=you@proton.me go run ./cmd/spike` — awaiting run against live account to record FINDING: lines. — see docs/meeting-notes/2026-05-29-1313-proton-moresync-scope-codereuse.md, docs/meeting-notes/2026-06-03-1447-captcha-safe-auth.md <!-- id:4438 -->
- [ ] **Define the output contract.** Standard `.vcf` (RFC 6350) / `.ics` (RFC 5545), one object/file by UID; Proton-specifics in `.meta/<uid>.json` sidecar. — see docs/meeting-notes/2026-05-29-1313-proton-moresync-scope-codereuse.md <!-- id:d3bb -->
- [ ] **Phase 1 build: full read-only backup.** Fetch + decrypt all contacts + all calendar events; git-versioned tree + sidecars; crib hydroxide's protonmail package. — see docs/meeting-notes/2026-05-29-1313-proton-moresync-scope-codereuse.md <!-- id:5416 -->
- [ ] **Phase 1: session storage hardening.** Evaluate OS keyring (libsecret) vs age-encrypted file; note headless-box friction (cartmanjaro/fievel have no Secret Service). — see docs/meeting-notes/2026-06-03-1447-captcha-safe-auth.md <!-- id:8a70 -->
- [ ] **Scheduling/trigger runner.** mbsync-hook pattern — script in repo, symlink into `.git/hooks` or systemd timer, journald-queryable. — see docs/meeting-notes/2026-05-29-1313-proton-moresync-scope-codereuse.md <!-- id:8af0 -->
- [ ] **Backup runbook.** Top-level doc referencing zkm (mail), rclone (drive), Pass export for full Proton coverage. — see docs/meeting-notes/2026-05-29-1313-proton-moresync-scope-codereuse.md <!-- id:22b4 -->
- [ ] **Phase 2/3 north star (design later).** P2 = Radicale/vdirsyncer live-view; P3 = two-way, gated on rehearsal round-trip, contacts-write before calendar-write. — see docs/meeting-notes/2026-05-29-1313-proton-moresync-scope-codereuse.md <!-- id:e436 -->
- [ ] **Cross-repo (zkm): generic `zkm-vcard` / `zkm-calendar` ingestion plugins.** Source-agnostic; one-liner forward-flag in zkm's TODO. — see docs/meeting-notes/2026-05-29-1313-proton-moresync-scope-codereuse.md <!-- id:eef8 -->
- [ ] **BLOCKED: HV/CAPTCHA solve.** Browser-embedded solve is infeasible — Proton enforces `frame-ancestors` on both verify.proton.me AND mail.proton.me/captcha/v1/assets (only calendar/drive/mail.proton.me allowed). Re-approach properly in a fresh session. — see docs/meeting-notes/2026-06-03-1614-captcha-frame-ancestors-wall.md <!-- id:488c -->
- [ ] **Research Proton Bridge HV handling.** Clone `github.com/ProtonMail/proton-bridge` (not in Go module cache); grep HumanVerification/captcha/9001 — find how it satisfies Proton's frame-ancestors allowlist; mirror that. Gates the HV-approach choice. — see docs/meeting-notes/2026-06-03-1614-captcha-frame-ancestors-wall.md <!-- id:8a9e -->
- [ ] **Unblock decrypt proof via session import.** Seed `session.json` (UID + refresh token) from an existing authenticated Proton client; `NewClientWithRefresh` reuses it with no CAPTCHA — proves contact+event decrypt independently of the CAPTCHA solver. — see docs/meeting-notes/2026-06-03-1614-captcha-frame-ancestors-wall.md <!-- id:9fe2 -->

## Done

- [x] **Run spike cold: collect HV methods for this account.** Result: CAPTCHA-only. — verified by run 2026-06-03 <!-- id:7668 -->
- [x] **Implement HV-token tee + session persistence.** `hvCaptureTransport` (422-body tee via AddPreRequestHook) recovers the dropped HV token/methods; `NewClientWithRefresh` + 0600 session file. — verified by run 2026-06-03 (HV methods captured live) <!-- id:7669 -->
