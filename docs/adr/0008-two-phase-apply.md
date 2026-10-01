# 0008. Applying an update is plan → commit → verify

Status: Accepted 2026-10-01

## Context

Applying an update means replacing files of a running app. The app keeps state in memory and writes it back, so files changed under it get overwritten. Elgato says to close the app before changing its files ([R3](../references.md), documented). Elgato offers no CLI or URL to import or restart ([R17](../references.md)). Smart Profiles are deactivated while the Stream Deck window is open ([R13](../references.md)).

## Decision

Applying is split into phases, and **the app restart is the commit point. Everything that can fail is checked before it.**

![apply state diagram](../apply-states.png)

1. **Plan** (the app is still running and nothing is touched). Abort on any failure:
   - Stage every target copy in a scratch dir on the same volume, with variables expanded ([0006](0006-normalization-and-variables.md)), `Device.UUID` set to the receiving deck, and folder id = uuid5(profile, deck).
   - Staged normalized hash == R.
   - Geometry matches ([0003](0003-decks-are-local-geometry-compatibility.md)).
   - Schema guard passes ([0015](0015-schema-guard.md)).
   - Rollback snapshot written, then re-read and verified.
   - Free disk is sufficient.
   - Plugin policy applied ([0014](0014-plugin-handling.md)).
2. **Journal:** write `plan.json` (targets, staged paths, snapshot ids, expected hashes) and fsync it.
3. **Quit** the app, then wait for it to exit. On timeout, abort with nothing touched.
4. **Swap** with atomic renames on the same volume.
5. **Relaunch**, then wait for the app to settle (process up, files quiet).
6. **Verify** by re-hashing every target. If L == R, set B := R and clear the journal.
7. **On verify failure:** log the expected and actual hashes, then follow `apply.on_verify_failure` in the common config:
   - `rollback` (default): restore the snapshot and quit/relaunch again.
   - `keep`: leave the files as applied and flag them.

   Either way, notify and clear the journal.

**Crash recovery:** a run that finds a journal either completes the swap or restores the snapshot. It never guesses. The selected profile per deck is never touched ([0019](0019-selected-profile-stays-per-host.md)).

## Consequences

- Good: an aborted plan has no side effects, and a crash after the journal is recoverable.
- Good: applies to inactive subscribed copies too ([0004](0004-shared-profiles-and-subscriptions.md)).
- Bad: every apply restarts the app, so the deck blanks for a few seconds. Accepted by the project owner ("always auto").
- Risk: the "settle" heuristic (process up, files quiet) is observed behavior, not a documented signal. A too-short window could verify before the app's own rewrite, making verify pass and then fail later. A too-long window delays recovery.
- Risk: quitting via AppleScript depends on the app honoring the quit event.

## Alternatives considered

- **Swap files while the app runs:** the app overwrites them ([R3](../references.md)).
- **Elgato's import UI** (`open file.streamDeckProfile`): interactive, not unattended ([R5](../references.md)).
- **Full backup restore:** "will overwrite existing profiles", all of them ([R4](../references.md)).
- **Ask before every apply:** rejected by the project owner in favor of always-automatic.

## Verified by

No check yet; to be written in the plan:
- An injected failure at each phase ⇒ the expected end state: plan failure → nothing touched; crash after the journal → recovered on the next run; verify failure → rollback.
- A test that a verify mismatch with `rollback` restores the byte-identical pre-apply tree.

## References

- [R3](../references.md): close the app first; file management unsupported (documented)
- [R4](../references.md): full restore overwrites all (documented)
- [R5](../references.md), [R17](../references.md): no CLI/URL import (documented by absence)
- [R13](../references.md): Smart Profiles deactivated while the window is open (documented)
