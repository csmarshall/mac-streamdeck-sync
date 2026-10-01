# 0021. Any host whose copy changed may push

Status: Accepted 2026-10-01 (issue #2 remains open only as a confirmation; it no longer blocks this decision)

## Context

The maintainer's framing of the system: an abstracted profile for a given deck size; every node carries a timestamped last-updated value; when what's on a machine becomes newer than what's on the shared mount, the other machines sync.

The first design allowed a push only from a host where a compatible deck was attached. The intent was that a host whose deck had been switched away could never publish stale state. However, Elgato documents that disconnected decks stay visible and editable in the app ([R11](../references.md), documented). A user can deliberately edit a profile with no deck attached, and that rule would silently block the edit.

## Decision

**Any host whose copy has changed since its last sync (L ≠ B) may push, whether or not a deck is attached.**

"Newer than the mount" is put into practice as **changed since last sync** by the 3-way hash ([0005](0005-direction-detection-three-way-hash.md)), not as a clock comparison. Clocks skew between hosts, and the app touches files with no edit ([R15](../references.md)), so neither wall-clock time nor mtime can show which side is newer.

Every copy and every `current.json` still records `last_updated` (UTC) and `updated_by`. These are shown in `status`, notifications and the log, but never used to choose direction (see "Timestamps" in [0005](0005-direction-detection-three-way-hash.md)).

A host whose copy did **not** change (L == B) can never push, attached or not. The only risk the attached-only rule guarded against was a false L ≠ B caused by the app's own rewrites. That is normalization's job ([0006](0006-normalization-and-variables.md)), and the per-profile backoff in [0012](0012-triggers.md) contains any damage from a normalization bug.

## Consequences

- Good: real offline edits are always published, and there's one fewer rule to explain.
- Good: works the same for virtual decks, which are never "attached" over USB ([0003](0003-decks-are-local-geometry-compatibility.md)).
- Bad: a normalization bug on an idle host could push noise. It would be hash-only noise, caught by the launch-rewrite tests ([0006](0006-normalization-and-variables.md)), and repeated pushes/applies are slowed by the exponential backoff ([0012](0012-triggers.md)).

## Alternatives considered

- **Only a host with a compatible deck attached may push:** an extra safety net against normalization bugs, but edits made with the deck switched away (documented as possible, [R11](../references.md)) are silently not published until a deck is attached. Rejected.
- **Newest timestamp wins:** clock skew and the app's no-edit rewrites make timestamps unreliable ([0005](0005-direction-detection-three-way-hash.md)). Rejected.

## Verified by

No check yet; to be written in the plan:
- A test where an idle host (L == B) with no deck attached runs a sync ⇒ no push.
- A test where a host with no deck attached edits a copy (L ≠ B, R == B) ⇒ push.
- The launch-rewrite tests in [0006](0006-normalization-and-variables.md) and the ping-pong backoff test in [0012](0012-triggers.md).
- Issue #2 confirms on real hardware that an offline edit changes `ProfilesV3` on disk.

## References

- [R11](../references.md): disconnected decks remain editable (documented)
- [R15](../references.md): the app rewrites files with no user edit (observed)
