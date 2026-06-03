# 2026-06-03 — CAPTCHA frame-ancestors wall (handover)

**Started:** 2026-06-03 16:14
**Session:** ea8e87db-ae1e-4e67-905a-437986b4f264
**Mode:** Implementation handover (Class 1 dispatch of id:488c hit a systemic wall — stop and re-approach)
**Topic:** Why the browser-embedded CAPTCHA solve cannot work, and what the next session should investigate instead.

## TL;DR

Proton's CAPTCHA refuses to be framed by any non-Proton origin via a
`frame-ancestors` CSP. Every iframe/popup/local-relay variant we tried hit
this wall. The browser-embedding family of approaches is **dead**. The next
session should **not** write more relay/proxy workarounds — instead read how
the open-source Proton Bridge solves HV, and pick a proper approach from that.

## What was attempted this session (all committed, all dead ends)

Commits 631b1e3 → 931c63c on `main`, all in `cmd/spike/captcha.go`:

1. **iframe verify.proton.me** → blocked: `Framing 'https://verify.proton.me/'
   violates frame-ancestors https://mail.proton.me https://calendar.proton.me
   https://drive.proton.me`.
2. **window.open popup to verify.proton.me** → popup loads, but standalone
   verify.proton.me only renders a *"You need to specify recovery methods"*
   account nag — it only shows the CAPTCHA when embedded inside an official
   Proton web app. No postMessage token ever arrives.
3. **Proxy the widget via `GetCaptcha` + serve locally in an iframe** →
   infinite recursion: the widget's root-relative asset URL fell through to
   our `/` catch-all and re-served the relay page nested in itself.
4. **Serve widget top-level with injected `<base href=mail.proton.me>` +
   pm_captcha relay** → the widget's inner asset iframe
   (`https://mail.proton.me/captcha/v1/assets/...`) is itself blocked:
   `Framing 'https://mail.proton.me/' violates frame-ancestors
   https://calendar.proton.me https://drive.proton.me`.

So **two** CSP walls, not one:
- `verify.proton.me` → `frame-ancestors mail/calendar/drive.proton.me`
- `mail.proton.me/captcha/v1/assets/` → `frame-ancestors calendar/drive.proton.me`

## Hard technical facts established (reusable, don't re-derive)

- **HV methods for this account = `[captcha]` only.** No email/SMS HV path is
  offered for login (the cheap 6-digit-code escape from the 2026-06-03-1447
  meeting is not available here). HV token is 24 chars.
- **`Manager.GetCaptcha(ctx, hvToken)`** (`/core/v4/captcha?Token=...&ForceWebMessaging=1`)
  returns the real CAPTCHA widget HTML (~4060 bytes). Dumped sample at
  `/tmp/proton-captcha-widget.html` (regenerated each run; token rotates).
- **Widget token flow** (from the widget's inline script):
  1. Widget builds an inner iframe `src=/captcha/v1/assets/?purpose=login&token=<hvToken>`
     (root-relative → resolves to `mail.proton.me` since the API host is
     `https://mail.proton.me/api`).
  2. Inner assets iframe, on solve, posts `{type:'proton_captcha', token:<resp>}`
     to `window.parent`.
  3. Widget catches it, calls `tokenCallback(resp)` → `sendToken(prefix+resp)`
     → posts `{type:'pm_captcha', token: '<hvToken>:<prefix><resp>'}` to
     `window.parent` with target `'*'`.
  - **Final HV header value** = that `pm_captcha.token` =
    `hvToken + ':' + prefix + response`. `prefix` is per-fetch and embedded in
    the widget HTML (the `tokenCallback` line, e.g.
    `'D96oyG+/...'+'gX405...'`). Sent back as
    `x-pm-human-verification-token` + `x-pm-human-verification-token-type: captcha`.
- **go-proton-api v0.4.0 has no HV handling** beyond the `9001` constant;
  `APIError` drops the `Details` block. Our `hvCaptureTransport` (an
  `http.RoundTripper` wrapper installed via `AddPreRequestHook`) successfully
  tees the raw 422 body to recover `HumanVerificationToken` + `Methods` before
  resty's `catchAPIError` stops the chain. **This part works and should be
  kept** (`cmd/spike/session.go`).

## What works and is worth keeping

- `cmd/spike/session.go` — session persistence (`NewClientWithRefresh` + 0600
  JSON file) and the `hvCaptureTransport` HV-token tee. Solid.
- `cmd/spike/main.go` — auth/2FA/key-unlock + contact + calendar decrypt
  scaffold. Untested past auth because we can't get a session.
- The `GetCaptcha` probe + widget dump in `captcha.go` — useful evidence tool.

## What to throw away

- The local-server / iframe / popup / base-href relay machinery in
  `cmd/spike/captcha.go`. It cannot work. Replace, don't patch.

## Recommended next-session investigation (proper approach, not workarounds)

1. **Read how Proton Bridge solves HV.** Bridge is open source
   (`github.com/ProtonMail/proton-bridge`) and *must* handle login CAPTCHA. It
   is NOT in the local Go module cache — clone it and grep for
   `HumanVerification`, `captcha`, `9001`, `frame-ancestors`, `verify.proton.me`.
   Bridge almost certainly embeds a real browser engine (it ships a Qt/WebView
   GUI) where Proton's own origin allowlist is satisfied, or it bypasses CSP in
   a controlled WebView. Find out exactly which, then mirror it.
2. **Evaluate the three options the user deferred** (in light of #1):
   - (a) Headless/headed Chrome with `--disable-web-security` (chromedp or
     go-rod) so `frame-ancestors` is ignored; user solves in the window; relay
     `pm_captcha` → token. Adds a dep but is the most robust automated path.
   - (b) Open the assets URL top-level (no framing → no CSP block); manual
     token extraction + reconstruct `hvToken:prefix+response` in Go. Fragile if
     the assets page checks for an iframe parent.
   - (c) Import an existing authenticated session (UID + refresh token) from a
     logged-in Proton client and seed `session.json`; `NewClientWithRefresh`
     reuses it with no CAPTCHA. Cheapest one-time bootstrap; this is what the
     deleted devtools-bypass effectively did.
3. **Decide whether CAPTCHA solve belongs in the spike at all.** The
   2026-06-03-1447 meeting *deferred HV solve to Phase 1*. The spike's actual
   goal (prove one-contact + one-event decrypt) only needs *a* valid session by
   *any* means. Option (c) likely unblocks the decrypt proof fastest, leaving a
   real CAPTCHA solver as separate Phase-1 work informed by #1.

## Decisions

- Browser-embedded CAPTCHA solve via a non-Proton origin is **infeasible** —
  Proton enforces `frame-ancestors` on both `verify.proton.me` and the captcha
  assets path. Out of scope: any further local-relay/proxy/iframe/popup attempt.
- Next step is **research Proton Bridge's HV handling first**, then choose
  among headless-Chrome / top-level-paste / session-import — not more ad-hoc
  workarounds.
- The `hvCaptureTransport` 422-tee and session-persistence code are correct and
  retained; only the `captcha.go` relay machinery is discarded.

## Action items
- [ ] Clone + read `github.com/ProtonMail/proton-bridge` HV/CAPTCHA handling; record how it satisfies Proton's frame-ancestors allowlist. <!-- id:8a9e -->
- [ ] Choose HV approach (headless-Chrome / top-level-paste / session-import) informed by Bridge findings; implement in a fresh session. <!-- id:488c -->
- [ ] Consider seeding `session.json` from an existing authenticated session to unblock the decrypt proof independently of the CAPTCHA solver. <!-- id:9fe2 -->
