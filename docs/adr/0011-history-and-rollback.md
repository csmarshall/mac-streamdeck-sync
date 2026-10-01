# 0011. History: a local ring plus the shared commit chain; rollback is a new commit

Status: Accepted 2026-10-01. Revised 2026-10-01 (review F1, F24): the shared ring is now the commit chain kept by GC; `hold`/`resume` are commands (`rollback --hold` removed).

## Context

Any automated change can be wrong (a bad edit synced everywhere, a bad apply). The project owner wants X restore points per host and Y in the shared folder, both configurable, and the ability to roll back.

## Decision

| Ring | Size key (default) | Where | Written |
|---|---|---|---|
| Local | `retention.local` = X (20) | `~/Library/Application Support/schrodeck/history/<profile_id>/` | after the quit, before every swap ([0008](0008-two-phase-apply.md)); before rollback and resolve; after every push; the failed tree after a verify failure |
| Shared | `retention.shared` = Y (20) | the commit chain in the store ([contract D](../contracts/store-format.md)) | every commit is a restore point; GC keeps at least Y generations behind every live head ([0027](0027-store-lifecycle.md)) |

- Default sizes live in the common config. A host may override `retention.local` in its own `hosts/<host_id>.toml` ([0010](0010-host-identity-and-config-layering.md)).
- Local rotation is by count only, and **never removes the entries matching this host's B or the step-5 snapshot of an unfinished journal**.
- `schrodeck history <profile>` lists both rings. Entries with the same hash collapse into one line.
- `schrodeck rollback <profile> <id>` writes a new commit of `kind = rollback`, whose parent is the current R and whose content is the chosen entry. Every host converges on it through the normal Behind → apply path. History is never rewritten.
- `schrodeck hold <profile>` pauses sync for that profile on this host (no push, no apply). `schrodeck resume <profile>` ends it. To try an old version locally without publishing it, run `hold` and then `rollback --local <id>`, which applies through [0008](0008-two-phase-apply.md) but writes no commit while held.

## Consequences

- Good: the local ring survives a lost or corrupted store, and the shared chain survives a lost host.
- Good: rollbacks converge everywhere, and are themselves reversible.
- Bad: storage use is about (X + Y) × profile size. Profiles are small (~100 KB each, observed), but icon-heavy profiles may be larger. Identical trees are stored once, because trees are content-addressed.
- Risk: a hold left on forever silently stops sync for that profile. `status` and a once-per-hold notification surface it ([0016](0016-notifications.md)).

## Alternatives considered

- **Separate snapshot copies in the store** (the first design): duplicates what the immutable commit chain already holds.
- **Time-based pruning:** a long-idle host could lose its only restore point.
- **Rollback as a history rewrite (moving heads backwards):** heads that moved backwards would no longer descend from other hosts' heads, which reads as a fork.

## Verified by

No check yet; to be written in the plan:
- A rotation test with X = 2 where B's entry is the oldest ⇒ it survives.
- A rollback ⇒ a new commit whose parent is the old R; another simulated host applies it.
- `hold` ⇒ zero pushes and zero applies for that profile across runs; `resume` ⇒ normal evaluation.

## References

- None from Elgato.
