# 0018. Go core with OS ports; Swift only at the macOS edges

Status: Accepted 2026-10-01

## Context

The work is mostly sync and analysis: hashing, JSON canonicalization, store protocol, history, inventory. It is platform-neutral. The platform-specific part is small: controlling the app, enumerating decks, file watching, notifications, reading app preferences, and scheduling. A future UI is likely, and Windows support is plausible (Windows uses the same ProfilesV3 format under `%APPDATA%\Elgato\StreamDeck`, according to community sources; **unverified** here).

## Decision

- **Go** for all sync logic and the CLI. The CLI emits `--json` output, which is the contract for any UI.
- **OS ports** (interfaces) with one adapter set per OS, called a **connector**. The port list, signatures, invariants and filesystem guarantees live in exactly one place: [contract A, os-connector.md](../contracts/os-connector.md) (ADR [0024](0024-documented-contracts.md)). The macOS connector is the reference.

- **Swift only at the edges:** the notifier now, and a SwiftUI menu-bar app later that talks to the CLI.
- **Windows later** = a new adapter set. The core does not change.
- **Tests:** core tests run on Linux CI with fake adapters. Adapter tests run on macOS runners.

## Consequences

- Good: one implementation of the sync logic. A UI cannot become a second implementation, and Windows is additive.
- Good: single static binaries, no interpreter dependency under launchd.
- Bad: two toolchains (Go + Swift) in one repo.
- Risk: the port boundaries may be wrong for Windows in ways only a Windows adapter will reveal.

## Alternatives considered

- **Python 3.9:** no build step, but the Command Line Tools interpreter is unreliable in background/launchd shells on the owner's machines, and the notifier still needs Swift.
- **Swift-only:** native APIs everywhere and UI code sharing, but Windows would be a rewrite.
- **Rust:** strong correctness tooling and in-process access to macOS APIs via `objc2`, but a steeper curve and slower iteration for a small tool.

## Verified by

No check yet; to be written in the plan:
- CI with a Linux job running the core tests (no macOS dependency allowed in the core package; a build constraint or import check fails if one appears).
- A macOS job for the adapters.

## References

- None from Elgato.
