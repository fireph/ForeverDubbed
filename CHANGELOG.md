# Changelog

## 0.9.5

- Give the Windows updater executable and native dialogs the same application icon as the main app.
- Replace the standalone updater's themed UI with native progress and error dialogs, removing its Fyne dependency. Preserve update validation, rollback, and restart; closing progress does not interrupt installation.

## 0.9.4

- Omit the unused Mimi reference-audio encoder from release packages, saving approximately 38 MiB of installed space. Load it only when encoding reference audio; saved voice playback requires no encoder.
- Use maximum DEFLATE compression for release ZIPs, including portable packages, while retaining standard ZIP and updater compatibility.
- Reduce release size by including only PocketTTS preset voices referenced by `tts/voices.json`. The current custom-only configuration excludes all eight default presets, saving approximately 52 MiB of installed space.
- Strip debug information and symbol tables from the Windows release executable.
- Reduce macOS runtime libraries to the release's target architecture before signing, preserving library loader names and updater compatibility.

## 0.9.3

- Use DXGI desktop capture on Windows 10 to avoid the yellow capture border. Capture starts only with a visible window owned by the configured WoW executable and releases when it becomes unavailable. Keep the data square uncovered; only the requested game-relative crop reaches the decoder. Windows 11 keeps Windows Graphics Capture.

## 0.9.2

- Normalize all-caps words and remove matching stutter prefixes before PocketTTS synthesis (`GET OUT!` → `Get out!`, `T-That's` → `That's`). Preserve listed WoW acronyms, Roman numerals, identifiers, and ordinary hyphenated words.
- Normalize curly quotes and apostrophes, convert `…` to `...`, and collapse repeated exclamation/question marks. Preserve mixed punctuation such as `?!` and keep ellipsis dots together.
- Centralize speech text replacements and acronym exceptions in `internal/speech/normalize.go`; keep displayed dialogue unchanged. Add combined-dialogue, chunking, narrator-routing, and fuzz tests.

## 0.9.1

- Enable **Queue new dialogue** by default when no preference has been saved. Preserve existing queue settings.
- Refresh the README with current setup instructions and playback controls.

## 0.9.0

- Read text enclosed in `<...>` with the narrator voice, omitting the brackets from speech. Resume the NPC's voice for surrounding dialogue.
- Support multiple and multiline narrator passages in the same utterance while keeping playback continuous.

## 0.8.9

- Fix **Stop all audio** acting like Skip: Stop now cancels the current line and clears queued dialogue; Skip cancels only the current line and advances the queue.
- Apply the distinction to desktop buttons, WoW slash commands, minimap clicks, and keybindings. New dialogue can still play after Stop.

## 0.8.8

- Use dedicated addon logo artwork for the minimap button.
- Enlarge the minimap icon from 20×20 to 26×26 and add a circular mask to fill the border without showing square corners.

## 0.8.7

- Replace the generic book minimap icon with Forever Dubbed artwork, bundled as a transparent TGA texture.

## 0.8.6

- Regenerate the Windows executable's compiled icon resource to match the updated artwork.
- Add a build test that detects an outdated compiled icon after the source ICO changes.

## 0.8.5

- Replace `%s` placeholders in world emotes with the speaker's name, fixing narration such as “Percent attempts to run away in fear.” Read emotes with the narrator voice.
- Add narrator introductions such as “Thrall says,” “yells,” or “whispers” before NPC dialogue. Keep the NPC's resolved voice for the dialogue itself and avoid repeating names in system speech.
- Add a saved **Voice volume** slider below Read aloud, from 0–100%. Apply changes during native playback; system speech uses the selected level for the next utterance.

## 0.8.4

- Bundle the addon with the desktop app and automatically install or update it in the selected game's `Interface/AddOns` directory. Preserve newer addon versions and unrelated files.
- Discover WoW Forever in standard Windows locations and the macOS Applications folder. Add a prominent **Locate WoW…** button and native file picker when the game cannot be found.
- Report the loaded addon version through the data square, exposed as the compact `v` JSON field. Extend FDB5 with flags 4 while retaining support for older messages.
- Announce installation and update instructions with the narrator voice. Show Restart only for a fresh installation and `/reload` for an existing addon update; clear the notice when the current version is received.
- Check installed files even while WoW is closed, avoiding stale reload notices when the addon is already current. Show permission recovery instructions with the destination folder and Windows administrator guidance.
- Improve addon notices with readable text and a red information icon. Rename status labels to **Game capture working**, **To reposition: /fdb unlock**, and **Non-quest dialog**.
- Restore automatic hiding of the locked data square after dialogue or startup version reporting expires. Keep the unlocked square visible for positioning.

## 0.8.2

- Restyle update prompts, progress displays, and error dialogs to match the desktop theme, with readable text and consistently positioned action buttons.

## 0.8.1

- Fix the WoW Forever Key Bindings section header displaying `HEADER_FOREVERDUBBED`; use the modern Category attribute while retaining the legacy header for older clients.
- Label addon actions **Skip current audio** and **Stop all audio** in Key Bindings and the minimap tooltip.
- Refine the application icon and desktop banner sizing, and improve build caching.

## 0.8.0

- Use the new Forever Dubbed banner for the centered desktop header and the square logo for window/tray icons, the Windows executable icon, and the macOS app bundle icon.
- Add a desktop **Voices** tab with a Male and Female dropdown for every race: Default (the `tts/voices.json` mapping), Narrator, or None. Selections persist in the desktop preferences and apply immediately, including stopping and pruning silenced active and queued speech. None also silences explicit voice and NPC overrides, and applies to the system speech backend.
- Remove Blood Elf and Draenei voice profiles; neither race is in classic WoW. `tts/voices.json` now ships 21 profiles (10 races × 2 genders plus the narrator), all custom voice states.

## 0.7.1

- Fix release packaging still requiring the removed Windows `.cmd` launcher. Launch the portable app directly with `foreverdubbed.exe`.

## 0.7.0

- Add Windows installer packaging and reorganize build tools, runtime files, and voice assets.
- Use addon metadata as the development version source and release tags for packaged builds. Generate release notes from commits since the previous release.
- Improve native speech generation to prevent short utterances ending before speech begins, and normalize sentence endings around punctuation.
- Improve voice cloning tools and handling of short voice references; standardize asset paths and strengthen update package validation.
- Refresh Night Elf voice states.

## 0.6.7

- Check GitHub releases at desktop startup on Windows and macOS. Confirm updates with OK, show download and installation progress, then restart automatically. Verify release checksums, reject unsafe archives, preserve macOS signing identity, and roll back failed file replacements.
- Include an independent updater and release manifest in app packages. Publish the macOS ZIP alongside the DMG for updates; stamp tagged builds with their release version.
- Replace bundled voice configuration and states during updates; keep desktop preferences. The WoW addon still installs separately.

## 0.6.6

- Add per-voice `fade_in_ms` and `fade_out_ms` settings for generated sentence boundaries. Enable 50 ms in and 100 ms out only for Undead male/female to soften abrupt reverb endings. Preserve audio length, decoding steps, and existing extra frames after EOS.
- Compatible with the 0.6.5 addon; no addon update is required.

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
