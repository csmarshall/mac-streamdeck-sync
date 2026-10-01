# 0023. Store freshness: ask the sync provider, through File Provider

Status: Accepted 2026-10-01

## Context

The store protocol (ADR [0009](0009-store-write-protocol.md)) detects a store that is **half delivered**: a `current.json` whose hash does not match the tree. It cannot detect a store that is **not delivered at all**, where another host pushed and this host's sync client has not fetched it yet. It also cannot tell whether our own push has actually reached the cloud.

On current macOS, Dropbox, iCloud Drive, Google Drive and OneDrive all integrate through Apple's File Provider framework, which exposes per-file sync state as URL resource values.

## Decision

Before **reading** the store (to compare or plan a pull) and after **writing** it (to confirm a push), query these keys for every file schrodeck uses:

| Key | Used for |
|---|---|
| `ubiquitousItemDownloadingStatus` | Read only when every file is `current`. `downloaded` (possibly stale) or `notDownloaded` (online-only placeholder) → request the download and treat the store as **in flight**: no apply this tick. |
| `ubiquitousItemIsUploaded` / `IsUploading` | A push is **confirmed** only when every file is uploaded. Until then, `status` says "pushed, not yet uploaded". |
| `ubiquitousItemHasUnresolvedConflicts` | The provider has made a conflict copy → treat as Diverged (ADR [0007](0007-conflict-policy.md)) and notify. |

- The store directory should be **kept downloaded**. `init` and `join` request it, and `doctor` warns if the provider has evicted it. Placeholder files block or trigger downloads on read.
- These are Apple framework calls, so they sit behind a seventh OS port, **`StoreSync`** (ADR [0018](0018-runtime-and-architecture.md)). A provider that reports none of these keys falls back to ADR 0009's in-flight detection only, and `status` says that freshness is unknown.

## Consequences

- Good: no apply from a store the provider knows is stale, and no "pushed" claim that hasn't reached the cloud.
- **Limit:** `current` means "the newest version **this host knows of**". If the host is offline, or the provider hasn't learned of another host's push yet, the store still reads `current`. Nothing local can see an update the provider hasn't heard about. That window is bounded only by the provider's own sync, and `status` must not claim more than this.
- Requires a Swift (or cgo) adapter. The Go core sees only `Fresh | InFlight | Conflict | Unknown`.

## Alternatives considered

- **Provider web APIs** (e.g. Dropbox `content_hash` via HTTP): definitive, but needs per-provider OAuth tokens and network access. Rejected for v1; it could become an optional adapter later.
- **Our own protocol only:** misses the "not delivered at all" case and push confirmation.
- **Provider CLIs** (`brctl`, `fileproviderctl`): debugging aids with unstable output; not a contract.

## Verified by

Spike on one Mac (2026-10-01, macOS 27): a Dropbox file (File Provider, under `~/Library/CloudStorage`) and an iCloud Drive file both returned the keys (`ubiquitous=true`, `uploaded=true`, `conflicts=false`, status `notDownloaded` because online-only was enabled). A plain local file returned nil for every key, so the check tells synced and unsynced files apart. Not yet tested: Google Drive, OneDrive, a file in the `current` state, a live conflict. To be covered in the plan.

## References

- Apple: [URLResourceKey.ubiquitousItemDownloadingStatusKey](https://developer.apple.com/documentation/foundation/urlresourcekey/ubiquitousitemdownloadingstatuskey) and related ubiquitous-item keys (documented by Apple).
- Provider behavior under File Provider: observed on one Mac for Dropbox and iCloud Drive only.
