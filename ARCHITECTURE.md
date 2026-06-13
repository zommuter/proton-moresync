# Architecture — proton-moresync

Design decisions for the Proton contacts + calendar backup CLI, each with the
rationale and the alternatives that were rejected. The narrative log of *how* the
decryption was figured out lives in `docs/meeting-notes/`; this file records *what*
was decided and *why*.

## Scope: the "mbsync role" only

**Decision.** This tool fetches, decrypts, and writes standard `.vcf`/`.ics` files
plus Proton-specific `.meta/` sidecars. It does NOT ingest, index, search, or sync
back.

**Rationale.** Mirrors the existing mail setup (`protonmail-bridge` + `mbsync` produce
a maildir; downstream tools consume it). One job, done well, with a stable on-disk
contract that any consumer can read. Keeps the dependency surface (and the blast
radius of a bug) small.

**Rejected.** (a) A monolithic "Proton everything" tool — too broad, couples
unrelated failure domains. (b) Writing directly into the downstream corpus tool's
format — would couple this repo's release cadence to the consumer and leak
Proton-specifics into a foreign schema.

## Output contract: standards-only files + out-of-band sidecars

**Decision.** `.vcf` (RFC 6350) and `.ics` (RFC 5545) files contain ONLY standard
fields — no Proton extensions inline. All Proton-specific data (object IDs, raw
encrypted card/event ciphertext, version) lives in parallel `.meta/**.json` sidecars.
Filenames are the standard vCard/iCal `UID`.

**Rationale.** The `.vcf`/`.ics` files must be loadable by any standards-compliant
client (DAVx5, Thunderbird, Apple Contacts) with zero surprises. Keeping ciphertext
in sidecars means a future Phase-3 two-way sync can re-derive the exact Proton
payload without re-deriving it from the lossy standard rendering. The `UID`-as-
filename gives a stable identity and idempotent re-runs (same object → same file →
no spurious git churn).

**Rejected.** (a) Embedding Proton `X-PM-*` fields inline — breaks non-Proton clients
and pollutes the canonical files. (b) A single combined database (sqlite) — defeats
the git-versioned, human-diffable, client-readable goal. (c) Hashes as filenames —
opaque, breaks idempotence when an object's content changes but identity does not.

## Idempotent git-versioned tree

**Decision.** The output directory is its own git repo. The runner
(`proton-backup-sync.sh`) commits only when `git status --porcelain` shows changes,
and pushes only when an SSH key is loaded (`BatchMode=yes`, never blocks).

**Rationale.** Daily unattended timer runs must never prompt, never produce empty
commits, and never hang on a missing SSH agent. The commit history becomes a natural
audit trail of what changed in the Proton account over time.

**Rejected.** Always-commit-with-timestamp — produces noise commits on no-op runs and
makes "did anything actually change?" hard to answer.

## Auth: SRP + 2FA via go-proton-api, session persisted in the OS keyring

**Decision.** Use the official `go-proton-api` (Bridge library) for SRP/2FA. Persist
`uid` + `refresh_token` + `salted_key_pass` + `mailbox_pass` in the OS keyring
(`zalando/go-keyring`, service `proton-moresync`). A legacy plaintext
`session.json` is migrated into the keyring once, then deleted.

**Rationale.** The hard part of Proton is the crypto, and go-proton-api is the
maintained reference — re-implementing SRP is a security liability. Keyring storage
lets the daily timer run fully unattended after a one-time interactive login.
Storing the salted passphrase (not just the password) lets the happy path skip a
`GetSalts` network call and key re-derivation entirely.

**Rejected.** (a) Plaintext session file (the migrated-from state) — secrets at rest
on disk. (b) Re-deriving the salted passphrase every run — extra network round-trip
and slower; only used as the fallback when the stored passphrase fails (key
rotation). (c) Hand-rolling SRP from the WebClients TypeScript — security risk, large
surface, no payoff over the official library.

## CAPTCHA / human-verification: manual one-time composite-token capture

**Decision.** On an HV challenge, open `verify.proton.me` via `xdg-open` and walk the
user through a DevTools console snippet that captures the composite token
(`hvToken:prefix+hcaptchaResponse`), which is then pasted back at the prompt. Session
persistence makes this a one-time event.

**Rationale.** Proton's CAPTCHA iframe sets `frame-ancestors` CSP that blocks
embedding (see meeting note `2026-06-03-1614-captcha-frame-ancestors-wall.md`), so
the token cannot be intercepted by a hosted proxy. A bare `hvToken` is rejected — the
composite token is mandatory. Because the refreshed session is reused, the user only
ever does this on the first login from a new client.

**Rejected.** (a) Embedded webview / headless chromedp automation — fragile against
Proton CSP and hCaptcha bot-detection; high maintenance. (b) Intercepting the token
via a local proxy — blocked by `frame-ancestors`. Both were investigated and shelved
(see meeting notes `2026-06-03-1447` and `2026-06-04-0843`).

## Calendar decryption: inline `decryptPart`, not `CalendarEventPart.Decode`

**Decision.** Decrypt each event part with a local `decryptPart` helper rather than
the upstream `CalendarEventPart.Decode`.

**Rationale.** The upstream `Decode` uses a **value receiver**, so the decrypted data
it assigns is discarded when the method returns — it silently yields nothing. The
local helper replicates the (otherwise turnkey) logic: key-packet split-message
decrypt under the calendar keyring, optional detached-signature verify under the
address keyring.

**Rejected.** Patching/forking go-proton-api for this one method — heavier than a
~30-line local helper and adds a fork to maintain.

## QR-login / session-fork: deferred, adoption-gated

**Decision.** The QR / session-fork login flow (`auth/v4/sessions/forks`) is
investigated and documented but NOT built. Reopen trigger: the first external user.

**Rationale.** Session-fork would eliminate the CAPTCHA bootstrap entirely, but it is
gated on an allowlisted `ChildClientID` that third-party clients do not have, and is
absent from go-proton-api v0.4.x. For a single-user backup tool the one-time CAPTCHA
is a tolerable cost; the work only pays off once there is a user who cannot do the
console-JS bootstrap. See `docs/meeting-notes/2026-06-05-1144-qr-login-session-fork.md`.

**Rejected (for now).** Building the fork flow speculatively — no payoff without an
allowlisted client ID and no current second user.

## Phasing

| Phase | Status | Description |
|-------|--------|-------------|
| Spike | done | End-to-end decrypt proof (contacts + calendar), session persisted. |
| 1 | done | Read-only backup — full fetch+decrypt to git-versioned tree. |
| 2 | deferred | Read-only live view via Radicale / vdirsyncer / DAVx5. |
| 3 | north star | Two-way sync; gated on a rehearsal round-trip against a test account; contacts-write before calendar-write. |

**Rationale for the gate.** Two-way sync can corrupt a live Proton account; it is
not attempted until a throwaway test account proves the round-trip (read → modify →
write → read-back-identical). Contacts-write lands before calendar-write because the
contact card model is simpler and lower-risk than the calendar key-packet model.
