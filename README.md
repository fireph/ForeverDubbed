<p align="center">
  <img src="https://raw.githubusercontent.com/fireph/ForeverDubbed/main/internal/appicon/assets/forever-dubbed-banner.png" alt="ForeverDubbed" width="640">
</p>

<p align="center">
  <strong>NPC voices and quest narration for WoW Forever.</strong><br>
  <a href="https://github.com/fireph/ForeverDubbed/releases/latest">Download</a> ·
  <a href="CHANGELOG.md">What’s new</a> ·
  <a href="https://github.com/fireph/ForeverDubbed/issues">Report a problem</a>
</p>

Have you always wondered what WoW would be like with full voice acting? Wonder no more! Using text-to-speed models that run locally on your computer, Forever Dubbed generates audio for any quest, conversation, or dialog in the game!

Features unique voices for each race/gender and it will even will say your character's name.

Try it out and experience Azeroth like never before!

## Install

| Platform | Download | Requirements |
| --- | --- | --- |
| Windows | [Installer](https://github.com/fireph/ForeverDubbed/releases/latest/download/ForeverDubbed-windows-amd64-setup.exe) · [Portable ZIP](https://github.com/fireph/ForeverDubbed/releases/latest/download/ForeverDubbed-windows-amd64-portable.zip) | Windows 10 (1903+) or Windows 11, 64-bit |
| macOS | [DMG](https://github.com/fireph/ForeverDubbed/releases/latest/download/ForeverDubbed-mac-arm64.dmg) | macOS 14+, Apple Silicon |

**Windows:** run the installer, then launch ForeverDubbed from the Start menu. For the portable version, extract the whole ZIP and run `foreverdubbed.exe`; keep the extracted files together.

**macOS:** open the disk image and drag **ForeverDubbed.app** into **Applications**. Grant **Screen Recording** permission in **System Settings → Privacy & Security**, then quit and reopen the app.

Both downloads include the addon. You can also [download the addon separately](https://github.com/fireph/ForeverDubbed/releases/latest/download/ForeverDubbed-addon.zip).

## Connect to WoW

1. Open ForeverDubbed. It finds WoW Forever in the usual Windows locations or `/Applications/World of Warcraft/_classic_beta_/World of Warcraft Beta.app` on macOS and installs or updates its bundled addon automatically. If needed, select `WowB.exe` (Windows) or `World of Warcraft Beta.app` (macOS) in the native file picker. Your selection is remembered.
2. Follow the narrator and banner: restart WoW for a fresh install, or type **`/reload`** for an update. Enable the addon in WoW’s AddOns list and keep WoW in **windowed or borderless mode**. After an install or update, the banner clears when the square reports the current addon version. On app startup, current addon files on disk need no notice, even with WoW closed.
3. Type **`/fdb test`** in WoW. The companion should show **Connected** and read the test aloud.
4. Use **`/fdb unlock`** to drag the small data square somewhere unobstructed, then **`/fdb lock`** to finish.

The square passes dialogue to the companion. Keep its entire border visible and your pointer off it; it hides after sending text (normally after 15 seconds). While unlocked for positioning, it stays visible until you lock it again.

Use **Locate WoW…** to change the installation or retry. If access is denied, run the Windows companion as administrator and retry, or manually copy the bundled **ForeverDubbed** addon folder into the exact `Interface/AddOns` location shown in the error banner. The bundled folder is beside the Windows executable under `addon`; on macOS, use **Show Package Contents → Contents/Resources/addon** in Finder. Installation errors do not trigger a success announcement.

## Make it yours

- **Settings:** choose quest dialogue, NPC conversations, and ambient NPC speech. Quest titles and objectives are optional and off by default. The **Voice volume** slider below Read aloud adjusts playback from 0–100% and remembers your setting. Changes apply during native voice playback; the system speech fallback applies them to the next utterance.
- **Voices:** choose **Default**, **Narrator**, or **None** for each race and gender. **None** silences that selection, including speech already playing or queued. Your choices are saved.
- **Queue new dialogue:** finish each message before starting the next. When off, new dialogue interrupts the current line.
- **Stop** cancels the current line and clears queued dialogue. **Skip** cancels the current line and plays the next queued message. New dialogue can still play after Stop. In WoW, use `/fdb stop` or `/fdb skip`, or the minimap button: **left-click to Skip**, **right-click to Stop**. Shortcuts can be assigned under **Key Bindings → ForeverDubbed**.

Use **Minimize to tray** to keep listening in the background. Reopen the app from its Windows tray or macOS menu-bar icon; choose **Quit** to exit.

## Need help?

| Problem | Try this |
| --- | --- |
| No data square | Check that the addon is enabled, then use `/fdb on`, `/fdb reset`, and `/fdb test`. |
| Square visible, but not connected | Keep its border clear, use windowed/borderless mode, and try `/fdb cell 3`. On macOS, check Screen Recording permission. |
| Connected, but silent | Check your audio output, enabled dialogue categories, and whether the voice is set to **None**. |
| Dialogue keeps interrupting | Enable **Queue new dialogue**. Use `/fdb chat` to toggle ambient NPC speech. |

More help: [Windows guide](docs/windows.md) · [macOS guide](docs/macos.md).

The app checks for updates at startup and asks before installing. Your preferences are kept. **Update the WoW addon separately** by copying the new addon folder into `Interface/AddOns`; keep the app and addon up to date together.
