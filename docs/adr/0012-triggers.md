# 0012. Triggers: file watchers, a safety timer, the CLI, and an optional USB accelerator

Status: Accepted 2026-10-01

## Context

Updates should land quickly after a change elsewhere, and local edits should publish promptly. The app rewrites its own files on launch, including after our own relaunch ([R15](../references.md)), so a naive watcher would loop.

## Decision

- **Watchers:** launchd `WatchPaths` on `ProfilesV3/` (local edits → evaluate push) and on the store's per-profile `current.json` (remote changes → plan an apply). Events are debounced so a burst is one run.
- **Safety timer:** `StartInterval` of about 15 minutes, because sync clients and watchers can miss events.
- **CLI:** every action can be run by hand.
- **Optional accelerator:** a launchd IOKit matching event on the Elgato USB vendor id (`0x0fd9`) triggers a run when a deck is attached. It is never needed for correctness ([0003](0003-decks-are-local-geometry-compatibility.md)).
- **A single lock** (`flock`) prevents overlapping runs.
- **Required property: no feedback loop.** An apply followed by the app's launch rewrite must produce no second apply. Normalization ([0006](0006-normalization-and-variables.md)) makes the rewrite hash-identical.

## Consequences

- Good: near-real-time behavior without polling, and the timer bounds the worst case.
- Bad: `WatchPaths` fires on any change in the directory, so most runs are no-ops. Cheap, but it shows up in DEBUG logs.
- Risk: if normalization misses a runtime field, the watcher turns that bug into a restart loop. The required test and a rate limit (max N applies per hour, then stop and notify) contain it.

## Alternatives considered

- **Timer only:** up to 15 minutes of staleness after a switch.
- **USB attach as the primary trigger:** excludes virtual decks and ties correctness to hardware events.
- **A long-running daemon with FSEvents:** more moving parts than launchd provides for free.

## Verified by

No check yet; to be written in the plan:
- The no-feedback-loop integration test (apply → simulated launch rewrite → run ⇒ zero applies).
- A rate-limit test ⇒ the loop stops after N applies.

## References

- [R15](../references.md): launch rewrites (observed)
