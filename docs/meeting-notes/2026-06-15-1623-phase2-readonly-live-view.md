# 2026-06-15 — Phase 2: read-only live view (Radicale / vdirsyncer / DAVx5)

**Started:** 2026-06-15 16:23
**Session:** relay-20260615-161339-1270 (HARD-execute child, id:d407)
**Attendees:** 🏗️ Archie (architect), 😈 Riku (devil's advocate), ✂️ Petra (productivity), 🗄️ Cassi (sync-vs-backup separation), 🔐 Dario (DAV protocols + E2E-PIM reuse)
**Topic:** Pick ONE serving path for Phase 2 "read-only live view" so that the backup tree
(`contacts/*.vcf`, `calendar/<cal>/*.ics`) is reachable by a standard CalDAV/CardDAV client
(phone, Thunderbird, Apple Contacts) — without building a sync engine and without a write path.

## Surfaced discoveries
- [2026-05-29 proton-moresync founding] "Sync engine: reuse DAV tooling (Radicale / vdirsyncer /
  DAVx5); build no custom sync/conflict engine at any phase." — id:e436, the umbrella this item
  refines.
- [2026-05-29 proton-moresync] Output contract is *already* standards-only: `.vcf` (RFC 6350) /
  `.ics` (RFC 5545), one object per file keyed by UID, Proton-specifics in `.meta/` sidecars.
  Phase 2 needs **no format work** — the on-disk tree is already DAV-servable as-is.
- [ARCHITECTURE.md] Scope is the "mbsync role" only: fetch + decrypt + emit. Phase 2 must not
  leak into ingestion, search, or write-back.

## The framing correction (whole decision turns on this)

The three names in the deferred-phase line are **not three alternatives at one layer** — they
sit at three different layers of the same DAV stack, and the founding note listed them together
only as "the DAV ecosystem we will reuse":

| Tool | Layer | Role w.r.t. the backup tree |
|------|-------|------------------------------|
| **Radicale** | CalDAV/CardDAV **server** | Publishes a local collection *over* DAV so any client can mount it. |
| **DAVx5** | Android DAV **client** | *Consumes* a DAV server; maps collections into Android's native Contacts/Calendar. |
| **vdirsyncer** | client-side **sync** between a DAV server and a local `vdir` (a `.vcf`/`.ics` tree) | Bridges a DAV endpoint and an on-disk vdir — the *opposite* direction we need here. |

So "pick ONE" is really "pick the **serving layer**" (Radicale vs vdirsyncer-as-server vs a
trivial static DAV) and note that the consumer side (DAVx5 / Thunderbird / Apple) is
client-of-choice, out of our control, and needs no decision from us.

## Agenda
1. What does "read-only live view" actually require, minimally?
2. Server choice: Radicale vs vdirsyncer vs a bare static-file DAV (or none).
3. The vdir-shape mismatch: does our tree need re-layout to be servable?
4. Verdict: implement now / spawn a sized ROUTINE / defer-with-trigger.

## Discussion

### Agenda 1 — what "live view" minimally requires

✂️ **Petra:** Phase 1 already writes a readable tree. "Live view" adds exactly one capability:
a phone/desktop sees *today's* contacts and calendar by mounting a DAV URL, instead of the user
manually importing `.vcf`/`.ics`. That's it — read-only, no conflict handling, no write-back
(that's Phase 3, gated). The smallest thing that delivers it is "point a DAV server at the tree."

🗄️ **Cassi:** And it must stay *downstream of the backup*, never inline with it. The backup
timer owns the tree; the server is a separate, restartable process that reads the tree. Keep the
two concerns split exactly as backup-vs-ingest is split — a crashed/absent server must never
affect a backup run, and vice-versa.

### Agenda 2 — server choice

🔐 **Dario:** **Radicale** is the natural fit: pure-Python CalDAV+CardDAV server, read-from-disk
storage backend, MIT-ish (GPL3), actively maintained, runs as a user service. Its `multifilesystem`
storage *is* a directory of `.ics`/`.vcf` collections — almost our layout. It speaks both CardDAV
(contacts) and CalDAV (calendar) from one process, which matches our two data types exactly.

😈 **Riku:** vdirsyncer is the wrong tool and we should say so loudly so nobody reaches for it:
it's a *sync client*, not a server. Using it here would mean standing up *another* DAV server for
it to sync *against*, i.e. Radicale anyway, plus a redundant copy. It earns a place only in
Phase 3 (as the round-trip mechanism), not Phase 2. Strike it from the Phase 2 candidate set.

🏗️ **Archie:** The third option is "no server at all — a static read-only DAV via a generic
file server." Rejected: plain HTTP file listing is *not* DAV; clients like DAVx5/Apple need
`PROPFIND`/`REPORT`/`ctag`/`getetag` semantics to enumerate a collection. Hand-rolling those is
exactly the "build no custom sync/conflict engine" line we already drew. Radicale gives them for
free.

🔐 **Dario:** One real mismatch to flag (Agenda 3): Radicale's `multifilesystem` expects
`<collection>/<item>.ics`, and a **per-collection `.Radicale.props`** marking each directory as a
CalDAV vs CardDAV collection with a UUID + component-type. Our calendar tree already nests as
`calendar/<cal-id>/<uid>.ics` (one collection per Proton calendar — good), but contacts are a
*flat* `contacts/*.vcf` with no collection wrapper, and neither side has the `.Radicale.props`
markers. So a small adapter is needed: either (a) a tiny generator that drops the
collection-marker files next to the existing tree, or (b) a thin symlink/overlay that re-presents
our tree in Radicale's expected shape — **without** polluting the canonical backup tree (the
`.vcf`/`.ics` files stay standards-only; markers are Radicale-private metadata kept *outside*
the git-versioned backup, in Radicale's own collection root).

✂️ **Petra:** That adapter is the *only* code Phase 2 needs, and it's small and mechanical —
a single ROUTINE-sized item, fully testable offline (assert the generated collection tree mounts
the right component types and points at the real files). No live account, no network, no crypto.

### Agenda 3 — vdir-shape mismatch (resolved above)

Consensus: keep the canonical backup tree untouched (standards-only, the existing contract).
Materialise a **separate** Radicale collection root that references the backup files (symlink or
copy-on-change) and carries the `.Radicale.props` markers. Contacts get wrapped in a single
address-book collection; each `calendar/<cal-id>/` becomes a calendar collection. This is a
pure, offline, deterministic transform — perfect ROUTINE shape.

### Agenda 4 — verdict

🏗️ **Archie:** The *decision* is settled and is not itself code: **Radicale is the Phase 2
serving path; vdirsyncer is explicitly deferred to Phase 3; DAVx5/Thunderbird/Apple are
client-of-choice and need no decision.** That decision is this note. The *build* (the
collection-marker adapter + a documented `radicale.conf` snippet) is a separate, sized, offline
ROUTINE — spawned below, not done here, because it touches the runner/runbook and wants its own
red-green test, which is an executor's job, not a design turn's.

😈 **Riku:** Agreed, with one guard: the spawned ROUTINE must **not** open a network port by
default or auto-start a daemon from the backup timer. Read-only-localhost, opt-in, documented —
otherwise we've quietly grown a long-running service into a backup tool. Put that constraint in
the item's acceptance.

🗄️ **Cassi:** And the server process stays *out* of the backup unit — a separate
`proton-moresync-dav.service` (or just a documented manual `radicale --config` invocation), never
chained off `proton-backup.service`.

## Decisions

- **Phase 2 serving path = Radicale** (CalDAV + CardDAV from one read-from-disk process). It
  serves the *existing* standards-only backup tree; no output-format change. <!-- id:d407 -->
- **vdirsyncer is NOT a Phase 2 tool** — it is a sync *client*, reclassified to Phase 3 (the
  round-trip / write-back mechanism). Removing it from the Phase 2 candidate set is part of this
  decision.
- **DAVx5 / Thunderbird / Apple Contacts are consumer clients**, client-of-choice, out of our
  control — no decision needed, they just mount the Radicale URL.
- **The canonical `.vcf`/`.ics` backup tree stays standards-only and untouched.** Radicale's
  per-collection markers (`.Radicale.props`) live in a *separate* collection root that references
  the backup files; they are never written into the git-versioned backup tree.
- **No write path, no daemon-by-default, no coupling to the backup unit.** Read-only, localhost,
  opt-in; the DAV server is a separate process/service from `proton-backup.service`.

### Rejected alternatives
- **vdirsyncer as the Phase 2 server** — it is a sync client, not a server; using it would
  require standing up Radicale anyway plus a redundant synced copy. Wrong layer.
- **A bare static file server (plain HTTP) instead of DAV** — clients need
  `PROPFIND`/`REPORT`/`ctag`/`getetag` collection semantics; plain HTTP listing is not DAV and
  hand-rolling those reintroduces the "no custom sync engine" line we already drew.
- **Embedding a DAV server inside the Go backup binary** — couples a long-running network
  service to a one-shot fetch tool, enlarges the blast radius, and re-implements what Radicale
  already does. The mbsync-role scope says emit files; a separate off-the-shelf server reads them.
- **Re-laying-out the canonical tree to Radicale's native shape** — would pollute the
  standards-only contract with server-private markers; rejected in favour of a separate
  referencing collection root.

## Spawned follow-up (sized ROUTINE)

Added to ROADMAP.md as a `[ROUTINE]` item (offline, executor-sized):

- **id:6aad — Phase 2 Radicale collection adapter + config + runbook.** Generate a separate
  Radicale collection root that references the backup tree (single address-book collection for
  `contacts/`, one calendar collection per `calendar/<cal-id>/`), emit the `.Radicale.props`
  markers there (never in the backup tree), ship a documented read-only-localhost `radicale.conf`
  snippet, and add a runbook section. Acceptance gates: canonical tree byte-unchanged; markers
  outside the git-versioned backup; no port opened / no daemon auto-started by the backup timer;
  server lives in its own service, not chained off `proton-backup.service`. Offline-testable
  (no live account / no network / no crypto).

## Status of id:d407
Decision rendered (Radicale). No reopen trigger — the choice is firm. id:d407 is closed by this
note; the remaining work is the sized id:6aad ROUTINE above.
