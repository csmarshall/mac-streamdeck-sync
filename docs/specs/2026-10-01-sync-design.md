# schrodeck — design

Status: DRAFT for review, aligned with ADRs 0001–0028 after the 2026-10-01 independent review (2026-10-01). No code exists yet.

This document is the narrative of how schrodeck fits together. The *why* behind each choice, and the alternatives rejected, live in the [ADRs](../adr/README.md). Exact formats live in the [contracts](../contracts/README.md), not here. Each section links the ADRs and contracts it relies on. Facts about the Stream Deck app are cited from [references.md](../references.md) as `[Rn]` and marked *documented* (Elgato states it) or *observed* (seen on a real machine, not promised).

## Problem

One or more Stream Decks are shared between several computers, for example through a Thunderbolt or KVM switch. You edit a profile on Mac A, flip the switch, and Mac B shows the old layout. The Stream Deck app keeps profiles per machine. It has no automatic cross-machine sync for desktop decks, only manual export/import ([R18], documented by absence).

## Goals

- Run unattended on every host and keep **shared profiles** converged through a shared cloud folder: Dropbox, iCloud Drive, or any folder a sync client replicates (ADR [0002](../adr/0002-transport-shared-cloud-folder.md)).
- Let each host decide on its own, per shared profile, whether its copy is **behind**, **ahead**, **in sync**, or **diverged**. Push when ahead, apply when behind, and **never silently drop an edit**, including when two hosts push at once.
- Need nothing beyond the shared folder and local CLI tools. **No network access between hosts**: machines on the same desk can have different firewalls, VPNs or endpoint agents.
- Support **any number of hosts and decks**, with no membership list and no coordination. A profile created on one machine can be used on others, including on a **different physical deck** with the same geometry.
- Build the macOS version first, with the OS-specific parts behind interfaces so that other OS connectors can be added later (ADR [0018](../adr/0018-runtime-and-architecture.md)).

## Non-goals (v1)

- Merging concurrent edits. Diverged or forked profiles stop and ask (ADR [0007](../adr/0007-conflict-policy.md)).
- Re-flowing a profile onto a deck with a different geometry, such as 32 keys onto 15 (ADR [0004](../adr/0004-shared-profiles-and-subscriptions.md)).
- Installing or copying plugins, and syncing plugin global settings (ADR [0014](../adr/0014-plugin-handling.md); copying is investigated in issue #4).
- Syncing which profile is selected on each deck (ADR [0019](../adr/0019-selected-profile-stays-per-host.md)).
- Propagating deletes (ADR [0025](../adr/0025-deletion-and-unshare.md)).

## Prior art

- [dominik-ba/stream-deck-profile-sync](https://github.com/dominik-ba/stream-deck-profile-sync): manual `push`/`pull` through a cloud folder. It doesn't detect direction, handle conflicts, rewrite paths, restart the app, or run on its own (ADR [0001](../adr/0001-build-vs-adopt.md)).
- Elgato's "deploying profiles at scale" (`custom_default_profiles`) seeds defaults only. Elgato marks file-level management "not officially supported and may break with future software updates" ([R3], documented). That is why the schema guard exists.

## Concepts

| Term | Meaning |
|---|---|
| **Host** | One macOS user account on one machine. Identity is derived, never configured (ADR [0010](../adr/0010-host-identity-and-config-layering.md)). The number of hosts never matters to any decision. |
| **Deck** | A Stream Deck as one host's app sees it: physical or virtual. Each host enumerates *its own* decks from the app's data. The sync logic needs no USB access (ADR [0003](../adr/0003-decks-are-local-geometry-compatibility.md)). |
| **Geometry** | Columns × rows, plus dial/encoder count, from Elgato's DeviceType table ([R8], documented). Two decks are **compatible** when their geometry matches. |
| **Shared profile** | The unit of sync: one Stream Deck profile (all its pages and folders) that a user opted in with `share`. It has a permanent `profile_id` (ADR [0026](../adr/0026-profile-identity.md)). Unshared profiles are never touched. |
| **Copy / subscription** | A host's local copy of a shared profile, installed onto one compatible local deck. **Every subscribed copy is kept up to date**, whether or not it is the deck's active profile. |
| **Store** | The shared cloud folder. Its format is [contract D](../contracts/store-format.md). |
| **Commit / head** | Each version of a shared profile is an immutable **commit** in the store. Each host owns a **head** naming the commit its copy is at. **R** is the newest commit all heads descend from. |

## Facts the design rests on

Observed on Stream Deck 7.5.1, macOS 27. The schema guard re-checks them after every app update. Contracts [B](../contracts/client-os.md) and [C](../contracts/profile-format.md) turn them into probes.

| Fact | Status | Consequence |
|---|---|---|
| Profiles live in `~/Library/Application Support/com.elgato.StreamDeck/ProfilesV3/<uuid>.sdProfile/`, with a top-level `manifest.json` and `Profiles/<page>/manifest.json` + `Images/` | observed [R1, R2] | Copy whole profile trees; they're small. |
| Profiles are device-specific; layouts differ across models | documented [R10] | Compatibility is by geometry. |
| The app rewrites top-level manifests on launch and touches page manifests at runtime with no edit; runtime fields include action `State` and `Pages.Current` | observed [R15] | mtime can't show direction; hash normalized content (ADRs [0005](../adr/0005-direction-detection-three-way-hash.md), [0006](../adr/0006-normalization-and-variables.md)). |
| Each profile names its deck in `Device.UUID` | observed [R9] | Excluded from the hash; set per receiving deck on install. |
| `Open` actions store absolute paths under the user's home | observed [R16] | `{{HOME}}` variable. |
| The app keeps state in memory and must be closed before its files are changed | documented [R3] | Apply = quit → re-check → swap → relaunch → verify (ADR [0008](../adr/0008-two-phase-apply.md)). |
| Disconnected decks stay editable | documented [R11] | Any host whose copy changed may push (ADR [0021](../adr/0021-who-may-push.md)). |
| Virtual decks need a physical deck seen within 30 days | documented [R12] | A deck can disappear; that copy stops and notifies (ADR [0026](../adr/0026-profile-identity.md)). |
| Plugin global settings are machine-local; action settings travel with profiles | documented [R7] | Global settings never sync (ADR [0014](../adr/0014-plugin-handling.md)). |
| The selected profile per deck is stored in the app's prefs (`ESDProfilesPreferred`) | observed [R14] | Never synced or written (ADR [0019](../adr/0019-selected-profile-stays-per-host.md)). |
| There is no official CLI or URL scheme to import profiles or quit/relaunch the app | documented by absence [R17] | AppControl uses `osascript` + `open`. |

## User workflow

Onboarding (ADR [0022](../adr/0022-onboarding-init-and-join.md)):

```
host A:  schrodeck init                     # preflight the app; pick the shared dir and profile(s); first push
host B:  schrodeck join <dir>               # preflight; DRY-RUN rundown; choose what to subscribe and onto which deck;
                                            # confirm; first sync runs in the foreground; then the agent is installed
```

Day to day:

```
host A:  schrodeck share "Work"                     # opt in; published to the store; may offer the rename "schrodeck · Work · 8×4"
host B:  (notification: "Shared profile Work (8×4) is available")
host B:  schrodeck subscribe "Work" --deck <deck>   # installs a NEW copy on a local deck with 8×4 geometry
         # edit Work on either host; the other host applies it automatically
host B:  schrodeck unsubscribe "Work"               # stop syncing; the local copy stays, detached
host A:  schrodeck unshare "Work"                   # tombstone; every subscriber keeps a detached copy
```

The full CLI: `schrodeck init | join | status | sync | share | unshare | subscribe | unsubscribe | push | pull | resolve | unblock | history | rollback | hold | resume | log | inventory | doctor | config | config resolve | migrate | gc | forget-host | uninstall | agent`. Every command accepts `--json`, which is **the contract for any UI** (contract E, ADR [0018](../adr/0018-runtime-and-architecture.md)).

## Direction: hashes over a commit graph

ADR [0005](../adr/0005-direction-detection-three-way-hash.md), [0021](../adr/0021-who-may-push.md), [contract D](../contracts/store-format.md).

For each subscribed copy on each host:

```
L = hash(normalize(local copy))
R = the newest commit all heads descend from (none if InFlight or Forked)
B = (commit, local_hash) this host last synced for this copy
```

The full decision table is in ADR 0005. In short:

- L == R → **InSync**.
- L == B, R moved on → **Behind** → apply.
- L changed, R == B → **Ahead** → push a commit whose parent is B.
- Both moved → **Diverged** → push the local edit as its own commit, which makes a visible fork; notify; wait for `resolve`.
- Heads not ancestry-ordered → **Forked** → no host applies anything until `resolve`.
- A commit or tree isn't fully delivered → **InFlight** → wait.
- Tombstone, lost store, deleted local copy, missing deck, version mismatch → **stop and notify** (ADRs [0025](../adr/0025-deletion-and-unshare.md), [0026](../adr/0026-profile-identity.md), [0027](../adr/0027-store-lifecycle.md)).

![sync state diagram](../sync-states.png)

**Why a commit graph:** two hosts pushing from the same base produce two commits with the same parent. Neither overwrites the other, and every host sees the fork. A single shared "current" file would let the last writer silently win (review F1).

**Timestamps are metadata, never direction.** Commits and heads carry `updated_at` and `updated_by`, shown in `status`, notifications and the log. "This machine is newer than the mount" is put into practice as *changed since the last sync*.

## Normalization and variables

ADR [0006](../adr/0006-normalization-and-variables.md), [contract C](../contracts/profile-format.md).

- **The store holds full trees**, exactly as the app wrote them, with only variable values replaced by placeholders. Install is close to a byte-copy: expand the placeholders, set `Device.UUID`, and name the folder.
- **Normalization is used only to compute the hash.** It strips the runtime fields and `Device.UUID`, canonicalizes JSON with RFC 8785, and hashes an allow-listed file set. The exact definition, including `norm_version`, is [contract C § normalized hash](../contracts/profile-format.md#normalized-hash). Stray `.DS_Store` or conflict-copy files are outside the allow-list.

**Variables (v1):**

| Variable | Declared | Value per host |
|---|---|---|
| `{{HOME}}` | built-in | this host's home directory |
| e.g. `{{HA_URL}}` | `<store>/config.toml` (name + default) | `<store>/hosts/<host_id>.toml`, e.g. a LAN URL on one host and a remote URL on a firewalled host |

- Substitution is **path-boundary-aware**: `/Users/<al>` never matches inside `/Users/<alice>`.
- A literal `{{` is escaped as `{{_}}`.
- **Collision guard:** a push is refused if the local copy contains another host's value as a literal (e.g. the other Mac's home path baked into an `Open` button).
- **Changing a variable's value re-materializes** the local copy through the apply. It never pushes.

## Applying: plan, commit, verify

ADR [0008](../adr/0008-two-phase-apply.md), [0019](../adr/0019-selected-profile-stays-per-host.md).

Changing the app's files is surgery on a live system. **The app restart is the commit point. Everything that can fail is checked before it, and local state is re-checked once the app can no longer write.**

A run goes:
1. Pushes.
2. Re-read heads.
3. **One batched apply** of every Behind copy on this host, active or not, with one app restart.

![apply state diagram](../apply-states.png)

1. **Plan** (nothing touched):
   - Stage each target with this host's values; its staged hash must equal R.
   - Geometry matches; the incoming fingerprint is known here; the schema guard passes; disk is sufficient.
   - Check plugin presence.
   - Record L_plan and whether the app was running.
2. **Journal** `plan.json` with fsync. Its `step` is advanced at every stage.
3. **Quit** gracefully. On timeout, abort with nothing touched.
4. **Post-quit re-check:** if any L ≠ L_plan (an edit, or a flush by the app while quitting), abort the swap, relaunch, and re-evaluate. The edit is preserved as Ahead or Diverged.
5. **Snapshot** each target into the local history, *after* the quit.
6. **Swap** with atomic renames.
7. **Relaunch** if it was running, and settle.
8. **Verify:** L == hash(R) ⇒ B and head := R.
9. **Verify failure:**
   - `rollback` (default): restore every target, then **BLOCKED(R)**, with one deduplicated notification ("schrodeck won't update *Work* on this Mac: the incoming version failed verification"). There is no retry until R changes or the user runs `resolve`/`unblock`.
   - `keep`: B := (R, actual hash), flagged.

Crash recovery follows the journal step: before the swap, discard; at or after the swap, roll forward through Quit and Verify. **The selected profile on each deck is never read or written for sync** ([R14]). After the restart each deck shows what it showed before, and Smart Profiles keep switching locally ([R13]).

## The store

ADR [0009](../adr/0009-store-write-protocol.md), [0023](../adr/0023-store-freshness-via-file-provider.md), [0027](../adr/0027-store-lifecycle.md). The single definition of paths, records and the write protocol is **[contract D](../contracts/store-format.md)**.

- **No file has two writers.** Trees and commits are write-once and content-addressed. Each host writes only its own head, host file, events, inventory and tmp. The common `config.toml` is the only exception: rarely written, with `config resolve`.
- **Push:** stage and verify the tree → rename it into `trees/` → write the commit → wait (bounded) for the provider to confirm upload → move this host's head **last**.
- **Read:** a head whose commit or tree is incomplete means InFlight; ordering on the wire doesn't matter.
- **Freshness:** before reading and after writing, ask the provider via File Provider: read only when every file is `current`; a push is confirmed once uploaded. A profile stuck InFlight for more than an hour raises one alarm. The limit: `current` means the newest version *this host knows of*. Observing the `current` state on Dropbox is a gate for M2.
- **Lifecycle:** `FORMAT` / `norm_version` changes go through an explicit `migrate`; hosts on older versions go read-only. GC is generation-based. `uninstall` never touches profiles.

![store write protocol](../adr/store-write.png)

## Configuration layers

ADR [0010](../adr/0010-host-identity-and-config-layering.md).

| Layer | Where | Writer | Holds |
|---|---|---|---|
| Host identity | derived at runtime | — | `host_id` |
| Local pointer | `~/.config/schrodeck/config.toml` (optional) | this host | `store = "<path>"` only |
| Per-host config | `<store>/hosts/<host_id>.toml` | this host only | friendly name, variable values, subscriptions, host overrides |
| Common config | `<store>/config.toml` | any host, rarely | variable declarations, retention defaults, `notify.*`, `apply.*`, script replication map |

**Store discovery:** `--store` → local pointer → Dropbox `info.json` → iCloud Drive. If more than one candidate holds a `FORMAT` file, refuse and ask. The set of host files is a **display-only registry**: no decision counts hosts, and a retired host blocks nothing.

## History and rollback

ADR [0011](../adr/0011-history-and-rollback.md).

| Ring | Size (default) | Where |
|---|---|---|
| Local | `retention.local` = X (20) | `~/Library/Application Support/schrodeck/history/<profile_id>/`, written after the quit before each swap, before rollback/resolve, after each push |
| Shared | `retention.shared` = Y (20) | the commit chain; GC keeps at least Y generations behind every head |

- `schrodeck rollback <profile> <id>` writes a new commit (kind `rollback`) whose parent is R. Every host converges on it.
- `schrodeck hold <profile>` / `resume <profile>` pause and resume sync of a profile on this host. While held, `rollback --local <id>` applies an old version without publishing it.
- `schrodeck resolve <profile> --keep …` ends a fork with a commit whose parents are every tip (ADR [0007](../adr/0007-conflict-policy.md)).

## Deletion, unshare and identity

ADR [0025](../adr/0025-deletion-and-unshare.md), [0026](../adr/0026-profile-identity.md).

- `unshare` writes a tombstone. Subscribers stop and keep a detached copy.
- A locally deleted copy is **never** propagated: that host stops and notifies.
- A store copy that vanished without a tombstone stops and notifies. It is never resurrected or wiped.
- When a host has nothing left to sync, the agent suggests `schrodeck uninstall` once, and never uninstalls itself.
- `profile_id` is permanent. The sharing host keeps its folder, and subscribers get `uuid5(profile_id, deck)`, so the mapping can be rebuilt. With two matching decks, `--deck` is required. If a deck disappears from the app, that copy stops until it returns.

## Triggers and loop protection

ADR [0012](../adr/0012-triggers.md).

- **A resident agent** (LaunchAgent, `KeepAlive`) holds a **recursive** FSEvents watcher on `ProfilesV3/` and the store's `profiles/`, debounced. launchd `WatchPaths` isn't recursive, so it would miss page edits.
- **A safety timer** runs about every 15 minutes. **Every action is also in the CLI.**
- **An optional USB-attach accelerator** fires on Elgato's vendor id `0x0fd9`. It is never needed for correctness.
- **One lock** prevents overlapping runs.
- **No feedback loop:** the app's own launch rewrite after an apply normalizes to the same hash. A required test proves that apply → launch rewrite ⇒ zero further applies.
- **Pushes debounce and never back off. Applies back off exponentially per profile** (1, 2, 4 … 60 min), with no cap by default. `notify.on_backoff` is on by default, and the hard cap `apply.max_per_hour` is opt-in. A verify failure isn't retried at all; it goes to BLOCKED.

## Scope: what is replicated, what is only checked

ADR [0013](../adr/0013-sync-scope-and-scripts.md), [0014](../adr/0014-plugin-handling.md).

| Item | Handling |
|---|---|
| Shared profiles (pages, folders, action settings) | replicated as full trees |
| Icon packs | replicated as write-once trees in the store when changed |
| Plugin global settings | **never synced**; may differ per host by design ([R7]) |
| Plugins (installed? version?) | inventoried. Missing → **apply anyway and notify, naming the profile, page and plugin**. Version skew → warn. v1 doesn't install or copy plugins (the only documented install path is opening a `.streamDeckPlugin`, [R5, R17]); copying is issue #4 |
| `Open` action paths | inventoried (exists and executable after variable expansion) |
| Shortcuts | inventoried against `shortcuts list` |
| BetterTouchTool triggers | reported only |
| Scripts | opt-in per path in the common config: `managed-elsewhere` (report only) or `store` (copied from the store, hash-checked). Off by default, because it means running code that arrived through a sync service |

Results go to `<store>/inventory/<host_id>.json` and `schrodeck inventory`.

## Schema guard

ADR [0015](../adr/0015-schema-guard.md).

Elgato publishes a JSON schema for plugin manifests, but **none for profiles** ([R2], observed). schrodeck records a known-good fingerprint from:
- the app version;
- the profile manifest `Version` (observed `"3.0"`);
- the key set at each manifest level, plus the set of file-name patterns.

If a **local** signal changes (an app update), **pushes and applies both pause** until `schrodeck doctor` passes and the user confirms. Each commit records the fingerprint it was made under, and the apply plan refuses commits whose fingerprint this host hasn't verified (e.g. saved by a newer app on another host).

## Notifications

ADR [0016](../adr/0016-notifications.md).

`SchrodeckNotifier.app` is a small Swift helper, built and ad-hoc signed at install time and installed into `~/Applications`. A spike on macOS 27 found that it is refused silently from a temp directory, and works from `~/Applications` after a one-time permission prompt. It uses an original icon with no Elgato marks ([R19]). The fallback is `osascript`.

**Every alert is deduplicated** by `(condition, profile, version)`: it is sent once when the condition is entered, not once per run. An optional reminder interval is available (`notify.remind_after`, default off). The event table, including BLOCKED, forks, collisions, stuck-in-flight, version mismatch and "nothing to sync", is in ADR 0016.

## Observability

ADR [0017](../adr/0017-observability.md).

- **Local log:** `~/Library/Logs/schrodeck/schrodeck.log`.
  - Levels: DEBUG for steps and InSync no-ops; INFO for every state transition with its trigger; WARN; ERROR with the state left behind.
  - `SCHRODECK_LOG_LEVEL` overrides the level.
- **Event trail:** `<store>/events/<host_id>.jsonl`, append-only, one file per host. `schrodeck log` merges all hosts into one timeline.
- Timestamps are for display only.
- Never logged: tokens, serials, raw `Device.UUID` or `IOPlatformUUID`.

## Architecture

ADR [0018](../adr/0018-runtime-and-architecture.md), [0024](../adr/0024-documented-contracts.md).

A **Go** core holds all sync logic: normalization, the commit graph, the store protocol, history, inventory, bindings, variables, backoff and notification dedup. It also holds the CLI and its `--json` output. Everything OS-specific sits behind ports, implemented once per OS as a **connector**:

- the port list, invariants and filesystem guarantees are [contract A](../contracts/os-connector.md);
- what each connector assumes about the Stream Deck app is [contract B](../contracts/client-os.md);
- the profile format and hash definition are [contract C](../contracts/profile-format.md);
- the store format is [contract D](../contracts/store-format.md).

Swift appears only at the macOS edges (the notifier now, a SwiftUI menu-bar app later). Both talk to the core through contract E, the `--json` CLI. Distribution is a Homebrew tap that builds from source (ADR [0028](../adr/0028-distribution-brew-tap-and-pkg.md)).

## Testing

- Fixtures are built from **redacted** real manifests. Every check has a known-bad case that makes it fail as well as a known-good one:
  - a launch-only rewrite ⇒ InSync, but Ahead with normalization disabled;
  - two hosts pushing from the same base ⇒ Forked on both; the old last-writer-wins store loses an edit in the same test;
  - an edit made during Quit ⇒ swap aborted, edit preserved;
  - a verify failure ⇒ one rollback, BLOCKED, one notification across many runs;
  - adversarial variables: prefix homes, a literal `{{`, another host's path baked in, a changed value;
  - a half-delivered or junk-file store ⇒ InFlight or unaffected, as appropriate;
  - apply → launch rewrite ⇒ zero further applies;
  - two-host ping-pong with a broken normalizer ⇒ bounded applies via backoff, and pushes never delayed;
  - "never written" properties (unshared profiles, the prefs plist, dry runs) are asserted at the filesystem-port level, not by byte comparison.
- **What these tests can't catch:** Elgato changing what the app rewrites at runtime. Only the schema guard plus a live `doctor` run on each new app version covers that. Also, provider behavior beyond what was observed: only Dropbox and iCloud were seen, and the `current` state has not been seen yet.

## Open questions

1. **Issue #2 (confirmation only):** on real hardware, does an edit made with the deck detached change `ProfilesV3` on disk? ADR 0021 is already accepted; this confirms its premise.
2. **Issue #4:** can a plugin bundle (open or encrypted Marketplace) be copied to another host and work? It decides whether opt-in plugin replication is ever offered.
3. **Device serial portability (informational):** `Device.UUID` contained the USB serial for one deck on one host (issue #1). Only deck sightings depend on it, never sync decisions.
4. **CLA tooling:** which CLA mechanism (e.g. cla-assistant) to adopt before the first outside pull request (ADR [0020](../adr/0020-project-hygiene-naming-license.md)).
5. **The M2 gate:** does Dropbox report `current` for an up-to-date downloaded file (ADR [0023](../adr/0023-store-freshness-via-file-provider.md))?
6. **The app tolerating our folder names:** does the app load a profile from a folder UUID it didn't create (contract B M4 round-trip probe, ADR [0026](../adr/0026-profile-identity.md))? This gates M3.

[R1]: ../references.md
[R2]: ../references.md
[R3]: ../references.md
[R5]: ../references.md
[R7]: ../references.md
[R8]: ../references.md
[R9]: ../references.md
[R10]: ../references.md
[R11]: ../references.md
[R12]: ../references.md
[R13]: ../references.md
[R14]: ../references.md
[R15]: ../references.md
[R16]: ../references.md
[R17]: ../references.md
[R18]: ../references.md
[R19]: ../references.md
