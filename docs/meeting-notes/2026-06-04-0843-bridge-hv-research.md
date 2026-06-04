# 2026-06-04 — Bridge HV research + CAPTCHA fix

**Started:** 2026-06-04 08:43
**Session:** defc6cc3-e511-40e5-bd18-3ef3814bf4b0
**Mode:** Class 2 planning record (no meeting was held — plan-mode output)
**Topic:** Research how Proton Bridge handles login CAPTCHA; implement the correct approach.

## Context

The prior session (2026-06-03-1614-captcha-frame-ancestors-wall.md) concluded
that browser-embedded CAPTCHA solve is infeasible due to Proton's `frame-ancestors`
CSP on both `verify.proton.me` and `mail.proton.me/captcha/v1/assets/`. It
recommended researching how Proton Bridge solves HV before attempting further
workarounds. This session executes that recommendation (TODO id:8a9e).

## Plan

1. Read open-source Bridge source (`github.com/ProtonMail/proton-bridge`) for HV
   handling — verified via WebFetch/WebSearch against GitHub.
2. Read local go-proton-api v0.4.0 module cache for HV API surface.
3. Map against our existing `cmd/spike/captcha.go` to identify the exact bugs.
4. Implement the Bridge pattern; build + vet.

## Implementation findings

**Bridge's HV mechanism (verified against source):**

- `internal/hv/hv.go::FormatHvURL` builds:
  `https://verify.proton.me/?methods=<comma-joined methods>&token=<hvToken>`
- GUI (`Login.qml`): `Qt.openUrlExternally(hvLinkUrl)` — system browser, top-level.
- CLI (`accounts.go`): prints the URL + "press ENTER when completed" + `ReadLine()`.
- **Nothing is captured back from the browser.** Solving validates the token
  server-side. Bridge retries login with the **same original challenge token
  verbatim**; `addHVToRequest` (go-proton-api master `hv.go`) sets
  `x-pm-human-verification-token` = `details.Token`,
  `x-pm-human-verification-token-type` = comma-joined methods.

**Correction to Bridge research (verified empirically 2026-06-04):**
The earlier conclusion "nothing is captured back from the browser; retry reuses
original hvToken" was WRONG. Live run showed "CAPTCHA validation failed" when
retrying with bare hvToken.

Network analysis of verify.proton.me solve (user captured DevTools traffic):
- `GET verify-api.proton.me/captcha/v1/api/validate?token=<hexToken>&contestId=<uuid>&purpose=login`
- `POST verify-api.proton.me/captcha/v1/api/finalize?contestId=<uuid>&purpose=login`
These verify-api calls are internal to the browser session and do NOT mark
hvToken as solved on Proton's auth API. The auth API expects the **composite
token**: `hvToken:prefix+hcaptchaResponse` in `x-pm-human-verification-token`.

**Three bugs in our prior `captcha.go`:**
1. Opened bare `verify.proton.me` — the `?methods=captcha&token=<TOKEN>` query
   string was missing; page showed recovery nag instead of CAPTCHA.
2. Tried to harvest composite token via postMessage relay to a local server —
   approach was right (composite token IS needed), but delivery was wrong (CSP
   blocked the inner iframe).
3. The Bridge research agent's conclusion ("retry with bare hvToken") was incorrect
   for our app-version; the composite token is required.

**Fix that worked:** DevTools paste — user adds a `window.addEventListener('message',...)`
listener before solving; Console logs `%%CAPTCHA_TOKEN%% <hvToken>:<prefix><response>`;
user pastes composite token into spike stdin. Auth succeeded with TOTP as well.

**go-proton-api v0.4.0 vs. master:**
- v0.4.0 drops `APIError.Details` (no HV helpers). Bridge's own `go.mod` pins
  `v0.4.1-0.20260424150947-6bf7f5a61eb8` (untagged master) to get `Details`,
  `GetHVDetails`, `addHVToRequest`, `NewClientWithLoginWithHVToken`.
- Our `hvCaptureTransport` 422-tee already works around the missing `Details`
  field by tee-ing the raw response body — equivalent result. Decision: **keep
  v0.4.0** for the spike to avoid breaking the calendar/contact API surface.
  Dep bump deferred to Phase 1.

**What was changed:**
- `cmd/spike/captcha.go` — fully replaced with the Bridge pattern:
  top-level `xdg-open` of the correct URL + ENTER wait. 44 lines → 40 lines.
  Dropped: local HTTP server, widget fetch, `patchWidget`, postMessage relay,
  `<base>` injection, stdin-vs-postMessage race. Kept: nothing (the old approach
  was structurally wrong).
- `cmd/spike/main.go` — adjusted solve call: `solveCaptcha(hv.Methods, hv.Token)`
  (no ctx/manager); retry uses `hvPreRequestHook(hv.Token)` (original token).
  Updated top-of-file comment.

`go build ./cmd/spike` + `go vet ./cmd/spike` both clean.

## Live run findings (2026-06-04, session defc6cc3)

**Auth:** SRP + CAPTCHA (composite token via DevTools paste) + TOTP — all work.
Session persisted to `~/.local/state/proton-moresync/session.json`; subsequent
runs reuse session with no CAPTCHA.

**Contact decrypt:** `contact.Cards.Merge(kr)` is TURNKEY — but must iterate ALL
`addrKRs`, not just the first. With 12 address keyrings, the first was wrong key.
Fixed by trying each keyring until success. FINDING: contact decrypt TURNKEY.

**Calendar decrypt:** FULLY TURNKEY end-to-end:
- `GetCalendarPassphrase + Decrypt` → passphrase (turnkey)
- `GetCalendarKeys + Unlock` → calendar keyring (turnkey)
- Per-part decrypt of SharedEvents → VEVENT data (turnkey, but `CalendarEventPart.Decode`
  has a value-receiver bug that discards the result; inlined as `decryptPart` helper)
- SharedEvents[0] type=0 (unencrypted skeleton), [1] type=1 (encrypted SUMMARY+DESCRIPTION)
- CalendarEvents empty for this event; SharedEvents carry the content.

**Calendar key-unwrap:** TURNKEY — not hand-assembly. The 2026-05-29 concern was unfounded.

**CAPTCHA composite token (correction to earlier Bridge research):**
Bare `hvToken` is REJECTED ("CAPTCHA validation failed"). The auth API requires
`hvToken:prefix+hcaptchaResponse`. The verify-api.proton.me validate+finalize calls
are internal to the browser session and do NOT mark hvToken as solved on Proton's
auth API. Confirmed by network analysis during live solve.
Phase 1 fix: chromedp (captures pm_captcha postMessage automatically), or session
persistence (already works — CAPTCHA is one-time-only).

## Decisions

- Contact and calendar decrypt: **both TURNKEY** via go-proton-api. Phase 1 build
  can proceed without any hand-assembly crypto.
- CAPTCHA: composite token required. For Phase 1, use chromedp for automation or
  rely on session persistence to make CAPTCHA a one-time login event.
- Keep go-proton-api v0.4.0 for now; dep bump to Phase 1.
- `CalendarEventPart.Decode` value-receiver bug: must inline decrypt logic.

## Action items

- [x] Run spike live: contact + calendar decrypt both TURNKEY confirmed. <!-- id:7807 -->
- [ ] Phase 1: bump go-proton-api; delete `hvCaptureTransport` workaround. <!-- id:488c -->
