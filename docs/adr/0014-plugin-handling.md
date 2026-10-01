# 0014. Plugins: settings split, report don't install, warn on skew

Status: Accepted 2026-10-01

## Context

Elgato splits plugin configuration ([R7](../references.md), documented):
- **Action settings:** per button, plain text, included in profile exports.
- **Global settings:** "stored securely on the user's local machine".

Exports exclude plugin binaries ([R6](../references.md)). The only documented install path is opening a `.streamDeckPlugin` file, which prompts in the app ([R5](../references.md), [R17](../references.md)). There is no Marketplace install API.

Observed on one Mac (22 plugins):
- 15 are HTML/JS (portable; the app ships Node).
- 5 contain universal (x86_64 + arm64) binaries.
- 1 is arm64-only.
- 13 have encrypted manifests (Marketplace distribution).

## Decision

- **Action settings** travel with the shared profile and take part in its hash and conflict handling.
- **Global settings are never synced.** They are deliberately allowed to differ per host, e.g. a different service URL for a firewalled host. Per-host values inside *action* settings use variables ([0006](0006-normalization-and-variables.md)).
- **Checks:** only "installed" and "version" can be checked, not "configured" (global settings are opaque).
- **Missing plugin:** apply anyway, then notify, naming the profile, the page, and the plugin.
- **Version skew between hosts:** warn only.
- **v1 does not install or copy plugins.** Copying plugin bundles is investigated in issue #4, with a `lipo` architecture check against the receiving host.

## Consequences

- Good: secrets never enter the store. Per-host plugin configuration is natural.
- Bad: a button can sync correctly yet fail on a host where the plugin is installed but unconfigured. schrodeck cannot detect that.
- Risk: a newer plugin may write action settings an older one misreads. We warn but cannot verify.

## Alternatives considered

- **Sync plugin directories:** encrypted bundles may be machine-bound (unknown, #4); architecture may not match; global settings would carry secrets.
- **Block an apply when a plugin is missing:** leaves the whole profile stale for one button.
- **Block on major-version skew:** rejected by the project owner in favor of a warning.

## Verified by

No check yet; to be written in the plan:
- A profile fixture referencing a plugin absent from the fake host ⇒ apply proceeds and a notification names the plugin.
- A test that the store never contains plugin directories or global settings files.

## References

- [R5](../references.md), [R17](../references.md): install only by opening a plugin file (documented / by absence)
- [R6](../references.md): exports exclude plugins (documented)
- [R7](../references.md): action vs global settings (documented)
