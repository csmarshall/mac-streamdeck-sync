# 0006. Normalization and per-host variables

Status: Accepted 2026-10-01 (variables in v1)

## Context

The hash in [0005](0005-direction-detection-three-way-hash.md) must change on a real user edit and on nothing else. Some content legitimately differs per host:
- `Open` actions store absolute paths under the user's home, and usernames differ ([R16](../references.md), observed).
- Some buttons may need per-host values, e.g. a service URL that is reachable differently from a firewalled host.
- `Device.UUID` names the local deck ([R9](../references.md)).

Some fields change at runtime with no edit: the action `State` and `Pages.Current` ([R15](../references.md), observed).

## Decision

**Normalize** before hashing: parse the JSON, then:
1. Drop the runtime-only fields: action `State`, `Pages.Current`.
2. Drop `Device.UUID`. It is rewritten per receiving deck on install.
3. Replace variable values with placeholders (below).
4. Sort the keys, serialize canonically, then hash the canonical bytes together with the image bytes.

The strip list (1–2) is **one named constant**. Every entry has a test proving that a launch-only rewrite does not change the hash.

**Variables** (in v1):
- A built-in `{{HOME}}`, plus user-defined variables (e.g. `{{HA_URL}}`) whose per-host values live in the common config ([0010](0010-host-identity-and-config-layering.md)).
- On push, each host's value is replaced with its placeholder. On install, placeholders are expanded with the receiving host's values.
- A per-host difference therefore never registers as an edit or a conflict.

## Consequences

- Good: per-host differences are first-class, not a source of false conflicts.
- Good: `{{HOME}}` is a special case of a general mechanism, so there is only one code path.
- Bad: replacing a value with its placeholder is a string replacement. If a host's variable value also appears as unrelated text, it gets replaced too. Variable values should be specific (full URLs, full paths), and the tool should warn on short values.
- Risk: a new runtime field added by an app update breaks the hash stability. The schema guard ([0015](0015-schema-guard.md)) and the launch-rewrite test catch it.

## Alternatives considered

- **Raw byte hash:** fails on every launch.
- **Per-host path maps (old → new):** a special case of variables, with more configuration.
- **Ignore whole manifests the app rewrites:** loses real edits.

## Verified by

No check yet; to be written in the plan:
- One test per strip-list entry: fixture pair (before/after launch rewrite) ⇒ equal hashes. The same pair with normalization disabled ⇒ unequal hashes (known-bad).
- A variable round-trip test: canonicalize(expand(x, hostA), hostA) == canonicalize(expand(x, hostB), hostB).

## References

- [R9](../references.md): `Device.UUID` (observed)
- [R15](../references.md): `State`, `Pages.Current` runtime fields (observed)
- [R16](../references.md): absolute paths in `Open` (observed)
