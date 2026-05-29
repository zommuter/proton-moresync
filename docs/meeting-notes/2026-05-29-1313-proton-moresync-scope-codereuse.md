# 2026-05-29 — proton-moresync: scope & code-reuse for non-mail Proton sync

**Started:** 2026-05-29 13:13
**Session:** e6493b85-e4da-4000-b4ff-95892ef619f6
**Attendees:** 🏗️ Archie (architect), 😈 Riku (devil's advocate), ✂️ Petra (productivity), 🗄️ Cassi (sync-vs-backup separation), 🔐 Dario (new — DAV protocols + E2E-encrypted-PIM API reuse)
**Topic:** Decide reasonable scope and a code-reuse strategy for backing up / syncing Proton contacts, calendar (and possibly more) given that Mail Bridge only exposes IMAP.

## Research baseline

- **Mail** already covered: `protonmail-bridge` installed; `~/src/zkm` pulls mail via mbsync into a maildir. Out of scope.
- **hydroxide** (emersion, Go): Proton bridge supporting IMAP/SMTP/**CardDAV (contacts)**. **No CalDAV/calendar.** Active (v0.2.32, 2026-05-10). Implements full E2E contact decryption in its `protonmail` Go package.
- **go-proton-api** (official Proton, Go): the library Bridge uses — SRP auth, 2FA, key unlock, contacts, and a `calendar.go` with raw calendar endpoints. Event *decryption* is not a documented turnkey helper.
- **rclone protondrive backend**: exists but beta + currently unmaintained (candidate for "unsupported"). Drive backup is "mostly solved elsewhere" — don't depend on it here.
- **Proton Pass**: official JSON/CSV export. Manual, solved.
- **Hard truth:** contacts & calendar are E2E-encrypted; the server is blind by design — which is *why* no CalDAV exists. Calendar is the one piece with **no reusable bridge**: the forcing function for every architecture choice.

## Agenda
1. Scope — what does this project actually sync?
2. Backup vs one-way vs two-way — v1 target + phasing.
3. Code-reuse + language/architecture + the zkm relationship.
4. Sync engine — build vs reuse (folded into 3).

## Discussion

### Agenda 1 — Scope
Petra: subtract what's solved (mail/drive/pass); contacts + calendar are the only types with no turnkey local path, and they're the N=2 consumers of any shared auth/crypto layer. Dario: not arbitrary — both share the same substrate (SRP auth → user-key unlock → per-object gopenpgp decryption); Drive is a different API surface already wrapped by rclone. Cassi: rclone backend being shaky argues for *not depending on it*, not re-implementing — Drive backup belongs in the infra layer. Riku: name the deferral honestly (notes/wallet exist) — build the core so a 3rd E2E type *could* be added, but don't build it; and confirm goal is backup-or-sync (→ agenda 2). Archie: separate *project scope* (this = E2E-PIM bridge) from *coverage* (umbrella also includes mail via zkm, drive via rclone, referenced from a runbook).

### Agenda 2 — Backup vs one-way vs two-way
Dario: the asymmetry is the whole decision — reading E2E data is non-destructive; *writing back* re-encrypts + signs and can corrupt the canonical copy on a server that can't validate semantics. Cassi: sync vs backup are different *concerns*, not two dial settings; backup is the cheap 80% + DR floor and should ship first, standalone. Archie: phases stack on shared crypto — P1 read-only backup → P2 read-only live-view → P3 two-way. Riku: P3 gate must include a *rehearsal* (write to a throwaway test calendar/group, round-trip diff, confirm Proton web renders it); contacts-write before calendar-write (calendar write has no reference impl). Dario: in P1, persist each object's Proton ID + original ciphertext/version alongside the readable form — cheap, and the difference between "backup" and "backup you can sync from."

### Agenda 3 — Language / architecture + zkm
Dario: "don't reinvent the API" ⇒ **go-proton-api** (Go); Python has only auth-only `proton-python-client`, so Python = reinventing the whole client + calendar key-unwrap. Archie: maps 1:1 onto the zkm split — a Go tool plays the **mbsync role** (fetch+decrypt+emit), a future zkm plugin plays the **zkm-eml role** (ingest the tree); the backup tree is the contract. Cassi: keeps backup and corpus-ingest separate; run the Go tool on the mbsync-hook pattern. Petra: the zkm plugin is *downstream* (lives in zkm later), out of scope here. Riku: concede Go but require a **spike** decrypting one contact + one event end-to-end before committing — calendar key-unwrap may be undocumented. Dario: later phases reuse Radicale/vdirsyncer/DAVx5; no custom sync engine ever (= agenda 4).

User refinement: standardize output so *generic* zkm-vcard / zkm-calendar plugins consume it with no Proton adaptation. Dario: `.vcf`/`.ics` stay vanilla RFC 6350 / RFC 5545; Proton-specifics go in an out-of-band `.meta/<uid>.json` sidecar keyed by standard UID — generic consumers read only the standard files; the write-back path reads the sidecar. Cassi: makes `zkm-vcard`/`zkm-calendar` genuinely source-agnostic (Proton today, exported Google tomorrow) — proper N≥2 framing, and it lives in zkm.

## Decisions

- **Scope = contacts + calendar only.** Mail (zkm), Drive (rclone), Pass (export) explicitly out of scope, referenced from a backup runbook. Core must not *preclude* a future 3rd E2E type but gets **no speculative abstraction** now. <!-- id:4438 not an action; scope statement -->
- **Phased, backup-first.** v1 = Phase 1 read-only backup. Phase 2 = read-only live-view. Phase 3 = two-way (north star), gated on a rehearsal round-trip against a throwaway test calendar/group; **contacts-write before calendar-write**. No conflict engine / DAV server built speculatively. *Out of scope for v1: anything that writes to Proton.*
- **Core = standalone Go CLI** on **go-proton-api** + hydroxide's `protonmail` contact-decryption, gated by a spike. *Out of scope: pure-Python reimplementation of the Proton API.*
- **Output contract = standards-compliant `.vcf` (RFC 6350) + `.ics` (RFC 5545)**, one object per file keyed by standard UID, git-versioned tree (`contacts/*.vcf`, `calendar/<cal>/*.ics`); **Proton-specifics (object ID, ciphertext, version) in out-of-band `.meta/<uid>.json` sidecar, never inline.** *Out of scope: Proton-coupled output formats.*
- **Sync engine: reuse DAV tooling** (Radicale / vdirsyncer / DAVx5) for later phases; **build no custom sync/conflict engine** at any phase. *Out of scope: a bespoke sync engine.*
- **zkm ingestion is downstream, not in this repo.** Generic `zkm-vcard` / `zkm-calendar` plugins live in `~/src/zkm` and consume the standard tree unaware of Proton.

## Action items

- [ ] **Spike: prove end-to-end decrypt.** Go program on go-proton-api that authenticates (SRP + 2FA) and decrypts exactly **one contact** and **one calendar event**; record whether calendar key-unwrap is turnkey or needs hand-assembly. Gates the rest. Contract: a future test runs the spike against a live account and asserts a decrypted vCard + VEVENT are produced. <!-- id:4438 -->
- [ ] **Define the output contract.** Standard `.vcf` (RFC 6350) / `.ics` (RFC 5545), one object/file by UID; Proton-specifics in `.meta/<uid>.json` sidecar. Contract: a generic vCard/iCal parser reads the standard files with zero Proton-specific code; the sidecar carries object ID + ciphertext + version. <!-- id:d3bb -->
- [ ] **Phase 1 build: full read-only backup.** Fetch + decrypt all contacts → `contacts/*.vcf`; all calendars/events → `calendar/<cal>/*.ics`; git-versioned tree + sidecars; crib hydroxide's `protonmail` package for contact decryption. Contract: re-run is idempotent and a wiped tree is fully reconstructable from the account. <!-- id:5416 -->
- [ ] **Scheduling/trigger runner.** mbsync-hook pattern — script in repo, symlink into `.git/hooks` (or a systemd timer), output piped through `systemd-cat`. Contract: a scheduled run refreshes the tree and is journald-queryable. <!-- id:8af0 -->
- [ ] **Phase 2/3 north star (design later).** P2 live-view via Radicale/vdirsyncer; P3 two-way gated on rehearsal round-trip, contacts-write before calendar-write. Contract: no write to Proton ships before a test-account round-trip (write → re-fetch → diff) passes and Proton web renders the object. <!-- id:e436 -->
- [ ] **Backup runbook.** Top-level doc referencing zkm (mail), rclone (drive), Pass export for full-Proton coverage; this repo owns only contacts+calendar. Contract: the runbook lists every Proton data type and which tool covers it. <!-- id:22b4 -->
- [ ] **Cross-repo flag (belongs in `~/src/zkm`, not built here):** generic `zkm-vcard` / `zkm-calendar` ingestion plugins consume the standard tree. Contract: a one-line item in zkm's TODO; source-agnostic (Proton is one producer). <!-- id:eef8 -->
