# Contract C: Stream Deck profile format ↔ Go core

The on-disk profile format, as consumed by the platform-neutral core (normalization, hashing, staging). It is **cross-OS**, owned by Elgato, undocumented [R3](../references.md), and versioned by the profile manifest's `Version` field rather than by OS. Each assertion is *documented* or *observed*, with evidence in [references.md](../references.md). `schrodeck doctor` runs every probe; the schema guard (ADR [0015](../adr/0015-schema-guard.md)) refuses an unverified format.

This file is also **the single definition of the normalized hash** and the file allow-list. ADR [0006](../adr/0006-normalization-and-variables.md) links here.

Index of all contracts: [README.md](README.md).

## Assertions

| id | Assertion | Basis | Probe |
|---|---|---|---|
| P1 | Profiles live in `ProfilesV3/<UUID>.sdProfile/` with a top-level `manifest.json` and per-page `Profiles/<page>/manifest.json` + `Images/` | observed [R2](../references.md) | the structural key fingerprint and the set of file-name patterns match the recorded ones |
| P2 | The top-level manifest has `"Version": "3.0"` | observed | read and compare |
| P3 | Top-level `Device.UUID` binds the profile to one deck; `Device.Model` gives the model. It embeds the deck's USB serial, so schrodeck stores it only as the `{{DEVICE}}` placeholder ([contract D](store-format.md)) | observed [R9](../references.md) | present on every profile |
| P4 | Runtime-only fields: action `State`, `Pages.Current`; the app rewrites top-level manifests on launch | observed [R15](../references.md) | **launch-rewrite probe** (M3, because it restarts the app): hash every profile (normalized) → quit → relaunch → settle → re-hash. Normalized hashes must be equal. Any new differing field fails the probe and is reported by key path. |
| P5 | Action settings are plain JSON inside the page manifest; global plugin settings are not in profiles | documented [R7](../references.md) | none needed (documented) |
| P6 | `Open` actions store absolute paths in `Settings.path` | observed [R16](../references.md) | report any absolute path outside `{{HOME}}` |
| P7 | Every file in a profile matches the allow-list below | observed | an unexpected file inside a profile folder trips the schema guard: a new file type means the format changed |
| P8 | Actions that switch to or open **another profile** (e.g. a switch-profile action) reference the target by that profile's **folder UUID**, and may also embed a device id | **likely, unverified** (review F46) | scan action settings for UUID-shaped values that match another local `.sdProfile` folder name, and for `@(` device ids. Report each reference with its key path. A matched profile reference is an inventory dependency (ADR [0013](../adr/0013-sync-scope-and-scripts.md)); a device id other than this copy's own refuses the push (ADR [0006](../adr/0006-normalization-and-variables.md)) |

## File allow-list

Relative to the `<UUID>.sdProfile/` folder. The folder's own name is never part of a path.

- `manifest.json`
- `Images/*`
- `Profiles/<page>/manifest.json`
- `Profiles/<page>/Images/*`

Known junk is **ignored silently** for hashing and copying: `.DS_Store`, sync-client artifacts (`* (conflicted copy)*`, `*.icloud`, `~$*`), and editor temp files. Any other file outside the allow-list trips P7, because it may be app data we don't understand.

## Normalized hash

`hash` is what direction detection compares (ADR [0005](../adr/0005-direction-detection-three-way-hash.md)). It must change on a real user edit and on nothing else. The current definition is `norm_version = 1`:

1. Take every allow-listed file. Paths are relative, use `/` as the separator, and are in Unicode **NFC**.
2. For each `manifest.json`:
   - parse it as JSON;
   - remove the strip-list fields (action `State`, `Pages.Current`, top-level `Device.UUID`), which are one named constant in the code;
   - **canonicalize it with RFC 8785 (JSON Canonicalization Scheme)**.

   A stored tree is already in placeholder form (variables and `{{DEVICE}}`). A local copy is first put into placeholder form (ADR 0006).
3. For each image, use the raw bytes.
4. `hash = sha256` over the lines `<path>\0<sha256(canonical bytes)>\n`, sorted bytewise by path.

Any change to steps 1–4, including the strip list, increments `norm_version` and ships as a store `FORMAT` migration (ADR [0027](../adr/0027-store-lifecycle.md)). Hashes are only ever compared under the same `norm_version`.

The store's `tree_digest` uses the same file set and line format, but over the **raw** bytes with nothing stripped ([contract D](store-format.md)).
