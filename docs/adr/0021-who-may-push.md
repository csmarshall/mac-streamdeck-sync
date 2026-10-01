# 0021. Who may push a changed copy

Status: **Proposed** (pending issue #2)

## Context

The first design allowed a push only from a host where a compatible deck was attached. The intent was that a host whose deck was switched away could never publish stale state. However, Elgato documents that disconnected decks stay visible and editable in the app ([R11](../references.md), documented). A user could deliberately edit a profile on a host with no deck attached, and that rule would block the edit.

With 3-way hashing ([0005](0005-direction-detection-three-way-hash.md)), a host whose copy did not change (L == B) can never push, attached or not. The only risk the attached-only rule guarded against is a false L ≠ B caused by app rewrites. That is the job of normalization ([0006](0006-normalization-and-variables.md)) and its tests.

## Options

1. **Any host whose copy has L ≠ B may push** (recommended): correct edits are never blocked, and normalization carries the load.
2. **Only a host with a compatible deck attached may push:** an extra safety net against normalization bugs, but blocks real offline edits.

## Recommendation

Option 1, plus the rate limit from [0012](0012-triggers.md), which bounds damage from a normalization bug. Decide after issue #2 confirms whether offline edits change `ProfilesV3` on disk.

## Consequences

- Option 1: one fewer rule to explain. A normalization bug on an idle host could push noise, but it would be hash-only noise and would be caught by the launch-rewrite tests and the rate limit.
- Option 2: offline edits are silently never published until a deck is attached.

## Verified by

Pending the decision. For option 1, the launch-rewrite tests ([0006](0006-normalization-and-variables.md)) are the guard.

## References

- [R11](../references.md): disconnected decks remain editable (documented)
