# Contract C: Stream Deck profile format ↔ Go core

The on-disk profile format, as consumed by the platform-neutral core (normalization, hashing, staging). It is **cross-OS**, owned by Elgato, undocumented [R3], and versioned by the profile manifest's `Version` field rather than by OS. Each assertion is *documented* or *observed* with evidence in [references.md](../references.md). `schrodeck doctor` runs every probe; the schema guard (ADR [0015](../adr/0015-schema-guard.md)) refuses an unverified format.

Index of all contracts: [README.md](README.md).

| id | Assertion | Basis | Probe |
|---|---|---|---|
| P1 | Profiles live in `ProfilesV3/<UUID>.sdProfile/` with a top-level `manifest.json` and per-page `Profiles/<page>/manifest.json` + `Images/` | observed [R2] | the structural key fingerprint matches the recorded one |
| P2 | The top-level manifest has `"Version": "3.0"` | observed | read and compare |
| P3 | Top-level `Device.UUID` binds the profile to one deck; `Device.Model` gives the model | observed [R9] | present on every profile |
| P4 | Runtime-only fields: action `State`, `Pages.Current`; the app rewrites top-level manifests on launch | observed [R15] | **launch-rewrite probe:** hash every profile (normalized) → quit → relaunch → settle → re-hash. Normalized hashes must be equal. Any new differing field fails the probe and is reported by key path. |
| P5 | Action settings are plain JSON inside the page manifest; global plugin settings are not in profiles | documented [R7] | none needed (documented) |
| P6 | `Open` actions store absolute paths in `Settings.path` | observed [R16] | report any absolute path outside `{{HOME}}` |

