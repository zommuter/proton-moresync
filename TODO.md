# proton-moresync TODO

## Current

- [ ] **Phase 1: session storage hardening — verify unattended run.** Keyring backend implemented (`secrets.go`); run backup interactively once to store `salted_key_pass`, then verify `backup </dev/null` completes with zero prompts. — see docs/meeting-notes/2026-06-04-1140-session-storage-hardening.md <!-- id:8a70 -->
- [ ] **Scheduling/trigger runner.** mbsync-hook pattern — script in repo, symlink into `.git/hooks` or systemd timer, journald-queryable. — see docs/meeting-notes/2026-05-29-1313-proton-moresync-scope-codereuse.md <!-- id:8af0 -->
- [x] **Backup runbook.** Top-level doc referencing zkm (mail), rclone (drive), Pass export for full Proton coverage. — see docs/meeting-notes/2026-05-29-1313-proton-moresync-scope-codereuse.md <!-- id:22b4 --> — written at docs/proton-backup-runbook.md 2026-06-04
- [ ] **Phase 2/3 north star (design later).** P2 = Radicale/vdirsyncer live-view; P3 = two-way, gated on rehearsal round-trip, contacts-write before calendar-write. — see docs/meeting-notes/2026-05-29-1313-proton-moresync-scope-codereuse.md <!-- id:e436 -->
- [x] **Cross-repo (zkm): generic `zkm-vcard` / `zkm-calendar` ingestion plugins.** Source-agnostic; one-liner forward-flag in zkm's TODO. — see docs/meeting-notes/2026-05-29-1313-proton-moresync-scope-codereuse.md <!-- id:eef8 --> — already in zkm/TODO.md (V-prefix + C-prefix sections, 2026-06-01)
- [ ] **Phase 1: fix app-version string for login alerts.** Login shows "unknown" in Proton's security notification. Research correct `x-pm-appversion` format for third-party clients (currently `Other_0.1.0` in `cmd/spike/main.go:53`); find a name that appears clearly in Proton's "new login" emails. <!-- id:da3a -->
- [ ] **Phase 1: investigate QR-code login.** Proton mobile apps support QR-code-based login (scan on phone → app grants session tokens to desktop). If go-proton-api exposes this flow it would replace CAPTCHA for headless/first-login use. Probe: search go-proton-api for `QRCode`/`SSO`/`ExternalSSO`; check Bridge for any equivalent; record whether it avoids HV entirely. <!-- id:761e -->
## Done

- [x] **Spike: prove end-to-end decrypt.** Contact + calendar decrypt both TURNKEY via go-proton-api. Calendar key-unwrap turnkey (not hand-assembly). Session persisted. — verified 2026-06-04 <!-- id:4438 -->
- [x] **Run spike live against real account.** Contact + calendar decrypt both TURNKEY. Session persisted. — verified 2026-06-04 <!-- id:7807 -->
- [x] **Run spike cold: collect HV methods for this account.** Result: CAPTCHA-only. — verified by run 2026-06-03 <!-- id:7668 -->
- [x] **Implement HV-token tee + session persistence.** `hvCaptureTransport` (422-body tee via AddPreRequestHook) recovers the dropped HV token/methods; `NewClientWithRefresh` + 0600 session file. — verified by run 2026-06-03 (HV methods captured live) <!-- id:7669 -->
- [x] **Phase 1: bump go-proton-api dep.** Pinned to `v0.4.1-0.20260424150947-6bf7f5a61eb8`; dropped hvCaptureTransport/registerHVProbe/hvPreRequestHook; added ProtonMail resty fork replace; fixed Unlock async.PanicHandler arg. — go build + go vet clean 2026-06-04 <!-- id:488c -->
- [x] **Research Proton Bridge HV handling.** Bridge opens `verify.proton.me/?methods=captcha&token=<TOKEN>` top-level in system browser; nothing captured back; retry reuses original token. Fixed `captcha.go` accordingly. — see docs/meeting-notes/2026-06-04-0843-bridge-hv-research.md <!-- id:8a9e -->
- [x] **Unblock decrypt proof via session import.** Superseded — the Bridge-pattern CAPTCHA fix (id:8a9e) unblocks the spike directly; session import no longer needed as a separate step. <!-- id:9fe2 -->
- [x] **Define the output contract.** Standard `.vcf` (RFC 6350) / `.ics` (RFC 5545), one object/file by UID; Proton-specifics in `.meta/` sidecars (contacts: proton_id+cards+version; events: proton_id+cal_id+key_packets+parts+version). — implemented 2026-06-04 <!-- id:d3bb -->
- [x] **Phase 1 build: full read-only backup.** Fetch + decrypt all contacts + all calendar events; git-versioned tree + sidecars with ciphertext. — implemented 2026-06-04 (cmd/backup/) <!-- id:5416 -->
