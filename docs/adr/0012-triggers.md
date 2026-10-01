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
- **Death-spiral protection: per-profile exponential backoff, no hard cap by default.**
  - Each consecutive apply (or push) of the *same* shared profile on a host waits longer before the next is allowed: 1, 2, 4, 8 … minutes, capped at 60 minutes.
  - The backoff resets after a quiet period with no apply of that profile, for twice the current backoff.
  - A run that hits the backoff is skipped, and logged at INFO with the time until the next allowed attempt. Other profiles are unaffected.
  - When backoff engages, a notification fires ("Profile *Work* is changing repeatedly; slowing down. See `schrodeck log`."). This is controlled by `notify.on_backoff` in the common config, default on.
  - An optional hard cap, `apply.max_per_hour` (default **unset**), stops and notifies after N applies of a profile in an hour, for anyone who wants a hard stop.

## Consequences

- Good: near-real-time behavior without polling, and the timer bounds the worst case.
- Bad: `WatchPaths` fires on any change in the directory, so most runs are no-ops. Cheap, but it shows up in DEBUG logs.
- Risk: if normalization misses a runtime field, the watcher turns that bug into a restart loop, and two hosts can ping-pong a profile. The required no-feedback-loop test is the first line of defense. The exponential backoff guarantees that even an undetected bug decays to at most one apply per hour per profile, and the notification surfaces it.
- Good: no arbitrary cap blocks legitimate bursts of edits, such as an evening spent reworking a profile.

## Alternatives considered

- **Timer only:** up to 15 minutes of staleness after a switch.
- **USB attach as the primary trigger:** excludes virtual decks and ties correctness to hardware events.
- **A long-running daemon with FSEvents:** more moving parts than launchd provides for free.
- **A hard rate limit by default (e.g. 6 applies per hour, then stop):** rejected by the maintainer. It blocks legitimate bursts of edits, and a stopped sync needs manual recovery. Backoff slows a spiral without ever stopping a healthy profile. The hard cap remains available as opt-in config.

## Verified by

No check yet; to be written in the plan:
- The no-feedback-loop integration test (apply → simulated launch rewrite → run ⇒ zero applies).
- A ping-pong simulation test: two fake hosts sharing one fake store, with a deliberately broken normalizer that makes every launch look like an edit. It must show the backoff growing (1, 2, 4 … 60 min) and a bounded number of applies over a simulated day, not an unbounded number. Known-good counterpart: with the correct normalizer, the same simulation produces zero applies after convergence.
- If `apply.max_per_hour` is set, the same simulation must stop at exactly N applies and notify.

## References

- [R15](../references.md): launch rewrites (observed)
