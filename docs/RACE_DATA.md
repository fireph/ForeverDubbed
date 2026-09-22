# NPC race lookup data

ForeverDubbed bundles 15,444 display-ID identities across 21 races from
[VoiceOver](https://github.com/mrthinger/wow-voiceover/tree/5cf7627fc5ba99d7c2545cf0c9f323eaef31ef09),
pinned to commit `5cf7627fc5ba99d7c2545cf0c9f323eaef31ef09`. No other addon,
database server, network connection, or Python installation is needed in-game.

VoiceOver uses database race and sex fields when generating its recorded speech.
We reuse its exported data for live identity lookup: `PlayerModel:GetDisplayInfo()`
returns a display ID, which joins `CreatureDisplayInfo.ID` to
`CreatureDisplayInfo.ExtendedDisplayInfoID`, then to `CreatureDisplayInfoExtra.ID`.
The extra record supplies `DisplayRaceID` and `DisplaySexID` (0 male, 1 female).
Display IDs are distinct from NPC IDs and model FileDataIDs.

Race priority is saved NPC override, public `UnitRace`, confirmed NPC-ID mapping,
cached identity, display lookup, then known model appearance. A cached model guess
can be upgraded by a later display lookup. Public `UnitSex` takes priority over
database/model gender. Model probing waits at most 600 ms; a superseded dialogue
or changed unit cannot acquire another NPC's identity.

The snapshot covers Human, Orc, Dwarf, Night Elf, Undead (`Scourge`), Tauren,
Gnome, Troll, Goblin, Blood Elf, Draenei, Fel Orc, Naga, Broken, Skeleton, Vrykul,
Tuskarr, Forest Troll, Taunka, Northrend Skeleton, and Ice Troll. Detection does
not add voice profiles: races absent from `tts/voices.json` use its default voice.

This is legacy display data, not a verified catalog of Forever NPCs. New or
reassigned displays may be missing or inaccurate; a display can also represent
an appearance rather than lore race. Skyborne are absent. The confirmed
Zephras Citizen NPC mapping and saved assignments take priority. Unknown display
IDs fall back to model appearance or remain unassigned. Use `/fdb npc` to inspect
the race source, display ID, and model FileDataID. Correct individual NPCs with
`/fdb race Skyborne` (or another race), then reopen dialogue.

## Rebuilding the data

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
checking. It produces `addon/ForeverDubbed/DisplayRaces.lua`, the bundled license,
and `data/voiceover-display-manifest.json` with source paths, hashes, and counts.
Records without extra data, missing joins, or unsupported race/sex are skipped.

The source is distributed under the Unlicense; its exact license is included as
[`VOICEOVER-LICENSE.txt`](../addon/ForeverDubbed/VOICEOVER-LICENSE.txt) in the addon
and release packages. Source tables:
[CreatureDisplayInfo](https://github.com/mrthinger/wow-voiceover/blob/5cf7627fc5ba99d7c2545cf0c9f323eaef31ef09/assets/sql/exported/CreatureDisplayInfo.sql),
[CreatureDisplayInfoExtra](https://github.com/mrthinger/wow-voiceover/blob/5cf7627fc5ba99d7c2545cf0c9f323eaef31ef09/assets/sql/exported/CreatureDisplayInfoExtra.sql).
