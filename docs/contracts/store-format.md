# Contract D: store format (schrodeck ↔ schrodeck, across hosts and versions)

The on-disk format of the shared store. **This file is the single definition** of store paths, record fields, the write protocol, how R is derived, and what garbage collection may delete. ADRs and the spec link here and don't restate fields. Owner: us. Versioned by the store's `FORMAT` file. Decided in ADR [0009](../adr/0009-store-write-protocol.md) (protocol), ADR [0005](../adr/0005-direction-detection-three-way-hash.md) (direction), ADR [0025](../adr/0025-deletion-and-unshare.md) (tombstones, reshare) and ADR [0027](../adr/0027-store-lifecycle.md) (migration, GC).

Index of all contracts: [README.md](README.md).

## Invariant: every store file has exactly one writer

Every path written by schrodeck contains the `host_id` of the only host that ever writes it. Two hosts never write the same path, even with identical content. The sync client therefore never has two versions of one file to reconcile, and never needs to make a conflicted copy of a schrodeck file.

Write-once objects that two hosts might both produce (identical commits, identical trees) are stored **once per writer** under a host-scoped path. A reader accepts any writer's copy that passes its digest check. The only exception is the common `config.toml`, which is rarely written and has an explicit conflict path (ADR [0010](../adr/0010-host-identity-and-config-layering.md)).

Revised 2026-10-01 (review F26): the first version let two hosts write the same `commits/<id>.json`, `trees/<digest>/` and `tombstone.json`.

## Layout

```
<store>/
  FORMAT                                        integer store format version (text). Written only by `migrate` (ADR 0027)
  config.toml                                   common config: shared settings only (ADR 0010). The only multi-writer file
  hosts/<host_id>.toml                          per-host config: friendly name, variable values, subscriptions, pins. Owner: that host
  profiles/<profile_id>/
    profile.json.<host_id>                      write-once at share: {profile_id, geometry}. Owner: the sharing host
    heads/<host_id>.json                        this host's head for this profile. Owner: that host
    commits/<commit_id>/<host_id>.json          commit record, written once by each host that produced it
    trees/<tree_digest>/<host_id>/              stored tree, written once by each host that uploaded it
    tombstones/<host_id>.json                   this host's unshare records (append-only list). Owner: that host
    reshares/<host_id>.json                     this host's reshare records (append-only list). Owner: that host
  icon-packs/<pack_id>/<tree_digest>/<host_id>/ icon-pack trees (ADR 0013)
  scripts/<path_id>/<sha256>.<host_id>          opt-in script copies (ADR 0013)
  events/<host_id>.jsonl                        append-only event trail. Owner: that host (ADR 0017)
  inventory/<host_id>.json                      dependency inventory. Owner: that host (ADR 0013, 0014)
  tmp/<host_id>/                                staging area. Owner: that host; cleaned by that host only
```

`profile_id` is a random UUIDv4 assigned by `share` (ADR [0026](../adr/0026-profile-identity.md)). `host_id` is defined in ADR [0010](../adr/0010-host-identity-and-config-layering.md).

## Records

**Commit** (`commits/<commit_id>/<host_id>.json`, write-once). The file contains **exactly** the fields that make up the id, serialized with RFC 8785 (JCS), so every writer produces byte-identical content:

| field | meaning |
|---|---|
| `hash` | normalized content hash of the tree, per [contract C § normalized hash](profile-format.md#normalized-hash) |
| `tree` | `tree_digest` of the stored tree (below) |
| `parents` | list of parent `commit_id`s: `[]` for the root, one for an ordinary push, two or more for a `resolve` or `rebase` that joins tips |
| `norm_version` | normalization version used for `hash` (integer, ADR [0006](../adr/0006-normalization-and-variables.md)) |
| `fingerprint` | the format fingerprint the source files matched (ADR [0015](../adr/0015-schema-guard.md)) |
| `kind` | `edit` \| `resolve` \| `rollback` \| `rebase` (norm/format migration, not a content change) |

`commit_id = sha256(JCS(commit record))`. Author and time are **not** in the commit. Who wrote a commit, when, and with which app version is per-host metadata: the writer's head (below) and its event line (ADR [0017](../adr/0017-observability.md)). `status` shows "updated by <host>" from the first head and event that reference the commit.

**Head** (`heads/<host_id>.json`, owned by that host): `{commit_id, updated_at, app_version}`. It means "this host's copy of the profile is at `commit_id`". `updated_at` (UTC) and `app_version` are display-only (ADR 0005). A host rewrites its own head only after a successful push or apply, and B is set only after that write succeeds (review F34).

**Tree** (`trees/<tree_digest>/<host_id>/`, write-once): the full profile directory as the app wrote it, with variables replaced by placeholders (ADR [0006](../adr/0006-normalization-and-variables.md)), **including the reserved placeholder `{{DEVICE}}` in place of every `Device.UUID` value**. A raw device id, which embeds the deck's USB serial ([R9](../references.md)), therefore never reaches the shared folder (review F27). Runtime fields are kept; they are excluded only from `hash`. `tree_digest` = sha256 over the sorted lines `<NFC relative path>\0<sha256(file bytes)>\n` for every file under the allow-list in [contract C](profile-format.md#file-allow-list), computed on the stored (placeholder) bytes. The folder's own name is not part of the relative paths.

**Tombstone and reshare records** (ADR [0025](../adr/0025-deletion-and-unshare.md)). Each host appends to its own list:

- `tombstones/<host_id>.json`: a list of `{record_id, profile_id, at_commit, supersedes: [reshare record_id…], updated_at}`.
- `reshares/<host_id>.json`: a list of `{record_id, profile_id, supersedes: [tombstone record_id…], updated_at}`.

`record_id = sha256(JCS(record without record_id and updated_at))`. Each record lists the records of the other kind it has seen and supersedes. A profile is **Unshared** if any tombstone record is not superseded by some reshare record. A reshare written while another host concurrently unshares does not supersede the unseen tombstone, so the profile stays unshared. That is the safe outcome: nothing is applied, and a second `reshare` fixes it.

**Event line** (`events/<host_id>.jsonl`): `{ts, host_id, profile_id, from_state, to_state, action, commit_ids, trigger, result}` (ADR [0017](../adr/0017-observability.md)).

## Deriving R from the heads

This is evaluated per profile, **after** the freshness check: if the provider does not report the profile's store files as current, the profile is InFlight and nothing below runs (review F30).

1. Read every `heads/*.json`. Resolve each head's `commit_id` and its ancestors through `commits/` (any writer's copy whose JCS bytes hash to the id). Commits are never garbage-collected, so a missing commit can only mean the sync client hasn't delivered it yet → **InFlight**.
2. **Equivalence.** Two commits are *equivalent* (≡) if they are the same commit, or have the same `hash` under the same `norm_version`. Equivalent commits have identical normalized content, so they are never treated as a fork (review F27, owner's decision).
3. **Subsumption.** Commit X *subsumes* head H if H's commit is X, an ancestor of X, or equivalent to X or to one of X's ancestors.
4. **Version filter.** Only heads whose commit's `norm_version` is ≥ this host's current `norm_version` count as live tips. A head at an older `norm_version` is a host that hasn't upgraded yet. It is subsumed through the `rebase` commit's parent link once `migrate` has run, and otherwise ignored for R (review F28). If **no** head is at this host's `norm_version` or newer, the store hasn't been migrated to this host's version yet: the profile is **VersionMismatch** (read-only, "run `schrodeck migrate`"). An upgraded host never pushes into an older-`FORMAT` store except through `migrate` (ADR [0027](../adr/0027-store-lifecycle.md)).
5. If one commit among the live tips subsumes every live head, **R = that commit**. When several equivalent commits qualify, R is the one with the lowest `commit_id`, so every host picks the same one.
6. Otherwise (two live heads that neither subsume each other nor are equivalent) → **Forked**. There is no R until a `resolve` commit whose parents include every tip.
7. To apply R, its tree must be present: at least one writer's copy of `trees/<R.tree>/` must pass its digest check. If none does → **InFlight**.

A head belonging to a retired host is simply subsumed by R and changes nothing. No logic counts hosts or needs a membership list.

**Anomaly** (review F34): if R does not subsume this host's own `B.commit` (for example, the host's head was lost, or a provider restore reverted the store), the profile stops with state `Anomaly` and notifies once. It is never read as Behind, because applying R would silently revert content. `schrodeck resolve` chooses.

## Write protocol (push)

1. Stage the tree in `tmp/<host_id>/<random>/`, re-read it, and verify `tree_digest` and `hash`.
2. Rename it to `trees/<tree_digest>/<host_id>/`. If this host's copy already exists and passes its digest check, delete the staged copy. **Another host's copy is never trusted as a substitute**: it may be mid-GC on its writer or partially delivered (review F25).
3. Write `commits/<commit_id>/<host_id>.json` via stage + rename (skip only if this host's own copy exists and is valid).
4. Wait (bounded, default 5 minutes) for the provider to report the tree and commit as uploaded (ADR [0023](../adr/0023-store-freshness-via-file-provider.md)). The head is moved only after that, so other hosts don't see a head before its data can reach them. If the provider reports `Unknown` freshness, or the wait times out, move the head anyway: readers' InFlight rule makes that safe. `status` then says "pushed, upload unconfirmed".
5. Rewrite `heads/<host_id>.json` **last**, via stage + rename.
6. Only after step 5 succeeds: set this host's local B := (commit_id, L).

Readers never trust a head whose commit or tree is not fully present and digest-valid. Ordering on other hosts therefore doesn't matter: a head that arrives before its data only yields InFlight.

## Garbage collection: trees only

Owner's decision (review F25). The ancestry walk needs commits, and commits are small (a few hundred bytes), so **commit records are never deleted**. GC deletes **tree copies only**, and each host deletes only **its own** copies (`trees/<digest>/<host_id>/`). A tree digest is **pinned**, and none of its copies is deleted, while it is:

- the tree of R or of any head's commit (the tips);
- the tree of any commit within `retention.shared` (Y) generations behind any head (restore points, ADR [0011](../adr/0011-history-and-rollback.md));
- listed in any host's `hosts/<host_id>.toml` `pins` (e.g. a held `rollback --local` target, or a live B awaiting re-materialization, ADR [0006](../adr/0006-normalization-and-variables.md)).

A rollback or re-materialization whose target tree has been collected fails cleanly ("tree collected; choose a newer restore point"). It never applies a partial tree.

## Versioning

- `FORMAT` is an integer. A host reads only the formats it knows and **writes only its own format**. A host that sees a newer `FORMAT` is read-only: it reports status, refuses to push or apply, and notifies once that it needs upgrading (ADR [0027](../adr/0027-store-lifecycle.md)).
- `norm_version` lives in each commit. Hashes are compared only under a matching `norm_version` (ADR [0006](../adr/0006-normalization-and-variables.md)). A change to `norm_version` ships as a `FORMAT` bump with a `rebase` migration (ADR 0027).

## Enforced by

- A reader built for `FORMAT` n refuses `FORMAT` n+1 and writes nothing (test).
- **Single writer:** a store written by several simulated hosts, including two hosts producing the identical commit and tree, contains no path written by more than one host, except `config.toml`. Known-bad: the same simulation with host-less commit paths must show a shared path.
- **Fork detection:** two heads with the same parent and different hashes ⇒ Forked on every host. **Equivalence:** two heads with different commit ids but the same hash and `norm_version` ⇒ InSync, not Forked.
- **In flight:** a head whose commit is missing, or whose R tree is missing a file or has a truncated file ⇒ InFlight, no apply. Extra files (`.DS_Store`, `* (conflicted copy)*`, `.icloud` placeholders) are outside the allow-list and don't change `tree_digest`.
- **GC never wedges R:** 50 commits with Y = 2, then GC on every host ⇒ R still derivable on every host, with no InFlight. Known-bad: a GC that also deletes commits must wedge this test.
- **Anomaly:** a host's own head reverted to an ancestor ⇒ Anomaly, no apply.
- **No raw device ids in the store:** every stored tree contains `{{DEVICE}}` and no `@(` device id string (scan test over a store written from real-shaped fixtures).
