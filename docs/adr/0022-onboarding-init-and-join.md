# 0022. Onboarding: `init` on the first host, `join` on every other host

Status: Accepted 2026-10-01

## Context

A sync group starts on one host and grows. Joining must never surprise anyone. The second host may already have its own profiles, different plugins, and paths that need variable values. And joining is the first time schrodeck rewrites that host's Stream Deck files.

## Decision

**`schrodeck init`** on the first host:

1. Preflight: the Stream Deck app is installed and its config is readable (profiles directory, prefs, app version passes the schema guard, ADR [0015](0015-schema-guard.md)). If not, fail with a plain message saying what is missing. Never create a half-configured group.
2. Choose the shared directory. schrodeck suggests candidates (Dropbox, then iCloud Drive, ADR [0010](0010-host-identity-and-config-layering.md)) and writes `FORMAT`, the common config and this host's registry entry.
3. Choose the profile(s) to share (ADR [0004](0004-shared-profiles-and-subscriptions.md)).
4. **Naming suggestion:** offer to rename each shared profile to `schrodeck · <name> · <cols>×<rows>` (e.g. `schrodeck · Work · 8×4`), so the Stream Deck app shows at a glance which profiles are synced and for which geometry. This is a suggestion only. Identity is schrodeck's own profile id, so a later rename is an ordinary synced edit and breaks nothing.
5. First push, then a summary.

**`schrodeck join <dir>`** on each additional host:

1. Same preflight. Also refuse if `<dir>` has no store `FORMAT` file or an unknown format version, or if its freshness check fails (ADR [0023](0023-store-freshness-via-file-provider.md)).
2. **Dry run, nothing touched:** print a rundown of exactly what accepting would do:
   - which shared profiles are available, and which local deck each would land on (geometry match);
   - local profiles that would be replaced, versus those left alone (unshared profiles are never touched);
   - the restore point that will be taken first;
   - missing plugins, scripts and Shortcuts;
   - variables that need a value on this host.
3. Ask for confirmation. `--yes` is for scripted installs; `--json` prints the rundown for a UI.
4. **On accept, the first sync runs synchronously in the foreground** through the normal two-phase apply (ADR [0008](0008-two-phase-apply.md)), with progress output. The background agent (ADR [0012](0012-triggers.md)) is installed only **after** the first sync succeeds. If it fails, the host is not left half-joined: everything rolls back, and the host is not registered.

## Consequences

- Good: the riskiest moment, a host's first apply, happens while the user is watching, after a full preview.
- Good: a failed join leaves no background agent fighting a broken state.
- Bad: `join` is interactive by default. Fleet installs need `--yes`, and then the preview is only in the log.
- The naming suggestion makes synced profiles visible in Elgato's UI. Users who decline it lose that cue and nothing else.

## Alternatives considered

- **Join silently and let the background agent do the first sync:** rejected. The first apply rewrites the app's files on a host whose state we have never seen, so it deserves a preview and a watching human.
- **Identity by profile name** (e.g. anything named `schrodeck · …` is synced): rejected. Renames would break sync, and a user could accidentally opt in by naming a profile. The name stays a visual cue only.
- **`join` without a directory, by auto-discovery:** rejected as the default. Two candidate stores are ambiguous (ADR 0010). Discovery only *suggests*.

## Verified by

No check yet; to be written in the plan. Required tests:
- `join` in dry-run mode leaves the target tree byte-identical, mtimes included (known-bad: a dry run that stages beside the target must fail the test).
- A join whose first apply fails leaves no LaunchAgent installed and no registry entry.
- Preflight fails cleanly when the app is absent.

## References

- Close the app before changing its files: [R3](../references.md) (documented).
- Profiles are device-specific, hence geometry matching: [R8, R10](../references.md) (documented).
