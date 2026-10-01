# mac-streamdeck-sync — design

Status: DRAFT for review (2026-10-01). No code exists yet.

## Problem

One or more Elgato Stream Decks are shared between several Macs, for example through a Thunderbolt/KVM switch. You edit a profile on Mac A, flip the switch, and Mac B shows the old layout. The Stream Deck app keeps profiles per machine. It has no automatic cross-machine sync, only manual export/import.

## Goal

A tool that runs unattended on every Mac and keeps each deck's profiles converged through a shared cloud folder (Dropbox, iCloud Drive, or any folder that a sync client replicates):

- Decide on its own whether this Mac is **behind**, **ahead**, **in sync**, or **diverged** for each deck.
- Push when ahead. Pull and apply when behind. Never silently drop an edit.
- Need nothing beyond the shared folder and local CLI tools. **No peer-to-peer network access between Macs**: hosts on the same desk can have different firewall rules, VPN or endpoint agents.
- Support **N Macs** and **N decks**.

## Non-goals

- Syncing plugin binaries or plugin global settings (API tokens and the like). Those are host-specific and often encrypted. We **inventory and report** plugins; we do not copy them.
- Windows. Profile layout and paths differ; this is macOS only.
- Merging concurrent edits. Diverged state stops and asks.

## Prior art

- [dominik-ba/stream-deck-profile-sync](https://github.com/dominik-ba/stream-deck-profile-sync): manual `push`/`pull` through a cloud folder, MD5-based `status`, timestamped backups. It does not detect direction automatically, handle conflicts, rewrite paths, restart the app, or run on a schedule.
- [Elgato: deploying profiles at scale](https://www.elgato.com/us/en/explorer/products/stream-deck/stream-deck-profiles-at-scale/): close the app before changing its files. `custom_default_profiles` seeds defaults only. Elgato marks file-level management as "not officially supported and may break with future software updates". That is why the schema guard below exists.

## Facts the design rests on

Observed on Stream Deck 7.4.2 to 7.5.1, macOS. Re-verify after each app update. The schema guard enforces this.

| Fact | Consequence |
|---|---|
| Profiles live in `~/Library/Application Support/com.elgato.StreamDeck/ProfilesV3/<uuid>.sdProfile/` (a top-level `manifest.json` plus `Profiles/<page-uuid>/manifest.json` and `Images/`). | Small: about 350 KB and 50 files for 3 decks. Copy whole trees. |
| Each top-level manifest names its deck: `Device.Model` + `Device.UUID`. | The **sync unit is one deck** (`Device.UUID`), not the whole directory. |
| The app rewrites every top-level manifest on launch, and page manifests at runtime, with no user edit. Actions carry a runtime `State` field. | **File mtime cannot tell which side is newer.** Compare hashes of normalized content. |
| `system.open` actions store absolute paths, e.g. `/Users/<user>/bin/x.sh`. Usernames differ between Macs. | Paths under `$HOME` are stored as `~/…` in the shared copy and expanded on install. |
| The app (`Stream Deck --runinbk`) keeps state in memory and writes it back out. | Applying a pull = quit → swap → relaunch. Otherwise the app overwrites the swap. |
| The app writes its own `.streamDeckProfilesBackup` (a stored zip) into `BackupV3/`. | Every push also writes one, as a manual recovery path using Elgato's own import. |
| Elgato USB vendor ID is `0x0fd9` (4057). | launchd IOKit matching can fire on deck attach. |

## Architecture: shared tool plus local overlay

```
repo (public)                      per-Mac, never committed
├── sdsync (Python CLI)            ~/.config/mac-streamdeck-sync/config.toml
├── launchd/ plist templates         store path, host id, deck allowlist,
├── install.sh                       script replication map, notify prefs
└── docs/                          ~/Library/Application Support/mac-streamdeck-sync/
                                     state.json (B hashes per deck), logs, pre-swap backups
```

The repo holds no host names, usernames, device serials or tokens. Everything specific to one Mac lives in the local overlay. `install.sh` renders the LaunchAgent from a template and copy-deploys it, with no symlinks. `install.sh --check` reports drift.

## Shared store layout

```
<store>/                                  e.g. ~/Dropbox/mac-streamdeck-sync
├── FORMAT                                store schema version (refuse unknown)
├── decks/<device-uuid-hash>/
│   ├── current/                          normalized .sdProfile tree(s) for this deck
│   ├── current.json                      {hash, pushed_by, pushed_at, app_version, model}
│   └── snapshots/<ts>-<host>/            last N pushes plus every diverged pair
├── inventory/<host>.json                 per-host: plugins, icon packs, script checks
├── icon-packs/<id>.sdIconPack/           replicated (see Inventory)
└── backups/<ts>-<host>.streamDeckProfilesBackup
```

- Directory names use a short hash of `Device.UUID`, so serial-like IDs never appear in a folder name. Any shared folder can end up being sent somewhere else.
- Write order for a push: `current.tmp-<host>/` → verify hash → rename to `current/` → write `current.json` **last**. A reader that finds `current.json`'s hash ≠ the hash of the tree it reads treats the store as **in flight** (the cloud client hasn't finished) and retries next tick. It never pulls a half-synced tree.

## Sync algorithm (per deck)

```
L = hash(normalize(local profiles for this deck))
R = current.json.hash                (absent → empty store)
B = local state.json[deck]           (last hash this Mac synced)
```

| Condition | State | Action |
|---|---|---|
| L == R | InSync | B := L |
| L == B, R ≠ B | Behind | pull |
| L ≠ B, R == B | Ahead | push (only if the deck is attached here; see open question 2) |
| L ≠ B, R ≠ B, L ≠ R | Diverged | snapshot both, notify, touch nothing |
| no B, R absent | FirstRun → Ahead | push |
| no B, L ≠ R | FirstRun → Diverged | notify |

![sync state diagram](../sync-states.png)

This scales to N Macs without clocks or coordination. Each Mac keeps only its own B. A Mac that has been away for many generations is simply Behind. Two Macs that both edited since their last sync are Diverged, and the second one to notice keeps both snapshots.

**Normalization** (what makes L stable): parse JSON, drop runtime-only fields (`State`, plus any found in testing), rewrite `$HOME`-prefixed strings to `~`, sort keys, hash the canonical bytes plus the image bytes. The list of dropped fields is a single named constant, and each entry has a test that shows a launch-only rewrite does not change L.

## Applying a pull (always automatic)

1. Stage: build the expanded tree (`~` → this `$HOME`) in a scratch dir and verify its hash.
2. Back up the current local deck tree to `pre-swap/<ts>/`.
3. Quit: `osascript -e 'quit app "Elgato Stream Deck"'`, then wait for the process to exit (timeout → abort, nothing touched).
4. Swap: rename the old tree aside and rename the staged tree in. Both live on the same volume, so each rename is atomic.
5. Relaunch: `open -gj -a "Elgato Stream Deck"`.
6. Verify: after the app settles, re-hash. L ≠ R → restore the pre-swap tree, then quit/relaunch again, and notify.

## Triggers

- **Deck attach:** a LaunchAgent with `LaunchEvents` → `com.apple.iokit.matching` on `idVendor = 0x0fd9`. It fires when the switch hands the deck to this Mac, which is exactly when Behind matters.
- **Timer:** `StartInterval` of about 15 min, to push edits made while attached.
- **CLI:** `sdsync status | sync | push | pull | resolve --keep local|remote|<snapshot> | inventory | doctor`.
- A single lock file (`flock`-style) keeps triggers from overlapping.

## Inventory and scripts

Each run writes `inventory/<host>.json` and reports differences from the deck's requirements:

| Category | Detected from | Check | Replication |
|---|---|---|---|
| Plugins | action `UUID` prefixes vs `Plugins/*.sdPlugin` | installed? version? | report only |
| Icon packs | `IconPacks/*.sdIconPack` | present? | copy via store (about 70 MB, only when changed) |
| File scripts/apps | `system.open` `path` | exists after `~` expansion? executable? | per the local `script replication map`: `managed-elsewhere` (dotfiles repo etc., report only) or `store` (copied from `<store>/scripts/`) |
| Shortcuts | `shortcut.run` `shortcutName` | listed by `shortcuts list`? | report only (iCloud syncs Shortcuts) |
| BetterTouchTool | `com.folivora.btt.action` `btt_identifier` | report the id | report only (BTT owns its config) |
| Other plugin-backed | everything else | plugin present | report only |

Replicating scripts through a cloud folder means **running code that arrived from a sync service**. It is off by default, per path, opted in from the local config, and copied files keep a hash that is checked before each run.

## Safety rails

- **Schema guard:** refuse to run if the app version or the manifest key set differs from the known set, until `sdsync doctor` passes and the user confirms.
- **Dry run** builds everything in a scratch dir and writes nothing beside the target (not even mtimes).
- Never delete a snapshot or pre-swap backup automatically except by count-based rotation (keep the last N).
- Logs: timestamp, level, deck short-hash, state, action. `SDSYNC_LOG_LEVEL` sets the level. No tokens or full device IDs in logs.
- Notifications through `osascript display notification` (no extra dependency).

## Testing

- Fixture trees built from redacted real manifests, including a **known-bad** pair for every check: a launch-only rewrite must stay InSync; a real button edit must flip to Ahead; a wrong `$HOME` must fail the path check.
- Store tests for a half-synced tree (`current.json` hash mismatch → in flight, no pull).
- What these tests **cannot** catch: Elgato changing what the app rewrites at runtime. Only the schema guard and a live `doctor` run on each new app version cover that.

## Open questions

1. **Is `Device.UUID` the same on every Mac for the same physical deck?** It looks derived from the serial, which would make it portable, but this has not been checked on a second Mac. If it is host-specific, the deck key becomes `Model + serial` read from IOKit.
2. **Can you edit a deck's profile while that deck is not attached?** If yes, "only the attached Mac pushes" would block real edits, and pushes should be gated on L ≠ B alone.
3. Snapshot retention N (proposed: 20 per deck).
4. License (proposed: MIT).
