# 0002. A shared cloud folder is the only transport

Status: Accepted 2026-10-01

## Context

The hosts sharing a deck often sit on the same desk but can have very different network policies: firewalls, VPN or endpoint agents that steer traffic. A direct host-to-host connection can work on one machine and fail on its neighbour. Every host does have a folder replicated by a cloud sync client (Dropbox, iCloud Drive, …), and local CLI tools.

## Decision

schrodeck communicates **only** through a shared folder that some sync client replicates. There is no LAN discovery, no peer-to-peer connection, and no server of our own. Each host needs only the folder and local commands.

## Consequences

- Good: works across hosts with asymmetric network rules; there are no ports, no discovery, and no credentials of our own.
- Good: any sync client works; the tool never talks to the cloud directly.
- Bad: latency and ordering depend on the sync client. Files can arrive partially or out of order, which forces the write protocol in [0009](0009-store-write-protocol.md).
- Bad: sync clients create "conflicted copy" files when two hosts write the same file. Hence per-host files ([0009](0009-store-write-protocol.md), [0017](0017-observability.md)).
- Risk: iCloud may evict files to cloud-only (dataless). Reads must handle that (force download, or treat the store as in flight).

## Alternatives considered

- **LAN sync (Bonjour/HTTP):** fails under asymmetric firewall/VPN rules, which is the motivating case.
- **Our own server:** an operational burden and a credential to manage, for a personal tool.
- **Git as transport:** needs credentials and network access on every host, and gives merge semantics we don't want ([0007](0007-conflict-policy.md)).

## Verified by

No check yet. To be written in the implementation plan: a store test that simulates partial and out-of-order arrival (see [0009](0009-store-write-protocol.md)).

## References

- None from Elgato. This is an environmental constraint stated by the project owner.
