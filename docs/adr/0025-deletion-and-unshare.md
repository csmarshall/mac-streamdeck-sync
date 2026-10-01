# 0025. Deletion and unshare: tombstones for unshare; a local delete detaches that Mac

Status: Accepted 2026-10-01 (review F7; maintainer's decision). Revised 2026-10-01 (review round 2: F26, F30; owner's decision): tombstones are per host, `reshare` undoes an unshare, and StoreLost is declared only for a store the provider reports as fresh and empty. Revised 2026-10-01 (reframe: template-seeded setups; fail closed, [0030](0030-fail-closed-detach.md)): a local delete makes that Mac drop out of the setup (DETACHED(local-deleted), suggested resolution: unmap); StoreLost becomes DETACHED(store-lost) and no longer clears by itself.

## Context

The direction table ([0005](0005-direction-detection-three-way-hash.md)) originally had no rows for a missing local copy, a missing store copy, or an unshared profile. Read naively, any subscriber would resurrect an unshared profile, and a local delete would either be re-installed forever or propagate as a wipe to every host. A sync tool that spreads one accidental delete everywhere is worse than one that never deletes.

## Decision

- **`schrodeck unshare <profile>`** (on any host) appends a tombstone record to **this host's own** `profiles/<profile_id>/tombstones/<host_id>.json` ([contract D](../contracts/store-format.md)). Tombstones are per host, so two hosts unsharing at once never write the same file (review F26). Every subscribing host, on seeing an unsuperseded tombstone:
  - stops syncing that profile;
  - **keeps its local copy, detached** (it becomes an ordinary local profile), but remembers the subscription as *detached by unshare*;
  - notifies once ([0016](0016-notifications.md)).
- **`schrodeck reshare <profile_id>`** (on any host) undoes an accidental unshare (owner's decision). It appends a reshare record to this host's `reshares/<host_id>.json` that **supersedes** every tombstone record it has seen. The profile keeps its `profile_id` and its full revision history. Hosts whose subscriptions were *detached by unshare* **resume** automatically:
  - a copy left untouched while detached is InSync or Behind and simply catches up;
  - a copy edited while detached is Ahead or Diverged under the normal rules ([0005](0005-direction-detection-three-way-hash.md)), so nothing is lost.

  Hosts that ran `unsubscribe` don't resume. Ordering needs no clocks: a profile is unshared while **any** tombstone record is not superseded by a reshare record ([contract D § records](../contracts/store-format.md#records)). A concurrent unshare the resharing host hadn't seen yet therefore keeps it unshared, which is the safe outcome, and a second `reshare` fixes it.
- **`schrodeck unsubscribe <profile>`** on a host stops syncing there and leaves the local copy in place, detached. Nothing is written to the store except this host's own `hosts/<host_id>.toml`.
- **Local delete** (a member copy's folder disappears from `ProfilesV3` while B exists): **never propagated.** That Mac drops out of the setup for that copy: state **DETACHED(local-deleted)** ([0030](0030-fail-closed-detach.md)). Nothing is pushed or pulled for it, other Macs are unaffected, and one deduplicated notification says so. `schrodeck resolve <setup>` offers **unmap** (the suggested default: accept the delete and leave the setup on this Mac) or **rejoin** (install the setup again as a new profile on a chosen device, [0029](0029-problem-statement-and-setup-model.md)).
- **Store copy gone** (StoreLost): declared only when the provider reports the profile's store folder as **fresh** and it has no heads, B is present, and there is no tombstone. If freshness is not current, or `Unknown`, the profile is InFlight instead (review F30): a store that hasn't been enumerated yet (a fresh `join`, an online-only folder, eviction) must never look deleted. With `Unknown` freshness, StoreLost additionally requires the condition to persist across runs for at least the stuck-in-flight alarm period (default 1 hour). When declared, the copy becomes **DETACHED(store-lost)** ([0030](0030-fail-closed-detach.md)): never resurrected from the local copy, never deleting the local copy, and it **doesn't clear by itself** if heads reappear. The user resolves it: **publish** (push this copy so the setup's history continues from it) or **unmap**.
- **Nothing left to sync:** when the agent runs and this host has no shares and no subscriptions (e.g. after the last unsubscribe), it notifies **once** that it has nothing to do and suggests `schrodeck uninstall` ([0027](0027-store-lifecycle.md)). It never uninstalls itself.

## Consequences

- Good: no operation, accidental or not, removes a profile from another host's app.
- Good: every one of these states has an explicit exit (a command the notification names), and every unexpected one follows the same fail-closed model ([0030](0030-fail-closed-detach.md)).
- Bad: cleaning up a profile everywhere takes `unshare` plus deleting each detached copy by hand in the app. That is deliberate.
- Good: a mistyped `unshare` is one `reshare` away from undone, with history intact.
- Risk: a user who deletes a copy expecting it to vanish everywhere will be surprised. The notification explains what happened, and `unshare` is the way to retire a setup everywhere.

## Alternatives considered

- **Propagate deletes** (after a restore point): one accidental delete spreads to every host. Rejected by the maintainer.
- **Unshare deletes subscribers' copies:** removes profiles from machines the user may not be looking at. Rejected.
- **Permanent tombstones, re-sharing creates a new profile** (the first version of this ADR): one mistyped command would mean re-subscribing on every host. Rejected by the owner.
- **A single shared `tombstone.json`:** two hosts could write it. Rejected (F26).
- **Re-install a locally deleted copy automatically:** fights the user's explicit action on every run.
- **StoreLost and DeckGone clear by themselves when the condition goes away** (the previous version): resumed sync after a state nobody had looked at. Replaced by fail-closed DETACHED ([0030](0030-fail-closed-detach.md)); automatic re-attach can come back later, case by case, with its own ADR.
- **Auto-uninstall when idle:** removes the agent without consent. A one-time suggestion instead.

## Verified by

No check yet; to be written in the plan:
- Tombstone ⇒ subscribers stop, local copies are untouched (no write-open on any app file, asserted at the filesystem-port level), and exactly one notification is sent.
- Local folder removed ⇒ DETACHED(local-deleted): no push, no store write except the event line, one notification, and the state persists across runs. Known-bad: an implementation that treats "missing" as "edited" pushes, and fails this test.
- All heads removed with no tombstone, store reported fresh ⇒ DETACHED(store-lost), no apply, no push, local copy untouched; heads restored ⇒ **still** DETACHED until `resolve` (known-bad: an implementation that auto-clears must fail). Same with freshness not current ⇒ InFlight, never StoreLost (known-bad: checking tombstones/heads before freshness must report StoreLost here and fail).
- `unshare` then `reshare` ⇒ detached subscribers resume; a copy edited while detached becomes Ahead and is pushed, losing nothing. Concurrent `unshare` on host A and `reshare` on host B that hadn't seen A's record ⇒ still unshared on every host.
- Idle agent ⇒ exactly one "nothing to sync" notification over 10 runs, and the agent is still installed.

## References

- None from Elgato (policy decision).
