# 0022. Onboarding: `init` on the first host, `join` on every other host

Status: Accepted 2026-10-01. Revised 2026-10-01 (review F13, F16): join never replaces existing local profiles, subscribing is chosen in the rundown, the rename suggestion and the agent install move to the milestones that can do them safely.

## Context

A sync group starts on one host and grows. Joining must never surprise anyone. The second host may already have its own profiles, different plugins, and paths that need variable values. Joining is also the first time schrodeck writes that host's Stream Deck files.

## Decision

**`schrodeck init`** on the first host:

1. Preflight: the Stream Deck app is installed and its config is readable (profiles directory, prefs, an app version that passes the schema guard, ADR [0015](0015-schema-guard.md)). If not, fail with a plain message saying what is missing. Never create a half-configured store.
2. Choose the shared directory. schrodeck suggests candidates (Dropbox, then iCloud Drive, ADR [0010](0010-host-identity-and-config-layering.md)). It writes `FORMAT`, the common config and this host's `hosts/<host_id>.toml`.
3. Choose the profile(s) to share (ADR [0004](0004-shared-profiles-and-subscriptions.md)). Each gets a `profile_id` (ADR [0026](0026-profile-identity.md)). The first push follows. Only after it succeeds, and from M4 on, the background agent (ADR [0012](0012-triggers.md)) is installed. The package never installs it ([0028](0028-distribution-brew-tap-and-pkg.md)).
4. **Naming suggestion** (from milestone M3, because renaming writes a manifest and goes through the two-phase apply, ADR [0008](0008-two-phase-apply.md)): offer to rename each shared profile to `schrodeck · <name> · <cols>×<rows>` (e.g. `schrodeck · Work · 8×4`), so the Stream Deck app shows at a glance which profiles are synced and for which geometry. It is a suggestion only. Identity is the `profile_id`, so a later rename is an ordinary synced edit.

**`schrodeck join <dir>`** on each additional host (M3):

1. Same preflight. Also refuse if `<dir>` has no store `FORMAT` file or has an unknown/newer one, or if its freshness check fails (ADR [0023](0023-store-freshness-via-file-provider.md)).
2. **Dry run, nothing touched.** Print a rundown of exactly what accepting would do:
   - the shared profiles available, with their geometry;
   - for each one the user **selects** to subscribe, which local deck it would land on. If two local decks match, the user picks one; `--deck` is required for non-interactive use. Join subscribes to nothing by default;
   - that each subscription installs a **new** copy, and that **no existing local profile is replaced or modified**;
   - the restore point that will be taken first;
   - missing plugins, scripts and Shortcuts;
   - variables that need a value on this host.
3. Ask for confirmation. `--yes` is for scripted installs, and requires explicit `--subscribe`/`--deck` choices. `--json` prints the rundown for a UI.
4. **On accept, the first sync runs synchronously in the foreground** through the normal two-phase apply (ADR [0008](0008-two-phase-apply.md)), with progress output.
5. **Only after it succeeds:**
   - this host's `hosts/<host_id>.toml` is written;
   - from M4 on, the background agent (ADR [0012](0012-triggers.md)) is installed.

   If the first sync fails, everything rolls back, no head or host file is written, and no agent is installed.

## Consequences

- Good: the riskiest moment, a host's first apply, happens while the user is watching, after a full preview.
- Good: a failed join leaves no background agent fighting a broken state, and no trace in the store.
- Bad: `join` is interactive by default. Fleet installs need `--yes` with explicit choices, and then the preview is only in the log.
- The naming suggestion makes synced profiles visible in Elgato's UI. Users who decline it lose that cue and nothing else.

## Alternatives considered

- **Join silently and let the background agent do the first sync:** the first apply rewrites the app's files on a host whose state we have never seen, so it deserves a preview and a watching human.
- **Auto-subscribe every compatible profile:** surprising, and it can fill decks with profiles the user didn't want.
- **Adopt an existing local profile as the subscribed copy:** would need a content comparison and risks overwriting local work. Deferred; installing a new copy is always safe.
- **Identity by profile name** (e.g. anything named `schrodeck · …` is synced): renames would break sync, and a user could opt in by accident. The name stays a visual cue only.
- **`join` without a directory, by auto-discovery:** two candidate stores are ambiguous (ADR 0010). Discovery only *suggests*.

## Verified by

No check yet; to be written in the plan. Required tests:
- Dry-run `join`: **no file under the app's data root or the store is opened for writing**, asserted at the filesystem-port level. Byte comparison of the app's files would be fooled by the running app's own rewrites. Known-bad: a dry run that stages beside the target must fail.
- A join whose first apply fails leaves no LaunchAgent, no `hosts/<host_id>.toml`, and no head.
- Preflight fails cleanly when the app is absent.
- With an existing local profile of the same name, join's subscribe creates a new copy and leaves the existing one untouched.

## References

- Close the app before changing its files: [R3](../references.md) (documented).
- Profiles are device-specific, hence geometry matching: [R8, R10](../references.md) (documented).
