# 0030. Fail closed: an unexpected change drops that Mac out of the setup

Status: Accepted 2026-10-01 (owner's decision; governs every "stop" path in the other ADRs). Revised 2026-10-01 (review F50, F54, F57; owner's decision): a detached copy is **never destroyed**; an explicit rejoin onto the same device **archives** it (moved + renamed) and tells the user. "Replace this detached copy" is folded into that rejoin. The unreachable "duplicate" row becomes "canonical folder occupied". Revised 2026-10-02 (issue #5, owner's decision): a detached copy's own head stays where it was; heads are per member copy, so a Mac with two copies of a setup can have one detached and the other syncing.

## Context

schrodeck edits files the app's owner doesn't support editing ([R3](../references.md)). The earlier ADRs each invented a local reaction to something going wrong: stop and notify (StoreLost, LocalDeleted, DeckGone, Anomaly), refuse the push (foreign device id, variable collision), auto-resume when the condition clears. That made recovery behavior different per path, and some paths resumed automatically after a state nobody had looked at.

The owner's rule: **any configuration change outside schrodeck's expectations makes that Mac drop out of that setup until a human resolves it.** Automatic handling of specific common cases can be added later, one ADR per case, once there's evidence it's safe.

## Decision

**DETACHED(reason)** is a per-copy state (one setup's member copy on one Mac):

- **The local profile is never destroyed.** While DETACHED it is left exactly as it is: no apply, no restore, no rename, no delete. The only thing that ever moves it is an explicit rejoin onto the same device, which **archives** it (moved to its own folder, renamed `<name>-<datestamp>`) and calls that out to the user ([0026](0026-profile-identity.md)).
- **Nothing is pushed or pulled for that copy.** Its head stays where it was, and other Macs treat it like any other head ([contract D](../contracts/store-format.md)). Usually that is an older revision, which other Macs' R already subsumes, so it changes nothing for them. If the Mac detached while its head was a **fork tip** (it had pushed a concurrent edit, then detached), the setup stays Forked on every Mac, and a `resolve` on any other Mac must account for that tip like any other: the detached Mac's edit is never lost, and the detach doesn't block resolving it elsewhere (review F54).
- **One notification**, deduplicated ([0016](0016-notifications.md)), naming the setup, the reason, and the resolve command. The same reason is shown in `status` and the event trail.
- **It never clears by itself.** It persists across runs and reboots until the user resolves it.
- **`schrodeck resolve <setup>`** offers only the options the reason allows:
  1. **Rejoin:** install the setup's current version (R) on a compatible device chosen by the user, as the member copy under the canonical folder and name. If that device already holds this detached copy (or an earlier copy of the setup), that copy is **archived** first: moved to its own folder and renamed `<name>-<YYYY-MM-DD-HHMM>`, named in the dry-run rundown before confirmation and in a notification afterwards ([0026](0026-profile-identity.md)). On another device, the detached profile simply stays as an ordinary local profile. Either way nothing is destroyed.
  2. **Publish this copy:** push the detached copy's content as a new revision whose parent is R, so it becomes the setup's version everywhere. Offered only when the cause allows it (see the table); still subject to the schema guard, the device-id check and the collision guard.
  3. **Unmap:** leave the setup on this Mac. The profile becomes an ordinary local profile, and the subscription is removed from this host's `hosts/<host_id>.toml`.

  Consistent with [0029](0029-problem-statement-and-setup-model.md): resolving never brings another profile's config into the setup, and the only profile it ever moves is the detached copy (archived, never overwritten).

**Classification of every non-normal state.** Each is either an **expected** state with defined handling, or **unexpected**, in which case it becomes DETACHED(reason):

| State / condition | Class | Handling | Resolve options |
|---|---|---|---|
| InSync, Behind, Ahead, FirstShare, Install, Recover | expected (normal) | sync as specified ([0005](0005-direction-detection-three-way-hash.md)) | n/a |
| InFlight (store not current, revision or tree not delivered) | expected (transient) | wait; stuck-in-flight alarm ([0023](0023-store-freshness-via-file-provider.md)) | n/a |
| Diverged / Forked (two Macs edited concurrently) | expected | push the local edit as its own revision; wait for `resolve --keep …` ([0007](0007-conflict-policy.md)) | as in 0007 |
| BLOCKED(R) (an incoming version failed verification here) | expected | rolled back once, blocked until R changes or `unblock` ([0008](0008-two-phase-apply.md)) | as in 0008 |
| HoldLocal (local edit while BLOCKED or the incoming fingerprint is unknown) | expected | edit kept local, not pushed ([0005](0005-direction-detection-three-way-hash.md)) | n/a |
| Unshared (a tombstone from an explicit `unshare`) | expected | detached by unshare; resumes on `reshare` ([0025](0025-deletion-and-unshare.md)) | `reshare`, or unmap |
| VersionMismatch (this Mac's `norm_version` or store `FORMAT` older than the store's) | expected (upgrade window) | read-only until upgraded or migrated ([0027](0027-store-lifecycle.md)) | upgrade / `migrate` |
| Pending older-version edit (seen on upgraded Macs) | expected (upgrade window) | reported, never applied ([0027](0027-store-lifecycle.md)) | n/a on the upgraded Macs |
| Schema guard tripped (this Mac's app changed the profile format) | expected, host-wide | all pushes and applies pause until `doctor` passes ([0015](0015-schema-guard.md)). Already fail-closed, at host scope | `doctor` |
| **Member copy deleted in the app** | **unexpected** | DETACHED(local-deleted) | rejoin, unmap (default suggestion) |
| **Store copy vanished** (store fresh, no heads, no tombstone; persisting) | **unexpected** | DETACHED(store-lost). No longer auto-clears when heads reappear: the user decides | publish (re-creates the setup's history from this copy), unmap |
| **Store went backwards** (R doesn't subsume B; the copy's own head lost, or a provider restore) | **unexpected** | DETACHED(store-went-backwards) | rejoin, publish, unmap |
| **Deck gone** (the member copy's device disappeared from the app's device list, e.g. an expired virtual deck) | **unexpected** | DETACHED(deck-gone). No longer auto-resumes when the deck returns | rejoin (onto another compatible device, or onto the same one once it is back, which archives this copy), unmap |
| **Member copy re-bound** (its `Device.UUID` now names a different device) | **unexpected** | DETACHED(rebound) | rejoin, unmap |
| **Foreign device id** in the copy (a device id other than its own) | **unexpected** | DETACHED(foreign-device-id). Replaces "refuse the push" ([0006](0006-normalization-and-variables.md)) | rejoin, unmap; publish after the user removes the reference |
| **Variable collision** (a literal equal to another host's variable value) | **unexpected** | DETACHED(variable-collision). Replaces "refuse the push" ([0006](0006-normalization-and-variables.md)) | rejoin, unmap; publish after the user fixes the button |
| **Canonical folder occupied** (the member copy's canonical uuid5 folder holds a profile that local state can't account for and that copy's head can't recover) | **unexpected** | DETACHED(folder-conflict) (review F57; replaces the unreachable "duplicate" row) | rejoin (archives the occupant), unmap |
| **Late older-version edit** (this Mac edited under an old `norm_version` after `migrate`, then upgraded) | **unexpected** | DETACHED(stale-version-edit). Replaces the former Anomaly path ([0027](0027-store-lifecycle.md)) | publish (re-hashed under the new version), rejoin, unmap |
| **Apply journal unreadable or inconsistent** after a crash | **unexpected** | DETACHED(apply-recovery) for every target in the journal; the app's running state is restored ([0008](0008-two-phase-apply.md)) | rejoin, publish, unmap |
| Anything else a check finds that isn't in this table | **unexpected** | DETACHED(unknown: <detail>) | rejoin, unmap |

The last row is the rule itself: **a state not classified as expected is unexpected.** Implementations must not add silent fallbacks.

**Later automation.** Any specific unexpected path may later get automatic handling, e.g. "a virtual deck returned with the same key: re-attach". Each such case needs its own ADR, a test, and a known-bad case that shows the automation doesn't fire where it shouldn't.

## Consequences

- Good: one recovery model for every surprise. The user always sees *why* and chooses *what*, and no surprise is ever "fixed" by overwriting someone's profile.
- Good: other Macs keep syncing. A detach is local to one copy on one Mac.
- Bad: more manual steps in v1 for cases that could be automated (deck briefly gone, provider hiccup that looked like store loss). The persistence and freshness checks keep these rare, and later ADRs can automate them case by case.
- Bad: previously auto-resuming paths (StoreLost, DeckGone) now need a `resolve`. Deliberate.

## Alternatives considered

- **Per-path ad-hoc handling** (the previous state): inconsistent, and some paths resumed without a human ever looking.
- **Fail open** (keep syncing, best effort): risks spreading a broken or foreign state to every Mac.
- **Stop the whole host on any surprise:** one odd profile would halt every setup. DETACHED is per copy.

## Verified by

No check yet; to be written in the plan:
- For **each** unexpected row: a fixture that triggers it ⇒ DETACHED(that reason), no write-open on any app file (filesystem-port assertion), no store write except this host's event line, exactly one notification over 10 runs, and the state persists across runs. Known-bad: an implementation that auto-clears (e.g. heads reappear after StoreLost) must fail.
- `resolve` offers exactly the options listed for the reason (golden `--json` output per reason, contract E).
- Rejoin onto the same device archives the detached copy (moved + renamed, content otherwise byte-identical) and never opens any other profile for writing; with another profile of the same name on that device, that profile is untouched. Known-bad: an implementation that overwrites the detached copy in place must fail (its original bytes must still exist afterwards).
- A Mac that detaches while holding a fork tip ⇒ the setup stays Forked on other Macs, and `resolve --keep` on another Mac can choose that tip's content (review F54).
- An unclassified condition injected by a test hook ⇒ DETACHED(unknown), never a silent continue.

## References

- [R3](../references.md): file-level management is unsupported (documented), which is why the default is to stop and ask.
