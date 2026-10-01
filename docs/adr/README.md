# Architecture decision records

Each ADR records one decision, the alternatives rejected, and **how a violation would be detected** ("Verified by"). Facts are cited from [../references.md](../references.md) and marked *documented* (Elgato states it) or *observed* (seen on a real machine, not promised). ADRs revised after the 2026-10-01 independent review carry a "Revised … (review F#)" line under their status.

| # | Title | Status | Decision |
|---|---|---|---|
| [0001](0001-build-vs-adopt.md) | Build vs adopt | Accepted | Build schrodeck; existing tools are prior art |
| [0002](0002-transport-shared-cloud-folder.md) | Transport | Accepted | A shared cloud folder is the only channel; no LAN, peers, or server |
| [0003](0003-decks-are-local-geometry-compatibility.md) | Decks and compatibility | Accepted | Each host finds its own decks from the app's data; compatible = same geometry; no USB in the core |
| [0004](0004-shared-profiles-and-subscriptions.md) | Shared profiles | Accepted | Unit of sync = a shared profile; opt in with `share`, install with `subscribe` onto any compatible deck |
| [0005](0005-direction-detection-three-way-hash.md) | Direction detection | Accepted, revised | Normalized hashes over a commit graph of per-host heads; equal-hash tips are converged; freshness checked first; HoldLocal and Anomaly states; timestamps display-only |
| [0006](0006-normalization-and-variables.md) | Normalization and variables | Accepted, revised | Store holds full trees; normalization only for hashing; boundary-aware variables, collision guard, `norm_version`; variable changes re-materialize, never push |
| [0007](0007-conflict-policy.md) | Conflict policy | Accepted, revised | Diverged edits are pushed as their own commit (a visible fork); nothing applied until `resolve` writes a multi-parent commit |
| [0008](0008-two-phase-apply.md) | Two-phase apply | Accepted, revised | Plan → journal → quit → **post-quit re-check** → snapshot → swap → relaunch → verify; failure ⇒ rollback + BLOCKED(R); per-step crash recovery |
| [0009](0009-store-write-protocol.md) | Store write protocol | Accepted, revised | No file has two writers: write-once trees and commits, per-host heads written last ([contract D](../contracts/store-format.md)) |
| [0010](0010-host-identity-and-config-layering.md) | Host identity and config | Accepted, revised | Derived `host_id`; per-host data in single-writer `hosts/<host_id>.toml`; registry display-only, no membership |
| [0011](0011-history-and-rollback.md) | History and rollback | Accepted, revised | X local restore points + the shared commit chain; rollback = new commit; `hold`/`resume` commands |
| [0012](0012-triggers.md) | Triggers | Accepted, revised | Resident agent with recursive FSEvents + safety timer + CLI + optional USB; pushes debounce, applies back off; one-time "nothing to sync" notice |
| [0013](0013-sync-scope-and-scripts.md) | Sync scope and scripts | Accepted | Profiles + icon packs replicated; dependencies inventoried; script copy opt-in |
| [0014](0014-plugin-handling.md) | Plugins | Accepted | Action settings sync; global settings never; missing → apply + notify; skew → warn |
| [0015](0015-schema-guard.md) | Schema guard | Accepted, revised | Fingerprint = profile `Version` + key structure + file patterns (not the app version); a changed fingerprint blocks pushes and applies until `doctor` passes; incoming commits must match a known fingerprint |
| [0016](0016-notifications.md) | Notifications | Accepted, revised | Own Swift helper in `~/Applications`, original icon; alerts deduplicated by (condition, profile, version) |
| [0017](0017-observability.md) | Observability | Accepted | Local log + per-host event trail; timestamps display-only |
| [0018](0018-runtime-and-architecture.md) | Runtime and architecture | Accepted | Go core + per-OS connectors ([contract A](../contracts/os-connector.md)); Swift only at the edges |
| [0019](0019-selected-profile-stays-per-host.md) | Selected profile | Accepted | Never sync or write the selected profile |
| [0020](0020-project-hygiene-naming-license.md) | Naming, license, hygiene | Accepted | schrodeck; MPL-2.0 + CLA; leak scan before every push |
| [0021](0021-who-may-push.md) | Who may push | Accepted, revised | Any host whose copy changed (L ≠ B), attached or not; refused while Forked, guarded, colliding or held |
| [0022](0022-onboarding-init-and-join.md) | Onboarding | Accepted, revised | `init`; `join <dir>` previews every change, subscribes only what is chosen, never replaces local profiles, first sync in the foreground |
| [0023](0023-store-freshness-via-file-provider.md) | Store freshness | Accepted, revised | Provider sync state via File Provider; stuck-in-flight alarm; observing `current` gates M2 |
| [0024](0024-documented-contracts.md) | Documented contracts | Accepted | Five contracts (connector, client per OS, profile format, store format, `--json`), each with one home, an owner and an enforcement mechanism |
| [0025](0025-deletion-and-unshare.md) | Deletion and unshare | Accepted | `unshare` writes per-host tombstones and `reshare` undoes it; local deletes never propagate; store loss (only for a fresh, empty store) stops and notifies; one-time uninstall suggestion |
| [0026](0026-profile-identity.md) | Profile identity | Accepted | `profile_id` UUIDv4; sharer keeps its folder, subscribers get uuid5(profile, deck); deck loss stops and notifies |
| [0027](0027-store-lifecycle.md) | Store lifecycle | Accepted | Explicit `migrate` for FORMAT/`norm_version` (old heads become ancestors); GC deletes trees only, commits kept forever; `uninstall` never touches profiles |
| [0028](0028-distribution-brew-tap-and-pkg.md) | Distribution | Accepted (pkg Deferred) | Homebrew tap formula built from source (CLI + notifier, no brew services, agent installed by `init`/`join`); auto-merge only bot PRs limited to url + sha256 with recomputed checksum and verified provenance; signed and notarized `.pkg` deferred until there is a Developer ID |

## Delivery order

ADR numbers are permanent ids, in the order they were decided. This is the order in which they get **built**. Each milestone is usable and testable on its own. **No milestone before M3 writes to the Stream Deck app's files or restarts the app.**

| Milestone | Delivers | ADRs |
|---|---|---|
| **M0 Foundations** | Repo, CI, Go core with no OS imports, port interfaces + fakes + conformance skeleton | [0020](0020-project-hygiene-naming-license.md), [0018](0018-runtime-and-architecture.md), [0024](0024-documented-contracts.md) (context: [0001](0001-build-vs-adopt.md), [0002](0002-transport-shared-cloud-folder.md)) |
| **M1 Read-only insight** | `status`, `doctor` (read-only probes only), `inventory`: enumerate decks and profiles, normalize, hash, check the app version and schema. **Writes nothing** | [0003](0003-decks-are-local-geometry-compatibility.md), [0019](0019-selected-profile-stays-per-host.md), [0006](0006-normalization-and-variables.md), [0015](0015-schema-guard.md), [0010](0010-host-identity-and-config-layering.md), [0017](0017-observability.md), [0026](0026-profile-identity.md) (identity mapping) |
| **M2 Publish** | `init`, `share`, `push`: write the store (heads, commits, trees), confirm upload with the provider. **Push refuses while Forked or Diverged** (resolve arrives in M3). The schema guard needs only `doctor`'s **read-only tier** here ([0015](0015-schema-guard.md)): pushing never restarts the app. **Gate:** the `current`-state spike ([0023](0023-store-freshness-via-file-provider.md)) must pass first | [0004](0004-shared-profiles-and-subscriptions.md), [0009](0009-store-write-protocol.md), [0023](0023-store-freshness-via-file-provider.md), [0005](0005-direction-detection-three-way-hash.md), [0021](0021-who-may-push.md), [0022](0022-onboarding-init-and-join.md) (`init`, without the rename offer) |
| **M3 Apply** | `join`, `subscribe`, `pull`, `resolve`, `unblock`, `history`, `rollback`, `hold`/`resume`, `unshare`/`reshare`/`unsubscribe`: the first writes to the app's files, all behind the two-phase apply. Also: `doctor`'s app-restarting probes ([contract C](../contracts/profile-format.md) P4, [contract B](../contracts/client-os.md) M4, which must pass before any other M3 apply ships), the profile rename offer, the minimal plugin-presence check the apply plan needs from [0014](0014-plugin-handling.md), and the **core notification deduplication** (keyed on condition, profile and version) that BLOCKED and HoldLocal rely on; until the notifier app lands in M4, deduplicated alerts go to the log and `status` | [0008](0008-two-phase-apply.md), [0007](0007-conflict-policy.md), [0011](0011-history-and-rollback.md), [0025](0025-deletion-and-unshare.md), [0022](0022-onboarding-init-and-join.md) (`join` + rename offer), [0014](0014-plugin-handling.md) (presence check only) |
| **M4 Unattended** | Resident agent, recursive watcher, timer, apply backoff, the notifier app delivering the already-deduplicated alerts, `init`/`join` install the agent, `uninstall`, GC (trees only). Dogfooding on a second Mac needs the tap formula from M6 (or a local build) | [0012](0012-triggers.md), [0016](0016-notifications.md), [0027](0027-store-lifecycle.md) |
| **M5 Dependencies** | Icon-pack replication, script inventory and opt-in copy, full plugin inventory and notifications | [0013](0013-sync-scope-and-scripts.md), [0014](0014-plugin-handling.md) |
| **M6 Distribution** | Homebrew tap formula and release automation; `.pkg` deferred | [0028](0028-distribution-brew-tap-and-pkg.md) |
