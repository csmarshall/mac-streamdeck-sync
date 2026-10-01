# Contract D: store format (schrodeck ↔ schrodeck, across hosts and versions)

The on-disk format of the shared store. **This file is the single definition** of store paths, record fields and the write protocol. ADRs and the spec link here and don't restate fields. Owner: us. Versioned by the store's `FORMAT` file. Decided in ADR [0009](../adr/0009-store-write-protocol.md) (protocol), ADR [0025](../adr/0025-deletion-and-unshare.md) (tombstones) and ADR [0027](../adr/0027-store-lifecycle.md) (migration, GC).

Index of all contracts: [README.md](README.md).

## Invariant: no file in the store has two writers

Every file is either **write-once** (content-addressed or uniquely named, never modified after its final rename) or **owned by exactly one host** (its path contains that host's `host_id`, and only that host writes it). The sync client therefore never has two versions of one file to reconcile, so it never needs to make a conflicted copy of a schrodeck file. The one exception is the common `config.toml`, which is rarely written and has an explicit conflict path (ADR [0010](../adr/0010-host-identity-and-config-layering.md)).

## Layout

```
<store>/
  FORMAT                                   integer store format version (text). Unknown or newer → refuse to write (ADR 0027)
  config.toml                              common config: shared settings only (ADR 0010). The only multi-writer file
  hosts/<host_id>.toml                     per-host config: friendly name, variable values, subscriptions, overrides. Owner: that host
  profiles/<profile_id>/
    profile.json                           write-once at share: {profile_id, name_at_share, geometry, created_by, created_at}
    heads/<host_id>.json                   this host's head for this profile. Owner: that host
    commits/<commit_id>.json               write-once commit records
    trees/<tree_digest>/                   write-once full profile trees (variables substituted)
    tombstone.json                         write-once, written by `unshare` (ADR 0025)
  icon-packs/<pack_id>/<tree_digest>/      write-once icon-pack trees (ADR 0013)
  scripts/<path_id>/<sha256>               write-once opt-in script copies (ADR 0013)
  events/<host_id>.jsonl                   append-only event trail. Owner: that host (ADR 0017)
  inventory/<host_id>.json                 dependency inventory. Owner: that host (ADR 0013, 0014)
  tmp/<host_id>/                           staging area. Owner: that host; cleaned by that host only
```

`profile_id` is a random UUIDv4 assigned by `share` (ADR [0026](../adr/0026-profile-identity.md)). `host_id` is defined in ADR [0010](../adr/0010-host-identity-and-config-layering.md).

## Records

**Commit** (`commits/<commit_id>.json`, write-once):

| field | meaning |
|---|---|
| `hash` | normalized content hash of the tree, per [contract C § normalized hash](profile-format.md#normalized-hash) |
| `tree` | `tree_digest` of the stored tree (below) |
| `parents` | list of parent `commit_id`s: `[]` for the first commit, one for an ordinary push, two or more for a `resolve` |
| `norm_version` | normalization version used for `hash` (integer, ADR [0006](../adr/0006-normalization-and-variables.md)) |
| `app_version` | Stream Deck app version that wrote the source files |
| `fingerprint` | id of the schema fingerprint the source files matched (ADR [0015](../adr/0015-schema-guard.md)) |
| `kind` | `edit` \| `resolve` \| `rollback` \| `rebase` (norm/format migration, not a content change) |
| `updated_by` | `host_id` of the author (display only) |
| `updated_at` | UTC ISO-8601 (display only; never used for decisions, ADR [0005](../adr/0005-direction-detection-three-way-hash.md)) |

`commit_id = sha256(JCS({hash, tree, parents, norm_version, kind}))`. It excludes author and time, so two hosts that independently produce the same content from the same parent produce the same commit.

**Head** (`heads/<host_id>.json`, owned by that host): `{commit_id, updated_at}`. It means "this host's copy of the profile is at `commit_id`". A host rewrites its own head after a successful push or apply.

**Tree** (`trees/<tree_digest>/`, write-once): the full profile directory as the app wrote it, with only variables substituted (ADR [0006](../adr/0006-normalization-and-variables.md)). Runtime fields are **kept**; they are excluded only from `hash`. `tree_digest` = sha256 over the sorted lines `<NFC relative path>\0<sha256(file bytes)>\n` for every file under the allow-list in [contract C](profile-format.md#file-allow-list). The tree folder's own name and `Device.UUID` values are not part of the relative paths.

**Tombstone** (`tombstone.json`, write-once): `{profile_id, commit_id_at_unshare, by, at}` (ADR [0025](../adr/0025-deletion-and-unshare.md)).

**Event line** (`events/<host_id>.jsonl`): `{ts, host_id, profile_id, from_state, to_state, action, commit_ids, trigger, result}` (ADR [0017](../adr/0017-observability.md)).

## Deriving R from the heads

For a profile, read every `heads/*.json` and resolve each `commit_id` (and its ancestors) through `commits/`.

- If any referenced commit or its tree is missing or fails its digest → **InFlight** (the sync client hasn't delivered it yet). Apply nothing for this profile this tick.
- If the head commits are **totally ordered by ancestry** (each is an ancestor of, or equal to, one maximal commit), then **R = that maximal commit**.
- Otherwise (two heads with no ancestor relation, e.g. two pushes with the same parent and different hashes) → **Fork**. There is no R until a `resolve` commit whose parents include every tip.

A head belonging to a retired host is simply an ancestor of R and changes nothing. No logic counts hosts or needs a membership list.

## Write protocol (push)

1. Stage the tree in `tmp/<host_id>/<random>/`, re-read it, and verify `tree_digest` and `hash`.
2. Rename it to `trees/<tree_digest>/`. If the target already exists with the same digest, delete the staged copy: identical content is already present.
3. Write `commits/<commit_id>.json` via stage + rename. If it already exists, it is identical by construction.
4. Wait (bounded, default 5 minutes) for the provider to report the tree and commit as uploaded (ADR [0023](../adr/0023-store-freshness-via-file-provider.md)). The head is moved only after that, so other hosts don't see a head before its data can reach them. If the provider reports `Unknown` freshness, or the wait times out, move the head anyway: readers' InFlight rule makes that safe. Then `status` says "pushed, upload unconfirmed".
5. Rewrite `heads/<host_id>.json` **last**, via stage + rename.

Readers never trust a head whose commit or tree is not fully present and digest-valid (the InFlight rule above). Ordering on other hosts therefore doesn't matter: a head that arrives before its tree only yields InFlight.

## Versioning

- `FORMAT` is an integer. A host reads only the formats it knows and **writes only its own format**. A host that sees a newer `FORMAT` is read-only: it reports status, refuses to push or apply, and notifies once that it needs upgrading (ADR [0027](../adr/0027-store-lifecycle.md)).
- `norm_version` lives in each commit. A host compares hashes only under a matching `norm_version` (ADR [0006](../adr/0006-normalization-and-variables.md)). A change to `norm_version` ships as a `FORMAT` bump with a `rebase` migration (ADR 0027).

## Enforced by

- A reader built for `FORMAT` n refuses `FORMAT` n+1 and writes nothing (test).
- The fork-detection test: two heads with the same parent and different hashes ⇒ Fork on every host.
- The in-flight test: a head whose tree is missing a file, or contains a truncated file ⇒ InFlight, no apply. Extra files (`.DS_Store`, `* (conflicted copy)*`, `.icloud` placeholders) are outside the allow-list and don't change `tree_digest` (test).
- The single-writer test: a store written by two simulated hosts contains no path written by both, except `config.toml`.
