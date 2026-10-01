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

No check yet; to be written in the plan: an apply test asserting the app preference file is byte-identical before and after.

## References

- [R13](../references.md): Smart Profiles (documented)
- [R14](../references.md): selected profile location (observed)
