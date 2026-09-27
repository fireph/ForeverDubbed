<p align="center">
  <img src="https://raw.githubusercontent.com/fireph/ForeverDubbed/main/internal/appicon/assets/forever-dubbed-banner.png" alt="ForeverDubbed" width="640">
</p>

<p align="center">
  <strong>NPC voices and quest narration for WoW Forever.</strong><br>
  <a href="https://www.foreverdubbed.com/">Website</a> ·
  <a href="https://www.curseforge.com/wow/addons/foreverdubbed">CurseForge</a> ·
  <a href="CHANGELOG.md">What’s new</a> ·
  <a href="https://github.com/fireph/ForeverDubbed/issues">Report a problem</a>
</p>

> [!IMPORTANT]
> **The addon requires the ForeverDubbed companion app to work.** The addon cannot generate or play voices on its own.

Have you always wondered what WoW would be like with full voice acting? Wonder no more! Using text-to-speed models that run locally on your computer, Forever Dubbed generates audio for any quest, conversation, or dialog in the game!

Features unique voices for each race/gender and it will even will say your character's name.

Try it out and experience Azeroth like never before!

## Install

| Platform | Download | Requirements |
| --- | --- | --- |
| Windows | [Installer](https://github.com/fireph/ForeverDubbed/releases/latest/download/ForeverDubbed-windows-amd64-setup.exe) | Windows 10/11, 64-bit |
| macOS | [DMG](https://github.com/fireph/ForeverDubbed/releases/latest/download/ForeverDubbed-mac-arm64.dmg) | macOS 14+, Apple Silicon |

**Windows:** run the installer, then launch ForeverDubbed from the Start menu.

**macOS:** open the disk image and drag **ForeverDubbed.app** into **Applications**. Grant **Screen Recording** permission in **System Settings → Privacy & Security**, then quit and reopen the app.

The application will install the WoW addon automatically.

## Connect to WoW

1. Open ForeverDubbed. It finds WoW Forever in the usual Windows locations or `/Applications/World of Warcraft/_classic_beta_/World of Warcraft Beta.app` on macOS and installs or updates its bundled addon automatically. If needed, select your WoW client with **Locate WoW…**: usually `WowB.exe` (Windows) or `World of Warcraft Beta.app` (macOS). Renamed clients are supported; on Windows, switch the picker to **All executables**. The selected client must be inside its WoW installation, and capture uses that exact path.
2. Follow the narrator and banner: restart WoW for a fresh install, or type **`/reload`** for an update. Enable the addon in WoW’s AddOns list and keep WoW in **windowed or borderless mode**.
3. Type **`/fdb test`** in WoW. The companion should show **Connected** and read the test aloud.
4. Use **`/fdb unlock`** to drag the small data square somewhere unobstructed, then **`/fdb lock`** to finish.

On Windows 10, the companion uses desktop capture while the game is visible to avoid the yellow capture border. Other windows must not cover the data square. Windows 11 uses game-window capture.

The square passes dialogue to the companion. Keep its entire border visible and your pointer off it; it hides after sending text (after ~15 seconds). While unlocked for positioning, it stays visible until you lock it again.

Use **Locate WoW…** to change the installation or retry. If access is denied, run the Windows companion as administrator and retry, or manually copy the bundled **ForeverDubbed** addon folder into the exact `Interface/AddOns` location shown in the error banner. The bundled folder is beside the Windows executable under `addon`; on macOS, use **Show Package Contents → Contents/Resources/addon** in Finder.

## Make it yours

- **Settings:** choose quest dialogue, NPC conversations, and ambient NPC speech. Quest titles and objectives are optional and off by default. The **Voice volume** slider below Read aloud adjusts playback from 0–100% and remembers your setting. Changes apply during native voice playback; the system speech fallback applies them to the next utterance.
- **Voices:** choose **Default**, **Narrator**, or **None** for each race and gender. **None** silences that selection, including speech already playing or queued. Your choices are saved.
- **Queue new dialogue:** finish each message before starting the next. When off, new dialogue interrupts the current line.
- **Stop** cancels the current line and clears queued dialogue. **Skip** cancels the current line and plays the next queued message. New dialogue can still play after Stop. In WoW, use `/fdb stop` or `/fdb skip`, or the minimap button: **left-click to Skip**, **right-click to Stop**. Shortcuts can be assigned under **Key Bindings → ForeverDubbed**.

Only one copy of ForeverDubbed runs per user, even across different installation folders. Additional launches exit immediately.

Use **Minimize to tray** to keep listening in the background. Reopen the app from its Windows tray or macOS menu-bar icon; choose **Quit** to exit.

## Need help?

| Problem | Try this |
| --- | --- |
| No data square | Check that the addon is enabled, then use `/fdb on`, `/fdb reset`, and `/fdb test`. |
| Square visible, but not connected | Keep its border clear, use windowed/borderless mode, and try `/fdb cell 3`. On macOS, check Screen Recording permission. |
| Connected, but silent | Check your audio output, enabled dialogue categories, and whether the voice is set to **None**. |
| Dialogue keeps interrupting | Enable **Queue new dialogue**. Use `/fdb chat` to toggle ambient NPC speech. |

More help: [Windows docs](docs/windows.md) · [macOS docs](docs/macos.md).

The app checks for updates at startup and asks before installing. Your preferences are kept. **Update the WoW addon separately** by copying the new addon folder into `Interface/AddOns`; keep the app and addon up to date together.

## License

Copyright (c) 2026 fireph. All rights reserved. See [LICENSE](LICENSE).
Third-party materials remain subject to their respective licenses and notices.
