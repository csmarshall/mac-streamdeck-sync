# 0006. Normalization (for hashing only) and per-host variables

Status: Accepted 2026-10-01 (variables in v1). Revised 2026-10-01 (review F3, F4, F8, F9): the store holds full trees; normalization is used only for hashing; substitution is path-boundary-aware; `norm_version` added; variable changes never push. Revised 2026-10-01 (review round 2: F3, F27, F29): a wider boundary set, `Device.UUID` stored as `{{DEVICE}}`, and re-materialization checks for unpushed edits under the old values first. Revised 2026-10-01 (review F46): `{{DEVICE}}` substitution is value-based, anywhere in the tree; a foreign device id refuses the push.

## Context

The hash in [0005](0005-direction-detection-three-way-hash.md) must change on a real user edit and on nothing else. Some content legitimately differs per host:

- `Open` actions store absolute paths under the user's home, and usernames differ ([R16](../references.md), observed).
- Some buttons may need per-host values, e.g. a service URL that is reachable differently from a firewalled host.
- `Device.UUID` names the local deck ([R9](../references.md)).

Some fields change at runtime with no edit: action `State` and `Pages.Current` ([R15](../references.md), observed).

## Decision

**Three representations, kept distinct:**

| Form | Where | Contents |
|---|---|---|
| Local copy | the app's `ProfilesV3` | exactly what the app wrote, with this host's literal values |
| Stored tree | `trees/<digest>/<host_id>/` in the store ([contract D](../contracts/store-format.md)) | the **full** tree as the app wrote it, with variable values replaced by placeholders and every `Device.UUID` value replaced by the reserved `{{DEVICE}}`. Runtime fields are kept. No raw device id (which embeds the deck's USB serial, [R9](../references.md)) ever reaches the store. |
| Normalized form | in memory only | the stored form minus the strip list, JCS-canonicalized; used **only** to compute `hash` |

- **`{{DEVICE}}` is value-based** (review F46). When a tree is put into placeholder form, **every occurrence of this copy's own deck id string, anywhere in the tree** (the top-level `Device.UUID`, and any action setting that embeds it), becomes `{{DEVICE}}`, using the same path-boundary rules as other variables. Any *other* device-id-shaped value left afterwards (`@(` …) is a different deck's id: the push is refused and the inventory reports it, because it would leak a serial and point at the wrong deck on other hosts.
- **Install** = take the stored tree, expand placeholders with this host's values, expand `{{DEVICE}}` to the receiving deck's id, and name the folder per [0026](0026-profile-identity.md). That is close to a byte-copy, so we depend as little as possible on the app tolerating fields we invented.
- **The exact hash definition and the file allow-list live in [contract C § normalized hash](../contracts/profile-format.md#normalized-hash).** The strip list is one named constant there.
- **`norm_version`:** every revision records the normalization version it was hashed under. Hashes are compared only under the same `norm_version`. Changing normalization means a `FORMAT` migration ([0027](0027-store-lifecycle.md)): B is re-based without pushing, and hosts on an older version go read-only instead of looping.

**Variables (v1):**

- The built-in `{{HOME}}`, plus user-defined variables (e.g. `{{HA_URL}}`). Their names and defaults are declared in the common config; each host's values live in its own `hosts/<host_id>.toml` ([0010](0010-host-identity-and-config-layering.md)).
- **Boundary-aware substitution.** A value is replaced only where it is preceded by start-of-string or a boundary character, and followed by end-of-string or a boundary character. The boundary set is `/` `\` `?` `#` `"` `'` `:` `,` `)` `]` `}` `(` `[` `{` `=` `&` `;` `@` and whitespace. `/Users/<al>` therefore never matches inside `/Users/<alice>`, while `https://ha.lan` *is* substituted inside `https://ha.lan:8123/api` (review F3: the first boundary set lacked `:`, so URLs with ports were never substituted). Longer values are matched first.
- **Escaping.** A literal `{{` in profile content is stored as the reserved placeholder `{{_}}` and expanded back on install, so user text can never be mistaken for a placeholder.
- **Collision guard.** Before pushing, scan the local copy for a literal that equals **another** registered host's value of any variable (path-boundary-aware). An example is the other Mac's `/Users/<other>/…` home path. If one is found, **refuse the push** for that profile and notify, naming the button. That literal would be wrong on that host, and pushing it would bake one machine's value into everyone's copy. `schrodeck inventory` lists the offending buttons.
- **A variable-value change never pushes by itself.** Each copy's local state records the variable values it was last materialized with (`vars_used`). When this host's values change, the next run:
  1. computes L **under `vars_used`** (the old values), so the old literals canonicalize to placeholders exactly as before;
  2. if that L ≠ B.local_hash, the copy has an unpushed edit: **push it first** (canonicalized under the old values), so the edit is never overwritten (review F29);
  3. then **re-materializes** the copy: re-installs the stored tree of the (now current) B revision with the new values, through the normal two-phase apply ([0008](0008-two-phase-apply.md)), and records the new `vars_used`.

  B's tree is **pinned** against GC while a re-materialization is pending ([contract D § garbage collection](../contracts/store-format.md#garbage-collection-trees-only)). Without step 1, old literals would look like an edit and be pushed as literals (review scenario 3).

## Consequences

- Good: per-host differences are first-class, not a source of false conflicts.
- Good: `{{HOME}}` is a special case of one general mechanism.
- Good: the store holds what the app actually wrote, so install doesn't depend on reinserting stripped fields.
- Bad: substitution is still a text operation. A variable value that appears in unrelated text, at a boundary, is replaced too. Values should be specific (full URLs, full paths), and the tool warns on short values (under 8 characters).
- Risk: a new runtime field added by an app update breaks hash stability. The schema guard ([0015](0015-schema-guard.md)) and the launch-rewrite probe ([contract C](../contracts/profile-format.md), P4) catch it.

## Alternatives considered

- **Store the stripped, normalized tree** (the first design): install would have to reinvent `State`, `Pages.Current` and other fields, betting the app accepts them. Rejected after review (F9).
- **Raw byte hash:** fails on every launch.
- **Plain substring substitution:** `/Users/<al>` corrupts `/Users/<alice>`; rejected (F3).
- **Per-host path maps (old → new):** a special case of variables, with more configuration.

## Verified by

No check yet; to be written in the plan:
- One test per strip-list entry: a fixture pair (before/after a launch rewrite) ⇒ equal hashes; the same pair with normalization disabled ⇒ unequal hashes (known-bad).
- **Adversarial cross-host fixtures** (review F20), where both sides of each comparison are not derived from the same placeholder tree:
  - prefix homes (`/Users/<al>` vs `/Users/<alice>`);
  - a literal `{{` in a button title;
  - another host's home path baked into a button, which must refuse the push;
  - a variable value change, which must re-materialize, not push;
  - a variable value change **with an unpushed edit** on the copy, which must push the edit first and then re-materialize, losing nothing (known-bad: re-materializing first must lose the edit);
  - a URL with a port (`https://ha.lan:8123/x` with `HA_URL = https://ha.lan`), which must substitute;
  - a stored tree, which must contain `{{DEVICE}}` and no `@(` device id;
  - a value appearing mid-word, which must not be substituted.
- A cross-host round trip using **two distinct host configs**: expand on host A, edit nothing, canonicalize on host B ⇒ the same hash as the stored tree.

## References

- [R9](../references.md): `Device.UUID` (observed)
- [R15](../references.md): `State`, `Pages.Current` runtime fields (observed)
- [R16](../references.md): absolute paths in `Open` (observed)
