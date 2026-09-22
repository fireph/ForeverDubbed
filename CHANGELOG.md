# Changelog

## 0.3.2

- Bundle 15,444 display-ID race/gender mappings from VoiceOver across 21 legacy races, with a pinned source, verified importer, and license.
- Resolve NPC display IDs before shared model appearances; preserve API identity, confirmed Skyborne mappings, and saved NPC overrides.
- Show display IDs in `/fdb npc`; add regression coverage for delayed display loads, cached model upgrades, and fallback behavior.
- Keep the optical protocol and existing Pocket TTS voice configuration compatible with 0.3.1.

## 0.3.1

- Resolve missing NPC races using confirmed NPC IDs, cached identities, and known character model file IDs. Wait up to 600 ms for model loading without publishing superseded dialogue.
- Recognize Zephras Citizen (254100) as Skyborne. Add Skyborne, Goblin, Blood Elf, and Draenei voice profiles.
- Add `/fdb npc` diagnostics and saved per-NPC race assignments through `/fdb race NAME` and `/fdb race clear`.
- Use the `jean` preset through a separate `narrator_male` fallback for male or unknown-gender speakers whose race is unavailable or unmapped.
- Expand regression coverage for absent NPC race APIs, model loading, secret values, identity changes, race assignments, and voice selection.

## 0.3.0

- Add local CPU speech using Pocket TTS 3.1.0 with cached presets, Windows setup/launch scripts, and the Go audio player. Retain Windows SAPI as a fallback.
- Add race/gender voice profiles and per-NPC voice overrides.
- Extend FDB3 with optional race, gender, and NPC ID metadata while continuing to accept text-only FDB3 messages.
- Prefetch one short speech chunk during playback; interrupt playback and discard stale audio when new dialogue arrives.

## 0.2.1

- Correct physical pixel sizing across UI scales and screen resolutions, including 4K, and migrate saved tile positions.
- Add version reporting and optional desktop snapshots for capture diagnostics.

## 0.2.0

- Make 16 calibrated colors the sole optical format: 1,124 payload bytes per page in a 100 × 100 pixel square at the default two-pixel cell size.
- Automatically locate the movable square, validate page/message checksums, and assemble repeated pages before speaking.

## 0.1.0

- Initial NPC/quest dialogue addon and Windows Go screen reader using an earlier eight-color format.
