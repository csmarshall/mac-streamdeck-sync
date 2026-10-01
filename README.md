# mac-streamdeck-sync

Keep Elgato Stream Deck profiles in sync across several Macs that share the same deck(s), for example through a Thunderbolt or KVM switch, using nothing but a shared cloud folder (Dropbox, iCloud Drive, …).

> **Status: design phase.** No working code yet. See the [design spec](docs/specs/2026-10-01-sync-design.md).

## The problem

You edit a profile on Mac A, flip the switch, and Mac B shows yesterday's layout. The Stream Deck app keeps profiles per machine and has no automatic cross-machine sync.

## The approach

- A small CLI (`sdsync`) runs on every Mac from a LaunchAgent. It triggers when the deck is attached, on a timer, or by hand.
- For each deck, it compares **normalized content hashes** of the local profiles, the shared copy, and the last state this Mac synced. That tells it whether this Mac is behind, ahead, in sync, or diverged without trusting file timestamps or clocks. The app rewrites its own files on every launch, so timestamps are useless here.
- Behind → quit the app, swap in the shared profiles, relaunch. Ahead → publish atomically to the shared folder. Diverged → keep both, touch nothing, notify.
- Paths under your home folder are made portable, because usernames differ between Macs. It also reports which plugins, icon packs, Shortcuts, and scripts each deck needs and whether this Mac has them.
- No network connection between the Macs is needed.

![sync state diagram](docs/sync-states.png)

## Prior art

- [dominik-ba/stream-deck-profile-sync](https://github.com/dominik-ba/stream-deck-profile-sync): manual push/pull via a cloud folder. A good fit if you want to stay in control of when syncs happen.
- [Elgato: deploying profiles at scale](https://www.elgato.com/us/en/explorer/products/stream-deck/stream-deck-profiles-at-scale/). Note that Elgato does not officially support managing profile files directly. This tool therefore checks the app version and profile schema, and refuses to run on anything it doesn't recognize.

## Caveats

This works on the Stream Deck app's internal files, which Elgato may change in any update. It keeps backups before every change and is designed to stop rather than guess. Hopefully that is enough, but treat it as early software.

Human-directed and AI-assisted: the design and every change are reviewed and steered by a human.
