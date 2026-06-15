# Decryption-failure observability pass (id:0dfb)

Date: 2026-06-15
Status: decided + implemented (relay HARD-execute, Opus)

## Problem

`backupContacts`/`backupCalendars` skip any object whose decrypt/verify step
fails, printing only `WARN: ... <err>` lines and a per-section
`N written, M skipped` count to stdout. A scheduled (systemd-timer) backup runs
unattended; a silent partial backup — e.g. one key rotation breaks 200 contacts —
looks identical to a clean run in the timer log tail, and the skip count scrolls
away. There was no durable, machine-readable record of *which* objects were lost
or *why*, and no way for the timer to alert on degradation.

## Options weighed (from the ROADMAP acceptance)

1. **Exit non-zero above a threshold.** Lets a timer/cron alert. Risk: the
   "acceptable skip rate" is unknown — a hard fail on the first transient skip
   could mask an otherwise-good 99%-complete backup, and we have no evidence yet
   for where the line is.
2. **Write a `.meta/skipped.json` manifest.** Durable, auditable, diffable across
   runs. Pure observability — changes no exit semantics. Lowest risk.
3. **Stay advisory** (status quo). Zero work, zero signal.

## Decision

Do **(2) + a gated (1)**, keep (3) as the default — i.e. *observe before
preventing* (global design heuristic: build the logger first, gather evidence,
then decide the prevention threshold).

- **Always** write `.meta/skipped.json` (`skipManifest`: `total` + per-entry
  `{kind, id, reason}`) when anything was skipped, and **remove** a stale manifest
  when the run is clean — so the file's presence is itself a reliable
  "this backup is partial" signal. This is the evidence-gathering logger.
- **Opt-in** `--max-skip-rate <float>` flag. Default `1.0` = advisory, never trips
  (Phase-1 behaviour preserved exactly). A user who wants their timer to fail loud
  sets e.g. `--max-skip-rate 0.05`; exceeding it exits non-zero with a
  `FATAL skip-rate:` line. Comparison is *strictly greater* (50% == 0.5 does not
  trip) and a run that attempted nothing never trips (empty account ≠ failure).
- Whole-calendar failures (key unlock / no member match) are recorded as a single
  skip against the calendar ID — previously they only produced a `WARN:` and never
  counted toward any skip metric.

Rationale for the advisory default: n is currently 0 — no real account has been
observed skipping at scale. Picking a hard threshold now would be speculation
(the pilot-sample heuristic: a guessed threshold is asymmetric-cost-justified only
when the bulk action is expensive; here missing an alert is cheap once the
manifest exists). The manifest gives us the data to set an evidence-based default
later; until then, failing loud is the operator's explicit choice.

## Implementation

- `cmd/backup/skiplog.go`: `skipLog` (accumulator), `skipManifest`/`skipEntry`,
  `writeManifest` (write-or-clear), `exceedsRate` (strict-greater, zero-attempt
  guard).
- `backupContacts`/`backupCalendars`/`backupCalendar` now take a `*skipLog`,
  record `(kind, id, reason)` per skip, and return their written counts.
- `main.go`: one run-wide `skipLog`; writes the manifest after both backups;
  prints `backup complete: N written, M skipped` + a pointer to the manifest;
  applies the threshold gate.

## Tests

`cmd/backup/skiplog_test.go` (`# roadmap:0dfb`): record/count, manifest
write-when-nonempty, manifest removed-when-empty (stale-clear), and the
`exceedsRate` table (advisory default, strict-greater boundary, zero-attempt).
All green; full `go test ./...` + `go vet ./...` green.

## Not done / future

- No evidence-based default threshold yet — revisit once a real account has
  produced a skipped.json with non-trivial `total` (reopen trigger).
- Manifest is per-run (overwritten), not historical; a diff-across-runs watcher is
  out of scope here.
