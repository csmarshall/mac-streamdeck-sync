# 0026. Profile identity: a schrodeck `profile_id`, mapped to local folders per host

Status: Accepted 2026-10-01 (review F16). Revised 2026-10-01 (review round 2: F31, F38): the deck key is the app's prefs device key and must be unique on the host; a lost local state is rebuilt from this host's own head.

## Context

The app names each profile folder `ProfilesV3/<UUID>.sdProfile/` ([R2](../references.md), observed). The design said receivers get a folder id of uuid5(profile, deck), but never defined:

- how the sharing host's original folder relates to the shared profile;
- where the mapping is stored;
- what happens when the target deck disappears.

Virtual decks need a physical deck seen within the last 30 days, or they go away ([R12](../references.md), documented). Profiles can also be renamed at any time.

## Decision

- **`profile_id`** is a random UUIDv4 assigned by `share`, recorded in the write-once `profiles/<profile_id>/profile.json` ([contract D](../contracts/store-format.md)). It never changes. A profile's display name is just content, so renames sync as ordinary edits.
- **The sharing host keeps its original folder.** Its local state maps `local folder UUID → (profile_id, deck)`.
- **Subscribers** install into a new folder named `uuid5(NAMESPACE_SCHRODECK, profile_id + ":" + deck_key)`. The name is deterministic, so a host that loses its local state can rebuild the mapping by recomputing the names. A subscribed copy never reuses an existing local folder.
- **`deck_key`** is the key of the deck's entry in the app's prefs `Devices` dictionary ([R14](../references.md), observed; [contract A](../contracts/os-connector.md) `DeviceEnumerator.AppDeviceID`), not the manifest's `Device.UUID` (review F31). A deck can only be subscribed onto if its key is **unique** among this host's devices. Virtual decks are observed with the empty id `@(0)[]` ([R12](../references.md)): one virtual deck on a host is fine, but if two devices share a key, `subscribe` refuses to target either of them and says why, rather than letting two copies collide on one folder name.
- **Where the mapping lives:**
  - local state, which is authoritative for this host;
  - `hosts/<host_id>.toml`'s subscriptions (`profile_id → deck`), which `status` on other hosts can display.

  On the sharing host, the original folder UUID is also recorded in its `hosts/<host_id>.toml`, so its mapping can be rebuilt as well.
- **Choosing a deck:** `subscribe` requires `--deck` when more than one local deck has a matching geometry. `join`'s rundown asks ([0022](0022-onboarding-init-and-join.md)). Join never auto-subscribes.
- **One profile on two decks** on one host means two copies with two folders and two B values. They are applied in one batch ([0008](0008-two-phase-apply.md)).
- **Deck loss:** if the deck a copy is bound to disappears from the app's device list (as opposed to being disconnected, which the app keeps listing, [R11](../references.md)), including a virtual deck that expired:
  - that copy stops syncing (state `DeckGone`) and notifies once;
  - it resumes automatically if the deck reappears;
  - `schrodeck subscribe <profile> --deck <other>` moves the subscription to another deck.

## Consequences

- Good: renames, a lost local state, and two copies on one host all have defined behavior.
- Good: a subscriber's folder name reveals nothing about the deck. It is a hash, and no serial appears in it.
- Bad: on the sharing host, the profile's folder UUID differs from every subscriber's. That is harmless, because the folder name is excluded from hashes ([contract C](../contracts/profile-format.md)).
- Risk: whether the app tolerates a profile whose folder UUID it didn't create is **observed only through the round-trip probe** ([contract B](../contracts/client-os.md) M4), which must pass before M3 ships.

## Alternatives considered

- **Use the sharing host's folder UUID everywhere:** two copies on one host (two decks) would collide.
- **Identity by profile name:** breaks on rename and allows accidental opt-in ([0022](0022-onboarding-init-and-join.md)).
- **Random folder UUIDs for subscribers:** the mapping can't be rebuilt if local state is lost.

## Verified by

No check yet; to be written in the plan:
- Deleting local state and re-running ⇒ the same folder ↔ profile mapping is rebuilt, and B is recovered from this host's own head ([0005](0005-direction-detection-three-way-hash.md) Recover row), with no Diverged on the sharing host or on subscribers. Known-bad: without head-based recovery, the same test reports Diverged.
- Two devices with the same prefs key (e.g. two virtual decks) ⇒ `subscribe` refuses both, no install.
- Subscribing one profile onto two decks ⇒ two distinct folders, one batched apply.
- Removing a deck from the fake device list ⇒ DeckGone, no apply, one notification; restoring it ⇒ resumes.
- Two matching decks and no `--deck` ⇒ refuse.

## References

- [R2](../references.md): profile folder layout (observed)
- [R11](../references.md): disconnected decks stay visible (documented)
- [R12](../references.md): virtual decks need a physical deck within 30 days (documented)
