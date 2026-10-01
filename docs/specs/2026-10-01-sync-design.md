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

## Architecture: shared tool, common config, thin local pointer

Configuration has three layers. Each fact has exactly one home:

| Layer | Where | Holds | Differs per host? |
|---|---|---|---|
| **Host identity** | derived at runtime, never stored as config | `host_id = sha256(IOPlatformUUID + ":" + username)[:12]` | yes, but computed |
| **Local pointer** | `~/.config/mac-streamdeck-sync/config.toml` (optional) | `store = "<path>"`: where the shared directory is mounted *on this Mac* | yes: path only |
| **Common config** | `<store>/config.toml` | deck allowlist, `retention.local` (X) / `retention.shared` (Y) restore points per deck, script-replication map, `notify.level`, host registry (`host_id → friendly name`) | no: one copy, shared by all hosts |

- **Why hardware + username:** `IOPlatformUUID` survives renames and OS reinstalls; `ComputerName`/hostname are user-editable and can collide. Stream Deck profiles are per macOS user, so two accounts on one Mac are two hosts. A new logic board or a new Mac gets a new `host_id` and starts as a clean FirstRun, which is safe by construction.
- **Store discovery order:** `--store` flag → local pointer → Dropbox's `~/.dropbox/info.json` (`personal`/`business` path) + `/mac-streamdeck-sync` → iCloud Drive `~/Library/Mobile Documents/com~apple~CloudDocs/mac-streamdeck-sync`. More than one candidate containing a store `FORMAT` file → refuse and ask. Never guess between two stores.
- **The local pointer is the only per-host config**, and it holds a path, not settings. Anything else a user wants to set goes in the common config, so a change made on one Mac applies on all of them. The CLI's `sdsync config set` writes the common config with the same temp → verify → rename discipline as profile pushes.
- **Hosts register themselves:** on first run a host adds its `host_id` (+ friendly name, default `ComputerName`) to the registry. Snapshots, inventory files and notifications name hosts by friendly name. Store paths use `host_id`.

```
repo (public)                      per-Mac, never committed
├── sdsync (Python CLI)            ~/.config/mac-streamdeck-sync/config.toml   (store path only, optional)
├── launchd/ plist templates       ~/Library/Application Support/mac-streamdeck-sync/
├── install.sh                       state.json (B hashes per deck), logs, history/ (local restore points)
└── docs/
```

The repo holds no host names, usernames, device serials or tokens. `install.sh` renders the LaunchAgent from a template and copy-deploys it, with no symlinks. `install.sh --check` reports drift.

## Shared store layout

```
<store>/                                  e.g. ~/Dropbox/mac-streamdeck-sync
├── FORMAT                                store schema version (refuse unknown)
├── config.toml                           common config (see Architecture)
├── decks/<device-uuid-hash>/
│   ├── current/                          normalized .sdProfile tree(s) for this deck
│   ├── current.json                      {hash, pushed_by, pushed_at, app_version, model}
│   └── snapshots/<ts>-<host_id>/         last N pushes plus every diverged pair
├── inventory/<host_id>.json              per-host: plugins, icon packs, script checks
├── icon-packs/<id>.sdIconPack/           replicated (see Inventory)
└── backups/<ts>-<host_id>.streamDeckProfilesBackup
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
2. Snapshot the current local deck tree into the local history ring (`pre-pull`).
3. Quit: `osascript -e 'quit app "Elgato Stream Deck"'`, then wait for the process to exit (timeout → abort, nothing touched).
4. Swap: rename the old tree aside and rename the staged tree in. Both live on the same volume, so each rename is atomic.
5. Relaunch: `open -gj -a "Elgato Stream Deck"`.
6. Verify: after the app settles, re-hash. L ≠ R → restore that `pre-pull` entry, then quit/relaunch again, and notify.

## Triggers

- **Deck attach:** a LaunchAgent with `LaunchEvents` → `com.apple.iokit.matching` on `idVendor = 0x0fd9`. It fires when the switch hands the deck to this Mac, which is exactly when Behind matters.
- **Timer:** `StartInterval` of about 15 min, to push edits made while attached.
- **CLI:** `sdsync status | sync | push | pull | resolve --keep local|remote|<snapshot> | history | rollback | hold | resume | log | inventory | doctor`.
- A single lock file (`fcntl.flock`) keeps triggers from overlapping.

## History and rollback

Two independent rings of restore points per deck, sized separately in the common config:

| Ring | Size key (default) | Where | Written | Survives |
|---|---|---|---|---|
| **Local** (X) | `retention.local` (20) | `~/Library/Application Support/mac-streamdeck-sync/history/<deck>/<ts>-<reason>/` | before every pull, rollback, and resolve, and after every push (the tree as published) | a corrupted or deleted shared store |
| **Shared** (Y) | `retention.shared` (20) | `<store>/decks/<deck>/snapshots/<ts>-<host_id>/` | every push (every shared change), and both sides of every diverge | losing or replacing a Mac |

- Both values live in the common config, so every Mac keeps the same X. A Mac that needs a different X (small disk, say) gets a host-keyed override **in the common config** (`[hosts.<host_id>] retention_local = 5`). That keeps one home per fact and leaves the local pointer as a path only.

- Each entry carries a small `meta.json`: normalized hash, reason (`pre-pull`, `pushed`, `pre-rollback`, `diverged-local`, `diverged-remote`), host, app version, timestamp.
- Rotation is by count only. Each ring keeps its newest X / Y entries and **never** removes the entry matching the current R or this host's B, so the live state always has a restore point.
- `sdsync history [--deck D]` lists both rings with a short id, age, host, reason and hash. Entries with the same hash collapse into one line.
- `sdsync rollback <id> [--deck D]` stages the chosen entry, takes a `pre-rollback` local snapshot, then applies it with the normal quit → swap → relaunch → verify path. Afterwards L ≠ B, so the next sync is **Ahead** and the rollback is **pushed as a new generation**. Every Mac converges on it, and nothing in the history is rewritten.
- `sdsync rollback <id> --hold` applies the rollback locally and pauses automatic sync **for that deck on this host** until `sdsync resume`. Use it to try an old config without publishing it. While held, `status` and notifications say so. Without a hold, the next tick would see the rollback as a local edit and push it, which is right for "this config broke, revert everywhere" and wrong for "let me just look".
- A rollback never deletes anything. Rolling back to a rollback is an ordinary rollback.

## Logging and the event trail

Every run logs; every **state change** is also recorded as an event.

- **Local log:** `~/Library/Logs/mac-streamdeck-sync/sdsync.log`, so Console.app shows it under Log Reports. Format: `2026-10-01T12:30:05-0500 INFO  [deck 3f2a91] Behind -> Pull: R=9c1e… B=41d0… (trigger=attach)`. Levels: DEBUG (each step), INFO (decisions and transitions), WARN (held, in flight, missing script/plugin), ERROR (failure + the state it left things in). `SDSYNC_LOG_LEVEL` overrides; default INFO. Size-rotated, keeping 5 files.
- **Event trail (shared):** `<store>/events/<host_id>.jsonl`, append-only, **one file per host** so two Macs never write the same file and Dropbox never makes a conflicted copy. Each line: `{ts, host_id, deck, from_state, to_state, action, hashes{L,R,B}, trigger, result}`. Rotated by count, the same as the log.
- `sdsync log [--deck D] [--host H] [--since 1d]` merges all hosts' event files by timestamp into one timeline: *who pushed what, who pulled it, where it diverged*. Timestamps are for **display only**. No decision ever reads them, so clock skew between Macs can make the timeline look out of order but can never change behavior.
- InSync ticks with no transition log at DEBUG only, so a 15-minute timer doesn't fill the log.
- Never logged: tokens, raw `Device.UUID`/serials, raw `IOPlatformUUID`. Decks and hosts appear as short hashes plus friendly names.

## Notifications

macOS Notification Center, with the tool's own icon.

- **Mechanism:** a tiny helper app, `SDSyncNotifier.app`: about 60 lines of Swift on `UserNotifications`, with its own bundle id and icon. `install.sh` builds it with `swiftc`, ad-hoc signs it, installs it to **`~/Applications/`** and registers it with LaunchServices. Verified on macOS 27: run from a temp dir, the helper is refused with no prompt (`UNErrorDomain 1`; LaunchServices cannot find it). From `~/Applications` it prompts once and then delivers banners with the custom icon. The first run asks the user to allow notifications once per Mac. A notification's icon belongs to the app that posts it, so this is the only supported way to get a custom icon. `osascript display notification` always shows Script Editor's icon, and `terminal-notifier`'s `-appIcon` depends on a private API that recent macOS ignores.
- **Fallback:** if the helper is missing (no Xcode/CLT on that Mac), fall back to `osascript`. The notification still arrives, with the generic icon.
- **Icon:** an *original* design: dark tile, 3×2 key grid with one accent key, and a circular sync badge (spike candidate "A", chosen 2026-10-01). It is generated from code at build time, so there is no binary to drift. It must **not** be Elgato's logo or app icon. This is a public repo, and borrowing their mark implies an affiliation that doesn't exist.
- **What notifies** (configurable in the common config, `notify.level`):

| Event | Default | Example |
|---|---|---|
| Pull starting (the deck is about to blank) | on | "Updating Stream Deck XL from <host>…" |
| Pull / push / rollback done | on | "Stream Deck XL updated (pushed by <host> 4 min ago)" |
| Diverged | on, persistent | "Stream Deck XL changed on two Macs. Run `sdsync resolve`." |
| Failed + restored | on, persistent | "Update failed; restored previous layout." |
| Missing plugin / script / shortcut after a pull | on | "2 buttons need things this Mac lacks. Run `sdsync inventory`." |
| Held deck skipped | once per hold | |
| InSync | never | |

- Clicking a notification opens the log in Console (for errors) or does nothing (for info).

## Inventory and scripts

Each run writes `inventory/<host>.json` and reports differences from the deck's requirements:

| Category | Detected from | Check | Replication |
|---|---|---|---|
| Plugins | action `UUID` prefixes vs `Plugins/*.sdPlugin` | installed? version? | report only |
| Icon packs | `IconPacks/*.sdIconPack` | present? | copy via store (about 70 MB, only when changed) |
| File scripts/apps | `system.open` `path` | exists after `~` expansion? executable? | per the common config's `script replication map`: `managed-elsewhere` (dotfiles repo etc., report only) or `store` (copied from `<store>/scripts/`) |
| Shortcuts | `shortcut.run` `shortcutName` | listed by `shortcuts list`? | report only (iCloud syncs Shortcuts) |
| BetterTouchTool | `com.folivora.btt.action` `btt_identifier` | report the id | report only (BTT owns its config) |
| Other plugin-backed | everything else | plugin present | report only |

Replicating scripts through a cloud folder means **running code that arrived from a sync service**. It is off by default, opted in per path in the common config, and copied files keep a hash that is checked before each run.

## Safety rails

- **Schema guard:** refuse to run if the app version or the manifest key set differs from the known set, until `sdsync doctor` passes and the user confirms.
- **Dry run** builds everything in a scratch dir and writes nothing beside the target (not even mtimes).
- Never delete a snapshot or local history entry automatically except by count-based rotation (`retention.local` / `retention.shared`).

## Testing

- Fixture trees built from redacted real manifests, including a **known-bad** pair for every check: a launch-only rewrite must stay InSync; a real button edit must flip to Ahead; a wrong `$HOME` must fail the path check.
- Store tests for a half-synced tree (`current.json` hash mismatch → in flight, no pull).
- What these tests **cannot** catch: Elgato changing what the app rewrites at runtime. Only the schema guard and a live `doctor` run on each new app version cover that.

## Open questions

1. **Is `Device.UUID` the same on every Mac for the same physical deck?** It looks derived from the serial, which would make it portable, but this has not been checked on a second Mac. If it is host-specific, the deck key becomes `Model + serial` read from IOKit.
2. **Can you edit a deck's profile while that deck is not attached?** If yes, "only the attached Mac pushes" would block real edits, and pushes should be gated on L ≠ B alone.
3. ~~License~~: MIT (decided 2026-10-01).
4. ~~Ad-hoc-signed notifier on macOS 27~~: works when installed in `~/Applications` (spike, 2026-10-01).
