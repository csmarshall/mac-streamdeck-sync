# 0029. Problem statement and setup model: the same setup on a deck at every computer

Status: Accepted 2026-10-01 (reframe: template-seeded setups; owner's decisions). Revised 2026-10-01 (review F52, F53; owner's decision): a setup is created by **publishing first, then installing** on the first Mac through the ordinary install path. No special seed path exists.

## Context

The original problem statement was "keep chosen profiles in sync between Macs". Working through the app's on-disk model ([streamdeck-config-model.md](../streamdeck-config-model.md)) sharpened it:

- What the owner actually wants: **a given kind of Stream Deck (same geometry) shows the same profile setup at every computer you sit at, synced automatically, with no manual profile shuffling.**
- The app has **no device-type → profile grouping**. A device has 1 → N profiles, and each profile belongs to exactly one device ([R2, R9](../references.md), observed). Two XLs, even on one computer, own separate profiles. The only type-level concept Elgato has is profile *templates* (plugin-bundled or Marketplace profiles that declare a `DeviceType`), and installing one still creates an ordinary per-device copy.
- So the "same setup on every XL" layer doesn't exist in the app. schrodeck has to provide it, without ever taking over or rewriting profiles the user didn't hand to it.

## Decision

**A setup** is schrodeck's unit of sync:

- one synced profile, identified by its permanent `profile_id` ([0026](0026-profile-identity.md));
- a **geometry** (columns × rows, + dials), recorded from the device the template was taken from ([0003](0003-decks-are-local-geometry-compatibility.md));
- a **user-provided name**.

Each host may have one **member copy** of a setup per chosen device. Member copies are ordinary profiles in the app, created by schrodeck, named `schrodeck - <cols>x<rows> - <user name>` (e.g. `schrodeck - 8x4 - Work`). The name is a visual cue only: identity is the `profile_id`, so renaming a member copy is an ordinary synced edit.

**Creating a setup (`init` on the first Mac, `share` on any later one):**

1. schrodeck reads the app's config and lists this Mac's devices.
2. The user picks a device, then a **template** profile on it, and names the setup.
3. **Publish (read-only on the app's side).** schrodeck reads the template's files and builds the setup's **root tree** in staging: the template's content, put into placeholder form ([0006](0006-normalization-and-variables.md)), with the profile `Name` set to `schrodeck - <cols>x<rows> - <name>`. It pushes that as the setup's root revision ([contract D](../contracts/store-format.md)). Nothing in the app's data is opened for writing in this step. **The template itself is never modified, never synced, and never becomes a member.** It's just where the content came from.
4. **Install on this Mac, like any other.** The first Mac is a peer from the start: it installs the root revision onto the chosen device through the ordinary install apply ([0008](0008-two-phase-apply.md), [0026](0026-profile-identity.md)), exactly as a joining Mac would. That creates the **new** member profile. There is no special seed path, so crash behavior is just the normal push and install behavior: a crash after the push leaves a published setup that this Mac is not yet a member of; `status` says so, and `schrodeck subscribe <setup>` installs it (a re-run of `init`/`share` detects the half-finished setup and offers the same).

**Joining a setup (`join <dir>` on a new Mac, `subscribe` later):**

1. schrodeck lists the setups in the store, **filtered to those whose geometry matches a device on this Mac**.
2. The user picks a setup and a **destination device**.
3. schrodeck **always creates a new profile** on that device ([0022](0022-onboarding-init-and-join.md), [0026](0026-profile-identity.md)). It never replaces or modifies an existing profile.

**Joiners never bring their own config into the system.** A Mac that joins receives the setup; its existing profiles stay exactly as they were, outside sync. **Merging two computers' configs is out of scope.** It is possible by hand (quit the app, edit the profile files), at the user's own risk, and schrodeck won't manage it. The documentation says so plainly.

**After seeding, all member copies are peers.** An edit on any member copy, on any Mac, syncs to every other member (the revision/heads model, [0005](0005-direction-detection-three-way-hash.md), unchanged). There is no master copy.

**Consequences of the model, stated once:**

- **Two decks of the same size on one Mac** need no special rule: the user picks the destination device explicitly, per setup.
- A deck can be the destination of several setups (e.g. `schrodeck - 8x4 - Work` and `schrodeck - 8x4 - Home`). Each is a separate profile on that deck.
- The **selected profile** on each deck stays per Mac ([0019](0019-selected-profile-stays-per-host.md)).
- **Profiles that aren't member copies are never read for sync, written, or deleted.** That includes every template.
- A member copy that is **deleted in the app**, or changed in any way schrodeck doesn't expect, makes that Mac **drop out** of the setup for manual resolution ([0030](0030-fail-closed-detach.md)). Other Macs are unaffected. `unshare` retires a setup everywhere ([0025](0025-deletion-and-unshare.md)).

## Consequences

- Good: the setup layer is explicit and visible in the app (`schrodeck - 8x4 - …` profiles), and the user's own profiles are never at risk, because schrodeck only ever creates new profiles.
- Good: onboarding needs no merge logic. There's exactly one source per setup (its template, once) and then peers.
- Bad: a user who wants their *current* profile synced ends up with a copy of it next to the original. They can delete the original by hand once happy.
- Bad: a joining Mac's existing configuration is not imported. That's deliberate; merging is out of scope.
- Risk: users may expect an edit to the *template* to sync. The template is explicitly not a member, and `status` lists which profiles are members.

## Alternatives considered

- **Sync the template profile in place** (the earlier "share this profile" design): the user's original becomes a sync target that other Macs can overwrite. Rejected: schrodeck should only manage profiles it created.
- **Template stays the source of truth** (one-way from Mac #1): you couldn't adjust the deck from another desk. Rejected for peers.
- **Join replaces the destination's selected or chosen profile:** brings risk to existing user profiles and blurs "joiners never bring config". Rejected: always a new profile.
- **Join merges the joiner's config into the setup:** ambiguous (whose button wins?) and unbounded. Out of scope.
- **Use Elgato's template mechanism directly** (plugin-bundled profiles): installs once through a prompt and can't be updated. Concept borrowed, mechanism rejected.
- **Sync every profile of a device type automatically (opt-out):** would take over profiles the user never chose. Rejected: setups are explicit.

## Verified by

No check yet; to be written in the plan:
- `init`/`share`: the template profile is **never opened for writing** (filesystem-port assertion). The publish step opens nothing under the app's data root for writing. After install, exactly one new profile appears, on the chosen device, named `schrodeck - <c>x<r> - <name>`. Its normalized hash equals the template's **with the template's `Name` overridden to the setup name** (review F53). Known-bad: a copy whose name wasn't changed must fail this check, and comparing without the override must report them different.
- A crash injected between the root push and the install ⇒ the setup exists in the store, this Mac has no member yet and no partial profile, and `subscribe` (or the re-run) installs it through the normal path.
- `join`/`subscribe`: the setup list contains only setups whose geometry matches a local device (known-bad: a fixture setup of another geometry must not be listed). Exactly one new profile is created on the chosen destination, and no existing profile is opened for writing.
- Peers: an edit on the joining Mac's member copy reaches the first Mac's member copy (and never the template).

## References

- [R2, R9](../references.md): profile layout; each profile bound to one device (observed)
- [R8, R10](../references.md): device types and geometry; profiles are device-specific (documented)
- [streamdeck-config-model.md](../streamdeck-config-model.md): the app's configuration model
