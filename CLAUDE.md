# schrodeck — rules for agents

## Golden rules

- **PUBLIC REPO.** Never commit hostnames, usernames, home paths of real machines, device serials/`Device.UUID` values, plugin tokens, or real profile manifests. Fixtures are redacted by hand. Use `<user>`, `<host>`, `<deck>` placeholders in docs. Run the leak scan before every push (`LICENSE` excluded).
- **Shared tool vs local overlay.** Repo = the tool, templates, docs. Host identity is derived (hash of IOPlatformUUID + username), never configured. The only per-host config is the store path pointer in `~/.config/schrodeck/config.toml`. Every other setting lives once in `<store>/config.toml`, shared by all hosts. Runtime state lives in `~/Library/Application Support/schrodeck/`. None of it goes in the repo.
- **Never trust mtime or clocks** for direction. The Stream Deck app rewrites manifests on launch. Direction comes from the 3-way normalized-hash compare (ADR 0005). Timestamps are display-only metadata.
- **Never touch ProfilesV3 while the app is running.** Plan → journal → quit → swap → relaunch → verify, with a verified rollback snapshot (ADR 0008).
- **Never read or write the selected profile (`ESDProfilesPreferred`) for sync** (ADR 0019). Unshared profiles are never touched (ADR 0004).
- **Stop, don't guess.** Unknown app version, profile `Version` or manifest key fingerprint → pause applies until `schrodeck doctor` passes (ADR 0015).
- Every check gets a known-bad fixture that makes it fail, as well as a known-good one.
- **The CLI's `--json` output is the contract for any UI.** UIs (e.g. a future SwiftUI menu-bar app) call the CLI. They never reimplement sync logic.
- **MPL-2.0 Exhibit A header on every source file.** Outside code contributions need a CLA (see CONTRIBUTING.md).
- Design sources of truth: `docs/adr/` (decisions) and `docs/specs/2026-10-01-sync-design.md` (narrative). Facts about the app are cited from `docs/references.md` as documented vs observed. Diagrams render with `mmdc` and are looked at before commit; `direction TB` only.

## Workflow

1. Issue first (`gh issue create`).
2. Branch in a worktree under `~/work/claude/schrodeck-worktrees/<n>-slug`.
3. PR with `Closes #n`; commits `fix(#n): …` / `feat(#n): …`.
4. CI (GitHub Actions) must be green before merge: Go core on Linux (gofmt check, go vet, staticcheck, go test); macOS adapters and the Swift notifier on macOS runners.
5. Code-review subagent on the diff, then human review.
6. Squash-merge. Deploy deliberately with `./install.sh` on each Mac. Use `./install.sh --check` for drift.

## Toolchain

- **Go** core and CLI (ADR 0018): all sync logic behind six OS ports (AppControl, DeviceEnumerator, Watcher, Notifier, AppPrefs, Scheduler). macOS adapters now; Windows is a future adapter set. Core tests use fake adapters and run on Linux. Tools: gofmt, go vet, staticcheck, go test.
- **Swift** only at the edges: `SchrodeckNotifier.app` (UserNotifications, built + ad-hoc signed by `install.sh`, installed into `~/Applications`, which is required for notifications to work), and later a SwiftUI menu-bar app.
