# 0027. Store lifecycle: FORMAT migration, garbage collection, uninstall

Status: Accepted 2026-10-01 (review F4, F17). Revised 2026-10-01 (review round 2: F25, F28, F38): GC deletes trees only and commits are kept forever (owner's decision); migration makes old heads ancestors and adopting hosts move their own head.

## Context

The store is shared by hosts that may run different schrodeck versions. It accumulates write-once commits and trees, staging leftovers, tombstones and event lines. Users also need a clean way to stop using schrodeck on a host. None of this was decided before.

## Decision

**FORMAT and `norm_version` migration:**
- The store's `FORMAT` file holds an integer. A host **writes only its own format**. If it sees a newer `FORMAT`, or a commit with a newer `norm_version`, it becomes **read-only**: status works, push and apply don't, and it notifies once that it needs upgrading.
- A change to the store layout, the record fields or normalization ([contract C](../contracts/profile-format.md), [contract D](../contracts/store-format.md)) ships as a `FORMAT` bump with a migration.
- **Who migrates:** the first upgraded host, only via an explicit `schrodeck migrate`, never automatically. For each profile, it writes a `kind = rebase` commit that carries the same content and the re-computed `hash` under the new `norm_version`, and whose **parents are every live tip** at the old `norm_version` (so it also joins tips that were equivalent). It then moves its own head to the rebase commit, and writes `FORMAT` n+1 last. A rebase is not a content change.
- **Old heads become ancestors.** Every head still at the old `norm_version` (an offline, retired or not-yet-upgraded host) is an ancestor of the rebase commit through its parent links, so it is subsumed when R is derived ([contract D § deriving R](../contracts/store-format.md#deriving-r-from-the-heads)). One stale host can no longer leave the upgraded hosts read-only (review F28).
- **Other upgraded hosts** re-compute their B hash under the new normalization from their local history snapshot of `B.commit`. If it matches the rebase commit's hash, they **move their own head to the rebase commit** and set B := (rebase, L), **without pushing content** (a head write is not a push). If it doesn't match, the copy has an unpushed edit or differs, and the normal rules apply (Ahead or Diverged) under the new version.
- **Hosts still on the old version** are read-only until upgraded. That is the safe direction (review scenario 5). A commit an old host managed to push under the old `norm_version` after the rebase (delivery lag) is not subsumed by the rebase commit, so it shows as Forked. An upgraded host resolves it, re-hashing that content under the new version.

**Garbage collection** (`schrodeck gc`, also run by the agent at most daily). The single definition is [contract D § garbage collection](../contracts/store-format.md#garbage-collection-trees-only). In short:
- **Commit records are never deleted** (owner's decision, review F25). R is derived by walking ancestry through commits, so deleting commits would turn every profile InFlight forever once history passed Y generations. Commits are a few hundred bytes each.
- **Only tree copies are deleted**, and each host deletes only its own (`trees/<digest>/<host_id>/`). A tree is pinned while it belongs to a tip (R or any head), to a commit within Y generations of any head, or to any host's `pins` (a held rollback target, a live B awaiting re-materialization).
- A host also cleans **its own** files: its `tmp/<host_id>/` entries older than 24 h, its rotated events, its stale inventory.
- A tombstoned (unshared, not reshared) profile's unpinned trees are removed 90 days after the tombstone. Its commits and records are kept, so a later `reshare` still has history ([0025](0025-deletion-and-unshare.md)).
- A host whose head hasn't changed in 365 days is listed by `schrodeck status --stale-hosts`. **Nothing about a host is deleted automatically.** `schrodeck forget-host <host>` removes that host's head and host file, by explicit command only.
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
- Good: R is always derivable, however long the history, because commits are never collected.
- Bad: commit records accumulate forever. At a few hundred bytes each, ten edits a day for ten years is about 10 MB per profile at worst.
- Risk: a host offline for more than Y generations still has its `B.commit` (commits are kept), but that commit's tree may be collected unless it is a tip. Comparisons need only hashes, so sync is unaffected; only rollback to that old version is lost, and its local ring still has it.

## Alternatives considered

- **Automatic migration by whichever host upgrades first:** a surprise write to every profile on an unattended host. Rejected.
- **Time-based GC of everything:** could delete what a long-offline host needs for diagnosis; generation-based GC is bounded by Y instead.
- **GC commits as well as trees** (the first version of this ADR): breaks R derivation after Y generations. Rejected after review (F25).
- **Uninstall deletes subscribed copies:** removes the user's working profiles. Rejected.

## Verified by

No check yet; to be written in the plan:
- A reader at FORMAT n with a store at n+1 ⇒ no writes (filesystem-port assertion) and one notification.
- `migrate` on a fixture store ⇒ a rebase commit per profile; a second upgraded host goes InSync without pushing.
- GC with Y = 2 over 50 commits ⇒ only unpinned tree copies are removed, every commit remains, and R is still derivable on every host with no InFlight. Known-bad: a GC that deletes commits must wedge R in this test.
- `migrate` with one host offline at the old `norm_version` ⇒ the upgraded hosts derive R from the rebase commit and keep syncing; the offline host's head is subsumed. Known-bad: deriving R across all heads regardless of version must report VersionMismatch here.
- `uninstall` ⇒ the agent is gone and every app profile file is untouched.

## References

- None from Elgato (store design).
