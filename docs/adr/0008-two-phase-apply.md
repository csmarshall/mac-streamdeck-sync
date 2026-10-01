# 0008. Applying an update is plan → commit → verify, with a post-quit re-check

Status: Accepted 2026-10-01. Revised 2026-10-01 (review F2, F5, F6, F11, F12, F18):
- a post-quit re-check;
- the incoming fingerprint is validated;
- a verify failure ends in BLOCKED, not a retry loop;
- per-step journal recovery;
- the app's running state is restored;
- batched runs.

## Context

Applying an update means replacing files of a running app. The app keeps state in memory and writes it back, so files changed under it get overwritten. Elgato says to close the app before changing its files ([R3](../references.md), documented). Elgato offers no CLI or URL to import or restart ([R17](../references.md)). Smart Profiles are deactivated while the Stream Deck window is open ([R13](../references.md)).

The review found that reading local state only **before** quitting loses edits. The user can change a button between the plan and the quit, and the app also flushes in-memory state while quitting. The swap then overwrites that edit (review scenario 2).

## Decision

**A run is ordered:**
1. Pushes.
2. Re-read the heads.
3. One batched apply of every copy on this host that is Behind.

Two copies of one profile on one host (on two decks) are applied in the same batch, so the app restarts once. Rollback is **all-or-nothing per run**.

**The app restart is the commit point. Everything that can fail is checked before it, and the local state is re-checked after the app has stopped writing.**

![apply state diagram](../apply-states.png)

1. **Plan** (the app still runs, and nothing is touched). Abort on any failure:
   - For each target: the incoming commit's `fingerprint` is in this host's known-good set, and its `norm_version` matches ([0015](0015-schema-guard.md), [0006](0006-normalization-and-variables.md)).
   - The target is not BLOCKED for this R (below).
   - Stage each target in a scratch dir on the same volume. Placeholders are expanded with this host's values, `Device.UUID` is set to the receiving deck, and the folder is named per [0026](0026-profile-identity.md). The staged copy must re-hash to `hash(R)`.
   - Geometry matches ([0003](0003-decks-are-local-geometry-compatibility.md)).
   - Schema guard passes for this app version ([0015](0015-schema-guard.md)).
   - Free disk is sufficient.
   - Plugin presence check ([0014](0014-plugin-handling.md)): a missing plugin means apply anyway and notify. Only presence is needed here; the full inventory is M5.
   - Record **L_plan** for every target and whether the app was running (**app_was_running**).
2. **Journal:** write `plan.json` (targets, staged paths, L_plan, expected hashes, app_was_running, `step = planned`) and fsync it plus its directory. Each later step updates `step` with an fsync.
3. **Quit** the app gracefully and wait for it to exit (contract A's `AppControl.Quit` guarantee). On timeout, abort: nothing touched, and the app is left running.
4. **Post-quit re-check** (the app can no longer write): re-hash every target. If any **L ≠ L_plan**, the user or the app's quit-time flush changed the copy. **Abort the swap**, relaunch the app, and re-evaluate on the next tick. That copy is now Ahead or Diverged, and its edit is preserved.
5. **Snapshot** (after the quit, so it includes any flushed state): copy each target's current local tree into the local history ring and verify it. `step = snapshotted`.
6. **Swap** with atomic renames on the same volume: rename the old folder aside, then the staged one in. `step = swapped`.
7. **Relaunch** if `app_was_running` (or if the user's setting says to always run it), then wait for it to settle (process up, files quiet). If the relaunch fails, retry once, then notify persistently. The files are already in place, and the app will load them when next opened.
8. **Verify:** re-hash every target. If L == hash(R), set B := (R, L) and move the head to R ([contract D](../contracts/store-format.md)). Clear the journal.
9. **On verify failure,** log the expected and actual hashes and the differing key paths, then follow `apply.on_verify_failure`:
   - **`rollback` (default):**
     1. Snapshot the failed post-apply tree first, for diagnosis.
     2. Restore the step-5 snapshot of **every** target in the run, through quit / swap / relaunch.
     3. Set a durable **BLOCKED(R)** in local state for that profile.
     4. Notify, deduplicated: "schrodeck won't update *<profile>* on this Mac: the incoming version failed verification. See `schrodeck status <profile>`."

     While BLOCKED(R), the profile isn't applied again until R changes (a new commit) or the user runs `schrodeck resolve`/`schrodeck unblock`. It is not retried on a timer.
   - **`keep`:** leave the applied files in place, set B := (R, L_actual), and flag the copy "applied, unverified". Because `B.local_hash` is the observed hash, the copy is neither Ahead (the app's repair isn't pushed) nor Behind (no re-apply loop). It is reported in `status` and notified once.

**Crash recovery** (a journal exists at the start of a run). The rule depends on the recorded step, and recovery always goes through Quit and Verify:
- `planned`: discard the staging, and leave the app as it is.
- `snapshotted`: discard the staging. The originals are untouched.
- `swapped` or later: roll forward. Quit, make sure every target's staged-in tree is present (finishing any rename that was half done), relaunch, then verify as in step 8.
- If the app was relaunched by the user or at login before recovery runs, recovery quits it first (step 3) and proceeds.

The selected profile per deck is never touched ([0019](0019-selected-profile-stays-per-host.md)).

## Consequences

- Good: an aborted plan has no side effects, an edit made during the apply window is preserved, and a crash at any step has one defined recovery.
- Good: a broken incoming version costs one restart and one rollback per host, not one per hour.
- Good: applies to inactive subscribed copies too ([0004](0004-shared-profiles-and-subscriptions.md)).
- Bad: every apply restarts the app, so the deck blanks for a few seconds. Accepted by the project owner ("always auto").
- Bad: BLOCKED needs a person to clear it, unless a new commit arrives. That is intentional.
- Risk: the settle heuristic (process up, files quiet) is observed behavior, not a documented signal. A too-short window can verify before the app's own rewrite. The launch-rewrite probe ([contract C](../contracts/profile-format.md) P4) measures the needed window per app version.
- Risk: quitting via AppleScript depends on the app honoring the quit event ([contract B](../contracts/client-os.md) M3).

## Alternatives considered

- **Re-check before the quit only** (the first design): loses edits made between the plan and the quit, or flushed by the quit. Rejected after review (F2).
- **Retry a failed verify under backoff** (the first design): restarts the app hourly forever on a version that won't verify. Rejected (F6).
- **Swap files while the app runs:** the app overwrites them ([R3](../references.md)).
- **Elgato's import UI** (`open file.streamDeckProfile`): interactive, not unattended ([R5](../references.md)).
- **Full backup restore:** "will overwrite existing profiles", all of them ([R4](../references.md)).

## Verified by

No check yet; to be written in the plan:
- Injected failures at each step ⇒ the expected end state: plan failure → no app file opened for writing (filesystem-port assertion); quit timeout → app still running; crash at each journal step → the recovery rule above.
- **Post-quit edit:** a fake app that modifies a target during Quit ⇒ the swap is aborted, and the copy is Ahead on the next tick. Known-bad: with the re-check disabled, the same test must lose the edit.
- **Verify failure:** a fake app that rewrites a field on launch ⇒ one rollback, BLOCKED(R) set, exactly one notification across 10 subsequent runs, and zero further applies until R changes.
- Two copies of one profile on one host ⇒ one quit/relaunch.

## References

- [R3](../references.md): close the app first; file management unsupported (documented)
- [R4](../references.md): full restore overwrites all (documented)
- [R5](../references.md), [R17](../references.md): no CLI/URL import (documented by absence)
- [R13](../references.md): Smart Profiles deactivated while the window is open (documented)
