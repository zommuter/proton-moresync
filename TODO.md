# proton-moresync TODO

## Current

- [ ] **Publish to GitHub.** Add `github` remote and push. — see docs/meeting-notes/2026-06-05-1059-github-publishable.md <!-- id:f8fb -->

- [ ] **Phase 2/3 north star (design later).** P2 = Radicale/vdirsyncer live-view; P3 = two-way, gated on rehearsal round-trip, contacts-write before calendar-write. — see docs/meeting-notes/2026-05-29-1313-proton-moresync-scope-codereuse.md <!-- id:e436 -->

## Done

- [x] **One-time scheduling setup.** bare repo on `<backup-host>`, ~/proton-backup seeded + pushed, EnvironmentFile config, Makefile install target, timer enabled. — see docs/meeting-notes/2026-06-04-1407-scheduling-trigger-runner.md <!-- id:4aae --> — completed 2026-06-04
- [x] **Phase 1: investigate QR-code login.** Probed go-proton-api v0.4.1 and all Proton Go modules in cache for `QRCode`/`SSO`/`ExternalSSO`/`LoginToken`/`Fork` — zero matches. Feature is not exposed in the library; no client-side implementation path exists. CAPTCHA+session-reuse remains the only headless-first-login option. <!-- id:761e --> — probed 2026-06-04, closed as not available

- [x] **Phase 1: session storage hardening — verify unattended run.** Keyring backend implemented (`secrets.go`); run backup interactively once to store `salted_key_pass`, then verify `backup </dev/null` completes with zero prompts. — see docs/meeting-notes/2026-06-04-1140-session-storage-hardening.md <!-- id:8a70 --> — verified by user 2026-06-04
- [x] **Scheduling/trigger runner.** proton-backup-sync.sh + systemd/proton-backup.{service,timer} shipped; mirrors ~/mail pattern. — see docs/meeting-notes/2026-06-04-1407-scheduling-trigger-runner.md <!-- id:8af0 --> — implemented 2026-06-04
- [x] **Backup runbook.** Top-level doc referencing zkm (mail), rclone (drive), Pass export for full Proton coverage. — see docs/meeting-notes/2026-05-29-1313-proton-moresync-scope-codereuse.md <!-- id:22b4 --> — written at docs/proton-backup-runbook.md 2026-06-04
- [x] **Cross-repo: generic `vcard` / `calendar` ingestion plugins for downstream corpus tool.** Source-agnostic; one-liner forward-flag in the corpus tool's TODO. — see docs/meeting-notes/2026-05-29-1313-proton-moresync-scope-codereuse.md <!-- id:eef8 --> — already tracked downstream (V-prefix + C-prefix sections, 2026-06-01)
- [x] **Phase 1: fix app-version string for login alerts.** `Other_<SemVer>` is the correct format for third-party clients; Proton renders `Other` as "unknown" in security notifications — this is a Proton-side limitation with no client-side fix. Updated comments in `cmd/backup/auth.go` and `cmd/spike/main.go` to document the finding. <!-- id:da3a -->
- [x] **Spike: prove end-to-end decrypt.** Contact + calendar decrypt both TURNKEY via go-proton-api. Calendar key-unwrap turnkey (not hand-assembly). Session persisted. — verified 2026-06-04 <!-- id:4438 -->
- [x] **Run spike live against real account.** Contact + calendar decrypt both TURNKEY. Session persisted. — verified 2026-06-04 <!-- id:7807 -->
- [x] **Run spike cold: collect HV methods for this account.** Result: CAPTCHA-only. — verified by run 2026-06-03 <!-- id:7668 -->
- [x] **Implement HV-token tee + session persistence.** `hvCaptureTransport` (422-body tee via AddPreRequestHook) recovers the dropped HV token/methods; `NewClientWithRefresh` + 0600 session file. — verified by run 2026-06-03 (HV methods captured live) <!-- id:7669 -->
- [x] **Phase 1: bump go-proton-api dep.** Pinned to `v0.4.1-0.20260424150947-6bf7f5a61eb8`; dropped hvCaptureTransport/registerHVProbe/hvPreRequestHook; added ProtonMail resty fork replace; fixed Unlock async.PanicHandler arg. — go build + go vet clean 2026-06-04 <!-- id:488c -->
- [x] **Research Proton Bridge HV handling.** Bridge opens `verify.proton.me/?methods=captcha&token=<TOKEN>` top-level in system browser; nothing captured back; retry reuses original token. Fixed `captcha.go` accordingly. — see docs/meeting-notes/2026-06-04-0843-bridge-hv-research.md <!-- id:8a9e -->
- [x] **Unblock decrypt proof via session import.** Superseded — the Bridge-pattern CAPTCHA fix (id:8a9e) unblocks the spike directly; session import no longer needed as a separate step. <!-- id:9fe2 -->
- [x] **Define the output contract.** Standard `.vcf` (RFC 6350) / `.ics` (RFC 5545), one object/file by UID; Proton-specifics in `.meta/` sidecars (contacts: proton_id+cards+version; events: proton_id+cal_id+key_packets+parts+version). — implemented 2026-06-04 <!-- id:d3bb -->
- [x] **Phase 1 build: full read-only backup.** Fetch + decrypt all contacts + all calendar events; git-versioned tree + sidecars with ciphertext. — implemented 2026-06-04 (cmd/backup/) <!-- id:5416 -->
