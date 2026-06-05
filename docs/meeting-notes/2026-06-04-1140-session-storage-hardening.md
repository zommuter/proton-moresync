# 2026-06-04 — Session storage hardening (non-interactive auth)

**Started:** 2026-06-04 11:40
**Session:** db6a5dc6-9452-4c20-867c-c6b7dacad2d8
**Mode:** Class 2 planning record (no meeting was held — plan-mode output)
**Topic:** Move secrets out of plaintext session.json into the OS keyring; enable unattended backup runs on <workstation>.

## Context

TODO `8a70` ("Phase 1: session storage hardening") is the prerequisite for the scheduling/trigger runner (`8af0`). Two prompts blocked unattended runs:
1. Refresh token stored in plaintext `~/.local/state/proton-moresync/session.json`.
2. Mailbox password re-prompted every run via `readSecret` at `main.go:38`; never persisted.

## Plan

**Backend:** `github.com/zalando/go-keyring` (pure-Go Secret Service client) on <workstation> (XFCE with `gnome-keyring-daemon`). Headless backend explicitly deferred — <gateway>/<backup-host> have no Secret Service daemon.

**Secrets stored:** the *salted* key passphrase (output of `SaltForKey`) rather than the raw Proton password — narrower blast radius. The refresh token was already in the session file; it migrates to the keyring automatically.

**Key design decisions:**
- `getPassword` is now a lazy closure — only called on fresh login or salt fallback, never on the happy path (session reuse + stored salted passphrase).
- `connect()` returns `nil` mailboxPass on session-reuse path, signalling to the caller to use the stored salt.
- The `unlockKeys()` helper tries the stored salted passphrase first; on failure (key rotation) it logs WARN and falls back to the full `GetSalts` + `SaltForKey` path.
- 9101 locked-session: `purgeSession()` deletes the keyring tokens and returns an error prompting the user to re-run.

## Implementation findings

- `proton.Unlock` returns `map[string]*crypto.KeyRing` (not a slice) — fixed the return type in `unlockKeys`.
- Migration run confirmed: session.json was imported to keyring and deleted; `session reused` appeared immediately after.
- `salted_key_pass` entry is written on the first successful interactive run (when `GetSalts` / `SaltForKey` / `Unlock` all succeed).
- `secret-tool search --all service proton-moresync` confirms `uid` + `refresh_token` are in the keyring post-migration.
- `getUserAndAddresses` was extracted but kept minimal (no 9101 retry — that path is handled by returning an error and asking the user to re-run, same as the original code post-purge).

## Decisions

- **Keyring backend only (<workstation>).** No age/headless backend, no `SecretStore` interface — single machine, N=1.
- **Store salted key passphrase, not raw password.** Documented escape hatch: swap one line in `saveSaltedKeyPass`/`loadStoredSecrets` to store raw password if salt approach proves brittle.
- **Lazy password acquisition.** `getPassword` closure; no prompt on session-reuse + valid stored salt.
- **Self-healing fallback.** Stale salt after key rotation logs WARN and re-derives automatically.
- **One-time migration.** `migratePlaintextSession()` at startup imports `session.json` if keyring is empty, then removes the file. Out of scope: spike (`cmd/spike/`) left untouched.

## Action items

- [x] Implement keyring backend + migration (`cmd/backup/secrets.go`) <!-- id:8a70 -->
- [ ] Run backup interactively once to store `salted_key_pass` in keyring; then verify unattended run with `</dev/null` <!-- id:8a70 -->
- [ ] Scheduling/trigger runner (`8af0`) is now unblocked once the unattended run is verified
