# 0009. Store write protocol: single-writer heads over immutable commits and trees

Status: Accepted 2026-10-01. Revised 2026-10-01 (review F1, F8, F14, F22): replaced the shared `current/` + `current.json` (last writer wins) with per-host heads and content-addressed, write-once commits and trees.

## Context

The store is replicated by a sync client ([0002](0002-transport-shared-cloud-folder.md)). A reader may see a partially delivered tree or files arriving out of order. When two hosts write the same path, the client keeps one version and may create a "conflicted copy". The first design had every host rewrite the same `current/` folder and `current.json`. The review showed that this loses updates ([0005](0005-direction-detection-three-way-hash.md)).

## Decision

**No file in the store has two writers.** Everything is either:
- write-once, content-addressed or uniquely named; or
- owned by one host, whose `host_id` is in its path.

The single definition of the layout, the record fields and the step-by-step write protocol is **[contract D: store format](../contracts/store-format.md)**. In outline:

- `trees/<tree_digest>/` holds full profile trees, write-once ([0006](0006-normalization-and-variables.md)).
- `commits/<commit_id>.json` holds write-once records `{hash, tree, parents, norm_version, …}`.
- `heads/<host_id>.json` is each host's own pointer, written **last** in a push.
- R is the commit that all heads descend from. No R means **Forked**, which is detectable.
- A reader that finds a head whose commit or tree is missing or digest-invalid treats the profile as **InFlight**: it retries next tick and never applies.
- Per-host data (`hosts/<host_id>.toml`, `events/<host_id>.jsonl`, `inventory/<host_id>.json`, `tmp/<host_id>/`) is owned by that host ([0010](0010-host-identity-and-config-layering.md)).
- The common `config.toml` is the only multi-writer file. It holds shared settings only, is rarely written, and has an explicit conflict path (`schrodeck config resolve`, [0010](0010-host-identity-and-config-layering.md)).
- File identity uses the allow-list from [contract C](../contracts/profile-format.md#file-allow-list), so stray `.DS_Store`, conflicted copies or `.icloud` placeholders can't wedge a profile in flight.

![store write protocol](store-write.png)

## Consequences

- Good: concurrent pushes can't overwrite each other; they show up as a fork ([0007](0007-conflict-policy.md)).
- Good: a half-delivered push is never applied, and its arrival order doesn't matter.
- Good: the history of every profile is in the store, so the shared history ring is just the commit chain ([0011](0011-history-and-rollback.md)).
- Bad: more files than the old design, and a GC is needed ([0027](0027-store-lifecycle.md)).
- Risk: a sync client that delivers a head long before its tree keeps the profile InFlight for that time. The stuck-in-flight alarm ([0023](0023-store-freshness-via-file-provider.md)) surfaces it.

## Alternatives considered

- **Shared `current/` + `current.json`, last writer wins** (the first design): loses updates. Rejected after review (F1).
- **Shared `current.json` with a `parent` field plus conflict-copy scanning:** still multi-writer, and depends on the provider's conflict-copy behavior, which we have never observed. Rejected.
- **Symlink `ProfilesV3` into the cloud folder:** the app rewrites files on every launch on every host, producing conflicted copies and no per-host path rewriting.
- **A single shared log/inventory file:** concurrent appends create conflicted copies.

## Verified by

No check yet; to be written in the plan (see also [contract D § enforced by](../contracts/store-format.md#enforced-by)):
- A head whose tree is missing a file or has a truncated file ⇒ InFlight, no apply.
- Extra files (`.DS_Store`, `x (conflicted copy).json`, `.icloud`) in a tree ⇒ same `tree_digest`, not InFlight.
- Two simulated hosts pushing concurrently ⇒ Forked on both. Known-bad: the old shared-file protocol loses one edit in the same test.
- An unknown or newer `FORMAT` ⇒ refuse to write.
- The single-writer property: over a simulated multi-host run, every store path is written by at most one host (except `config.toml`).

## References

- None from Elgato (store design).
