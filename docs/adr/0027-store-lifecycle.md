# 0027. Store lifecycle: FORMAT migration, garbage collection, uninstall

Status: Accepted 2026-10-01 (review F4, F17)

## Context

The store is shared by hosts that may run different schrodeck versions. It accumulates write-once commits and trees, staging leftovers, tombstones and event lines. Users also need a clean way to stop using schrodeck on a host. None of this was decided before.

## Decision

**FORMAT and `norm_version` migration:**
- The store's `FORMAT` file holds an integer. A host **writes only its own format**. If it sees a newer `FORMAT`, or a commit with a newer `norm_version`, it becomes **read-only**: status works, push and apply don't, and it notifies once that it needs upgrading.
- A change to the store layout, the record fields or normalization ([contract C](../contracts/profile-format.md), [contract D](../contracts/store-format.md)) ships as a `FORMAT` bump with a migration.
- **Who migrates:** the first upgraded host, only via an explicit `schrodeck migrate`, never automatically. It writes `FORMAT` n+1 last. For each profile tip, it writes a `kind = rebase` commit that carries the same tree and the re-computed `hash` under the new `norm_version`. A rebase is not a content change.
- **Other upgraded hosts** re-compute their B hash from their local history snapshot under the new normalization. If it matches the rebase commit's hash, they adopt it as InSync, **without pushing**.
- Hosts still on the old version are read-only until upgraded. That is the safe direction (review scenario 5).

**Garbage collection** (`schrodeck gc`, also run by the agent at most daily):
- A host deletes only:
  - **its own** files: its `tmp/<host_id>/` entries older than 24 h, its rotated events, its stale inventory;
  - **unreferenced write-once objects:** commits and trees not within `retention.shared` (Y) generations of any live head.

  Deleting a write-once object twice is harmless, so concurrent GC by two hosts is safe.
- A tombstoned profile's commits and trees are removed 90 days after the tombstone. The tombstone itself is kept.
- A host whose head hasn't changed in 365 days is listed by `schrodeck status --stale-hosts`. Nothing is deleted automatically. `schrodeck forget-host <host>` removes that host's head and host file, by explicit command only.
- `events/<host_id>.jsonl` is rotated by its owner (size-based, last 5 files kept).

**Uninstall** (`schrodeck uninstall`):
- Removes the LaunchAgent, the notifier app and local state (`--keep-history` keeps the local history ring).
- **Never touches any profile in the app**: subscribed copies stay where they are, detached.
- Unregisters the notifier from LaunchServices (`lsregister -u`) before deleting it. Observed once (macOS 27, notifier spike): after deleting and unregistering, macOS removed the app's entry from System Settings → Notifications by itself, so uninstall left no settings residue and a reinstall prompts again. Not guaranteed; `doctor` doesn't depend on it.
- Leaves the store untouched by default. `--leave-store` also removes this host's head and host file from the store (equivalent to `forget-host` for itself).

## Consequences

- Good: a mixed-version store degrades to read-only on old hosts instead of looping or corrupting.
- Good: no automatic action deletes anything another host might still need.
- Bad: a FORMAT migration needs one explicit command, and old hosts stay read-only until upgraded.
- Risk: a host offline for more than Y generations may find its B commit garbage-collected. That is harmless, because B's hash is kept in local state and only the hash is needed for comparison. Its rollback range shrinks to its local ring.

## Alternatives considered

- **Automatic migration by whichever host upgrades first:** a surprise write to every profile on an unattended host. Rejected.
- **Time-based GC of everything:** could delete what a long-offline host needs for diagnosis; generation-based GC is bounded by Y instead.
- **Uninstall deletes subscribed copies:** removes the user's working profiles. Rejected.

## Verified by

No check yet; to be written in the plan:
- A reader at FORMAT n with a store at n+1 ⇒ no writes (filesystem-port assertion) and one notification.
- `migrate` on a fixture store ⇒ a rebase commit per profile; a second upgraded host goes InSync without pushing.
- GC with Y = 2 ⇒ commits more than 2 generations behind every head are removed, and commits within 2 generations of a stale head are kept.
- `uninstall` ⇒ the agent is gone and every app profile file is untouched.

## References

- None from Elgato (store design).
