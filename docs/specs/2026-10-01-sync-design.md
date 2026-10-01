# schrodeck — design

Status: DRAFT for review, aligned with ADRs 0001–0021 (2026-10-01). No code exists yet.

This document is the narrative of how schrodeck fits together. The *why* behind each choice, and the alternatives rejected, live in the [ADRs](../adr/README.md). Each section links the ADRs it implements. Facts about the Stream Deck app are cited from [references.md](../references.md) as `[Rn]` and marked *documented* (Elgato states it) or *observed* (seen on a real machine, not promised).

## Problem

One or more Stream Decks are shared between several computers, for example through a Thunderbolt or KVM switch. You edit a profile on Mac A, flip the switch, and Mac B shows the old layout. The Stream Deck app keeps profiles per machine. It has no automatic cross-machine sync for desktop decks, only manual export/import ([R18], documented by absence).

## Goals

- Run unattended on every host and keep **shared profiles** converged through a shared cloud folder: Dropbox, iCloud Drive, or any folder a sync client replicates (ADR [0002](../adr/0002-transport-shared-cloud-folder.md)).
- Let a host decide on its own, per shared profile, whether its copy is **behind**, **ahead**, **in sync**, or **diverged**. Push when ahead, apply when behind, and never silently drop an edit.
- Need nothing beyond the shared folder and local CLI tools. **No network access between hosts**: machines on the same desk can have different firewalls, VPNs or endpoint agents.
- Support **any number of hosts and decks**. A profile created on one machine can be used on other machines, including on a **different physical deck** with the same geometry.
- Build the macOS version first, with the OS-specific parts behind interfaces so that a Windows adapter set can be added later (ADR [0018](../adr/0018-runtime-and-architecture.md)).

## Non-goals (v1)

- Merging concurrent edits. Diverged stops and asks (ADR [0007](../adr/0007-conflict-policy.md)).
- Re-flowing a profile onto a deck with a different geometry, such as 32 keys onto 15 (ADR [0004](../adr/0004-shared-profiles-and-subscriptions.md)).
- Installing or copying plugins, and syncing plugin global settings (ADR [0014](../adr/0014-plugin-handling.md); copying is investigated in issue #4).
- Syncing which profile is selected on each deck (ADR [0019](../adr/0019-selected-profile-stays-per-host.md)).

## Prior art

- [dominik-ba/stream-deck-profile-sync](https://github.com/dominik-ba/stream-deck-profile-sync): manual `push`/`pull` through a cloud folder. It does not detect direction, handle conflicts, rewrite paths, restart the app, or run on its own (ADR [0001](../adr/0001-build-vs-adopt.md)).
- Elgato's "deploying profiles at scale" (`custom_default_profiles`) seeds defaults only. Elgato marks file-level management "not officially supported and may break with future software updates" ([R3], documented). That is why the schema guard exists.

## Concepts

| Term | Meaning |
|---|---|
| **Host** | One macOS user account on one machine. Identity is derived, never configured: `host_id = sha256(IOPlatformUUID + ":" + username)[:12]` (ADR [0010](../adr/0010-host-identity-and-config-layering.md)). |
| **Deck** | A Stream Deck as one host's app sees it: physical or virtual. Each host enumerates *its own* decks from the app's data (the prefs `Devices` list and profile manifests). The sync logic needs no USB access (ADR [0003](../adr/0003-decks-are-local-geometry-compatibility.md)). |
| **Geometry** | Columns × rows, plus dial/encoder count, from Elgato's DeviceType table ([R8], documented). Two decks are **compatible** when their geometry matches. This brings virtual decks into scope. |
| **Shared profile** | The unit of sync: one Stream Deck profile (with all its pages and folders) that a user opted in with `share`. Unshared profiles are never read for sync, written, or deleted (ADR [0004](../adr/0004-shared-profiles-and-subscriptions.md)). |
| **Subscription** | A host's local copy of a shared profile, installed onto one compatible local deck. A host may subscribe the same profile onto several of its decks. **Every subscribed copy is kept up to date**, whether or not it is the deck's active profile. |
| **Store** | The shared cloud folder (ADR [0009](../adr/0009-store-write-protocol.md)). |

Deck sightings: when a hardware serial is visible in the app's device id (observed as `@(1)[vendor/product/serial]`, [R9]), schrodeck may record "deck seen on host" in the store. This is informational only, for `status` and `log`. No decision depends on it.

## Facts the design rests on

Observed on Stream Deck 7.4.2–7.5.1, macOS. The schema guard re-checks them after every app update.

| Fact | Status | Consequence |
|---|---|---|
| Profiles live in `~/Library/Application Support/com.elgato.StreamDeck/ProfilesV3/<uuid>.sdProfile/` with a top-level `manifest.json` and `Profiles/<page>/manifest.json` + `Images/` | observed [R1, R2] | Copy whole profile trees; they are small. |
| Profiles are device-specific; layouts differ across models | documented [R10] | Compatibility is by geometry. |
| The app rewrites top-level manifests on launch and touches page manifests at runtime with no edit; runtime fields include action `State` and `Pages.Current` | observed [R15] | mtime can't show direction; hash normalized content (ADRs [0005](../adr/0005-direction-detection-three-way-hash.md), [0006](../adr/0006-normalization-and-variables.md)). |
| Each profile names its deck in `Device.UUID` | observed [R9] | Dropped from the hash; rewritten per receiving deck on install. |
| `Open` actions store absolute paths under the user's home | observed [R16] | `{{HOME}}` variable. |
| The app keeps state in memory and must be closed before its files are changed | documented [R3] | Apply = quit → swap → relaunch (ADR [0008](../adr/0008-two-phase-apply.md)). |
| Disconnected decks stay editable | documented [R11] | Any host whose copy changed may push (ADR [0021](../adr/0021-who-may-push.md)). |
| Plugin global settings are machine-local; action settings travel with profiles | documented [R7] | Global settings never sync (ADR [0014](../adr/0014-plugin-handling.md)). |
| The selected profile per deck is stored in the app's prefs (`ESDProfilesPreferred`) | observed [R14] | Never synced or written (ADR [0019](../adr/0019-selected-profile-stays-per-host.md)). |
| There is no official CLI or URL scheme to import profiles or quit/relaunch the app | documented by absence [R17] | AppControl uses `osascript` + `open`. |

## User workflow

```
host A:  schrodeck share "Work"              # opt in; published to the store
host B:  (notification: "Shared profile Work (8×4) is available")
host B:  schrodeck subscribe "Work" --deck <deck>   # any local deck with 8×4 geometry
         # edit Work on either host; the other host applies it automatically
host B:  schrodeck unsubscribe "Work" [--deck <deck>]   # stop syncing; the local copy stays
```

The full CLI is `schrodeck status | sync | share | unshare | subscribe | unsubscribe | push | pull | resolve | history | rollback | hold | resume | log | inventory | doctor | config`. Every command accepts `--json`, which is **the contract for any UI** (ADR [0018](../adr/0018-runtime-and-architecture.md)).

## Direction: the 3-way hash

ADR [0005](../adr/0005-direction-detection-three-way-hash.md), [0021](../adr/0021-who-may-push.md).

For each subscribed copy on each host:

```
L = hash(normalize(local copy))
R = hash recorded in <store>/profiles/<id>/current.json
B = last hash this host synced for this copy (local state)
```

| Condition | State | Action |
|---|---|---|
| L == R | InSync | B := L |
| L == B, R ≠ B | Behind | two-phase apply |
| L ≠ B, R == B | Ahead | push, allowed from **any** host whose copy changed, deck attached or not |
| L ≠ B, R ≠ B, L ≠ R | Diverged | keep both, notify, wait for `resolve` |
| no B, R absent | FirstRun → Ahead | push |
| no B, R present, L ≠ R | FirstRun → Diverged | notify |

![sync state diagram](../sync-states.png)

**Timestamps are metadata, never direction.** Every copy and every `current.json` carries `last_updated` (UTC) and `updated_by`, shown in `status`, notifications and the log. "This machine is newer than the mount" is put into practice as *changed since the last sync* (L ≠ B). Comparing clocks fails because hosts skew and the app touches files with no edit.

This scales to any number of hosts without coordination. Each host keeps only its own B per copy.

## Normalization and variables

ADR [0006](../adr/0006-normalization-and-variables.md).

Before hashing:
1. Parse the JSON.
2. Drop the runtime-only fields (action `State`, `Pages.Current`) and `Device.UUID`.
3. Replace variable values with placeholders.
4. Sort keys, serialize canonically, and hash the result together with the image bytes.

The strip list is **one named constant**. Every entry has a test proving that a launch-only rewrite doesn't change the hash.

**Variables (v1):**

| Variable | Kind | Value per host |
|---|---|---|
| `{{HOME}}` | built-in | this host's home directory |
| e.g. `{{HA_URL}}` | user-defined in the common config | e.g. a LAN URL on one host, a remote URL on a firewalled host |

```toml
# <store>/config.toml
[variables.HA_URL]
default = "https://ha.example.lan"
"<host_id-B>" = "https://remote.example.net"
```

On push, each host's values are replaced with placeholders. On install, the receiving host's values are expanded. A per-host difference therefore never registers as an edit or a conflict.

## Applying: plan, commit, verify

ADR [0008](../adr/0008-two-phase-apply.md), [0019](../adr/0019-selected-profile-stays-per-host.md).

Changing the app's files is surgery on a live system. **The app restart is the commit point, and everything that can fail is checked before it.** An apply covers every behind subscribed copy on the host, active or not.

![apply state diagram](../apply-states.png)

1. **Plan** (the app still runs; nothing is touched). Abort on any failure:
   - Stage each target in a scratch dir on the same volume: variables expanded, `Device.UUID` set to the receiving deck, folder id = uuid5(profile, deck).
   - Staged hash == R.
   - Geometry matches.
   - Schema guard passes.
   - Rollback snapshot written and re-verified.
   - Free disk is sufficient.
   - Plugin policy applied.
2. **Journal:** write `plan.json` (targets, staged paths, snapshot ids, expected hashes) and fsync it.
3. **Quit** the app and wait for it to exit. On timeout, abort with nothing touched.
4. **Swap** with atomic renames.
5. **Relaunch** and wait to settle (process up, files quiet).
6. **Verify:** re-hash every target. If L == R, set B := R and clear the journal.
7. **On verify failure:** log the expected and actual hashes, then follow `apply.on_verify_failure`:
   - `rollback` (default): restore and relaunch.
   - `keep`: flag the copy.

   Either way, notify and clear the journal.

A run that finds a leftover journal completes the swap or restores the snapshot; it never guesses. **The selected profile on each deck is never read or written for sync** ([R14]), so after the restart each deck shows what it showed before, and Smart Profiles keep switching locally ([R13]).

## The store

ADR [0009](../adr/0009-store-write-protocol.md), [0010](../adr/0010-host-identity-and-config-layering.md).

```
<store>/FORMAT                               store schema version; unknown → refuse
<store>/config.toml                          common config
<store>/profiles/<profile-id>/current/       normalized tree
<store>/profiles/<profile-id>/current.json   {hash, last_updated, updated_by, app_version, geometry}
<store>/profiles/<profile-id>/snapshots/     shared history ring
<store>/icon-packs/<id>.sdIconPack/          replicated icon packs
<store>/scripts/                             opt-in replicated scripts
<store>/events/<host_id>.jsonl               per-host, append-only
<store>/inventory/<host_id>.json             per-host
```

**Push protocol:**
1. Write `current.tmp-<host_id>/`.
2. Re-read and verify it.
3. Rename it to `current/`.
4. Write `current.json` **last**.

A reader whose tree hash differs from `current.json` treats the profile as **in flight**: it retries next tick and never applies. No file is written by more than one host, except through this protocol.

![store write protocol](../adr/store-write.png)

## Configuration layers

ADR [0010](../adr/0010-host-identity-and-config-layering.md).

| Layer | Where | Holds |
|---|---|---|
| Host identity | derived at runtime | `host_id` |
| Local pointer | `~/.config/schrodeck/config.toml` (optional) | `store = "<path>"` only |
| Common config | `<store>/config.toml` | subscriptions, variables, `retention.local` / `retention.shared`, `notify.*`, `apply.*`, script replication map, host registry (`host_id → friendly name`), host-keyed overrides |

**Store discovery:** `--store` → local pointer → Dropbox `info.json` → iCloud Drive. If more than one candidate holds a `FORMAT` file, refuse and ask. Hosts self-register on first run. Common-config writes use the same stage → verify → rename discipline.

## History and rollback

ADR [0011](../adr/0011-history-and-rollback.md).

| Ring | Size (default) | Where | Written |
|---|---|---|---|
| Local | `retention.local` = X (20) | `~/Library/Application Support/schrodeck/history/<profile>/` | before every apply, rollback, resolve; after every push |
| Shared | `retention.shared` = Y (20) | `<store>/profiles/<id>/snapshots/` | every push; both sides of every diverge |

- Rotation is by count only and never removes the entries for the current R or this host's B.
- `schrodeck rollback <id>` applies the entry through the two-phase apply, then pushes it as a **new generation**.
- `--hold` keeps a rollback on this host only, pausing that profile until `resume`.
- `schrodeck resolve <profile> --keep local|remote|<snapshot-id>` ends a divergence the same way (ADR [0007](../adr/0007-conflict-policy.md)).

## Triggers and death-spiral protection

ADR [0012](../adr/0012-triggers.md).

- **Watchers:** launchd `WatchPaths` on `ProfilesV3/` (local edits) and on each store `current.json` (remote changes), debounced.
- **Safety timer:** about every 15 minutes.
- **CLI:** every action can be run by hand.
- **Optional accelerator:** a launchd USB-attach (IOKit matching) event on Elgato's vendor id `0x0fd9`, so a deck switched to this host is updated before it's used. Never needed for correctness.
- **One lock** prevents overlapping runs.
- **No feedback loop:** the app's own launch rewrite after an apply normalizes to the same hash. A required test proves that apply → launch rewrite ⇒ zero further applies.
- **Exponential backoff, no cap by default:**
  - Consecutive applies or pushes of the same profile wait 1, 2, 4 … minutes, up to 60.
  - The backoff resets after a quiet period of twice the current backoff.
  - When backoff engages, a notification fires (`notify.on_backoff`, default on).
  - An optional hard cap, `apply.max_per_hour`, is unset by default.
  - A two-host ping-pong simulation with a deliberately broken normalizer must show bounded applies.

## Scope: what is replicated, what is only checked

ADR [0013](../adr/0013-sync-scope-and-scripts.md), [0014](../adr/0014-plugin-handling.md).

| Item | Handling |
|---|---|
| Shared profiles (pages, folders, action settings) | replicated |
| Icon packs | replicated via the store when changed |
| Plugin global settings | **never synced**; may differ per host by design ([R7]) |
| Plugins (installed? version?) | inventoried. Missing → **apply anyway and notify, naming the profile, page and plugin**. Version skew → warn. v1 doesn't install or copy plugins (only documented install path: opening a `.streamDeckPlugin`, [R5, R17]); copying is issue #4 with a `lipo` architecture check |
| `Open` action paths | inventoried (exists and executable after variable expansion) |
| Shortcuts | inventoried against `shortcuts list` |
| BetterTouchTool triggers | reported only |
| Scripts | opt-in per path in the common config: `managed-elsewhere` (report only) or `store` (copied from `<store>/scripts/`, hash-checked). Off by default, because it means running code that arrived through a sync service |

Results go to `<store>/inventory/<host_id>.json` and `schrodeck inventory`.

## Schema guard

ADR [0015](../adr/0015-schema-guard.md).

Elgato publishes a JSON schema for plugin manifests but **none for profiles** ([R2], observed). schrodeck records a known-good fingerprint from three signals:
1. The app version, from the app bundle's `Info.plist`.
2. The profile manifest `Version` (observed `"3.0"`).
3. A structural fingerprint: the key set at each level of top-level and page manifests.

Plugin manifests are validated against Elgato's published schema. A launchd watch on the app bundle notices updates. If any signal differs, **all applies pause** (status is still reported) until `schrodeck doctor` re-runs the launch-rewrite stability checks against the new app and the user confirms.

## Notifications

ADR [0016](../adr/0016-notifications.md).

`SchrodeckNotifier.app` is a small Swift helper, built and ad-hoc signed at install time and installed into `~/Applications`. A spike on macOS 27 found that it is refused silently from a temp directory, and works from `~/Applications` after a one-time permission prompt. It uses an original icon with no Elgato marks ([R19]). The fallback is `osascript` with a generic icon.

| Event | Default |
|---|---|
| Apply starting (the deck is about to blank) | on |
| Apply / push / rollback done (with `updated_by`, age) | on |
| New shared profile available | on |
| Diverged | on, persistent |
| Failed (+ rolled back) | on, persistent |
| Missing plugin / script / Shortcut | on |
| Backoff engaged | on (`notify.on_backoff`) |
| Held profile skipped | once per hold |
| InSync | never |

## Observability

ADR [0017](../adr/0017-observability.md).

- **Local log:** `~/Library/Logs/schrodeck/schrodeck.log`.
  - Levels: DEBUG for steps and InSync no-ops; INFO for every state transition with its trigger; WARN; ERROR with the state left behind.
  - `SCHRODECK_LOG_LEVEL` overrides the level.
- **Event trail:** `<store>/events/<host_id>.jsonl`, one append-only file per host. `schrodeck log` merges all hosts into one timeline.
- Timestamps are for display only.
- Never logged: tokens, serials, raw `Device.UUID` or `IOPlatformUUID`.

## Architecture

ADR [0018](../adr/0018-runtime-and-architecture.md).

A **Go** core holds all sync logic: normalization, the 3-way compare, the store protocol, history, inventory, bindings, variables and backoff. It also holds the CLI and its `--json` output. Everything OS-specific sits behind six ports:

| Port | macOS adapter | Future Windows adapter (unverified) |
|---|---|---|
| AppControl | `osascript` quit + `open` relaunch | process API |
| DeviceEnumerator | app prefs + manifests | the same data under `%APPDATA%` |
| Watcher | launchd WatchPaths / FSEvents | ReadDirectoryChangesW |
| Notifier | Swift `SchrodeckNotifier.app` | toast API |
| AppPrefs | plist | registry |
| Scheduler | launchd plists | Task Scheduler |

Swift exists only at the edges: the notifier now, and a SwiftUI menu-bar app later that talks to the CLI. Core tests run on Linux CI with fake adapters, and adapter tests run on macOS runners.

## Testing

- Fixtures are built from **redacted** real manifests. Every check has a known-bad case that makes it fail as well as a known-good one:
  - launch-only rewrite ⇒ InSync, but Ahead with normalization disabled;
  - a real button edit ⇒ Ahead;
  - a wrong variable value ⇒ the path check fails;
  - a half-synced store ⇒ in flight, no apply;
  - an idle host with no deck attached ⇒ no push;
  - apply → launch rewrite ⇒ zero further applies;
  - two-host ping-pong with a broken normalizer ⇒ bounded applies via backoff.
- **What these tests cannot catch:** Elgato changing what the app rewrites at runtime. Only the schema guard plus a live `doctor` run on each new app version covers that.

## Open questions

1. **Issue #2 (confirmation only):** on real hardware, does an edit made with the deck detached change `ProfilesV3` on disk? ADR 0021 is already accepted; this confirms its premise.
2. **Issue #4:** can a plugin bundle (open or encrypted Marketplace) be copied to another host and work? It decides whether opt-in plugin replication is ever offered.
3. **Device serial portability (informational):** `Device.UUID` contained the USB serial for one deck on one host (issue #1). Only deck sightings depend on it, never sync decisions.
4. **CLA tooling:** which CLA mechanism (e.g. cla-assistant) to adopt before the first outside pull request (ADR [0020](../adr/0020-project-hygiene-naming-license.md)).

[R1]: ../references.md
[R2]: ../references.md
[R3]: ../references.md
[R5]: ../references.md
[R7]: ../references.md
[R8]: ../references.md
[R9]: ../references.md
[R10]: ../references.md
[R11]: ../references.md
[R13]: ../references.md
[R14]: ../references.md
[R15]: ../references.md
[R16]: ../references.md
[R17]: ../references.md
[R18]: ../references.md
[R19]: ../references.md
