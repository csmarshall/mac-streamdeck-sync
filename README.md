# schrodeck

*Your Stream Deck profile is in superposition across every Mac it has been on, until you attach a deck and it collapses into the latest state.*

Give a Stream Deck the **same profile setup at every computer you sit at**, kept in sync automatically, including different physical decks with the same layout size, for example through a Thunderbolt or KVM switch, using nothing but a shared cloud folder (Dropbox, iCloud Drive, …).

> **Status: design phase.** No working code yet. See the [design spec](docs/specs/2026-10-01-sync-design.md) and the [architecture decision records](docs/adr/README.md).

## The problem

You have an XL at every desk (or one XL on a switch) and want the same buttons everywhere. You edit a profile on Mac A, and Mac B still shows yesterday's layout. The Stream Deck app keeps profiles per machine, has no automatic cross-machine sync, and has no notion of "the setup for all my XLs": every profile belongs to exactly one device. schrodeck adds that layer.

## The approach

- **Setups, seeded from a template.** On the first machine, `schrodeck init` lists your decks; you pick one and a profile on it as the **template**, and name the setup. schrodeck publishes it as the setup and installs it back as a **new** profile, e.g. `schrodeck - 8x4 - Work`. Your template is never modified.
- **Joining.** On another machine, `schrodeck join <shared folder>` lists only the setups that fit a deck you have (same columns × rows and dials), you pick a destination deck, and it creates a **new** profile there. It never replaces your existing profiles, and never pulls your other machine's config into the setup. From then on every copy is a peer: edit any of them and the others follow.
- **Fail closed.** If a synced profile changes in a way schrodeck doesn't expect (you deleted it, its deck disappeared, it suddenly references another deck…), that machine drops out of that setup, leaves the profile exactly as it is, tells you once, and waits for `schrodeck resolve`. Profiles that aren't part of a setup are never touched.
- A small CLI (`schrodeck`, written in Go) runs on every machine from launchd. It runs when profile files or the shared folder change, on a safety timer, or by hand. It can optionally also run on deck attach.
- Every version of a shared profile is an immutable revision in the shared folder, and each synced copy has its own small "head" file that only its machine writes, so no file ever has two writers. One machine can keep two same-size decks on the same setup; they sync like any two machines. Comparing **normalized content hashes** against that history tells each machine whether it is behind, ahead, in sync, or diverged, without trusting file timestamps or clocks. Two machines editing at once produce a visible fork, never a silently lost edit.
- Behind → plan and verify the change first, quit the app, re-check that nothing changed while it quit, swap in the update, relaunch, and verify. If verification fails, roll back once and stop updating that version until you look at it. Ahead → publish to the shared folder. Diverged → keep both versions, touch nothing, notify once.
- Per-machine differences such as your home folder, or a service URL that differs on a firewalled machine, are handled with variables, so they never count as edits. It also reports which plugins, icon packs, Shortcuts, and scripts a profile needs and whether this machine has them.
- Repeated applies back off exponentially, so a bug can't turn into a restart loop; your own edits are never delayed. Deletes never propagate: deleting a synced profile on one machine just makes that machine drop out of the setup. Alerts are sent once per problem, not once per run.
- No network connection between the machines is needed. macOS first; the OS-specific parts sit behind interfaces so other platforms can be added later.

![sync state diagram](docs/sync-states.png)

## Prior art

- [dominik-ba/stream-deck-profile-sync](https://github.com/dominik-ba/stream-deck-profile-sync): manual push/pull via a cloud folder. A good fit if you want to stay in control of when syncs happen.
- What Elgato documents vs. what we observed: [docs/references.md](docs/references.md)
- [Elgato: deploying profiles at scale](https://www.elgato.com/us/en/explorer/products/stream-deck/stream-deck-profiles-at-scale/). Note that Elgato does not officially support managing profile files directly. This tool therefore checks the app version and profile schema, and refuses to run on anything it doesn't recognize.

## Caveats

This works on the Stream Deck app's internal files, which Elgato may change in any update. It keeps backups before every change and is designed to stop rather than guess. Hopefully that is enough, but treat it as early software.

Not affiliated with or endorsed by Elgato or Corsair. "Elgato" and "Stream Deck" are trademarks of Corsair Memory, Inc.

Human-directed and AI-assisted: the design and every change are reviewed and steered by a human.

## License

[MPL-2.0](LICENSE). Contributions require a CLA; see [CONTRIBUTING.md](CONTRIBUTING.md).
