# 0020. Naming, license, and public-repo hygiene

Status: Accepted 2026-10-01

## Context

The repository is public. It is developed against real machines, whose hostnames, usernames, device serials and tokens must never be published. Elgato's branding guidance asks third parties to write "Stream Deck" as two words and not to imply official status ([R19](../references.md)). The original name, `mac-streamdeck-sync`, would be wrong once a Windows adapter exists ([0018](0018-runtime-and-architecture.md)).

The project owner wants changes to the project's own files to stay open, while leaving room to combine it with other code, including a possible future commercial product.

## Decision

- **Name: schrodeck.** Schrödinger + deck: a profile is in superposition across hosts until a deck shows it. The name avoids Elgato's marks. The README carries a not-affiliated notice.
- **License: MPL-2.0.**
  - It is file-level copyleft: modifications to schrodeck's files must stay open.
  - Combining schrodeck with other code, including commercial products, remains allowed.
  - Source files carry the MPL Exhibit A header once code exists.
- **Copyright:** the project owner is the sole copyright holder and may dual-license or build a commercial product. Outside contributions require a CLA (see CONTRIBUTING.md) so that option stays open.
- **Hygiene:** a leak scan runs before every push. It catches hostnames, usernames, home paths, serial or host-id samples and tokens. `LICENSE` is excluded (its copyright line is expected). Examples use `<user>`, `<host>`, `<deck>`. Screenshots of a live desktop are never committed.

## Consequences

- Good: improvements to schrodeck itself flow back. Commercial use and combination stay possible.
- Bad: the CLA adds friction for outside contributors.
- Risk: the leak scan is a fixed pattern list. It catches known identifiers, not new ones. Review is still required.

## Alternatives considered

- **MIT** (previous choice): allows a closed fork of our own files.
- **GPL-3.0:** too restrictive for a possible future commercial product.
- **Apache-2.0:** permissive like MIT. Its patent grant was noted, but it does not keep changes open.
- **Keep `mac-streamdeck-sync`:** platform-specific and uses "streamdeck" as one word.

## Verified by

The leak scan (`git grep` over a pattern list) runs before each push. It has been seen to catch a real hostname in a spec draft (2026-10-01), so it is known to fail on bad input. Still to be written: a CI job running the same scan, and a check for the MPL header once source files exist.

## References

- [R19](../references.md): Elgato branding guidelines (documented)
