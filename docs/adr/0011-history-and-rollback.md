# 0011. Two history rings; rollback is a new generation

Status: Accepted 2026-10-01

## Context

Any automated change can be wrong (a bad edit synced everywhere, a bad apply). The project owner wants X restore points per host and Y in the shared folder, both configurable, and the ability to roll back.

## Decision

| Ring | Size key (default) | Where | Written |
|---|---|---|---|
| Local | `retention.local` = X (20) | `~/Library/Application Support/schrodeck/history/<profile>/` | before every apply, rollback, and resolve; after every push |
| Shared | `retention.shared` = Y (20) | `<store>/profiles/<profile-id>/snapshots/` | every push; both sides of every diverge |

- Both sizes live in the common config. A host-keyed override is allowed there ([0010](0010-host-identity-and-config-layering.md)).
- Rotation is by count only and **never removes the entries matching the current R or this host's B**.
- `schrodeck history` lists both rings. Entries with the same hash collapse into one line.
- `schrodeck rollback <id>` applies the entry through [0008](0008-two-phase-apply.md), then pushes it as a **new generation**. History is never rewritten.
- `schrodeck rollback <id> --hold` keeps the rollback on this host only and pauses sync for that profile until `schrodeck resume`.

## Consequences

- Good: the local ring survives a lost or corrupted store, and the shared ring survives a lost host.
- Good: rollbacks converge everywhere, and are themselves reversible.
- Bad: storage use is about (X + Y) × profile size. Profiles are small (~100 KB each, observed), but icon-heavy profiles may be larger.
- Risk: `--hold` left on forever silently stops sync for that profile. `status` and a once-per-hold notification surface it.

## Alternatives considered

- **Time-based pruning:** a long-idle host could lose its only restore point.
- **Rollback as a history rewrite (reset R to an old entry):** other hosts' B would point at hashes that no longer exist as "current", which confuses the 3-way logic.

## Verified by

No check yet; to be written in the plan:
- A rotation test with X=2 where the R/B entries are the oldest ⇒ they survive.
- A rollback test ⇒ R changes to the old tree's hash, and the shared ring gains an entry.

## References

- None from Elgato.
