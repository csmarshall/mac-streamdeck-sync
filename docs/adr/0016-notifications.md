# 0016. Notifications come from our own helper app with an original icon

Status: Accepted 2026-10-01

## Context

The tool runs unattended and must tell the user when it restarts the app, fails, or needs a decision. On macOS, a notification's icon belongs to the app that posts it. `osascript display notification` always shows Script Editor's icon.

Spike on macOS 27, 2026-10-01, with an ad-hoc-signed Swift `UserNotifications` helper:
- From a temp directory it was refused with no prompt (`UNErrorDomain` code 1; LaunchServices could not resolve the app, -10814).
- From `~/Applications` it prompted once, then delivered banners with its custom icon.

## Decision

- **Mechanism:** a small Swift helper app, built and ad-hoc signed at install time, installed into `~/Applications` and registered with LaunchServices.
- **Fallback:** `osascript` (generic icon) when the helper is unavailable.
- **Icon:** an **original** design (a dark tile with a key grid, one accent key, and a circular sync badge). It is generated from code and uses no Elgato marks ([R19](../references.md)).

| Event | Default |
|---|---|
| Apply starting (deck about to blank) | on |
| Apply / push / rollback done | on |
| Diverged | on, persistent |
| Failed (+ rolled back) | on, persistent |
| Missing plugin / script / Shortcut | on |
| Held profile skipped | once per hold |
| New shared profile available | on |
| InSync | never |

## Consequences

- Good: clearly identifiable notifications, and native Notification Center behavior.
- Bad: the user must allow notifications once per host. Building needs a Swift toolchain.
- Risk: the spike was one run on one machine. Behavior on other macOS versions is projected, not verified.

## Alternatives considered

- **osascript only:** wrong icon. Notifications look like they come from Script Editor.
- **terminal-notifier:** its custom-icon option relies on behavior recent macOS no longer honors (3.1.0 is the current Homebrew version).
- **Reusing Elgato's icon:** implies affiliation; trademarked ([R19](../references.md)).

## Verified by

The spike (throwaway) showed delivery with the custom icon from `~/Applications` and refusal from a temp dir. A permanent check is still to be written: an install test asserting the helper's path is under an Applications directory, and a manual first-run checklist item.

## References

- [R19](../references.md): branding guidelines (documented)
