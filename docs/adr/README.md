# Architecture decision records

Each ADR records one decision, the alternatives rejected, and **how a violation would be detected** ("Verified by"). Facts are cited from [../references.md](../references.md) and marked *documented* (Elgato states it) or *observed* (seen on a real machine, not promised).

| # | Title | Status | Decision |
|---|---|---|---|
| [0001](0001-build-vs-adopt.md) | Build vs adopt | Accepted | Build schrodeck; existing tools are prior art |
| [0002](0002-transport-shared-cloud-folder.md) | Transport | Accepted | A shared cloud folder is the only channel; no LAN, peers, or server |
| [0003](0003-decks-are-local-geometry-compatibility.md) | Decks and compatibility | Accepted | Each host finds its own decks from the app's data; compatible = same geometry; no USB in the core |
| [0004](0004-shared-profiles-and-subscriptions.md) | Shared profiles | Accepted | Unit of sync = a shared profile; opt in with `share`, install with `subscribe` onto any compatible deck |
| [0005](0005-direction-detection-three-way-hash.md) | Direction detection | Accepted | 3-way normalized hash (L/R/B); never timestamps |
| [0006](0006-normalization-and-variables.md) | Normalization and variables | Accepted | Strip runtime fields; per-host variables (`{{HOME}}` + user-defined) |
| [0007](0007-conflict-policy.md) | Conflict policy | Accepted | Diverged → stop, keep both, notify, `resolve` |
| [0008](0008-two-phase-apply.md) | Two-phase apply | Accepted | Plan → journal → quit/swap/relaunch → verify → rollback or keep |
| [0009](0009-store-write-protocol.md) | Store write protocol | Accepted | Stage → verify → rename → `current.json` last; per-host files |
| [0010](0010-host-identity-and-config-layering.md) | Host identity and config | Accepted | Derived `host_id`; store path is the only local config; everything else in the store |
| [0011](0011-history-and-rollback.md) | History and rollback | Accepted | X local + Y shared restore points; rollback = new generation; `--hold` |
| [0012](0012-triggers.md) | Triggers | Accepted | Watchers + safety timer + CLI + optional USB accelerator; no feedback loop |
| [0013](0013-sync-scope-and-scripts.md) | Sync scope and scripts | Accepted | Profiles + icon packs replicated; dependencies inventoried; script copy opt-in |
| [0014](0014-plugin-handling.md) | Plugins | Accepted | Action settings sync; global settings never; missing → apply + notify; skew → warn |
| [0015](0015-schema-guard.md) | Schema guard | Accepted | App version + profile `Version` + key fingerprint; pause until `doctor` passes |
| [0016](0016-notifications.md) | Notifications | Accepted | Own Swift helper in `~/Applications`, original icon, osascript fallback |
| [0017](0017-observability.md) | Observability | Accepted | Local log + per-host event trail; timestamps display-only |
| [0018](0018-runtime-and-architecture.md) | Runtime and architecture | Accepted | Go core + 6 OS ports; Swift only at the edges |
| [0019](0019-selected-profile-stays-per-host.md) | Selected profile | Accepted | Never sync or write the selected profile |
| [0020](0020-project-hygiene-naming-license.md) | Naming, license, hygiene | Accepted | schrodeck; MPL-2.0 + CLA; leak scan before every push |
| [0021](0021-who-may-push.md) | Who may push | **Proposed** | Recommend: any host whose copy changed (pending #2) |
