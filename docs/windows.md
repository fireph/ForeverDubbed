# Windows game-window capture

The Windows companion selects the game window owned by **WoWB.exe** using process/window metadata. It requires Windows 10 version 1903 or newer (Windows 11 is supported).

- **Windows 10:** uses DXGI Desktop Duplication to avoid the yellow capture border. It starts capture only after finding a visible, non-minimized window owned by the configured game executable. It captures the monitor(s) intersecting that window and returns only the requested game-relative area to the decoder. Other windows covering that area are visible to capture, so keep the data square uncovered. Capture is released when the reader detects that the game has closed, minimized, or become unavailable. No desktop capture starts while waiting for the game.
- **Windows 11:** uses the existing Windows Graphics Capture backend, which captures the game window independently of other windows covering it. It does not switch to desktop capture if window capture fails.

Backend selection uses the actual OS version: Windows 10 client builds 18362–21999 use DXGI; Windows 11 and Server remain on Windows Graphics Capture. The desktop backend handles monitor offsets, rotated displays, games spanning monitors, and separate graphics adapters. Display changes or loss of access reset capture so the next read can reacquire it.

Run the prepared release to open the status dashboard:

```powershell
.\foreverdubbed.exe
```

Close or minimize the dashboard to keep it running in the system tray. Use **Show ForeverDubbed** to restore it or **Quit** to stop capture and audio. Use `-headless` for terminal-only capture and JSON output.

Quest speech reads the main dialogue by default. **Quest title** adds the title before the dialogue; **Quest objectives** adds objectives after it. Both default to off and are saved independently between launches. Changes apply when the next quest starts speaking, including quests already queued. Both Pocket TTS and system voices follow these settings. Headless mode uses the default of dialogue only.

Install the 0.6.5 addon as well as the desktop app for this feature, then `/reload` WoW. The addon sends the title, dialogue, and objectives separately; the desktop retains them all and chooses which text to speak. Older addons merge objectives into the dialogue, so the desktop cannot reliably exclude them until the addon is updated.

The executable filename is matched case-insensitively against the owning process's full image path. A window title alone cannot match, and `WoW.exe` is not selected. To target a particular installation, use its full executable path:

```powershell
.\foreverdubbed.exe -capture-app "C:\Games\World of Warcraft\_classic_beta_\WoWB.exe"
```

When the game is closed or minimized, the reader waits. It reacquires the window after it reopens or changes size. If two matching processes are running, specify the exact executable path or close the other instance. Window coordinates and snapshots are relative to the captured game window, not the desktop.

On Windows 11, the yellow outline is Windows' capture indicator. ForeverDubbed requests permission to hide it using the supported borderless-capture API (Windows build 20348 or newer, including Windows 11). Allow the Windows permission prompt if one appears. Capture continues while permission is pending; the outline disappears once access is granted. The request is made once per reader, and the setting is reapplied when the game window is reacquired. Restart ForeverDubbed after changing capture permissions in Windows.

On the Windows Graphics Capture backend, denied permission or another application capturing the same window with its border enabled can leave the outline visible. Windows 10 uses DXGI instead, so ForeverDubbed does not create that indicator; another application may still display one. These conditions do not prevent ForeverDubbed from reading the game. See Microsoft's [capture-border documentation](https://learn.microsoft.com/en-us/uwp/api/windows.graphics.capture.graphicscapturesession.isborderrequired).

Use windowed or borderless mode. Exclusive fullscreen, a minimized game, or a game that disables capture may not produce frames; the reader reports that condition. Windows 10 desktop capture requires the data square to be visible on screen. If the game is elevated and cannot be discovered, run the companion at the same privilege level. HDR or color filters can affect the optical palette; use SDR when diagnosing decoding failures.

`-snapshot` saves the selected game bounds. On Windows 10 this is a crop of the visible desktop and can include windows covering the game:

```powershell
.\foreverdubbed.exe -mute
.\foreverdubbed.exe -snapshot capture.png
.\foreverdubbed.exe -image capture.png
```

Normal captures stay in memory. Only an explicit `-snapshot` writes a PNG. On Windows 11, cover the game with another application to verify that window capture excludes it. On Windows 10, covering the data square should stop decoding until it is uncovered; snapshot pixels within the game bounds will include the covering application.

## Build and test

In `tts/voices.json`, `fade_in_ms` and `fade_out_ms` set optional Pocket TTS fades at each generated sentence boundary. Both accept 0–500 milliseconds; omitted or `0` disables that fade. Undead male and female use `"fade_in_ms": 50` and `"fade_out_ms": 100`; other profiles have no fades. Restart after editing. Fades preserve audio length and apply across decoder/playback buffers, not separately to each buffer. The final fade-out window is held back until its sentence ends. No voice regeneration is needed. The native engine's existing extra frames after EOS remain unchanged (3 normally, 5 for sentences of four words or fewer).

To adjust one Pocket TTS voice's playback volume, set `gain_db` in its profile in `tts/voices.json`, then restart the app. Tauren male starts at `4` (+4 dB); `0` or an omitted setting uses the original volume. Values from -24 to +12 dB are supported. This adjusts desktop playback without regenerating the voice or changing timing. Peaks are capped at the PCM limits, so reduce the gain if loud passages become distorted. Exported example clips do not use this playback setting.

Follow the [Windows build instructions](../native/README.md#build) to prepare dependencies and package this backend, including from Linux with MinGW-w64. Capture requires cgo and a Windows C/C++ compiler, even for a system-voice-only build. No extra capture DLL, Windows SDK download, C++/WinRT package, or Go module is needed. The small WinRT ABI declarations in `internal/platform/wgc_abi_windows.h` let the code compile with MinGW.

For an opt-in live regression test on Windows with the game open:

```powershell
$env:FDB_TEST_CAPTURE_APP = "WoWB.exe"
$env:CGO_ENABLED = "1"
go test ./internal/platform -run TestWindowsGameWindowCapture -count=1
```

On both Windows 10 and Windows 11, also check closing/reopening, minimizing/restoring, resizing, and moving the game between monitors. On Windows 10, verify no capture starts before WoW is open, the yellow capture border is absent, and closing/minimizing WoW releases desktop duplication. Test portrait monitors, negative monitor coordinates, a window spanning monitors, and lock/unlock or display-mode changes. On Windows 11, verify the existing window-capture behavior and border-permission handling remain unchanged. Linux builds and portable tests validate compilation, process matching, and recovery logic; they cannot exercise the Windows compositor or a live game.
