# 0010. Host identity is derived; config lives once, in the store

Status: Accepted 2026-10-01

## Context

Hosts need a stable identity (for B per host, snapshots, events). Hostnames and computer names are user-editable and can collide. Stream Deck profiles are per macOS user, so two accounts on one Mac are two hosts. The store can be mounted at different paths on different hosts. Settings should be changed once and apply everywhere.

## Decision

Configuration has three layers:

| Layer | Where | Holds |
|---|---|---|
| Host identity | derived at runtime, never configured | `host_id = sha256(IOPlatformUUID + ":" + username)[:12]` |
| Local pointer | `~/.config/schrodeck/config.toml` (optional) | `store = "<path>"` only |
| Common config | `<store>/config.toml` | subscriptions, retention, variables (values per host), notify level, apply policy, script replication map, host registry (`host_id → friendly name`), host-keyed overrides |

- **Store discovery order:** `--store` flag → local pointer → Dropbox's `info.json` → iCloud Drive. If more than one candidate holds a FORMAT file, refuse and ask.
- **Hosts self-register** on first run, with the computer name as the default friendly name.
- **Common config writes** use the same stage → verify → rename discipline as profile pushes ([0009](0009-store-write-protocol.md)).

## Consequences

- Good: one home per fact. A setting changed on one host applies on all of them.
- Good: renames and OS reinstalls keep the same `host_id`. A new logic board or a new Mac is a new host with a safe FirstRun.
- Bad: the common config is a file several hosts may write, so the sync client can produce a conflicted copy if two hosts change settings at once. Rare, and detectable by the tool (refuse to proceed while a conflicted copy exists).
- Risk: `IOPlatformUUID` is macOS-specific. The Windows adapter needs its own equivalent ([0018](0018-runtime-and-architecture.md)).

## Alternatives considered

- **Hostname / ComputerName:** editable and collision-prone.
- **A full per-host config file:** settings drift between hosts.
- **Store the identity in a file:** can be copied between machines (e.g. by migration tools), creating duplicate identities.

## Verified by

No check yet; to be written in the plan:
- A test that two candidate stores ⇒ refuse.
- A test that `host_id` differs for two usernames on the same platform UUID.
- A test that a config write with a mismatching re-read hash is not renamed into place.

## References

- None from Elgato.
