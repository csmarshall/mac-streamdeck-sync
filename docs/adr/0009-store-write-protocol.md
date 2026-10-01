# 0009. Store write protocol: stage, verify, rename, publish last

Status: Accepted 2026-10-01

## Context

The store is replicated by a sync client ([0002](0002-transport-shared-cloud-folder.md)). A reader may see a partially synced tree, files arriving out of order, or "conflicted copy" files when two hosts write the same path.

## Decision

Store layout (per shared profile):

```
<store>/FORMAT                               store schema version; refuse unknown
<store>/config.toml                          common config (0010)
<store>/profiles/<profile-id>/current/       normalized tree
<store>/profiles/<profile-id>/current.json   {hash, pushed_by, pushed_at, app_version, geometry}
<store>/profiles/<profile-id>/snapshots/     shared history ring (0011)
<store>/events/<host_id>.jsonl               per-host, append-only (0017)
<store>/inventory/<host_id>.json             per-host (0013, 0014)
```

Push protocol:
1. Write `current.tmp-<host_id>/`.
2. Re-read it and verify the hash.
3. Rename it to `current/`.
4. Write `current.json` **last**.

Readers compare `current.json`'s hash with the hash of the tree they actually read. A mismatch means **in flight**: retry next tick and never apply. Files written by more than one host are avoided. Per-host files are append-only or owned by a single host.

![store write protocol](store-write.png)

## Consequences

- Good: a half-synced tree is never applied.
- Good: per-host files make sync-client conflicted copies impossible for logs and inventory.
- Bad: renames are atomic only locally. On other hosts the sync client may deliver `current/` file by file, which is exactly what the in-flight check exists for.
- Risk: two hosts pushing the same profile at once both rename `current/`. The sync client picks one, and the loser's `current.json` describes a tree that isn't there. The in-flight check catches it, and the next tick re-evaluates. This only happens when both hosts are Ahead, which makes it a true Diverged that [0007](0007-conflict-policy.md) handles.

## Alternatives considered

- **Symlink `ProfilesV3` into the cloud folder:** the app rewrites files on every launch on every host, producing conflicted copies and no per-host path rewriting.
- **Overwrite files in place:** readers see half-written trees.
- **A single shared log/inventory file:** concurrent appends from hosts create conflicted copies.

## Verified by

No check yet; to be written in the plan:
- A test that truncates or removes files from `current/` after `current.json` is written ⇒ the reader reports in flight and applies nothing.
- A test that FORMAT is unknown ⇒ refuse.

## References

- None from Elgato (store design).
