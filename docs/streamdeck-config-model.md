# How the Stream Deck app organizes configuration

The data model schrodeck syncs, as far as we know it. Every statement is marked:

- **documented**: Elgato states it (source in [references.md](references.md));
- **observed**: seen on a real machine (one Mac, app 7.5.1, three physical decks + one virtual), not promised by Elgato;
- **unknown**: not yet established, with the probe that would settle it.

The checkable assertions derived from this page live in [contract B](contracts/client-os.md) and [contract C](contracts/profile-format.md). This page explains the model; the contracts test it.

## The model

```mermaid
erDiagram
    COMPUTER ||--o{ DEVICE : "has seen"
    DEVICE }o--|| DEVICE_TYPE : "is a"
    DEVICE ||--o{ PROFILE : "owns"
    DEVICE ||--o| PROFILE : "selected"
    PROFILE ||--|{ PAGE : "contains"
    PAGE ||--o{ ACTION : "key or dial slot"
    ACTION }o--o| PLUGIN : "implemented by"
    ACTION ||--o| PAGE : "folder opens"
    PROFILE }o--o| APPLICATION : "smart-profile link"
```

| Entity | What it is | Identity | Basis |
|---|---|---|---|
| **Computer** | one macOS user account on one machine | schrodeck's `host_id` (ADR [0010](adr/0010-host-identity-and-config-layering.md)) | ours |
| **Device** | one deck the app has seen on this computer: physical (USB) or virtual | the app's device key `@(1)[<vendor>/<product>/<serial>]`; virtual decks show `@(0)[]` | observed [R9, R14] |
| **Device type** | the hardware model, which fixes the geometry (columns × rows, + dials) | the USB product id; Elgato's DeviceType enum (Stream Deck, Mini, XL, +, Neo, + XL, Virtual…) | documented [R8] |
| **Profile** | a complete button layout for **one** device | the `.sdProfile` folder UUID (local to the computer) | observed [R2] |
| **Page** | one screen of a profile; a profile has an ordered list of pages | a page UUID folder under `Profiles/` | observed [R2] |
| **Folder** | a button that opens a sub-page; stored as another page folder in the same profile | page UUID | observed (structure); exact folder-vs-page encoding **unknown** (probe P9 below) |
| **Action** | what one key or dial does: plugin, settings, title, icon, states | its slot position on the page (`col,row`, e.g. `3,1`) **and** a per-instance `ActionID` that changes when copied | observed [R2, R20]; settings format documented [R7] |
| **Plugin** | code implementing actions; installed per computer; its global settings are per computer | plugin UUID (e.g. `com.elgato.…`) | documented [R6, R7] |
| **Selected profile** | the profile a device currently shows | stored **per device** in the app's preferences, not in the profile | observed [R14] |
| **Smart profile** | a profile that switches in when a given application is focused | `AppIdentifier` in the profile manifest. Observed values: **absent** (the XL's profile) and **`"*"`** (two other default profiles), which probably means "any application". A real smart profile presumably holds an app identifier (U3) | documented [R13] (feature); encoding partly observed |

## Cardinalities

| Relationship | Cardinality | Basis |
|---|---|---|
| Computer → Devices | **1 → N** (every deck the app has ever seen, connected or not, incl. virtual) | observed: 4 device records on one Mac, one of them virtual |
| Device → Device type | **N → 1** | documented [R8] |
| Device → Profiles | **1 → N** (a device can have many profiles; on the observed Mac each has exactly one) | observed + documented (profiles are created per device in the app's UI) |
| **Profile → Device** | **N → 1, exactly one.** A profile's manifest names exactly one `Device` (model + key). A profile is never shared between two devices, not even two of the same type | observed [R9] on every profile |
| Device → Selected profile | **1 → 0..1** | observed [R14]; the virtual deck's selected id points to a profile that **doesn't exist on disk** (U4) |
| Profile → Pages | **1 → 1..N**, ordered: `Pages.Pages` lists the user's pages in order, `Pages.Current` is the page last shown (runtime), and `Pages.Default` points to a separate **empty** page that is never in the list | observed on all 3 profiles |
| Profile → its "Default" page entry | **1 → 1**: every observed profile has one extra page folder with **0** actions that the manifest's `Pages.Default` points to | observed; **meaning unknown** (U2) |
| Page → Actions | **1 → 0..(columns × rows [+ dials])** | observed; the bound is from geometry [R8] |
| Action → Plugin | **N → 0..1** (built-ins like Open/Hotkey are `com.elgato.streamdeck.system.*`) | observed |

## Can a profile move between deck types?

**Short answer: a profile *instance* never moves. It belongs to exactly one device. A *copy* can be made for another device of the same type; across types is unknown and, for schrodeck, out of scope.**

| Move | What the app does | Basis | schrodeck |
|---|---|---|---|
| Same device type, different physical deck (XL → another XL) | Import lets you choose the target device (6.5+). On disk the copy differs only in `Device.UUID` | documented (choose device on import [R10]); same-type copy = rewriting `Device.UUID` is **observed by design, not yet probed** (contract B M4 round-trip) | **Yes.** This is exactly how a setup reaches another computer's XL (`{{DEVICE}}`, ADR [0006](adr/0006-normalization-and-variables.md)) |
| Different device type (XL 8×4 → Stream Deck 5×3) | **Unknown.** Elgato says profiles are device-specific ("layouts and button mappings differ across models") and the Marketplace lists which units a profile "supports" | documented that profiles are device-specific [R10]; cross-type import behavior **unknown** (U1) | **No, not in v1.** Geometry must match (ADR [0003](adr/0003-decks-are-local-geometry-compatibility.md)). Cross-geometry re-flow is a possible future feature |
| Virtual deck ↔ physical deck | Virtual decks have user-chosen geometry (up to 8×8) | documented [R12] | Same rule: allowed only if the geometry matches exactly |

## What a copy changes (and what it doesn't)

On the observed Mac, the same page exists in two profiles (a Stream Deck profile and an XL profile). Comparing them [R20, R21](references.md):

| Part | Same in both copies? |
|---|---|
| Which plugin action sits at which `col,row` | **yes** |
| Action settings, titles, states (except image references) | **yes** |
| Image bytes | **yes** |
| Each action's `ActionID` | **no**: new per copy |
| Page folder UUIDs (`Profiles/<page>`) | **no**: new per copy (the copied page has a different folder UUID) |
| Image file names (`Images/<id>.png`) | **no**: new per copy |
| Position on the bigger deck | kept as-is (top-left), no re-flow |

Consequence for schrodeck: "the same setup" has to mean **the same content**, not the same ids. The hash ignores `ActionID`, relabels page folders by their position in `Pages.Pages`, and compares images through their references by content ([contract C](contracts/profile-format.md) P9, P11). Two independently made identical setups therefore hash equal **as long as every page is in `Pages.Pages`**. Folder sub-pages keep their UUID in the hash until the folder encoding is known (U-row P9), so independently made setups with folders may hash different; copies schrodeck makes keep page UUIDs, so they're unaffected. When schrodeck installs a copy it keeps page UUIDs and image names and **regenerates every `ActionID`** ([ADR 0026](adr/0026-profile-identity.md)), so it never creates duplicate `ActionID`s on one Mac.

## Layout combinations schrodeck has to handle

| Situation | What it looks like in the app | Notes |
|---|---|---|
| One deck per computer, same type everywhere (the core case) | each computer: 1 XL device → its profiles | the "same setup at every computer" goal |
| Several decks of different types on one computer (e.g. XL + Mini + Stream Deck, as on the observed Mac) | one device node per deck, each with its own profiles | each type is independent |
| **Two decks of the same type on one computer** (two XLs) | two device nodes with the same geometry, each with its **own** profiles | needs an explicit mapping: which setup goes to which deck (ADR [0026](adr/0026-profile-identity.md)) |
| A deck that is not connected right now | its device node and profiles remain and stay editable | documented [R11] |
| A virtual deck | device key `@(0)[]` (no serial); may have zero profiles on disk while prefs name a selected one | observed; uniqueness of `@(0)[]` with several virtual decks is **unknown** (U5) |
| A computer with no Stream Deck app installed | nothing to read | `join` refuses (ADR [0022](adr/0022-onboarding-init-and-join.md)) |

## Unknowns and the probe for each

| id | Unknown | Probe |
|---|---|---|
| U1 | What the app does when a profile/page made for a **different device type** is imported or copied. **Partly answered** [R21](references.md): a smaller layout on a bigger deck keeps its `col,row` coordinates (top-left, no re-flow). Bigger onto smaller is still unknown (refuse? truncate?) | Manual, throwaway: copy an XL page with buttons beyond column 4 onto the 5×3 deck and inspect the result on disk. Doesn't block v1 (we refuse cross-geometry anyway) |
| U2 | What the 0-action page folder that `Pages.Default` points to is (an empty start page? a template?) | Create a fresh profile in the app, diff before/after; check whether `Default` changes when pages are reordered. Contract C row |
| U3 | What `AppIdentifier` means. It appears on two "Default Profile"s, so it may not mean "smart profile" | Create a smart profile linked to one app, diff the manifest; compare with a plain profile. Contract C row |
| U4 | Why the virtual deck's selected profile id has no folder on disk (lazy creation?) | Open the virtual deck in the app, check whether the folder appears |
| U6 | Whether two profiles on one Mac may share `ActionID`s | contract C P10. **Informational only:** schrodeck regenerates `ActionID`s on every install, so it never creates duplicates (review F51) |
| U5 | Whether several virtual decks share `@(0)[]` or get distinct keys | Create a second virtual deck, read the prefs `Devices` keys |
| P9 | How a folder button encodes its target page (the page UUID in action settings?) | Already covered by contract C P8 (profile-reference probe) |

## How schrodeck maps onto this

**The app has no device-type → profile relationship.** Profiles hang off individual devices (device 1 → N profiles, each profile → exactly one device). Two XLs, even on the same computer, each own separate profiles. The only type-level grouping Elgato has is for **profile templates** (plugin-bundled or Marketplace profiles declare a `DeviceType` [SDK profiles guide](https://docs.elgato.com/streamdeck/sdk/guides/profiles)), and installing one still creates an ordinary per-device copy.

So a "the same setup on every XL" layer **doesn't exist in the app**, and schrodeck has to provide it: a **setup** (keyed by geometry) holds the content, and each enrolled device on each computer gets its own materialized copy (new folder ids, `{{DEVICE}}` bound to that device). Equality between copies is judged on content, not ids (see *What a copy changes* above).

```
What the app has:                    What schrodeck adds:

device XL#1 ──1:N──▶ profiles        "XL setup" (8×4) ──1:N──▶ profiles (content)
device XL#2 ──1:N──▶ profiles               ├─▶ copy on Mac A's XL
device Mini ──1:N──▶ profiles               ├─▶ copy on Mac B's XL
(no type-level grouping)                    └─▶ copy on Mac C's XL
```

schrodeck syncs at the **profile** level and installs each copy onto a local **device** of the same geometry. Everything per device that isn't a profile (which profile is selected, brightness, device name) stays local (ADR [0019](adr/0019-selected-profile-stays-per-host.md)).

**How a setup is built** (ADR [0029](adr/0029-problem-statement-and-setup-model.md), onboarding in [0022](adr/0022-onboarding-init-and-join.md)):

1. **First computer (`init`, or `share` later):** schrodeck reads the app's config and lists the devices. You pick a device and a **template** profile on it, and name the setup. schrodeck reads the template **read-only**, publishes it to the store as the setup's root revision (with the name `schrodeck - <cols>x<rows> - <name>`, e.g. `schrodeck - 8x4 - Work`), and then installs it back onto that device through the ordinary install path, which creates a **new** profile: the setup's first **member copy**. The template is never modified or synced. The setup records the template device's geometry.
2. **Other computers (`join`, or `subscribe` later):** schrodeck lists the setups whose geometry matches one of this computer's devices. You pick a setup and a **destination device**, and schrodeck creates a **new** profile there. It never replaces or modifies an existing profile, and a joining computer's own profiles are never brought into a setup. Merging two computers' configs is out of scope (possible by hand, at your own risk).
3. **From then on, all member copies are peers.** An edit on any of them reaches every other.

Mapped onto the app's model:

| App concept | In a setup |
|---|---|
| Device | the user picks the destination device on each computer; two same-size decks are just two possible destinations |
| Profile | each member copy is an ordinary profile the app owns, created by schrodeck, in a folder named from (setup, device) |
| Selected profile | untouched; each computer shows whatever profile it shows ([0019](adr/0019-selected-profile-stays-per-host.md)) |
| Pages, folders, buttons | travel inside the member copy |
| Every other profile (templates included) | never read for sync, written or deleted |

Anything unexpected about a member copy (deleted in the app, re-bound to another device, its deck gone, …) makes that computer **drop out** of the setup until you resolve it (ADR [0030](adr/0030-fail-closed-detach.md)).
