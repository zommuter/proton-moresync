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

**Two bugs in our prior `captcha.go`:**
1. Opened bare `verify.proton.me` (or local relay widget) — the mandatory
   `?methods=captcha&token=<TOKEN>` query string was missing. Without it the
   standalone verify page shows a "specify recovery methods" account nag, not
   the CAPTCHA.
2. Tried to harvest a composite token (`<hvToken>:<prefix><resp>`) back via
   postMessage relay — unnecessary. Nothing is captured; the retry reuses the
   original challenge token.

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

## Decisions

- Bridge's HV mechanism is: top-level-browser `verify.proton.me/?methods=...&token=...`
  + ENTER wait + retry with original token. `frame-ancestors` never triggered.
  Out of scope: any embedding/framing/relay approach.
- Keep go-proton-api v0.4.0 for the spike; `hvCaptureTransport` tee remains as the
  v0.4.0 workaround. Dep bump to Phase 1.
- The CAPTCHA solve is now implemented and unblocks the live run of the spike (id:4438).

## Action items

- [ ] Run spike live against real account to collect FINDING: lines (requires PTY):
  `PROTON_USER=you@proton.me go run ./cmd/spike` <!-- id:7807 -->
- [ ] Phase 1: bump go-proton-api to Bridge's pseudo-version; delete `hvCaptureTransport`
  in favour of `GetHVDetails`/`NewClientWithLoginWithHVToken`. <!-- id:488c -->
