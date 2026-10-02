# 0010. Host identity is derived; shared settings live once; per-host data has one writer

Status: Accepted 2026-10-01. Revised 2026-10-01 (review F14, plus the maintainer's question "do we care how many Macs?"): per-host data moved to single-writer `hosts/<host_id>.toml`; the registry is display-only, with no membership. Revised 2026-10-02 (issue #5, owner's decision): heads and B are per member copy, and subscriptions are recorded by `copy_id`, so the per-host file never holds a deck's serial.

## Context

Hosts need a stable identity: for heads and B (per member copy, scoped by host), events and per-host variable values. Hostnames and computer names are user-editable and can collide. Stream Deck profiles are per macOS user, so two accounts on one Mac are two hosts. The store can be mounted at different paths on different hosts. Shared settings should be changed once and apply everywhere.

The first design put subscriptions, variable values and the host registry in one shared `config.toml`. The review showed that concurrent `join`/`subscribe` on two hosts then produces a conflicted copy that stops all sync.

The maintainer also asked whether the number of hosts matters. It doesn't. No logic counts hosts or waits for them. Identity matters only for three things:

- single-writer files;
- per-host variable values;
- attribution.

## Decision

| Layer | Where | Writer | Holds |
|---|---|---|---|
| Host identity | derived at runtime, never configured | — | `host_id = sha256(IOPlatformUUID + ":" + username)[:12]`; the hardware source is per OS ([contract A](../contracts/os-connector.md) `HostIdentity`) |
| Local pointer | `~/.config/schrodeck/config.toml` (optional) | this host | `store = "<path>"` only |
| Per-host config | `<store>/hosts/<host_id>.toml` | **this host only** | friendly name, variable values, subscriptions (`profile_id → [{copy_id, deck_model}]`: one entry per member copy, never the raw `deck_key`, [contract D](../contracts/store-format.md#layout)), GC `pins` ([contract D](../contracts/store-format.md#garbage-collection-trees-only)), host-specific overrides (e.g. `retention.local`) |
| Common config | `<store>/config.toml` | any host, rarely | truly shared settings: variable *declarations* (name + default), `retention.*` defaults, `notify.*`, `apply.*`, script replication map |

- **No membership.** There is no group roster that any decision depends on. A host participates by having heads (one per member copy); a retired host's files are inert. The set of `hosts/*.toml` files is a **display-only registry**, used for friendly names in `status`, notifications and the log. It is also used for the push collision guard ([0006](0006-normalization-and-variables.md)), which only *reads* other hosts' variable values.
- **Store discovery order:** `--store` flag → local pointer → Dropbox's `info.json` → iCloud Drive. If more than one candidate holds a `FORMAT` file, refuse and ask.
- **Writes** of `hosts/<host_id>.toml` and `config.toml` use stage → verify → rename.
- **Common-config conflicts:** if a sync-client conflicted copy of `config.toml` appears, report it. Sync continues using the last good parse. `schrodeck config resolve` shows both versions and keeps one. Only that file is affected, so a settings conflict no longer stops profile sync.

## Consequences

- Good: one home per fact, and no multi-writer file except a rarely written settings file.
- Good: renames and OS reinstalls keep the same `host_id` on macOS. A new logic board or a new Mac is a new host with a safe first run.
- Good: adding or retiring hosts changes nothing for the others.
- Bad: a retired host's `hosts/*.toml` and heads linger until someone runs `schrodeck forget-host` or that host ran `schrodeck uninstall --leave-store`. Nothing about a host is deleted automatically ([0027](0027-store-lifecycle.md)). They are inert: a retired head is subsumed by R and blocks nothing.
- Risk: the hardware id source differs per OS, and Windows' `MachineGuid` changes on reinstall (noted in contract A).

## Alternatives considered

- **Everything in one shared `config.toml`** (the first design): a multi-writer file on every `join`/`subscribe`. Rejected after review (F14).
- **Hostname / ComputerName:** editable and collision-prone.
- **A membership list with an expected host count:** nothing needs it, and it would make a retired or offline host block progress.
- **Store the identity in a file:** can be copied between machines (e.g. by migration tools), creating duplicate identities.

## Verified by

No check yet; to be written in the plan:
- Two candidate stores ⇒ refuse.
- `host_id` differs for two usernames on the same platform UUID, and is stable across two processes ([contract A](../contracts/os-connector.md) conformance).
- A config write with a mismatching re-read hash is not renamed into place.
- A conflicted copy of `config.toml` ⇒ profile sync continues, and `status` reports the conflict.
- Removing a host's files entirely ⇒ every other host's state is unchanged.

## References

- None from Elgato.
