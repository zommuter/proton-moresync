# proton-moresync TODO

## Current

- Relay: 1 open ROADMAP item <!-- lint-ok: relay ROADMAP summary line, not a work item --> <!-- id:2449 -->

- [ ] **QR-login spike (adoption-triggered, deferred).** Once there is a first external user, probe whether an `Independent` fork with a generic/borrowed `ChildClientID` is accepted by `GET auth/v4/sessions/forks/{selector}`. Decrypt AES-GCM Payload under `sk` → `keyPassword`. Goal: documented yes/no on ChildClientID acceptance; no production code before probe. — see docs/meeting-notes/2026-06-05-1144-qr-login-session-fork.md <!-- id:96d7 -->

- [ ] **Phase 2/3 north star (design later).** P2 serving path **decided 2026-06-15: Radicale** (vdirsyncer reclassified to P3; build = ROUTINE id:6aad), see docs/meeting-notes/2026-06-15-1623-phase2-readonly-live-view.md. P3 gate **designed 2026-06-15**: rehearsal round-trip protocol + pass/fail criteria + contacts-before-calendar ordering specified in docs/meeting-notes/2026-06-15-1707-phase3-rehearsal-harness.md (id:56c9); write path build-gated on a throwaway-account reopen trigger. — see docs/meeting-notes/2026-05-29-1313-proton-moresync-scope-codereuse.md <!-- id:e436 -->

## Done
