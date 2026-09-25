# Changelog

## 0.6.5

- Read only the main quest dialogue by default in both Pocket TTS and system speech. Add separate saved desktop options for Quest title and Quest objectives, both off by default, applied when each quest starts playback.
- Keep titles, main dialogue, and objectives as separate transmitted fields. Add FDB5 flags 3 for objectives; update both addon and desktop app. The desktop still accepts flags 0/1/2, but older addons combine objectives with dialogue and cannot support the new preference.

## 0.6.1

- Request borderless Windows game capture through the supported Windows permission API, removing the yellow capture outline when permitted. Keep capture working while permission is pending or unavailable, and preserve the setting across game-window changes.
- Compatible with the 0.6.0 addon; no addon update is required.

## 0.6.0

- Resolve race in the desktop app using the unchanged 15,444-record VoiceOver snapshot, separate custom mappings for 51 confirmed Skyborne NPC IDs, and character-model fallbacks.
- Keep `/fdb race NAME` overrides in SavedVariables and transmit them explicitly with dialogue; clearing an override restores the desktop lookup on the next dialogue.
- Send model file IDs alongside API race/gender and NPC ID; normal dialogue never probes or waits for display IDs. Explicitly observed display IDs remain optional metadata. Preserve model/display evidence and missing-display diagnostics when marking NPCs.
- Add editable `data/custom-races.json` and `-race-config`; restart the companion after file edits. Keep gender independent of NPC race assignments.
- Extend FDB5 with flags 2 metadata. Update addon and companion together to 0.6.0; the companion still reads flags 0/1 messages. Tile layout and settings are unchanged.

## 0.5.0

- Animate a crisp, light-blue sine wave with a constant two-cell stroke width at 15 fps leftward through the middle half of the data area, without touching the calibration ring. Shift one hard-coded loop by whole cells to avoid re-rasterization shimmer.
- Reserve wave cells during encoding and infer their positions from each captured frame during decoding; single-page messages animate too.
- Introduce FDB5 with 1,056 payload bytes per page. Update the addon and companion together; tile dimensions and existing settings are preserved.

## 0.4.0

- Use the dark Chromaglyph palette with a fixed 2px light-blue outline outside the calibration ring. Default tile size is now 104 × 104 physical pixels.
- Locate the outline and decode both ring and data using captured palette references. Reject clipped outlines, collapsed colors, ambiguous samples, and invalid checksums.
- Introduce FDB4; update the addon and companion together. Preserve existing position and valid FDB3 cell-size settings.

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
