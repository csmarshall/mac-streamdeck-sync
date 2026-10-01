# 0016. Notifications come from our own helper app, deduplicated, with an original icon

Status: Accepted 2026-10-01. Revised 2026-10-01 (review F6, F24; maintainer's dedup requirement): notifications are keyed by condition and sent once on entering it; event table updated. Revised 2026-10-01 (review round 3: F42, F46): rows for pending older-version edits and unresolved cross-profile references.

## Context

The tool runs unattended and must tell the user when it restarts the app, fails, or needs a decision. On macOS, a notification's icon belongs to the app that posts it. `osascript display notification` always shows Script Editor's icon.

Spike on macOS 27, 2026-10-01, with an ad-hoc-signed Swift `UserNotifications` helper:
- From a temp directory it was refused with no prompt (`UNErrorDomain` code 1; LaunchServices could not resolve the app, -10814).
- From `~/Applications` it prompted once, then delivered banners with its custom icon.

The agent re-evaluates every profile on every run (watcher events and a 15-minute timer, [0012](0012-triggers.md)). A persistent condition such as BLOCKED or Forked would therefore produce one alert per run unless notifications are deduplicated. The maintainer wants exactly one alert per condition.

## Decision

- **Mechanism:** a small Swift helper app, built and ad-hoc signed at install time, installed into `~/Applications` and registered with LaunchServices.
- **Fallback:** `osascript` (generic icon) when the helper is unavailable.
- **Icon:** an **original** design (a dark tile with a key grid, one accent key, and a circular sync badge). It is generated from code and uses no Elgato marks ([R19](../references.md)).
- **Deduplication** is part of the notifier design, in the core's notifier layer, so every OS connector gets it ([contract A](../contracts/os-connector.md) `Notifier`):
  - Each alert has a **key** `(condition, profile_id, version)`, where `version` is the revision or fork tip set involved.
  - An alert is sent **once, when the condition is entered**. It is not resent while the condition persists.
  - It is sent again only when the key changes (e.g. a new incoming version is also BLOCKED) or after the condition clears and re-enters.
  - An optional reminder interval (`notify.remind_after`, default **off**) re-sends a still-active persistent alert.
  - Sent-alert state lives in local state, so it survives agent restarts.
- **Events:**

| Event | Default | Key / dedup |
|---|---|---|
| Apply starting (the deck is about to blank) | on | per apply |
| Apply / push / rollback done | on | per revision |
| New shared profile available | on | per profile |
| Diverged / Forked | on, persistent | (fork, profile, tip set) |
| BLOCKED: "schrodeck won't update *<profile>* on this Mac: the incoming version failed verification", noting that an edit made during the restart is saved in history, + how to inspect ([0008](0008-two-phase-apply.md)) | on, persistent | (blocked, profile, revision) |
| HoldLocal: "this Mac can't take version X of *<profile>*, so your edit is local only" ([0005](0005-direction-detection-three-way-hash.md)) | on, persistent | (holdlocal, profile, R revision) |
| Incoming revision in a profile format this Mac hasn't verified ([0015](0015-schema-guard.md)) | on, persistent | (fingerprint, profile, fingerprint) |
| Anomaly: the store went backwards for *<profile>* ([0005](0005-direction-detection-three-way-hash.md)) | on, persistent | (anomaly, profile, R revision) |
| Pending edit from *<host>* on an older schrodeck version, waiting for that host to upgrade ([0027](0027-store-lifecycle.md)) | on | (pending-old, profile, that revision) |
| A button references another profile that isn't subscribed on this Mac ([0013](0013-sync-scope-and-scripts.md)) | on | (xref, profile, target profile) |
| Push refused: another host's value found in the profile (collision guard, [0006](0006-normalization-and-variables.md)) | on, persistent | (collision, profile, local hash) |
| Applied but unverified (`keep`) | on | (kept, profile, revision) |
| Backoff engaged | on (`notify.on_backoff`) | (backoff, profile) |
| Missing plugin / script / Shortcut | on | (missing, profile, dependency) |
| Unshared, store lost, local copy deleted, deck gone | on, persistent | (condition, profile) |
| Reshared: subscriptions on this Mac resumed ([0025](0025-deletion-and-unshare.md)) | on | (reshare, profile, record) |
| Deck not uniquely identifiable, subscription refused ([0026](0026-profile-identity.md)) | on | (deckkey, profile, deck key hash) |
| Stuck in flight beyond the alarm threshold | on | (inflight, profile, head set) |
| Version mismatch, upgrade needed | on, persistent | (version, store FORMAT) |
| Held profile skipped | once per hold | (hold, profile) |
| Nothing to sync on this Mac; run `schrodeck uninstall` | once | (idle, host) |
| InSync | never | — |

## Consequences

- Good: clearly identifiable notifications; one alert per problem, not one per run.
- Bad: the user must allow notifications once per host. Building needs a Swift toolchain.
- Risk: the spike was one run on one machine. Behavior on other macOS versions is projected, not verified.
- Risk: dedup state lost with local state ⇒ each active condition alerts once more. That's acceptable.

## Alternatives considered

- **osascript only:** wrong icon. Notifications look like they come from Script Editor.
- **terminal-notifier:** its custom-icon option relies on behavior recent macOS no longer honors (3.1.0 is the current Homebrew version).
- **Reusing Elgato's icon:** implies affiliation; trademarked ([R19](../references.md)).
- **Dedup in the macOS adapter only:** every other OS connector would have to reimplement it. Rejected.

## Verified by

- The spike (throwaway) showed delivery with the custom icon from `~/Applications` and refusal from a temp dir.
- Still to be written: an install test that the helper's path is under an Applications directory; a manual first-run checklist item.
- Dedup test: a BLOCKED condition persisting across 10 simulated runs ⇒ exactly one notification. A new revision that is also BLOCKED ⇒ one more. Known-bad: with dedup disabled, the same test sees 10.

## References

- [R19](../references.md): branding guidelines (documented)
