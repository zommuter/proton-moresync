# 2026-06-03 — CAPTCHA-safe auth for the Proton spike

**Started:** 2026-06-03 14:47
**Session:** e778b8b5-0d7f-4655-b605-475f2d024a38
**Attendees:** 🏗️ Archie (architect), 😈 Riku (devil's advocate), ✂️ Petra (productivity), 🔐 Dario (E2E PIM API reuse — re-onboarded)
**Topic:** Replace the devtools-bearer-token CAPTCHA bypass with a proper session-persistence approach, without risking account suspension.

## Surfaced discoveries
- [2026-05-29 proton-moresync] Proton calendar bootstrap has no turnkey helper in go-proton-api — the spike (hence auth) is the gating precondition before Phase 1.

## Agenda
1. Is "show the CAPTCHA in a browser, user solves it, capture token, retry login" actually feasible with go-proton-api v0.4.0?
2. Session persistence — does it make CAPTCHA a one-time event and obsolete the token-copy hack?
3. Account safety — what behaviors risk suspension/flagging, and how to avoid them?
4. Scope — what to build for the spike vs defer to Phase 1.

## Research facts (from go-proton-api v0.4.0 source)
- `response.go:27` — `HumanVerificationRequired Code = 9001` is the ONLY HV symbol. `APIError` (Status/Code/Message) does NOT deserialize the `Details` block, so the HV **token** and **allowed methods** Proton sends are silently dropped.
- `manager_user.go:7` — `GetCaptcha(ctx, token)` fetches raw bytes from `/core/v4/captcha?Token=...&ForceWebMessaging=1`, but needs the very token the lib drops; never called by the auth flow.
- No `WithHVToken`, no retry-with-token path, no `captcha` verify-type constant. Only generic `AddPreRequestHook` (manager.go:41 / client.go:67) for manual header injection.
- `manager_auth.go:20` — `NewClientWithRefresh(ctx, uid, ref)` exists. `NewClientWithLogin` returns `Auth{UID, AccessToken, RefreshToken}`. Refresh tokens ROTATE per use (`client.go:206` reassigns `c.ref`), and tokens auto-refresh (`client.go:174`).
- `manager.go:45` — `AddPostRequestHook(resty.ResponseMiddleware)` exists (Manager-level, fires on login call before a client exists).
- `client.go:53` — `AddAuthHandler(func(Auth))` fires on every token rotation with the new `Auth`.
- Library has no anti-abuse/suspension logic; only generic 429 `Retry-After` handling (response.go:103). Flagging is entirely server-side.

## Discussion

### Item 1 — feasibility of browser CAPTCHA flow
- 🏗️ Archie: sanctioned flow = read Details (HV token+methods) → open verify.proton.me with token → user solves → capture solved token via postMessage → retry login with `x-pm-human-verification-token` + `-type: captcha` headers. Lib blocks it: `APIError` drops `Details`; no retry-with-token path. Feasible only via hand-assembly (raw HTTP + local listener + AddPreRequestHook).
- 😈 Riku: verify.proton.me is a postMessage JS widget (needs real browser, not headless); iframe enforces origin allowlist for postMessage target — real risk our embedder origin is rejected; reimplements auth plumbing lib deliberately hides (reinvention/rot risk).
- 🔐 Dario (reframe): CAPTCHA fires only on fresh SRP login from unknown client. `NewClientWithRefresh` reloads a persisted session → no SRP, no CAPTCHA. Devtools hack = manual ephemeral session persistence. Clean version: solve HV ONCE, persist UID+refresh token, reuse forever (re-persist rotated token each run).
- ✂️ Petra: real question is "first login once," not "build a CAPTCHA solver." Options: (A) full embedded browser HV; (B) system browser — same postMessage problem; (C) devtools paste as one-time bootstrap into session store; (D) real login + EMAIL/SMS HV path (6-digit code, no widget) — lib has EmailTokenType/SMSTokenType constants.
- 😈 Riku: (D) underrated — captcha is one of several HV methods; email HV = same unblock, ~10% code, zero browser-origin risk. Unknown: is email HV offered for login? Need to read Details.HumanVerificationMethods (currently dropped).
- 🏗️ Archie: step zero regardless = stop discarding HV Details; small raw-HTTP shim. Then method choice is data-driven.
- 🔐 Dario (convergence): (1) session store first — persist {UID,RefreshToken}, reload via NewClientWithRefresh; eliminates per-run CAPTCHA + obsoletes devtools hack. (2) first-login HV: capture Details, prefer email if offered, embedded-browser CAPTCHA only as fallback. (3) CAPTCHA widget builder deferred unless email HV unavailable.

**Decision (Zommuter):** Session store first + raw-HTTP-free HV probe to log offered methods; pick captcha-vs-email after seeing what's offered.

### Item 3 — account safety
- 😈 Riku flagging vectors (worst-first): (1) repeated failed SRP logins — session persistence kills it; (2) bogus/spoofed AppVersion — honest-unofficial safer than mimicking official web string; (3) request cadence — obey Retry-After; (4) devtools token reuse across processes can trip session-consistency checks.
- 🔐 Dario: Proton's stance — personal read-only backup of your own data is fine (Bridge/export do this); suspension risk is for bulk scrape/multi-account/cred-stuffing. Honest-unofficial posture (hydroxide) has years without mass suspension.
- 🏗️ Archie guardrails: persist+reuse session (fresh login once-per-device); honest AppVersion (keep Other_0.1.0, don't spoof official); serial rate-respectful fetches; read-only; store refresh token securely + write back rotated token each run.
- ✂️ Petra: only buildable safety items = (a) session store with secure-at-rest + rotation write-back, (b) honest AppVersion (done). Rest is operational discipline. 0600 file in $XDG_STATE_HOME suffices for one personal refresh token.
- 😈 Riku sharp edge: refresh token rotates every run (client.go:206); crash after use before persist → stale token → next run falls back to login (acceptable; graceful fallback handles it).

**Decision (Zommuter):** storage = plain 0600 file in $XDG_STATE_HOME for spike; keyring/encryption deferred to Phase 1.

### Item 4 — scope + design resolution
- ✂️ Petra: spike scope = (1) session store load/save + NewClientWithRefresh + write-back rotated token; (2) HV probe logging Details.HumanVerificationMethods on 9001 (evidence only, no solve); (3) delete devtools-token bypass.
- 😈 Riku: probe must TEE the existing failed login's response, not fire a duplicate auth call.
- HV tee = `Manager.AddPostRequestHook(resty.ResponseMiddleware)` (manager.go:45, verified). Manager-level, fires before client exists; `resp.Body()` intact on 422; re-parse raw JSON to recover dropped `Details`. NO duplicate call.
- Rotated token observable via `Client.AddAuthHandler(func(Auth))` (client.go:53, verified).
- Mailbox password on refresh path: refresh skips SRP/CAPTCHA but NOT key unlock → still prompt for password regardless.

## Decisions
- Session persistence via `NewClientWithRefresh` (manager_auth.go:20) + 0600 JSON session file is the primary CAPTCHA fix; devtools bypass deleted.
- HV probe via `Manager.AddPostRequestHook` tees the raw login-failure response; logs offered methods as `FINDING: HV required — methods=[...]`. No solve implemented yet.
- Mailbox password always prompted (key unlock is independent of session auth).
- AppVersion stays `Other_0.1.0` — honest unofficial string; never spoof official web client.
- HV solve (email-code vs embedded-browser CAPTCHA) deferred to Phase 1, gated on probe data.
- Storage hardening (keyring/age encryption) deferred to Phase 1.
- Scope: read-only, serial, no new dependencies.

## Action items
- [ ] Run spike cold (no session file) to collect `FINDING: HV required — methods=[...]` for this account — gates Phase 1 HV-solve choice. <!-- id:7668 -->
- [ ] Phase 1: implement HV solve — email-code path if `email` in methods (cheap), else embedded-browser CAPTCHA. <!-- id:488c -->
- [ ] Phase 1: evaluate session storage hardening — OS keyring (libsecret) vs age-encrypted file; note headless-box (cartmanjaro/fievel) friction. <!-- id:8a70 -->
