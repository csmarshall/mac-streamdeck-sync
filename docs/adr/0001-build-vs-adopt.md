# 0001. Build schrodeck rather than adopt an existing tool

Status: Accepted 2026-10-01

## Context

The goal is unattended, conflict-safe sync of Stream Deck profiles across several computers. The closest existing tool, [dominik-ba/stream-deck-profile-sync](https://github.com/dominik-ba/stream-deck-profile-sync), offers manual `push`/`pull` through a cloud folder, MD5-based `status`, and timestamped backups. It does not detect direction automatically, handle conflicts, rewrite paths per host, restart the app around a change, or run on a schedule or trigger.

Elgato's own mechanism, `custom_default_profiles` ([R3](../references.md)), only seeds default profiles for fresh installs. It does not keep machines in sync. Elgato offers account sync for Stream Deck **Mobile** only ([R18](../references.md)).

## Decision

Build schrodeck as a new project. Cite dominik-ba's tool as prior art in the README.

## Consequences

- Good: the design can be built around the hard parts (direction detection, conflicts, live-app surgery) instead of retrofitting them.
- Good: no inherited assumptions (e.g. syncing plugins wholesale, which conflicts with [0014](0014-plugin-handling.md)).
- Bad: more code to own and maintain, and no existing user base or bug history to learn from.
- Risk: duplicating effort if the prior art grows these features. Revisit if it does.

## Alternatives considered

- **Fork dominik-ba's tool:** its core model (manual push/pull of whole directories) differs from ours (3-way hash, per shared profile, two-phase apply), so most of it would be rewritten.
- **Elgato `custom_default_profiles`:** a one-time seeding mechanism, not sync.
- **Symlink `ProfilesV3` into Dropbox:** rejected in [0009](0009-store-write-protocol.md).

## Verified by

Not testable as code. Re-checked by a prior-art search before each major release (manual).

## References

- [R3](../references.md): `custom_default_profiles`, "not officially supported" (documented)
- [R18](../references.md): account sync exists for Stream Deck Mobile only (documented)
