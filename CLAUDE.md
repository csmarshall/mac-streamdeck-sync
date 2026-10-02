# schrodeck — rules for agents

## Golden rules

- **PUBLIC REPO.** Never commit hostnames, usernames, home paths of real machines, device serials/`Device.UUID` values, plugin tokens, or real profile manifests. Fixtures are redacted by hand. Use `<user>`, `<host>`, `<deck>` placeholders in docs. Run the leak scan before every push (`LICENSE` excluded).
- **Shared tool vs local overlay.** Repo = the tool, templates, docs. Host identity is derived (hash of IOPlatformUUID + username), never configured. The only per-host config is the store path pointer in `~/.config/schrodeck/config.toml`. Every other setting lives once in `<store>/config.toml`, shared by all hosts. Runtime state lives in `~/Library/Application Support/schrodeck/`. None of it goes in the repo.
- **Never trust mtime or clocks** for direction. The Stream Deck app rewrites manifests on launch. Direction comes from normalized hashes over the store's revision graph (ADR 0005). Timestamps are display-only metadata.
- **Terminology:** a **revision** is schrodeck's own version record in the store (`revisions/<id>/`). **"Commit" only ever means git.** Never use "commit" for store records in docs, code identifiers, CLI output or logs.
- **No store file has two writers** (contract D, `docs/contracts/store-format.md`): write-once trees/revisions, per-copy heads (`heads/<host_id>/<copy_id>.json`) written last. Never add a shared mutable file to the store. `config.toml` is the only exception.
- **Fail closed (ADR 0030):** any change outside schrodeck's expectations makes that Mac drop out of that setup, DETACHED(reason): profile never destroyed (left as-is; a rejoin onto the same deck archives it as `<name>-<datestamp>` and says so), nothing synced for it, one notification, persists until `schrodeck resolve`. Never add a silent fallback or an automatic recovery without its own ADR. Deletes never propagate; a failed verify ends in BLOCKED, never a retry loop; alerts are deduplicated (ADRs 0025, 0008, 0016).
- **Setups are seeded from templates (ADR 0029):** schrodeck only ever creates new member copies (`schrodeck - <cols>x<rows> - <name>`). It never modifies a template or any profile that isn't a member copy, and joiners never bring their own config into a setup.
- **Never touch ProfilesV3 while the app is running.** Plan → journal → quit → **re-check after quit** → snapshot → swap → relaunch → verify (ADR 0008).
- **Never read or write the selected profile (`ESDProfilesPreferred`) for sync** (ADR 0019). Profiles that aren't member copies (templates included) are never touched (ADRs 0004, 0029).
- **Stop, don't guess.** Unknown app version, profile `Version` or manifest key/file fingerprint → pause pushes **and** applies until `schrodeck doctor` passes (ADR 0015).
- Every check gets a known-bad fixture that makes it fail, as well as a known-good one.
- **The CLI's `--json` output is the contract for any UI.** UIs (e.g. a future SwiftUI menu-bar app) call the CLI. They never reimplement sync logic.
- **MPL-2.0 Exhibit A header on every source file.** Outside code contributions need a CLA (see CONTRIBUTING.md).
- Design sources of truth: `docs/adr/` (decisions) and `docs/specs/2026-10-01-sync-design.md` (narrative). Facts about the app are cited from `docs/references.md` as documented vs observed. Diagrams render with `mmdc` and are looked at before revision; `direction TB` only.

- "Never written" properties are asserted at the filesystem-port level, not by before/after byte comparison (the running app rewrites its own files).
- **Contracts (ADR 0024):** five documented contracts in `docs/contracts/`. For ours (A connector, D store format, E `--json`), the code and the doc change in the same PR and breaking changes bump the version. For Elgato's (B client per OS, C profile format), rows change only with evidence: a passing `schrodeck doctor` or a cited source.

## Workflow

1. Issue first (`gh issue create`).
2. Branch in a worktree under `~/work/claude/schrodeck-worktrees/<n>-slug`.
3. PR with `Closes #n`; commits `fix(#n): …` / `feat(#n): …`.
4. CI (GitHub Actions) must be green before merge: Go core on Linux (gofmt check, go vet, staticcheck, go test); macOS adapters and the Swift notifier on macOS runners.
5. Code-review subagent on the diff, then human review.
6. Squash-merge. Releases ship through the release PR and the Homebrew tap (ADR 0028); nothing is deployed by hand. To dogfood an unreleased build on a Mac, build and install locally (`brew install --build-from-source` from a local tap checkout, or the repo's documented dev build), then `schrodeck doctor`. Packages never install the LaunchAgent; `init`/`join` do.

## Toolchain

- **Go** core and CLI (ADR 0018): all sync logic behind OS ports. The port list lives ONLY in `docs/contracts/os-connector.md` (contract A); never restate it or its count elsewhere. macOS adapters now; Windows is a future adapter set. Core tests use fake adapters and run on Linux. Tools: gofmt, go vet, staticcheck, go test.
- **Swift** only at the edges: `SchrodeckNotifier.app` (UserNotifications, built and ad-hoc signed by the Homebrew formula's source build, then copied into `~/Applications` by `init`/`join`, which is required for notifications to work), and later a SwiftUI menu-bar app.
