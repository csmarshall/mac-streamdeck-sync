# mac-streamdeck-sync — rules for agents

## Golden rules

- **PUBLIC REPO.** Never commit hostnames, usernames, home paths of real machines, device serials/`Device.UUID` values, plugin tokens, or real profile manifests. Fixtures are redacted by hand. Use `<user>`, `<host>`, `<deck>` placeholders in docs.
- **Shared tool vs local overlay.** Repo = the tool, templates, docs. Host identity is derived (hash of IOPlatformUUID + username), never configured. The only per-host config is the store path pointer in `~/.config/mac-streamdeck-sync/config.toml`. Every other setting lives once in `<store>/config.toml`, shared by all hosts. Runtime state lives in `~/Library/Application Support/mac-streamdeck-sync/`. None of it goes in the repo.
- **Never trust mtime** for direction. The Stream Deck app rewrites manifests on launch. Direction comes from the 3-way normalized-hash compare in the spec.
- **Never touch ProfilesV3 while the app is running.** Quit → swap → relaunch, with a pre-swap backup.
- **Stop, don't guess.** Unknown app version or manifest schema → refuse, until `sdsync doctor` passes.
- Every check gets a known-bad fixture that makes it fail, as well as a known-good one.
- Design source of truth: `docs/specs/2026-10-01-sync-design.md`. Diagrams render with `mmdc` and are looked at before commit; `direction TB` only.

## Workflow

1. Issue first (`gh issue create`).
2. Branch in a worktree under `~/work/claude/mac-streamdeck-sync-worktrees/<n>-slug`.
3. PR with `Closes #n`; commits `fix(#n): …` / `feat(#n): …`.
4. CI (GitHub Actions: ruff, mypy, pytest) must be green before merge.
5. Code-review subagent on the diff, then human review.
6. Squash-merge. Deploy deliberately with `./install.sh` on each Mac. Use `./install.sh --check` for drift.

## Toolchain

Python 3.9+ (the macOS CommandLineTools interpreter, so a stock Mac can run it with no installs). Standard library only at runtime. Ruff, mypy, pytest for development. Configured in `pyproject.toml`.
