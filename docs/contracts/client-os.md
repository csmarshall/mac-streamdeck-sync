# Contract B: OS connector ↔ Stream Deck app (per OS)

What each OS connector assumes about the Stream Deck app on that OS: where its data and prefs live, and how its process behaves. Owned by Elgato, observed by us [R3]. The profile **format** itself is contract C ([profile-format.md](profile-format.md)). Probes run in `schrodeck doctor`; the schema guard (ADR [0015](../adr/0015-schema-guard.md)) refuses unverified app versions.

Index of all contracts: [README.md](README.md).

## macOS

| id | Assertion | Basis | Probe |
|---|---|---|---|
| M1 | Data root `~/Library/Application Support/com.elgato.StreamDeck` | documented [R1] | exists and is readable |
| M2 | Prefs `~/Library/Preferences/com.elgato.StreamDeck.plist` → `Devices` → `<id>` → `ESDProfilesInfo.ESDProfilesPreferred` | observed [R14] | parse; each preferred UUID matches a profile (case-insensitively) |
| M3 | A graceful quit (`quit app`) flushes state, and nothing writes `ProfilesV3` afterwards | observed | quit → watch `ProfilesV3` for the settle window → no writes |
| M4 | Files swapped in while the app is quit are loaded as-is on launch | observed (by design of the apply; not yet probed) | **round-trip probe:** apply a known no-op change to a scratch profile → relaunch → it is present and unchanged |
| M5 | App version is in the bundle's `Info.plist` | observed | read |

## Windows

Not yet researched or verified. A Windows connector starts by writing this section, with a probe for each row, before its first apply. See contract A, [os-connector.md](os-connector.md).

## Verified versions

| OS | App version | Profile `Version` | Fingerprint | Probes passed | Date |
|---|---|---|---|---|---|
| macOS 27 | 7.5.1 | 3.0 | *(recorded by the first `doctor` run)* | M1, M2, M5 observed by hand; P4/M3/M4 **not yet run** (no code) | 2026-10-01 |

A row is added only by a passing `doctor` run. Hand observations are listed as such, never as "passed". An app update whose read-only fingerprint check found the profile format unchanged is listed as "fingerprint unchanged" (enough to keep pushing, ADR [0015](../adr/0015-schema-guard.md)); the restart probes still have to pass before it counts as fully verified.
