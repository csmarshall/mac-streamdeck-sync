# 0013. Sync scope: profiles and icon packs; scripts are inventoried, replicated only by opt-in

Status: Accepted 2026-10-01 Revised 2026-10-01 (review F46): cross-profile references are an inventory dependency.

## Context

Buttons depend on things outside the profile: icon packs, scripts and apps opened by path ([R16](../references.md)), macOS Shortcuts by name, BetterTouchTool triggers by opaque id, and plugins ([0014](0014-plugin-handling.md)). Observed on one Mac: the `Open` buttons pointed at another host's home directory and were broken.

## Decision

- **Replicated:** shared profiles ([0004](0004-shared-profiles-and-subscriptions.md)) and icon packs. Icon packs are copied via the store only when they change.
- **Inventoried per host** (`<store>/inventory/<host_id>.json`, `schrodeck inventory`):
  - file paths from `Open` actions, checked to exist and be executable after variable expansion;
  - Shortcuts, checked against `shortcuts list`;
  - BetterTouchTool trigger ids, reported only;
  - plugin-backed actions ([0014](0014-plugin-handling.md));
  - **cross-profile references** (review F46): buttons that switch to another profile by its folder UUID ([contract C](../contracts/profile-format.md) P8, likely, unverified). Receivers name folders by uuid5 ([0026](0026-profile-identity.md)), so a reference to another *shared* profile is rewritten on install to that profile's local folder on this host, when it is subscribed here. If the target isn't subscribed on this host (or isn't shared at all), the inventory warns and one deduplicated notification names the button: the button will do nothing until the target is subscribed.
- **Script replication is opt-in per path** in the common config: `managed-elsewhere` (report only, e.g. a dotfiles repo) or `store` (copied from `<store>/scripts/`). Replicated scripts carry a hash that is checked before they are installed.

## Consequences

- Good: users see exactly what a button needs and whether this host has it.
- Bad: inventory can only check existence, not that a script behaves the same on another host.
- Risk: replicating executables through a sync service means **running code that arrived over the network**. That is why it is off by default, per path, and hash-checked. A compromised store could still ship a malicious script that matches its own recorded hash, so the hash protects against corruption, not against a compromised store.

## Alternatives considered

- **Replicate all scripts:** silently runs synced code.
- **Ignore dependencies:** users find broken buttons only by pressing them.

## Verified by

No check yet; to be written in the plan:
- A missing-path fixture ⇒ an inventory warning.
- A switch-profile fixture whose target is shared and subscribed ⇒ the reference is rewritten to the local uuid5 folder; whose target isn't subscribed ⇒ one warning and one notification. Known-bad: with rewriting disabled, the subscribed case must report a dangling reference.
- A replicated script with a tampered byte ⇒ refused.

## References

- [R16](../references.md): absolute paths in `Open` (observed)
