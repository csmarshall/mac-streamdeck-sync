# 0019. The selected profile stays per host

Status: Accepted 2026-10-01

## Context

The app records each deck's selected profile in its preferences (`Devices` → `<device id>` → `ESDProfilesInfo.ESDProfilesPreferred`; observed, [R14](../references.md)). Smart Profiles switch automatically based on the focused app ([R13](../references.md)). A host may want a different active profile than its neighbor.

## Decision

schrodeck **never reads `ESDProfilesPreferred` for sync and never writes it.** It syncs profile content only. After an apply, each deck shows whatever profile it showed before. Smart Profiles keep switching locally.

## Consequences

- Good: no fighting with app-linked switching. The app's preferences file is never modified.
- Bad: "switch to profile P everywhere" is not a feature.
- Risk: none identified.

## Alternatives considered

- **Sync the selected profile (optionally per profile):** requires writing the app's preference plist with the app quit (more intrusive), and fights Smart Profiles. Rejected by the project owner.

## Verified by

No check yet; to be written in the plan (review F19):
- schrodeck **never opens the app's preference file for writing**, asserted at the filesystem-port level (the `AppPrefs` port is read-only by type).
- On a real machine (`doctor`, M3): `ESDProfilesPreferred` for every deck is **semantically** equal (same profile UUID, case-insensitive) before and after an apply. A byte comparison of the plist would be fooled by the app rewriting its own preferences on relaunch.

## References

- [R13](../references.md): Smart Profiles (documented)
- [R14](../references.md): selected profile location (observed)
