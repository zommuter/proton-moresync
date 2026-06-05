# 2026-06-05 — QR login: session-fork as an auth bootstrap path

**Started:** 2026-06-05 11:44
**Session:** a4ea5c7e-9df8-4aab-97ec-835a024343ab
**Attendees:** 🏗️ Archie (architect), 😈 Riku (devil's advocate), ✂️ Petra (productivity), 🔐 Dario (E2E PIM API reuse — re-onboarded from registry)
**Topic:** The Android Mail app offers "sign in with QR code"; does this contradict the prior "QR login not in go-proton-api" close (id:761e), and is it a viable CAPTCHA-free auth bootstrap for the headless backup CLI?

## Surfaced discoveries
- [2026-06-03 proton-moresync] CAPTCHA frame-ancestors wall — browser-embedded CAPTCHA solve infeasible; session persistence makes CAPTCHA one-time.
- [2026-06-04 proton-moresync] Session scope 9101 issue — refreshed sessions can 403 on GetSalts.

## Agenda
1. Does the Android QR option overturn the id:761e close ("QR login not in go-proton-api")?
2. Is hand-rolling session-fork worth it vs. the current CAPTCHA-once + persist?
3. Is borrowing an official ChildClientID acceptable risk?
4. Verdict: implement now / spike / document-and-defer.

## Research grounding (pre-meeting, 2 agents)
- **QR sign-in IS session-fork (ForkType LOGIN='3').** Origin = already-authenticated device (Android app) scans QR; destination displays QR carrying `selector` + `sk` (raw AES-GCM key) out-of-band.
- Fork consume: `GET auth/v4/sessions/forks/{selector}` → `{UID, RefreshToken, AccessToken, LocalID}`; AES-GCM Payload (encrypted under `sk`) yields `keyPassword` → full key-unlocked session. **No SRP / 2FA / CAPTCHA on the child path.**
- Endpoints live in WebClients (`packages/shared/lib/api/auth.ts`, `packages/shared/lib/authentication/fork/consume.ts`). Not a public spec.
- **Absent from go-proton-api v0.4.0** — grep-confirmed zero fork/selector/QR/device code.
- `ChildClientID` must be an allowlisted Proton app ID (`ForkableApps` set) — a 3rd-party client has no sanctioned ID.
- AuthDevice/EDM is a separate, heavier SSO/org "trusted device" flow — not the consumer QR feature.

## Discussion

### Items 1 & 2 — reframe the close; fork vs CAPTCHA-once

🏗️ **Archie:** id:761e was correct but under-scoped — checked the library, not the protocol. Consumer QR = session-fork (`auth/v4/sessions/forks`, ForkType LOGIN='3'). Android app is the *origin* (authorizes); QR-displaying device is *destination*. Consume → `UID+RefreshToken+keyPassword`, no SRP/2FA/CAPTCHA on child.

😈 **Riku:** It relocates CAPTCHA, doesn't kill it. Current bootstrap = solve-CAPTCHA-once + persist + refresh. Fork = scan-QR-once + persist + refresh. Identical tail. Delta is "phone-scan vs browser-CAPTCHA" — what justifies new code?

🔐 **Dario:** Delta is narrow but real: (1) sidesteps `verify.proton.me` frame-ancestors CSP wall + chromedp entirely; (2) delivers `keyPassword` directly, skips SRP/salt/2FA + the 9101 GetSalts scope issue; (3) it's the sanctioned cross-device mechanism. BUT not in go-proton-api — hand-roll AES-GCM payload decrypt + forks endpoints from WebClients TypeScript. Reinvention risk flagged at founding meeting.

✂️ **Petra:** Phase 1 is DONE — backup works today via CAPTCHA-once + keyring. This unblocks nothing; it's an alternative bootstrap trading a solved browser interaction for an unbuilt phone one. Net-new surface: types + AES-GCM + borrowed ChildClientID + undocumented WebClients internals that break on any Proton release. N=1. Defer.

🏗️ **Archie:** Concede not urgent, but the CAPTCHA path isn't fully robust either (chromedp + mutable hvToken). Fork is arguably more durable for unattended re-bootstrap. Not urgent though.

😈 **Riku:** ChildClientID is disqualifying for "implement now." Must present an ID from Proton's ForkableApps allowlist — we have none, so we'd impersonate an official client. Brittle (IDs rotate, payload versions pv1–3) + ToS-adjacent for a "well-behaved standards-only backup" tool. Minimum evidence to change: a sanctioned 3rd-party ID, or proof that an `Independent` fork with a generic ID is accepted.

### User intervention (Zommuter)
> "The CAPTCHA flow is okay for me for now, but should the tool find wider adoption, asking for a console js injection is definitely not gonna cut it."

Defer accepted, but the revisit trigger is **adoption/UX**, not breakage.

### Items 3 & 4 — gating risk + verdict (post-intervention)

🔐 **Dario:** Console-JS-injection bootstrap is fine for n=1 but a non-starter for anyone else — opaque, scary, breaks on any Proton web-bundle change. The eventual answer isn't "nicer CAPTCHA" — it's "replace the hostile bootstrap with QR-scan," which is genuinely good consumer UX (open app, scan, done).

✂️ **Petra:** This sharpens the defer, it doesn't weaken it. The trigger is not "CAPTCHA breaks" (works fine for n=1) but "first external user" — that's when console-injection fails and QR-fork is the designated replacement. Document it as *the planned adoption path*, not a curiosity.

😈 **Riku:** Record ChildClientID as the **gating unknown**. The whole path is worthless if Proton rejects a fork consumed by a non-allowlisted client. First action when picked up = confirm whether an `Independent` fork with a generic/borrowed client ID is accepted. One yes/no decides real-vs-fantasy. No production code before the probe answers it.

🏗️ **Archie:** Clean handoff — revisit-on-first-external-user; first action is a throwaway feasibility probe of ChildClientID acceptance, not implementation; protocol facts are captured so the probe starts from research, not zero.

🔐 **Dario:** Architecturally it stays a *bootstrap* concern — the fork yields the same `UID + RefreshToken` we already persist to the keyring, slotting behind existing `saveSession`/`NewClientWithRefresh` in `cmd/backup/auth.go`. It replaces the cold-login front-end only; no churn to the refresh/persist path.

## Decisions

- **Reframe id:761e.** The prior close "QR-code login not in go-proton-api" was accurate-but-under-scoped (checked library, not protocol). Correct framing: *Proton's consumer QR sign-in is the session-fork flow (`auth/v4/sessions/forks`, ForkType LOGIN='3'); it is hand-rollable but absent from go-proton-api v0.4.0 and gated on an allowlisted `ChildClientID`.*
- **Defer implementation.** CAPTCHA-once + keyring persistence stays the live auth path. Phase 1 backup works today; QR-fork unblocks nothing now. Out of scope: any production fork code, any AuthDevice/EDM (separate SSO/org flow, not the consumer QR feature).
- **Revisit trigger = first external user / wider adoption**, NOT "CAPTCHA breaks." The console-JS-injection bootstrap is the component that fails to scale; QR-scan is the adoption-grade replacement UX.
- **Gating unknown = ChildClientID acceptance.** First action when revisited is a *throwaway feasibility probe*: does Proton accept an `Independent` fork consumed with a generic/borrowed client ID? Yes/no decides whether the path is real. No production code before the probe answers it.
- **Architecture constraint (for whoever builds it):** fork is a bootstrap-only replacement for cold-login; it yields the same `UID + RefreshToken` persisted today, slotting behind `saveSession`/`NewClientWithRefresh` in `cmd/backup/auth.go`. Out of scope: touching the refresh/persist path.

## Action items
- [x] Reframe id:761e in TODO.md: update the closed item's note to the session-fork framing above — "hand-rollable via `auth/v4/sessions/forks` (ForkType LOGIN='3'), not in go-proton-api v0.4.0, gated on allowlisted ChildClientID; revisit trigger = first external user." <!-- id:33d3 -->
- [ ] (Deferred, adoption-triggered) Throwaway spike: probe whether an `Independent` fork with a generic/borrowed `ChildClientID` is accepted by `GET auth/v4/sessions/forks/{selector}`; decrypt AES-GCM Payload under `sk` → `keyPassword`. Goal: documented yes/no on ChildClientID acceptance; no production code. <!-- id:96d7 -->
