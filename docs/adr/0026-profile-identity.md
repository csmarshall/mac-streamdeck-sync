# 0026. Profile identity: a schrodeck `profile_id`, mapped to local folders per host

Status: Accepted 2026-10-01 (review F16). Revised 2026-10-01 (review round 2: F31, F38): the deck key is the app's prefs device key and must be unique on the host; a lost local state is rebuilt from this host's own head. Revised 2026-10-01 (reframe: template-seeded setups; fail closed): **every** member copy, including the first one created by `init`/`share`, is a new profile with a derived folder name. The template is never a member. Deck loss becomes DETACHED(deck-gone) ([0030](0030-fail-closed-detach.md)).

## Context

The app names each profile folder `ProfilesV3/<UUID>.sdProfile/` ([R2](../references.md), observed). The design said receivers get a folder id of uuid5(profile, deck), but never defined:

- how a host's local folder relates to the shared profile;
- where the mapping is stored;
- what happens when the target deck disappears.

Virtual decks need a physical deck seen within the last 30 days, or they go away ([R12](../references.md), documented). Profiles can also be renamed at any time.

## Decision

- **`profile_id`** is a random UUIDv4 assigned by `init`/`share` when a setup is created from a template ([0029](0029-problem-statement-and-setup-model.md)), recorded in the write-once `profiles/<profile_id>/profile.json.<host_id>` ([contract D](../contracts/store-format.md)). It never changes. A profile's display name is just content, so renames sync as ordinary edits.
- **Every member copy is a profile schrodeck created** ([0029](0029-problem-statement-and-setup-model.md)), on every host, including the one that created the setup from a template. The template's folder is never part of the mapping. Local state maps `member folder UUID → (profile_id, deck_key)`.
- **Cross-profile references** (a button that switches to another profile by its folder UUID, [contract C](../contracts/profile-format.md) P8, likely, unverified) are translated through the same mapping: stored as the target's `profile_id`, expanded on install to the target's local folder on this host if it is subscribed here, otherwise reported as a dangling dependency ([0013](0013-sync-scope-and-scripts.md), review F46).
- **Every member copy** (the creating host's first copy and every joiner's copy alike) lives in a new folder named `uuid5(NAMESPACE_SCHRODECK, profile_id + ":" + deck_key)`. The name is deterministic, so a host that loses its local state can rebuild the mapping by recomputing the names. A member copy never reuses an existing local folder.
- **`deck_key`** is the key of the deck's entry in the app's prefs `Devices` dictionary ([R14](../references.md), observed; [contract A](../contracts/os-connector.md) `DeviceEnumerator.AppDeviceID`), not the manifest's `Device.UUID` (review F31). A deck can only be subscribed onto if its key is **unique** among this host's devices. Virtual decks are observed with the empty id `@(0)[]` ([R12](../references.md)): one virtual deck on a host is fine, but if two devices share a key, `subscribe` refuses to target either of them and says why, rather than letting two copies collide on one folder name.
- **Where the mapping lives:**
  - local state, which is authoritative for this host;
  - `hosts/<host_id>.toml`'s subscriptions (`profile_id → deck`), which `status` on other hosts can display.

- **Choosing a deck:** the user always picks the destination device explicitly ([0022](0022-onboarding-init-and-join.md)); `--deck` is required non-interactively when more than one local deck has a matching geometry. This also covers two same-size decks on one host: each setup goes where the user puts it. Join never auto-subscribes.
- **One profile on two decks** on one host means two copies with two folders and two B values. They are applied in one batch ([0008](0008-two-phase-apply.md)).
- **Deck loss:** if the deck a copy is bound to disappears from the app's device list (as opposed to being disconnected, which the app keeps listing, [R11](../references.md)), including a virtual deck that expired, that copy becomes **DETACHED(deck-gone)** ([0030](0030-fail-closed-detach.md)): nothing is pushed or pulled for it, one notification is sent, and it does **not** resume by itself if the deck reappears. `schrodeck resolve <setup>` offers rejoin (a new copy on another compatible device) or unmap.

## Consequences

- Good: renames, a lost local state, and two copies on one host all have defined behavior.
- Good: a subscriber's folder name reveals nothing about the deck. It is a hash, and no serial appears in it.
- Good: one naming rule for every member copy on every host, so there's no special case for the host that created the setup. Folder names are excluded from hashes anyway ([contract C](../contracts/profile-format.md)).
- Risk: whether the app tolerates a profile whose folder UUID it didn't create is **observed only through the round-trip probe** ([contract B](../contracts/client-os.md) M4), which must pass before M3 ships.

## Alternatives considered

- **Use the sharing host's folder UUID everywhere:** two copies on one host (two decks) would collide.
- **The creating host keeps the template's folder as its member copy** (the previous version): made the user's own profile a sync target. Replaced by a new member copy ([0029](0029-problem-statement-and-setup-model.md)).
- **Identity by profile name:** breaks on rename and allows accidental opt-in ([0022](0022-onboarding-init-and-join.md)).
- **Random folder UUIDs for subscribers:** the mapping can't be rebuilt if local state is lost.

## Verified by

No check yet; to be written in the plan:
- Deleting local state and re-running ⇒ the same folder ↔ profile mapping is rebuilt, and B is recovered from this host's own head ([0005](0005-direction-detection-three-way-hash.md) Recover row), with no Diverged on any host, including the one that created the setup. Known-bad: without head-based recovery, the same test reports Diverged.
- Two devices with the same prefs key (e.g. two virtual decks) ⇒ `subscribe` refuses both, no install.
- Subscribing one profile onto two decks ⇒ two distinct folders, one batched apply.
- Removing a deck from the fake device list ⇒ DETACHED(deck-gone), no apply, one notification; restoring it ⇒ **still** DETACHED until `resolve` (known-bad: auto-resume must fail the test).
- Two matching decks and no `--deck` ⇒ refuse.

## References

- [R2](../references.md): profile folder layout (observed)
- [R11](../references.md): disconnected decks stay visible (documented)
- [R12](../references.md): virtual decks need a physical deck within 30 days (documented)
