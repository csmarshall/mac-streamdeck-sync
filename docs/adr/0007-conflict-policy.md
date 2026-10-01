# 0007. Diverged and forked profiles stop and ask; nothing is merged

Status: Accepted 2026-10-01. Revised 2026-10-01 (review F1): divergence is recorded in the commit graph, so it is detected rather than hoped for.

## Context

Two hosts can both edit a shared profile between syncs. With the commit graph ([0005](0005-direction-detection-three-way-hash.md), [contract D](../contracts/store-format.md)), that shows up in one of two ways:

- **Diverged** on one host: its local copy changed (L ≠ B) and R moved on (R ≠ B).
- **Forked** in the store: two heads that are not ancestry-related, e.g. two pushes from the same parent.

Profiles are nested JSON with images and positional keys, so there is no meaningful automatic merge.

## Decision

- **Preserve both sides in the store.** A Diverged host pushes its local edit as a commit whose parent is its B. That turns a local divergence into a visible fork, so no edit exists only on one machine. Commits and trees are immutable, so both versions are kept automatically.
- **Touch no app files** on any host while the profile is Forked. Hosts that didn't edit (L == B) do not apply either side. They report "forked, waiting for resolve".
- **Notify, deduplicated** ([0016](0016-notifications.md)): one notification per (fork, profile, set of tips) per host, persistent until resolved.
- **`schrodeck resolve <profile> --keep <commit>|local|<history-id>`** on any host writes a `resolve` commit:
  - its **parents are every current tip**;
  - its content is the chosen version.

  It descends from everything, so R is defined again and every host converges through the normal Behind → apply path. That apply happens on the resolving host too, if it chose a version other than its local copy.

## Consequences

- Good: an edit is never silently lost, and every host can see what is waiting.
- Bad: requires human action. Until resolved, that profile doesn't sync on any host. Other profiles continue.
- Risk: a user ignores the notification and keeps editing. Each further edit on a forked host is pushed as another commit on that host's branch (preserved), and `status` shows the profile as forked until resolved.

## Alternatives considered

- **Newest wins:** needs timestamps ([0005](0005-direction-detection-three-way-hash.md)) and silently drops an edit.
- **Shared copy wins:** silently drops the local edit.
- **Per-key merge:** ambiguous for positional layouts and images. A wrong merge is worse than asking.

## Verified by

No check yet; to be written in the plan:
- Both sides change ⇒ no app file is opened for writing on either host (asserted at the filesystem-port level), both commits exist, and both hosts report Forked.
- A third, uninvolved host ⇒ Forked, no apply.
- `resolve --keep local` on host A ⇒ a commit with both tips as parents; host B goes Behind and applies it; all heads become ancestry-ordered.

## References

- None from Elgato (policy decision).
