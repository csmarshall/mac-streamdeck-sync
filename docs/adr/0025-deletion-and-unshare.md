# 0025. Deletion and unshare: tombstones for unshare; local deletes never propagate

Status: Accepted 2026-10-01 (review F7; maintainer's decision)

## Context

The direction table ([0005](0005-direction-detection-three-way-hash.md)) originally had no rows for a missing local copy, a missing store copy, or an unshared profile. Read naively, any subscriber would resurrect an unshared profile, and a local delete would either be re-installed forever or propagate as a wipe to every host. A sync tool that spreads one accidental delete everywhere is worse than one that never deletes.

## Decision

- **`schrodeck unshare <profile>`** (on any host) writes a write-once `profiles/<profile_id>/tombstone.json` ([contract D](../contracts/store-format.md)). Every subscribing host, on seeing it:
  - stops syncing that profile;
  - **keeps its local copy, detached** (it becomes an ordinary local profile);
  - notifies once ([0016](0016-notifications.md)).

  Sharing it again later creates a new `profile_id` ([0026](0026-profile-identity.md)). A tombstone is never reverted.
- **`schrodeck unsubscribe <profile>`** on a host stops syncing there and leaves the local copy in place, detached. Nothing is written to the store except this host's own `hosts/<host_id>.toml`.
- **Local delete** (a subscribed copy's folder disappears from `ProfilesV3` while B exists): **never propagated.** That host stops syncing that copy, records `LocalDeleted`, and notifies once with two choices: `schrodeck unsubscribe <profile>` (accept the delete) or `schrodeck subscribe <profile>` (reinstall).
- **Store copy gone** (no heads for a profile, B present, and no tombstone, e.g. the store was wiped, moved or evicted): **stop and notify**. Never resurrect it from the local copy, and never delete the local copy. `schrodeck share` can republish it as a new profile.
- **Nothing left to sync:** when the agent runs and this host has no shares and no subscriptions (e.g. after the last unsubscribe), it notifies **once** that it has nothing to do and suggests `schrodeck uninstall` ([0027](0027-store-lifecycle.md)). It never uninstalls itself.

## Consequences

- Good: no operation, accidental or not, removes a profile from another host's app.
- Good: every one of these states has an explicit exit (a command the notification names).
- Bad: cleaning up a profile everywhere takes `unshare` plus deleting each detached copy by hand in the app. That is deliberate.
- Risk: a user who deletes a copy expecting it to vanish everywhere will be surprised. The notification explains what happened.

## Alternatives considered

- **Propagate deletes** (after a restore point): one accidental delete spreads to every host. Rejected by the maintainer.
- **Unshare deletes subscribers' copies:** removes profiles from machines the user may not be looking at. Rejected.
- **Re-install a locally deleted copy automatically:** fights the user's explicit action on every run.
- **Auto-uninstall when idle:** removes the agent without consent. A one-time suggestion instead.

## Verified by

No check yet; to be written in the plan:
- Tombstone ⇒ subscribers stop, local copies are untouched (no write-open on any app file, asserted at the filesystem-port level), and exactly one notification is sent.
- Local folder removed ⇒ no push, no store write except the event line, one notification. Known-bad: an implementation that treats "missing" as "edited" pushes, and fails this test.
- All heads removed with no tombstone ⇒ StoreLost, no apply, no push, local copy untouched.
- Idle agent ⇒ exactly one "nothing to sync" notification over 10 runs, and the agent is still installed.

## References

- None from Elgato (policy decision).
