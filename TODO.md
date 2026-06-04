# proton-moresync TODO

## Current

- [ ] **Define the output contract.** Standard `.vcf` (RFC 6350) / `.ics` (RFC 5545), one object/file by UID; Proton-specifics in `.meta/<uid>.json` sidecar. — see docs/meeting-notes/2026-05-29-1313-proton-moresync-scope-codereuse.md <!-- id:d3bb -->
- [ ] **Phase 1 build: full read-only backup.** Fetch + decrypt all contacts + all calendar events; git-versioned tree + sidecars; crib hydroxide's protonmail package. — see docs/meeting-notes/2026-05-29-1313-proton-moresync-scope-codereuse.md <!-- id:5416 -->
- [ ] **Phase 1: session storage hardening.** Evaluate OS keyring (libsecret) vs age-encrypted file; note headless-box friction (cartmanjaro/fievel have no Secret Service). — see docs/meeting-notes/2026-06-03-1447-captcha-safe-auth.md <!-- id:8a70 -->
- [ ] **Scheduling/trigger runner.** mbsync-hook pattern — script in repo, symlink into `.git/hooks` or systemd timer, journald-queryable. — see docs/meeting-notes/2026-05-29-1313-proton-moresync-scope-codereuse.md <!-- id:8af0 -->
- [ ] **Backup runbook.** Top-level doc referencing zkm (mail), rclone (drive), Pass export for full Proton coverage. — see docs/meeting-notes/2026-05-29-1313-proton-moresync-scope-codereuse.md <!-- id:22b4 -->
- [ ] **Phase 2/3 north star (design later).** P2 = Radicale/vdirsyncer live-view; P3 = two-way, gated on rehearsal round-trip, contacts-write before calendar-write. — see docs/meeting-notes/2026-05-29-1313-proton-moresync-scope-codereuse.md <!-- id:e436 -->
- [ ] **Cross-repo (zkm): generic `zkm-vcard` / `zkm-calendar` ingestion plugins.** Source-agnostic; one-liner forward-flag in zkm's TODO. — see docs/meeting-notes/2026-05-29-1313-proton-moresync-scope-codereuse.md <!-- id:eef8 -->
- [ ] **Phase 1: bump go-proton-api dep.** Pin to Bridge's master pseudo-version (`v0.4.1-0.20260424150947-6bf7f5a61eb8`); delete `hvCaptureTransport` workaround in favour of `GetHVDetails`/`NewClientWithLoginWithHVToken`. Deferred from spike. — see docs/meeting-notes/2026-06-04-0843-bridge-hv-research.md <!-- id:488c -->
## Done

- [x] **Spike: prove end-to-end decrypt.** Contact + calendar decrypt both TURNKEY via go-proton-api. Calendar key-unwrap turnkey (not hand-assembly). Session persisted. — verified 2026-06-04 <!-- id:4438 -->
- [x] **Run spike live against real account.** Contact + calendar decrypt both TURNKEY. Session persisted. — verified 2026-06-04 <!-- id:7807 -->
- [x] **Run spike cold: collect HV methods for this account.** Result: CAPTCHA-only. — verified by run 2026-06-03 <!-- id:7668 -->
- [x] **Implement HV-token tee + session persistence.** `hvCaptureTransport` (422-body tee via AddPreRequestHook) recovers the dropped HV token/methods; `NewClientWithRefresh` + 0600 session file. — verified by run 2026-06-03 (HV methods captured live) <!-- id:7669 -->
- [x] **Research Proton Bridge HV handling.** Bridge opens `verify.proton.me/?methods=captcha&token=<TOKEN>` top-level in system browser; nothing captured back; retry reuses original token. Fixed `captcha.go` accordingly. — see docs/meeting-notes/2026-06-04-0843-bridge-hv-research.md <!-- id:8a9e -->
- [x] **Unblock decrypt proof via session import.** Superseded — the Bridge-pattern CAPTCHA fix (id:8a9e) unblocks the spike directly; session import no longer needed as a separate step. <!-- id:9fe2 -->
