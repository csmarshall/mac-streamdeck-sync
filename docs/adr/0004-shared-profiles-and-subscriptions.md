# 0004. The unit of sync is a setup (one shared profile), with member copies

Status: Accepted 2026-10-01. Revised 2026-10-01 (reframe: template-seeded setups): `share` creates a new named copy from a template instead of syncing the chosen profile in place; `subscribe` always creates a new profile on a chosen destination device. See [0029](0029-problem-statement-and-setup-model.md).

## Context

A host has several profiles per deck, and some are app-linked "Smart Profiles" ([R13](../references.md)). Not every profile should be shared, and the same profile may need to run on different physical decks on different hosts. Pages and folders live inside a profile's directory (`<profile>.sdProfile/Profiles/<page>/`, observed [R2](../references.md)).

## Decision

- The unit of sync is a **setup**: one shared profile with a permanent `profile_id`, a geometry and a user-provided name ([0029](0029-problem-statement-and-setup-model.md)). "Shared profile" remains Elgato's term for what a setup contains.
- `schrodeck share` (and `init` on the first Mac): the user picks a device and a **template** profile on it. schrodeck creates a **new** profile on that device, copied from the template and named `schrodeck - <cols>x<rows> - <name>`. That copy is the setup's first **member copy**. The template is never modified or synced.
- `schrodeck subscribe` (and `join`): other Macs list the setups whose geometry matches one of their devices; the user picks a setup and a destination device, and schrodeck **always creates a new profile** there ([0003](0003-decks-are-local-geometry-compatibility.md), [0026](0026-profile-identity.md)). The destination can be a **different physical deck** from the template's.
- **Profiles that aren't member copies are never read for sync, written, or deleted** (templates included).
- **Every member copy is kept up to date**, whether or not it is the deck's active profile. Member copies are peers: an edit on any of them syncs to all.
- Pages and folders travel inside the profile.
- Re-flowing a profile onto a different geometry (e.g. 32 keys onto 15) is **out of scope for v1**.

## Consequences

- Good: explicit opt-in, and schrodeck only ever manages profiles it created. A user's own profiles, including the template, are never touched.
- Good: inactive copies can't go stale, so switching to a subscribed profile never shows an old layout. App-linked profiles are covered too.
- Good: "create on one machine, use on many" works, including different decks of the same geometry.
- Bad: setups and member copies are one more concept to learn, and sharing a profile leaves a copy next to the original.
- Risk: applying an update to an *inactive* copy still restarts the app ([0008](0008-two-phase-apply.md)), so a user can see the deck blink for a change they're not looking at.

## Alternatives considered

- **Sync the whole `ProfilesV3` tree:** touches everything, and breaks with per-host profiles.
- **Unit = deck:** a profile could never move to a different physical deck.
- **Sync the chosen profile in place** (the first version of this ADR): the user's own profile would become a target that other Macs overwrite. Replaced by template-seeded copies ([0029](0029-problem-statement-and-setup-model.md)).
- **Update only the active copy:** inactive copies go stale and diverge when edited while stale (rejected by the project owner).
- **Automatic re-flow across geometries:** ambiguous (which keys go where?). Deferred.

## Verified by

No check yet; to be written in the plan:
- A test that **no profile other than a member copy (templates included) is ever opened for writing**, asserted at the filesystem-port level (review F19). A before/after byte comparison would be fooled by the running app's own rewrites ([R15](../references.md)), and can't fail against fakes.
- A test that an inactive subscribed copy is updated.

## References

- [R2](../references.md): profile directory layout, pages inside (observed)
- [R10](../references.md): profiles are device-specific (documented)
- [R13](../references.md): Smart Profiles (documented)
