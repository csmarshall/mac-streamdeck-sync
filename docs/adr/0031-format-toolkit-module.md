# 0031. A standalone Stream Deck format toolkit, as its own Go module in this repo

Status: Accepted 2026-10-02

## Context

Everything schrodeck knows about the Stream Deck app's configuration comes from reverse engineering ([config model](../streamdeck-config-model.md), [contract C](../contracts/profile-format.md), [references](../references.md)). Several unknowns remain: folder encoding, switch-profile references, smart profiles, cross-type copies, duplicate `ActionID`s. Until now that knowledge has been gathered with throwaway scripts.

The knowledge is useful beyond sync: anyone building Stream Deck tooling needs to parse, compare and validate profiles. Elgato publishes no profile schema (R2) and doesn't support file-level management (R3), so an evidence-backed, test-covered model of the format has standalone value.

The format API will churn heavily while the unknowns are being settled, which is exactly the M1 work.

## Decision

Build a **format toolkit** as a **separate Go module inside the schrodeck repository** (working name `deckformat`, chosen to avoid Elgato's marks, see [R19](../references.md)). schrodeck is its first and currently only consumer.

What it contains:

| Piece | Responsibility |
|---|---|
| **Model / parser** | Reads ProfilesV3 into devices → profiles → pages → actions. **Unknown fields are preserved**, so a round trip never loses data the toolkit doesn't understand. |
| **Normalizer / hasher** | The contract C hash definition (P9, P11): strip runtime and instance fields, canonical JSON, images by content, pages labelled by position. |
| **Semantic diff** | Change reports in Stream Deck terms ("XL › page 2 › key 3,1: settings key `entity` changed"), used by dry-run rundowns, `resolve`, and observations. |
| **Observation harness** | `observe start` → the user performs one action in the app → `observe stop` produces a redacted semantic diff and an evidence row for [references.md](../references.md). This is how unknowns become documented observations. |
| **Probe runner** | Executes the assertions of contracts B and C against a real install. `schrodeck doctor` is built on it. |
| **Redacted fixture export** | Turns a real profile into a shareable test fixture (serials, user paths, tokens and device ids scrubbed), so samples from other decks, OS versions and app versions can be contributed safely. |

Boundaries:

- Own `go.mod`, own semver tags (`deckformat/v0.x.y`), own CHANGELOG, own CI job.
- **It must not import any schrodeck package**, which a CI check enforces. schrodeck imports it, never the reverse.
- **No OS-specific code.** It reads paths it is given. Finding the app's data directory is the connector's job ([contract A](../contracts/os-connector.md)).
- **Interoperability only:** it reads and writes the user's own configuration files. It never decrypts Elgato's encrypted plugin bundles or extracts or redistributes Elgato code or assets.
- Licensed MPL-2.0 like the rest of the repo.

**Extraction trigger:** move the module into its own repository when the first external project wants to depend on it, or when its API reaches v1.0, whichever comes first. Extraction is mechanical: move the directory with its history, keep the module's API, and update one import path in schrodeck.

**Delivery:** the toolkit **is milestone M1** (read-only insight), extended with `observe`. Its **first real use** in M1 is settling the open unknowns (U1–U6, P8, P10, P11) on a real Mac. The results update contract C **before** any write path (M3) exists.

## Consequences

- Good: format knowledge lives in one tested place with an explicit boundary. The sync engine stays about sync.
- Good: unknowns get settled by a repeatable procedure that produces evidence, not by one-off scripts.
- Good: a community-useful library becomes possible without committing to maintain one before v1.
- Bad: a second module in the repo means two version streams and a slightly more complex CI.
- Risk: the module name or import path changes if it's extracted under a different name. This is acceptable while the only consumer is in the same repo.

## Alternatives considered

- **A plain package inside schrodeck:** cheapest, but nothing enforces the boundary, so it would drift into depending on sync internals, which makes extraction a refactor instead of a move.
- **An independent repository now:** the cleanest separation, but every M1 discovery would need coordinated releases in two repos while the API changes weekly. Rejected until the extraction trigger.
- **Keep throwaway scripts:** they don't produce repeatable evidence, and they duplicate the parser that schrodeck needs anyway.

## Verified by

No check yet; to be written in the plan:
- A CI import check: `deckformat` importing any schrodeck package fails the build. Known-bad: a deliberate import in a test branch must fail.
- Round-trip test: parse, then serialize a fixture ⇒ byte-identical, including an unknown field injected into the fixture. Known-bad: a parser that drops unknown fields must fail.
- Hash tests are shared with contract C (P9, P11 known-good and known-bad pairs).
- An `observe` test against fixture before/after trees yields the expected semantic diff and a redacted evidence row. Known-bad: a fixture containing a serial or `/Users/<name>` path must be redacted, or the test fails.

## References

- [R2](../references.md) (no published profile schema, observed), [R3](../references.md) (file-level management unsupported, documented), [R19](../references.md) (branding).
- [docs/streamdeck-config-model.md](../streamdeck-config-model.md): the current model and its unknowns.
