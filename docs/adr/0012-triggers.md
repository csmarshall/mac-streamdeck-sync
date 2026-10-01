# 0012. Triggers: a resident agent with FSEvents, a safety timer, the CLI, and an optional USB accelerator

Status: Accepted 2026-10-01. Revised 2026-10-01 (review F10, F15): launchd `WatchPaths` replaced by a resident FSEvents watcher (WatchPaths isn't recursive); backoff applies to applies only, and pushes never back off.

## Context

Updates should land quickly after a change elsewhere, and local edits should publish promptly. Page edits land deep in the tree (`ProfilesV3/<p>.sdProfile/Profiles/<page>/manifest.json`), and launchd's `WatchPaths` doesn't watch subdirectories recursively. The app rewrites its own files on launch, including after our own relaunch ([R15](../references.md)), so a naive watcher would loop.

## Decision

- **Resident agent:** a per-user LaunchAgent (`RunAtLoad`, `KeepAlive`) runs `schrodeck agent`. It holds a **recursive FSEvents watcher** on `ProfilesV3/` (local edits) and on the store's `profiles/` tree (remote heads), debounced so that a burst is one run. Events are hints, never truth ([contract A](../contracts/os-connector.md) `Watcher`).
- **Safety timer** inside the agent, about every 15 minutes, because sync clients and watchers can miss events.
- **CLI:** every action can be run by hand.
- **Optional accelerator:** a USB-attach notification on the Elgato vendor id (`0x0fd9`) triggers a run when a deck is attached. It is never needed for correctness ([0003](0003-decks-are-local-geometry-compatibility.md)).
- **A single lock** (`flock`) prevents overlapping runs, including a CLI run during an agent run.
- **Required property: no feedback loop.** An apply followed by the app's launch rewrite must produce no second apply. Normalization ([0006](0006-normalization-and-variables.md)) makes the rewrite hash-identical.
- **Pushes are debounced (default 10 s of quiet), never backed off.** An evening spent editing a profile publishes promptly.
- **Applies have per-profile exponential backoff, with no hard cap by default:**
  - Each consecutive apply of the *same* profile on a host must wait longer before the next: 1, 2, 4, 8 … minutes, capped at 60.
  - It resets after a quiet period of twice the current backoff with no apply of that profile.
  - A run that hits the backoff skips that profile and logs at INFO. Other profiles are unaffected.
  - When backoff engages, a deduplicated notification fires (`notify.on_backoff`, default on).
  - An optional hard cap, `apply.max_per_hour` (default **unset**), stops and notifies after N applies of a profile in an hour.
  - A verify failure doesn't use backoff at all. It goes to BLOCKED ([0008](0008-two-phase-apply.md)).
- **Nothing to do:** if the agent runs and this host has no shares and no subscriptions, it notifies **once** (deduplicated, [0016](0016-notifications.md)): "schrodeck has nothing to sync on this Mac; run `schrodeck uninstall` to remove the background agent." It never uninstalls itself ([0025](0025-deletion-and-unshare.md), [0027](0027-store-lifecycle.md)).

## Consequences

- Good: near-real-time behavior for edits anywhere in the tree, and the timer bounds the worst case.
- Bad: a resident process instead of on-demand launchd jobs. It is small and idle almost always, and it is the only way to get recursive watching on macOS.
- Risk: if normalization misses a runtime field, the watcher turns that bug into repeated applies, and two hosts can ping-pong a profile. The no-feedback-loop test is the first defense; apply backoff bounds an undetected bug to at most one apply per hour per profile, and the notification surfaces it.
- Good: no arbitrary cap blocks legitimate bursts of edits.

## Alternatives considered

- **launchd `WatchPaths`** (the first design): not recursive, so page edits would wait for the timer. Rejected after review (F15).
- **Timer only:** up to 15 minutes of staleness after a switch.
- **USB attach as the primary trigger:** excludes virtual decks and ties correctness to hardware events.
- **Backoff on pushes too** (the first design): delays legitimate edits by up to an hour and creates avoidable forks. Rejected (F10).
- **A hard rate limit by default:** rejected by the maintainer. It blocks legitimate bursts, and a stopped sync needs manual recovery.

## Verified by

No check yet; to be written in the plan:
- The no-feedback-loop integration test: apply → simulated launch rewrite → run ⇒ zero applies.
- A watcher conformance test: a write three directory levels below the watched root produces an event. **Known-bad:** a non-recursive watcher fails it.
- A ping-pong simulation with two fake hosts, one fake store, and a deliberately broken normalizer ⇒ the apply backoff grows (1, 2, 4 … 60 min), applies stay bounded over a simulated day, and **pushes are never delayed beyond the debounce**. Known-good: with the correct normalizer, zero applies after convergence.
- If `apply.max_per_hour` is set, the same simulation stops at exactly N applies and notifies.
- An agent with no shares/subscriptions over 10 runs ⇒ exactly one "nothing to sync" notification and no uninstall.

## References

- [R15](../references.md): launch rewrites (observed)
