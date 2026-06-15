# 2026-06-15 — Phase 3: two-way sync rehearsal harness

**Started:** 2026-06-15 17:07
**Session:** relay-20260615-161339-1270 (HARD-execute child, id:56c9)
**Attendees:** 🏗️ Archie (architect), 😈 Riku (devil's advocate), ✂️ Petra (productivity),
🗄️ Cassi (sync-vs-backup separation), 🔐 Dario (Proton crypto + key-packet model),
🧪 Tessa (test-harness / fixtures)
**Topic:** Define the **rehearsal round-trip protocol** that must gate any Phase 3 write-back,
its pass/fail criteria, and justify the contacts-before-calendar write ordering — **before**
a single line of write-path code exists. No write path is implemented until this harness exists
and is gated green against a throwaway test account.

## Surfaced discoveries
- [ARCHITECTURE.md §Phasing] "Phase 3 north star: two-way sync; gated on a rehearsal round-trip
  against a test account; contacts-write before calendar-write." — id:e436 umbrella; this note
  refines the *gate*, not the write code.
- [ARCHITECTURE.md §Output contract] The `.meta/**.json` sidecars exist **precisely so a future
  Phase-3 two-way sync can re-derive the exact Proton payload without re-deriving it from the
  lossy standard rendering.** Contacts sidecar = `proton_id + cards + version`; event sidecar =
  `proton_id + calendar_id + shared_key_packet + calendar_key_packet + shared_events +
  calendar_events + version`. The round-trip protocol is built **on the sidecars**, not on the
  `.vcf`/`.ics` rendering.
- [2026-05-29 founding, id:e436] "Build no custom sync/conflict engine at any phase." Phase 3 is
  **write-back of a single edited object**, not a bidirectional conflict-resolving sync loop. The
  rehearsal harness must not smuggle a conflict engine in under the "two-way" label.
- [cmd/backup/contacts.go] Contact decrypt is one call: `contact.Cards.Merge(contactKR)` →
  `vcard.Card`. The card model is flat; the keyring is the user key + address keys.
- [cmd/backup/calendar.go] Event decrypt is multi-stage: per-calendar passphrase decrypt →
  `calKeys.Unlock(calPass)` → per-part split-message decrypt under `SharedKeyPacket` /
  `CalendarKeyPacket` (`decryptPart`), plus detached-signature verify under the **address**
  keyring. Two part-streams (`SharedEvents`, `CalendarEvents`) and two key-packets per event.

## The framing correction (whole decision turns on this)

"Phase 3 = two-way sync" reads as "build a sync engine." It is not. The deliverable this note
gates is far narrower:

> **A rehearsal harness that proves, against a disposable test account, that this tool can take
> one object it previously backed up, apply a known modification, write it back to Proton, and
> read it back byte-identical to the intended state — without corrupting any other object.**

If that round-trip cannot be proven green, **no write path ships at all.** The harness is the
*precondition*, written and gated first; the write path is built only behind a passing harness.
This inverts the usual order on purpose because the failure mode (corrupting a live Proton
account) is irreversible and the blast radius is the user's real contacts and calendar.

Three things are explicitly **out of scope** for Phase 3 and must not creep into the harness:
1. **Conflict resolution.** Round-trip assumes the rehearsal owns the object between read and
   write (no concurrent third-party edit). Conflict handling is a *later* phase with its own gate.
2. **Bulk / continuous sync.** The harness proves *one* object per type, not a daemon.
3. **Creation/deletion.** Phase 3 is **modify-existing** only. Create and delete are separate,
   later, separately-gated operations (delete is the most dangerous and comes last).

## Agenda
1. What must the rehearsal prove — the round-trip definition and its invariants?
2. The pass/fail gate: what counts as "read-back-identical", and against what baseline?
3. Test-account topology: how is a disposable Proton account provisioned and isolated?
4. Why contacts-write lands before calendar-write (the ordering justification).
5. Harness shape: where it lives, what's automatable vs `@manual`, and the reopen/build triggers.

## Discussion

### Agenda 1 — what the rehearsal must prove

🧪 **Tessa:** The round-trip is a five-step protocol per object, run against a **throwaway test
account** (never the user's real account):

1. **Backup** the object via the existing read-only path → canonical `.vcf`/`.ics` + `.meta/`
   sidecar. This is the *baseline snapshot*.
2. **Modify** a single, well-known field locally (e.g. a contact `NOTE:` line; an event
   `SUMMARY:`), producing a *target state* that differs from the baseline in exactly one place.
3. **Write back** the modified object to Proton through the (under-test) write path, re-using the
   sidecar payload to reconstruct the Proton-side encrypted representation.
4. **Re-backup** the same object from Proton (fresh fetch + decrypt), → *observed state*.
5. **Assert** observed == target on the modified field **and** observed == baseline on every
   other field of that object, **and** every *other* object in the account is byte-unchanged from
   its baseline snapshot (the no-collateral-damage invariant).

🔐 **Dario:** The non-obvious half is step 3. Proton does not accept a plaintext `.vcf`/`.ics`.
A contact card must be re-encrypted/re-signed under the user+address keyring; an event must be
re-encrypted under the calendar keyring with a valid `SharedKeyPacket` and a fresh detached
signature under the address key. The sidecar carries the *old* ciphertext and key-packets, but a
**modified** object needs **re-encryption**, not a replay of the stored ciphertext. So the harness
must exercise the *encrypt* path, not just POST the saved bytes back. Replaying the stored
ciphertext would only prove a no-op round-trip (write the same bytes, read the same bytes) — that
is a **weaker, insufficient** test and must be explicitly rejected as the gate (see Rejected).

🗄️ **Cassi:** And it stays *downstream of, and separate from, the backup unit* — same separation
we drew for Phase 2. The rehearsal harness is its own command/target, never chained off
`proton-backup.service`; it points at a test account via its own config, never the keyring entry
that holds the real session.

### Agenda 2 — the pass/fail gate

🏗️ **Archie:** "Read-back-identical" must be defined precisely or it is untestable. Three
candidate baselines, in increasing strictness:

| Baseline | Compares | Verdict |
|----------|----------|---------|
| (a) byte-equality of re-decrypted `.vcf`/`.ics` | rendered standard files | too strict — Proton re-serialization, property reordering, and added server fields (`REV`, `X-PM-*` round-tripping) cause benign diffs that would red-flag a *correct* write |
| (b) **semantic** field-set equality on the modified object + byte-equality on all *other* objects' sidecars | parsed vCard/iCal properties for the target object; raw sidecar bytes for bystanders | **chosen** — tolerates benign reserialization on the object under test while still catching any unintended field change, and is strict (byte) on everything we did *not* mean to touch |
| (c) full-account byte-equality | every file | impossible — the server bumps `REV`/version on *any* write to the modified object |

😈 **Riku:** Baseline (b) has a trap: "semantic field-set equality" must enumerate which
properties are *expected* to change as a side effect of a legitimate write (Proton bumps the
contact `REV` and the object `Version`; an event gets a new `SequenceNumber`/`MODIFIED`). Those go
on an **allow-changed** list in the gate, with a comment justifying each. Anything that changes
and is *not* on the allow-changed list **fails** the rehearsal. The allow-changed list is itself a
reviewed artifact — it is the precise boundary between "expected server bookkeeping" and
"corruption", and the human signs off on it once.

✂️ **Petra:** Pass/fail, then, is mechanical:
- **PASS** ⇔ modified field matches target ∧ all other fields of the object match baseline modulo
  the allow-changed list ∧ every bystander object's sidecar is byte-identical to its baseline.
- **FAIL** on any unexpected field change, any bystander mutation, any decrypt error on re-backup,
  or any write error. **A FAIL is a hard stop — the write path does not ship.**

🔐 **Dario:** One more gate clause specific to crypto: after write-back, the re-fetched object must
**still decrypt and still verify its signature** under the account keyring. A write that produces
an object Proton stores but that we can no longer decrypt/verify is the worst silent-corruption
case — it must be an explicit assertion, not an implicit consequence of the field comparison.

### Agenda 3 — test-account topology

🧪 **Tessa:** The rehearsal runs against a **dedicated throwaway Proton account**, provisioned
manually (Proton signup is CAPTCHA-gated — see id:8a9e; no automated provisioning). Isolation
rules:
- The test account's session lives under a **distinct keyring service** (e.g.
  `proton-moresync-rehearsal`), never the production `proton-moresync` entry. A config/env switch
  selects it; there is no default that could point the harness at the real account.
- The test account is seeded with a **known small fixture** (a handful of contacts, one calendar
  with a few events) so baselines are stable and a bystander-mutation check is meaningful.
- Because login/CAPTCHA/keyring are involved, the **live round-trip step is `@manual`** — it
  cannot run in CI or on the relay host (no account, no network, no crypto material). The harness's
  *pure* parts (modify-a-field, the comparison/gate logic, the allow-changed evaluator) ARE
  offline-unit-testable over fixtures and SHOULD be.

🗄️ **Cassi:** The split mirrors the existing test discipline (ARCHITECTURE / ROADMAP "Notes for
executors": no network/crypto/keyring in unit tests; those are `@manual` BDD). So Phase 3's build
items will be two-layered: offline-unit-tested gate logic + an `@manual` Gherkin runbook for the
live round-trip against the rehearsal account.

### Agenda 4 — contacts-write before calendar-write (the ordering justification)

🔐 **Dario:** This ordering is the safety-critical judgment the item flags, and the code makes the
reason concrete:

- **Contact write-back is the simpler, lower-risk key model.** Decrypt is a single
  `Cards.Merge(contactKR)`; the inverse is one encrypt/sign under the **user + address** keyring
  the tool already holds. One object, one keyring, one ciphertext blob. The failure surface is
  small and the blast radius is one contact card.
- **Calendar write-back is materially harder and higher-risk.** An event requires: the
  per-calendar **passphrase** decrypt → `calKeys.Unlock` → re-encryption under the **calendar**
  keyring (not the user keyring) → a correct `SharedKeyPacket` (and possibly `CalendarKeyPacket`)
  → a **detached signature under the address key** → and it spans **two** part-streams
  (`SharedEvents` canonical body + `CalendarEvents` personal annotations). More keys, more packets,
  more parts, more ways to write an object that stores but won't decrypt. A botched calendar write
  can also cascade (shared calendars, attendees) in ways a contact cannot.

🏗️ **Archie:** So the ordering is **risk-graded incrementalism**: prove the round-trip gate green
on the *simplest* object type first (contacts), establishing the harness, the allow-changed list,
and the encrypt-path confidence on the cheap model. Only once contacts round-trip green do we
extend the *same* harness to the calendar key-packet model. We never debug the hard crypto model
and the harness itself at the same time.

😈 **Riku:** And — non-negotiable — **delete and create are not in this ordering at all.** Phase 3
is modify-existing only. Create comes after both modify paths are green; delete comes last and gets
its own gate, because a wrong delete is the only fully-unrecoverable operation. State that
explicitly so a future executor does not "while I'm here" a delete path into a modify item.

### Agenda 5 — harness shape, location, triggers

🏗️ **Archie:** The harness is **not built now** — this note is the design and the gate spec. The
build is two sized follow-ups, but they are **gated, not yet spawned as ROUTINE**, because:
- they require a live throwaway account (a human provisioning step), and
- the write path is HARD (crypto re-encryption correctness), not ROUTINE.

So the build items stay as **design-gated HARD/manual successors**, reopened by a concrete trigger,
not dropped into the executor queue. The reopen trigger is: **a throwaway test account exists and
the user wants Phase 3** — until then there is no payoff to building a write path for a single-user
backup tool whose Phase-1 read-only role is already complete.

✂️ **Petra:** Concretely, when triggered, the work decomposes as:
1. **Harness scaffold (offline-testable):** a `rehearsal` command/target that (a) takes a baseline
   snapshot, (b) applies a single-field modification to a chosen object, (c) implements the
   comparison/gate incl. the allow-changed evaluator — all unit-tested over fixtures, no network.
2. **Contacts write-back path + live `@manual` round-trip** behind the gate.
3. **Calendar write-back path + live `@manual` round-trip** behind the gate (only after step 2 is
   green).

🧪 **Tessa:** The `@manual` Gherkin for the live round-trip belongs in `features/` alongside the
existing manual scenarios; the offline gate logic gets normal `cmd/backup/*_test.go` red-green
coverage when its build item is opened.

## Decisions

- **Phase 3 is gated on a rehearsal round-trip harness, defined here; no write path ships before
  the harness is green against a throwaway test account.** The harness is the precondition, built
  first. <!-- id:56c9 -->
- **The round-trip protocol is the 5-step per-object loop:** backup (baseline) → modify one field
  (target) → write-back (via re-encryption, not ciphertext replay) → re-backup (observed) → assert.
- **Pass/fail gate = semantic baseline (b):** modified field == target; all other fields of the
  object == baseline **modulo a reviewed allow-changed list** (server bookkeeping: contact `REV`,
  object `Version`, event `SequenceNumber`/`MODIFIED`); **every bystander object's sidecar
  byte-identical** to baseline; and the re-fetched object **still decrypts and verifies its
  signature**. Any unexpected change, bystander mutation, decrypt/verify failure, or write error =
  FAIL = no ship.
- **The write-back step must exercise the encrypt/sign path** (re-encrypt the modified object under
  the correct keyring), **never replay the stored ciphertext** — a replay only proves a no-op.
- **Test-account isolation:** dedicated throwaway Proton account under a **distinct keyring
  service** (`proton-moresync-rehearsal`), never the production entry; no default path can reach the
  real account; account provisioned manually (CAPTCHA-gated); the live round-trip is `@manual`
  (no CI/relay-host run); the modify + gate logic is offline-unit-testable over fixtures.
- **Separation preserved:** the rehearsal harness is its own command/target, never chained off
  `proton-backup.service`; same backup-vs-everything-else split as Phase 2.
- **Ordering: contacts-write before calendar-write**, justified by key-model complexity — contacts
  are a single user+address-keyring encrypt of a flat card; calendar events need per-calendar
  passphrase unlock, calendar-keyring re-encryption, key-packets, detached address-key signature,
  and span two part-streams. Prove the gate on the simple model first; extend the *same* harness to
  the hard model only once contacts round-trip green.
- **Scope fence: modify-existing only.** Conflict resolution, bulk/continuous sync, and
  create/delete are NOT Phase 3. Create follows both modify paths; delete is last with its own gate.

### Rejected alternatives
- **Ciphertext-replay round-trip** (POST the stored sidecar ciphertext back unchanged) — only
  proves write-the-same-bytes-read-the-same-bytes; never exercises re-encryption, which is the
  actual risk surface of a *modified* object. Insufficient as the gate.
- **Byte-equality of rendered `.vcf`/`.ics` as the pass criterion** (baseline (a)) — Proton
  re-serialization and benign property reordering/added server fields would red-flag a correct
  write. Replaced by semantic field-set equality on the object under test.
- **Full-account byte-equality** (baseline (c)) — impossible; the server bumps `REV`/`Version` on
  any legitimate write to the modified object.
- **Testing write-back against the user's real account** — the irreversible-corruption failure mode
  is the whole reason for the gate; a dedicated throwaway account under a distinct keyring service
  is mandatory.
- **Calendar-write first (or both at once)** — debugging the hard key-packet/calendar-keyring model
  simultaneously with bringing up the harness itself doubles the unknowns; risk-graded
  incrementalism puts the simple contact model first.
- **Bundling create/delete into Phase 3** — delete is the only fully-unrecoverable operation;
  smuggling it into a "two-way sync" item would put the most dangerous path behind the weakest
  review. Explicitly fenced out.
- **Building the harness/write path speculatively now** — no throwaway test account exists and the
  Phase-1 read-only role already meets the single-user need; building a write path with no account
  to gate it against produces unverifiable code. Reopen on trigger instead.

## Build triggers (gated, NOT yet spawned)

The two-way write path is **design-complete but build-gated**. Reopen trigger:

> **A disposable Proton test account exists AND Phase 3 write-back is wanted.**

When triggered, the build decomposes (all behind the gate above):
1. **Rehearsal harness scaffold** — baseline snapshot + single-field modify + comparison/gate
   (incl. allow-changed evaluator); offline-unit-tested over fixtures, no network. [HARD/ROUTINE
   split decided at build time; the gate logic is ROUTINE-able, the encrypt paths are HARD.]
2. **Contacts write-back path + `@manual` live round-trip** behind the gate.
3. **Calendar write-back path + `@manual` live round-trip** behind the gate (only after #2 green).

No id is minted for these now — minting an executor id for work that has no account to run against
would create a perpetually-blocked ROADMAP item. They are recorded here as the gated successor set;
a future strong session mints sized ids when the trigger fires.

## Status of id:56c9
Design rendered. The Phase-3 gate (rehearsal round-trip protocol, pass/fail criteria, test-account
isolation, contacts-before-calendar ordering) is fully specified, with the write path explicitly
build-gated behind a throwaway-account reopen trigger. No production code, per the item's
acceptance ("No write path is implemented until the rehearsal harness exists and is gated green").
id:56c9 is closed by this note; the successor build items are gated, not yet sized.
