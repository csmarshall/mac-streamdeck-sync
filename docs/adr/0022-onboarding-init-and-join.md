# 0022. Onboarding: `init` creates a setup from a template; `join` adds a new member copy

Status: Accepted 2026-10-01. Revised 2026-10-01 (review F13, F16): join never replaces existing local profiles, subscribing is chosen in the rundown, and the agent install moves to the milestone that can do it safely. Revised 2026-10-01 (reframe: template-seeded setups): `init` picks a device and a template and creates a new named member copy; `join` lists only geometry-compatible setups and always creates a new profile on a chosen destination device; the old rename suggestion is replaced by the member copy's fixed name. Revised 2026-10-01 (review F50, F52, F53; owner's decisions): `init`/`share` **publish first** (root built read-only from the template) and then install on the first Mac through the ordinary install path; a re-subscribe onto a deck that still holds an old copy **archives** it and says so in the rundown. Revised 2026-10-02 (issue #5, owner's decision): `subscribe` may add a second (or further) member copy of a setup on the **same** Mac, onto another compatible deck; each copy is a full peer with its own head.

## Context

A setup starts on one Mac and grows ([0029](0029-problem-statement-and-setup-model.md)). Joining must never surprise anyone. The second Mac may already have its own profiles, different plugins, and paths that need variable values. Onboarding is also the first time schrodeck writes that Mac's Stream Deck files.

## Decision

```mermaid
stateDiagram-v2
    direction TB
    [*] --> Preflight1: Mac 1 runs schrodeck init
    Preflight1 --> PickTemplate: app installed, config readable,<br/>schema guard passes
    PickTemplate --> BuildRoot: user picks a device and a template profile,<br/>names the setup
    BuildRoot --> Publish: root tree built read-only in staging,<br/>Name set to schrodeck - 8x4 - Work
    Publish --> InstallHere: root revision pushed,<br/>template never opened for writing
    InstallHere --> Preflight2: ordinary install apply on Mac 1,<br/>ActionIDs regenerated
    Preflight2 --> SetupList: Mac 2 runs schrodeck join dir,<br/>store FORMAT known and fresh
    SetupList --> Rundown: user picks a setup and a destination device<br/>from setups whose geometry matches
    Rundown --> Install: dry run shows every change<br/>and any archive, user confirms
    Install --> AgentInstalled: new member profile, first sync<br/>succeeds in the foreground
    Install --> RolledBack: failure, nothing kept,<br/>no host file, no agent
    AgentInstalled --> [*]
    RolledBack --> [*]
```

**`schrodeck init`** on the first Mac:

1. **Preflight:** the Stream Deck app is installed and its config is readable (profiles directory, prefs, a fingerprint that passes the schema guard, ADR [0015](0015-schema-guard.md)). If not, fail with a plain message saying what is missing. Never create a half-configured store.
2. **Choose the shared directory.** schrodeck suggests candidates (Dropbox, then iCloud Drive, ADR [0010](0010-host-identity-and-config-layering.md)) and writes `FORMAT`, the common config and this Mac's `hosts/<host_id>.toml`.
3. **Choose the template:** schrodeck lists this Mac's devices, then the profiles on the chosen device. The user picks a device, a template profile, and a setup name.
4. **Publish:** build the setup's root tree **read-only** from the template in staging, with the profile `Name` set to `schrodeck - <cols>x<rows> - <name>` (e.g. `schrodeck - 8x4 - Work`), assign a new `profile_id` ([0026](0026-profile-identity.md)), and push the root revision. **Nothing under the app's data root is opened for writing**, so this step needs no app restart. Its dry run shows the root tree and the target store.
5. **Install on this Mac** through the ordinary install apply ([0008](0008-two-phase-apply.md)), exactly as a joining Mac would ([0029](0029-problem-statement-and-setup-model.md)): a **new** member profile on the chosen device, `ActionID`s regenerated ([0026](0026-profile-identity.md)), with the same dry-run rundown and confirmation as `join`. Only after it succeeds, and from M4 on, is the background agent ([0012](0012-triggers.md)) installed. The package never installs it ([0028](0028-distribution-brew-tap-and-pkg.md)).

`schrodeck share` on any later Mac runs steps 3 to 5 to add another setup.

**`schrodeck join <dir>`** on each additional Mac:

1. **Preflight**, as above. Also refuse if `<dir>` has no store `FORMAT` file, or has an unknown or newer one, or if its freshness check fails (ADR [0023](0023-store-freshness-via-file-provider.md)).
2. **List the setups** in the store, **filtered to those whose geometry matches a device on this Mac**. Setups with no compatible local device are listed separately as "no compatible deck here", for information only.
3. **The user picks** a setup and a **destination device**. If more than one local device matches, the user chooses; `--deck` is required for non-interactive use. Join subscribes to nothing by default.
4. **Dry run, nothing touched.** Print a rundown of exactly what accepting would do:
   - for each chosen setup, the **new** profile that will be created, its name, and its destination device;
   - that **no existing local profile is replaced or modified**, and that this Mac's existing configuration is **not** brought into any setup;
   - **if the destination device already holds a copy of this setup** (a DETACHED copy, or one left behind by an earlier `unmap`): that it will be **archived**, i.e. moved to its own folder and renamed `<name>-<YYYY-MM-DD-HHMM>`, and kept. If it is currently the deck's selected profile, the app will select another one after the restart. Cross-profile buttons that pointed at it will point at the archive (listed). See [0026](0026-profile-identity.md);
   - the restore point that will be taken first;
   - missing plugins, scripts and Shortcuts;
   - variables that need a value on this Mac.
5. **Ask for confirmation.** `--yes` is for scripted installs and requires explicit setup and `--deck` choices. `--json` prints the rundown for a UI.
6. **On accept, the first sync runs synchronously in the foreground** through the normal two-phase apply ([0008](0008-two-phase-apply.md)), with progress output.
7. **Only after it succeeds:**
   - this Mac's `hosts/<host_id>.toml` is written;
   - from M4 on, the background agent ([0012](0012-triggers.md)) is installed.

   If the first sync fails, everything rolls back, no head or host file is written, and no agent is installed.

`schrodeck subscribe` later on a joined Mac runs steps 2 to 6 for one more setup, **or for a setup this Mac already holds, onto another compatible deck** (issue #5). Example: `init` makes the root from a profile on deck 1, then `schrodeck subscribe <setup> --deck <deck 2>` adds deck 2. Deck 2's copy is a **full peer**, not a mirror (owner's decision): an edit on it syncs to deck 1 and to every other Mac, and it has its own head ([contract D](../contracts/store-format.md)). A deck that already holds a live member copy of the setup is not offered as a destination; a deck holding a detached or unmapped one is offered, with the archive step in the rundown.

**Joiners never bring their own config into the system.** Merging two computers' existing configurations is out of scope. It can be done by hand (quit the Stream Deck app and edit the profile files, or copy buttons in the app), at the user's own risk. schrodeck won't manage it, and the documentation says so.

## Consequences

- Good: the riskiest moment, a Mac's first apply, happens while the user is watching, after a full preview.
- Good: a failed join leaves no background agent fighting a broken state, and no trace in the store.
- Good: no onboarding path can overwrite a profile the user already had. schrodeck only creates profiles.
- Good: the fixed member-copy name makes synced profiles visible in Elgato's UI at a glance.
- Bad: `join` is interactive by default. Fleet installs need `--yes` with explicit choices, and then the preview is only in the log.
- Bad: a user who wants their current profile synced gets a copy next to it, and must delete the original by hand if they want.

## Alternatives considered

- **Join silently and let the background agent do the first sync:** the first apply rewrites the app's files on a Mac whose state we have never seen, so it deserves a preview and a watching human.
- **Auto-subscribe every compatible setup:** surprising, and it can fill decks with profiles the user didn't want.
- **Adopt or replace an existing local profile as the member copy:** risks overwriting local work and imports the joiner's config. Rejected: always a new profile.
- **Share the chosen profile in place, with an optional rename** (the previous version of this ADR): the user's own profile became a sync target. Replaced by template-seeded copies.
- **Identity by profile name** (anything named `schrodeck - …` is synced): renames would break sync, and a user could opt in by accident. The name stays a visual cue only.
- **`join` without a directory, by auto-discovery:** two candidate stores are ambiguous (ADR 0010). Discovery only *suggests*.

## Verified by

No check yet; to be written in the plan. Required tests:
- `init`: the template is never opened for writing (filesystem-port assertion), and the publish step opens nothing under the app's data root for writing. After install, exactly one new profile exists, with the expected name; its normalized hash equals the template's **with the template's `Name` overridden to the setup name** (review F53). Known-bad: an unrenamed copy must fail, and so must a comparison without the override.
- Re-subscribe onto a deck holding an old copy of the setup ⇒ the rundown names the archive; after accepting, the old copy exists, byte-identical apart from its folder and `Name`, and one notification names it.
- Dry-run `init`/`join`: **no file under the app's data root or the store is opened for writing**, asserted at the filesystem-port level. Byte comparison of the app's files would be fooled by the running app's own rewrites. Known-bad: a dry run that stages beside the target must fail.
- `join`'s setup list contains only geometry-compatible setups (known-bad: a fixture of another geometry must not be offered as a destination).
- A join whose first apply fails leaves no LaunchAgent, no `hosts/<host_id>.toml`, and no head.
- Preflight fails cleanly when the app is absent.
- With an existing local profile of the same name on the destination, join creates a new profile and leaves the existing one untouched.
- `subscribe` of a setup this Mac already holds, onto a second compatible deck ⇒ a second member copy in its own canonical folder, a second head in the store, and no write-open on the first copy. An edit on the second copy reaches the first copy and other Macs. Known-bad: the deck already holding a live copy must not be offered.

## References

- Close the app before changing its files: [R3](../references.md) (documented).
- Profiles are device-specific, hence geometry matching: [R8, R10](../references.md) (documented).
