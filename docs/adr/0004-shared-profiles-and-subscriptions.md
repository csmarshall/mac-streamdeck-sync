# 0004. The unit of sync is a shared profile, with subscriptions

Status: Accepted 2026-10-01

## Context

A host has several profiles per deck, and some are app-linked "Smart Profiles" ([R13](../references.md)). Not every profile should be shared, and the same profile may need to run on different physical decks on different hosts. Pages and folders live inside a profile's directory (`<profile>.sdProfile/Profiles/<page>/`, observed [R2](../references.md)).

## Decision

- The unit of sync is a **shared profile**, using Elgato's own term for a profile.
- `schrodeck share "<profile>"` opts a profile in on any host and publishes it.
- Other hosts are notified that the profile is available. `schrodeck subscribe <profile> [--deck <deck>]` installs a copy onto **any local deck of compatible geometry** ([0003](0003-decks-are-local-geometry-compatibility.md)). That deck can be a **different physical deck**.
- **Unshared profiles are never read for sync, written, or deleted.**
- **Every subscribed copy is kept up to date**, whether or not it is the deck's active profile.
- Pages and folders travel inside the profile.
- Re-flowing a profile onto a different geometry (e.g. 32 keys onto 15) is **out of scope for v1**.

## Consequences

- Good: explicit opt-in. A user's private or experimental profiles are never touched.
- Good: inactive copies can't go stale, so switching to a subscribed profile never shows an old layout. App-linked profiles are covered too.
- Good: "create on one machine, use on many" works, including different decks of the same geometry.
- Bad: subscriptions are one more concept to learn, plus `share`/`subscribe`/`unsubscribe` commands.
- Risk: applying an update to an *inactive* copy still restarts the app ([0008](0008-two-phase-apply.md)), so a user can see the deck blink for a change they're not looking at.

## Alternatives considered

- **Sync the whole `ProfilesV3` tree:** touches everything, and breaks with per-host profiles.
- **Unit = deck:** a profile could never move to a different physical deck.
- **Update only the active copy:** inactive copies go stale and diverge when edited while stale (rejected by the project owner).
- **Automatic re-flow across geometries:** ambiguous (which keys go where?). Deferred.

## Verified by

No check yet; to be written in the plan:
- A test that **no unshared profile file is ever opened for writing**, asserted at the filesystem-port level (review F19). A before/after byte comparison would be fooled by the running app's own rewrites ([R15](../references.md)), and can't fail against fakes.
- A test that an inactive subscribed copy is updated.

## References

- [R2](../references.md): profile directory layout, pages inside (observed)
- [R10](../references.md): profiles are device-specific (documented)
- [R13](../references.md): Smart Profiles (documented)
