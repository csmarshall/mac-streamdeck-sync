# 0011. History: a local ring plus the shared revision chain; rollback is a new revision

Status: Accepted 2026-10-01. Revised 2026-10-01 (review F1, F24): the shared ring is now the revision chain kept by GC; `hold`/`resume` are commands (`rollback --hold` removed). Revised 2026-10-01 (review round 2: F25): the shared ring is bounded by tree GC; revisions are kept forever. Revised 2026-10-01 (review round 3: F40, F44): a rollback to an ancestor's content is guaranteed to win (equivalence is tips-only); a collected store tree falls back to the local ring.

## Context

Any automated change can be wrong (a bad edit synced everywhere, a bad apply). The project owner wants X restore points per host and Y in the shared folder, both configurable, and the ability to roll back.

## Decision

| Ring | Size key (default) | Where | Written |
|---|---|---|---|
| Local | `retention.local` = X (20) | `~/Library/Application Support/schrodeck/history/<profile_id>/` | after the quit, before every swap ([0008](0008-two-phase-apply.md)); before rollback and resolve; after every push; the failed tree after a verify failure |
| Shared | `retention.shared` = Y (20) | the revision chain in the store ([contract D](../contracts/store-format.md)) | every revision is a restore point. Revision records are kept forever, but GC removes **trees** more than Y generations behind every head ([contract D § garbage collection](../contracts/store-format.md#garbage-collection-trees-only)), so the restorable range is the last Y versions; older entries show in `history` as "tree collected" |

- Default sizes live in the common config. A host may override `retention.local` in its own `hosts/<host_id>.toml` ([0010](0010-host-identity-and-config-layering.md)).
- Local rotation is by count only, and **never removes the entries matching this host's B or the step-5 snapshot of an unfinished journal**.
- `schrodeck history <profile>` lists both rings. Entries with the same hash collapse into one line.
- `schrodeck rollback <profile> <id>` writes a new revision of `kind = rollback`, whose parent is the current R and whose content is the chosen entry. Every host converges on it through the normal Behind → apply path. History is never rewritten.
- `schrodeck hold <profile>` pauses sync for that profile on this host (no push, no apply). `schrodeck resume <profile>` ends it. To try an old version locally without publishing it, run `hold` and then `rollback --local <id>`, which applies through [0008](0008-two-phase-apply.md) but writes no revision while held. The held target's tree is added to this host's `pins` so GC keeps it until `resume`.

## Consequences

- Good: the local ring survives a lost or corrupted store, and the shared chain survives a lost host.
- Good: rollbacks converge everywhere, and are themselves reversible.
- Bad: storage use is about (X + Y) × profile size. Profiles are small (~100 KB each, observed), but icon-heavy profiles may be larger. Identical trees are stored once, because trees are content-addressed.
- Risk: a hold left on forever silently stops sync for that profile. `status` and a once-per-hold notification surface it ([0016](0016-notifications.md)).

## Alternatives considered

- **Separate snapshot copies in the store** (the first design): duplicates what the immutable revision chain already holds.
- **Time-based pruning:** a long-idle host could lose its only restore point.
- **Rollback as a history rewrite (moving heads backwards):** heads that moved backwards would no longer descend from other hosts' heads, which reads as a fork.

## Verified by

No check yet; to be written in the plan:
- A rotation test with X = 2 where B's entry is the oldest ⇒ it survives.
- A rollback ⇒ a new revision whose parent is the old R; another simulated host applies it.
- **Rollback to an ancestor's exact content** (review F40): A(h0) → C(h1), rollback to A ⇒ RB(h0, parent C) is R on every host, and no host re-applies C. Known-bad: with equivalence evaluated against ancestors (the round-2 rule), C also qualifies as R and the rollback can be silently reverted; the test must fail against that rule.
- A rollback whose store tree has been collected ⇒ restored from the local ring (review F44).
- `hold` ⇒ zero pushes and zero applies for that profile across runs; `resume` ⇒ normal evaluation.

## References

- None from Elgato.
