# 0023. Store freshness: ask the sync provider, through File Provider

Status: Accepted 2026-10-01. Revised 2026-10-01 (review F21, F24): the `current`-state spike gates M2, a stuck-in-flight alarm was added, Google Drive/OneDrive reworded as unverified, and the port count was removed.

## Context

The store protocol (ADR [0009](0009-store-write-protocol.md), [contract D](../contracts/store-format.md)) detects a push that is **half delivered**: a head whose commit or tree isn't fully present. It can't detect a push that hasn't been **delivered at all**, where another host pushed and this host's sync client hasn't fetched it yet. It also can't tell whether our own push has reached the cloud.

Apple's File Provider framework exposes per-file sync state as URL resource values. On current macOS, Dropbox and iCloud Drive are observed to use it. Google Drive and OneDrive are **expected** to, but that is **unverified**.

## Decision

Before **reading** a profile from the store and after **writing** it, query these keys for every file schrodeck uses:

| Key | Used for |
|---|---|
| `ubiquitousItemDownloadingStatus` | Read only when every file is `current`. `downloaded` (possibly stale) or `notDownloaded` (an online-only placeholder) → request the download and treat the profile as **InFlight**: no apply this tick. |
| `ubiquitousItemIsUploaded` / `IsUploading` | A push is **confirmed** only when its tree and commit are uploaded. Until then, `status` says "pushed, not yet uploaded". |
| `ubiquitousItemHasUnresolvedConflicts` | The provider has made a conflict copy of a schrodeck file. Under contract D's single-writer rule this should never happen for profile data, so it is reported as an error (and as a `config.toml` conflict when it's that file). |

- The store directory should be **kept downloaded**. `init` and `join` request it, and `doctor` warns if the provider has evicted it.
- **Stuck-in-flight alarm:** if a profile stays InFlight longer than `notify.inflight_after` (default 1 hour), notify once (deduplicated, [0016](0016-notifications.md)). This catches a provider that never reports `current`, or a tree that never arrives.
- These are Apple framework calls behind the **`StoreSync`** port ([contract A](../contracts/os-connector.md)). A provider that reports none of these keys falls back to contract D's InFlight detection alone, and `status` says freshness is unknown.
- **Gate for M2:** before any push ships, a spike must observe a Dropbox file in the `current` state (up to date and downloaded) on a real Mac. If Dropbox reports only `downloaded` for an up-to-date file, the rule above would hold every profile InFlight forever, and this ADR must be revised before M2.

## Consequences

- Good: no apply from a store the provider knows is stale, and no "pushed" claim that hasn't reached the cloud.
- **Limit:** `current` means "the newest version **this host knows of**". If the host is offline, or the provider hasn't learned of another host's push yet, the store still reads `current`. Nothing local can see an update the provider hasn't heard about. That window is bounded only by the provider's own sync, and `status` must not claim more than this.
- Requires a Swift (or cgo) adapter. The Go core sees only `Fresh | InFlight | Conflict | Unknown`.

## Alternatives considered

- **Provider web APIs** (e.g. Dropbox `content_hash` via HTTP): definitive, but needs per-provider OAuth tokens and network access. Rejected for v1; it could become an optional adapter later.
- **Our own protocol only:** misses the "not delivered at all" case and push confirmation.
- **Provider CLIs** (`brctl`, `fileproviderctl`): debugging aids with unstable output; not a contract.

## Verified by

- Spike on one Mac (2026-10-01, macOS 27): a Dropbox file (File Provider, under `~/Library/CloudStorage`) and an iCloud Drive file both returned the keys (`ubiquitous=true`, `uploaded=true`, `conflicts=false`, status `notDownloaded` because online-only was enabled). A plain local file returned nil for every key, so the check tells synced and unsynced files apart.
- **Not yet observed:** the `current` state (the M2 gate above), a live conflict, Google Drive, OneDrive.
- To be written: a StoreSync conformance test ([contract A](../contracts/os-connector.md)), and a stuck-in-flight test (a fake provider that never reports `current` ⇒ exactly one alarm).

## References

- Apple: [URLResourceKey.ubiquitousItemDownloadingStatusKey](https://developer.apple.com/documentation/foundation/urlresourcekey/ubiquitousitemdownloadingstatuskey) and related ubiquitous-item keys (documented by Apple).
- Provider behavior under File Provider: observed on one Mac for Dropbox and iCloud Drive only.
