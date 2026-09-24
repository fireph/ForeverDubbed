# NPC race lookup data

Race lookup runs in the desktop companion. The addon collects public API values,
NPC IDs, available appearance IDs, and saved user assignments, then sends them
through the optical square. Update both components to 0.6.0 for the extended
metadata. Neither component needs Python, a database server, or network access
at runtime.

## Separate data layers

- `data/voiceover-display.json`: all 15,444 original display-ID identities across
  21 races from [VoiceOver](https://github.com/mrthinger/wow-voiceover/tree/5cf7627fc5ba99d7c2545cf0c9f323eaef31ef09),
  pinned to commit `5cf7627fc5ba99d7c2545cf0c9f323eaef31ef09`. Only the serialization
  changes to JSON; every key, race, and gender is preserved. The previous Lua
  snapshot is retained byte-for-byte in `data/voiceover/DisplayRaces.lua` for
  provenance and reproducibility. It is no longer loaded by the addon.
- `data/custom-races.json`: 51 user-confirmed Forever beta Skyborne NPC IDs,
  with names and observed gender/model IDs as evidence. NPC race assignments do
  not fix gender: multiple spawns of the same NPC type can differ. Models
  `7478487` and `7478494` provide inferred adult Skyborne male/female fallbacks;
  their exclusivity is not established. Model `7865151` has one child observation
  and is deliberately not a general model rule. No display IDs were supplied in
  this collection, so none were invented or inserted into the VoiceOver table.
- `data/model-races.json`: the existing character-model FileDataID fallbacks
  from the [WoW file list](https://github.com/wowdev/wow-listfile), moved out of
  the addon without changing their identities.
- `ForeverDubbedDB.npcRaces`: user assignments made with `/fdb race NAME`, kept
  in WoW SavedVariables. The addon sends the applicable override explicitly on
  each dialogue. It is never merged into the upstream or custom desktop files.

Race priority is saved slash-command override, custom NPC mapping, addon/API
race, VoiceOver display lookup, then custom or standard model appearance.
Public gender takes priority over display and model gender; recorded
`observed_gender` is provenance only. A missing race remains unknown and uses
the voice configuration's default. Identity resolution runs before desktop
state, JSON output, and speech selection, including `-image` decoding.
Controls bypass identity resolution. The desktop's `race_source` JSON field
identifies which layer supplied the result.

The VoiceOver snapshot contains Human, Orc, Dwarf, Night Elf, Undead (`Scourge`),
Tauren, Gnome, Troll, Goblin, Blood Elf, Draenei, Fel Orc, Naga, Broken, Skeleton,
Vrykul, Tuskarr, Forest Troll, Taunka, Northrend Skeleton, and Ice Troll. Skyborne
are supplied exclusively by the custom dataset and user overrides. Detection
does not create voice profiles: races absent from `tts/voices.json` use its
configured default voice.

## Updating custom mappings

The desktop loads `data/custom-races.json` beside the executable, or from
`ForeverDubbed.app/Contents/Resources/data/` on macOS. Development runs also
check `data/custom-races.json` in the working directory. If no external file is
found, it uses the embedded default copy. `-race-config PATH` selects a different
custom file; a missing or invalid explicitly selected file reports an error.
File edits take effect after restarting the companion, without an addon update.
An external custom file replaces the embedded custom dataset; preserve entries
you still want when editing it. The immutable VoiceOver and standard model data
remain embedded in the desktop application.

The custom file schema is:

```json
{
  "source": "In-game confirmations",
  "npcs": {
    "254100": {
      "race": "Skyborne",
      "name": "Zephras Citizen",
      "observed_gender": "female",
      "observed_model_id": 7478494
    }
  },
  "models": {
    "7478494": {"race": "Skyborne", "gender": "female"}
  }
}
```

Only `race` is required in an NPC record. Model `gender` is optional. NPC and
model keys occupy separate namespaces. Model keys are FileDataIDs returned by
`GetModelFileID`, not the `ModelID` foreign key in `CreatureDisplayInfo`.

## Collecting and overriding in game

Use `/fdb race Skyborne` to assign the dialogue NPC, or your target when no NPC
dialogue is open. Close dialogue first when collecting targets. Reopen dialogue
to transmit the new assignment. `/fdb race clear` removes that NPC's override
and evidence; the next dialogue uses the normal desktop lookup. The desktop
keeps no second persistent override cache, so clears require no synchronization.
Existing saved assignments remain valid.

Marking also stores `ForeverDubbedDB.npcRaceEvidence`, keyed by NPC ID, with
`name`, `race`, `gender`, and available `displayID` and `modelID` fields.
`displayStatus` records whether the display API returned a usable ID, zero, nil,
a restricted or invalid value, failed, was unavailable, or was not read.
`/fdb npc` explicitly probes and prints these observations. It does not run the desktop's race lookup.
Each mark replaces the previous evidence for that NPC. Run `/reload` after
collecting to flush SavedVariables to disk.

`PlayerModel:GetDisplayInfo()` can return zero after `SetUnit()` even when
`GetModelFileID()` succeeds, as observed on the Forever beta. Display IDs are
optional; missing display IDs do not invalidate NPC/model evidence. Normal dialogue probes only the model file ID and never waits for a display ID.
Explicit inspection and manual marking can try the display API, returning as
soon as the model is available if the display API reports zero. A successful
display observation can be reused for that same GUID. Probes otherwise wait
up to 600 ms, and changing targets or superseding a request cannot attach
another NPC's appearance. Keep the NPC available while collecting. Restricted
race observations are not reconstructed through desktop ID lookups.

VoiceOver's generation queries obtain NPC-to-display links from an external
creature database, then join `CreatureDisplayInfo.ExtendedDisplayInfoID` to
`CreatureDisplayInfoExtra.ID` for `DisplayRaceID` and `DisplaySexID` (0 male,
1 female). Our imported snapshot contains the display-to-identity portion,
not the external NPC links. New/reassigned Forever displays may be missing or
inaccurate, and appearance can differ from lore identity. The desktop can use
the snapshot when a display ID is available, but does not depend on one.

## Rebuilding the upstream snapshot

Run from the repository root with Python 3.10 or newer:

```sh
python scripts/import-voiceover-races.py
python scripts/import-voiceover-races.py --check
python -m unittest discover -s tests -p 'test_*.py'
```

The importer downloads only the two pinned SQL exports and license, verifies
SHA-256 hashes, validates column order, and parses leading integer fields without
executing SQL. Use `--source-dir PATH` with local `CreatureDisplayInfo.sql`,
`CreatureDisplayInfoExtra.sql`, and `LICENSE` files for offline regeneration or
checking. It produces the archived Lua snapshot, desktop JSON, bundled license,
and `data/voiceover-display-manifest.json`. It never writes the custom dataset.
Records without extra data, missing joins, or unsupported race/sex are skipped.

The source is distributed under the Unlicense; its exact license is included as
[`VOICEOVER-LICENSE.txt`](../data/voiceover/VOICEOVER-LICENSE.txt) in desktop
release packages. Source tables:
[CreatureDisplayInfo](https://github.com/mrthinger/wow-voiceover/blob/5cf7627fc5ba99d7c2545cf0c9f323eaef31ef09/assets/sql/exported/CreatureDisplayInfo.sql),
[CreatureDisplayInfoExtra](https://github.com/mrthinger/wow-voiceover/blob/5cf7627fc5ba99d7c2545cf0c9f323eaef31ef09/assets/sql/exported/CreatureDisplayInfoExtra.sql).
