# 0015. Schema guard: stop when the app's files change shape

Status: Accepted 2026-10-01. Revised 2026-10-01 (review F5): the guard blocks pushes as well as applies; incoming commits are checked against this host's known-good fingerprints; the file-pattern set is part of the fingerprint; the app bundle is watched by the resident agent.

## Context

Elgato calls file-level profile management "not officially supported and may break with future software updates" ([R3](../references.md), documented). Elgato publishes a JSON schema for **plugin** manifests (`https://schemas.elgato.com/streamdeck/plugins/manifest.json`, returned 200 on 2026-10-01). No **profile** schema was found: the equivalent profiles URL returned 404, and the doc search found none ([R2](../references.md), observed). Profiles carry `"Version": "3.0"` (observed).

## Decision

schrodeck records a known-good fingerprint made of three signals:
1. The app version, from the app bundle's `Info.plist`.
2. The profile manifest `Version`.
3. A structural fingerprint: the set of keys at each level of top-level and page manifests, plus the set of file-name patterns present ([contract C](../contracts/profile-format.md) P7).

- Plugin manifests are validated against Elgato's published schema.
- The resident agent ([0012](0012-triggers.md)) watches the app bundle and notices updates.
- If any **local** signal differs from the known-good set, **both pushes and applies pause** (status is still reported) until `schrodeck doctor` passes and the user confirms. Pushes pause too: after an app update that migrated the profile format, L changes on every profile, and pushing would spread the new format to hosts still on the old app (review F5). `doctor` re-runs the launch-rewrite stability checks ([0006](0006-normalization-and-variables.md)) against the new app.
- Every commit records the `fingerprint` and `app_version` its source files matched ([contract D](../contracts/store-format.md)). The apply plan ([0008](0008-two-phase-apply.md)) refuses an incoming commit whose fingerprint is not in **this host's** known-good set (e.g. it was saved by a newer app). It notifies once ("*<profile>* was saved by Stream Deck <version>, which this Mac hasn't verified") and waits until this host's app is updated and `doctor` passes.

## Consequences

- Good: "stop, don't guess" when Elgato changes the format.
- Bad: every app update pauses syncing until `doctor` runs, which is friction for minor updates that change nothing.
- Risk: a semantic change that keeps the same keys and versions (e.g. a field's meaning changes) passes the guard. No structural check can catch that.

## Alternatives considered

- **Best effort (no guard):** an unrecognized format could be rewritten wrongly on every host.
- **Pin to exact app versions only:** too strict. Patch updates rarely change formats, and the fingerprint is more precise.

## Verified by

No check yet; to be written in the plan:
- A fixture with an added unknown key ⇒ guard trips.
- A changed `Version` ⇒ guard trips.
- A tripped guard ⇒ zero pushes and zero applies (filesystem-port assertion), with status still reported.
- An incoming commit with an unknown fingerprint ⇒ the plan refuses it; known-good: a matching fingerprint passes.
- The unchanged fixture ⇒ guard passes (known-good).

## References

- [R2](../references.md): ProfilesV3 layout (observed)
- [R3](../references.md): unsupported, may break (documented)
