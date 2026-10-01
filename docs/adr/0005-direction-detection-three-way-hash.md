# 0005. Direction is decided by a 3-way normalized content hash

Status: Accepted 2026-10-01

## Context

Each host must decide on its own, per shared profile, whether its copy is behind, ahead, in sync, or diverged. Timestamps are useless for this. The app rewrites every top-level manifest on launch and touches page manifests at runtime with no user edit ([R15](../references.md), observed). Host clocks can also disagree.

## Decision

For each subscribed copy of each shared profile, on each host:

```
L = hash(normalize(local copy))            see 0006
R = hash recorded in the store's current.json
B = last hash this host synced for this copy (local state)
```

| Condition | State | Action |
|---|---|---|
| L == R | InSync | B := L |
| L == B, R ≠ B | Behind | apply ([0008](0008-two-phase-apply.md)) |
| L ≠ B, R == B | Ahead | push ([0009](0009-store-write-protocol.md)); who may push: [0021](0021-who-may-push.md) |
| L ≠ B, R ≠ B, L ≠ R | Diverged | [0007](0007-conflict-policy.md) |
| no B, R absent | FirstRun → Ahead | push |
| no B, R present, L ≠ R | FirstRun → Diverged | notify |

![sync state diagram](../sync-states.png)

**No decision ever reads a file mtime or a wall-clock timestamp.**

## Consequences

- Good: scales to any number of hosts with no coordination. Each host stores only its own B.
- Good: immune to clock skew and to the app's own rewrites, provided normalization is right.
- Bad: correctness rests entirely on normalization ([0006](0006-normalization-and-variables.md)). A missed runtime field makes every launch look like an edit.
- Risk: if B is lost (local state deleted), the next run is FirstRun, which is safe (diverged → ask) but noisy.

## Alternatives considered

- **mtime / newest wins:** the app rewrites files with no edit ([R15](../references.md)), so this fires on every launch.
- **A generation counter in the store:** needs atomic increments, which a sync client can't provide, and still needs content comparison to detect local edits.
- **Two-way compare (L vs R only):** cannot tell "I'm behind" from "I'm ahead".

## Verified by

No check yet; to be written in the plan: a table-driven test covering each row above, including a known-bad case (a launch-only rewrite must yield InSync; if normalization is disabled, the same fixture must yield Ahead).

## References

- [R15](../references.md): runtime-only rewrites (observed)
