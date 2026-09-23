# Windows game-window capture

The Windows companion captures only the game window owned by **WoWB.exe**, using Windows Graphics Capture and Direct3D 11. It requires Windows 10 version 1903 or newer (Windows 11 is supported). There is no desktop capture fallback. Window discovery reads process/window metadata, then binds capture to that game's HWND.

Run the prepared release normally:

```powershell
.\foreverdubbed.exe
```

The executable filename is matched case-insensitively against the owning process's full image path. A window title alone cannot match, and `WoW.exe` is not selected. To target a particular installation, use its full executable path:

```powershell
.\foreverdubbed.exe -capture-app "C:\Games\World of Warcraft\_classic_beta_\WoWB.exe"
```

When the game is closed or minimized, the reader waits. It reacquires the window after it reopens or changes size. If two matching processes are running, specify the exact executable path or close the other instance. Window coordinates and snapshots are relative to the captured game window, not the desktop. Windows may draw a capture indicator around the game.

Use windowed or borderless mode. Exclusive fullscreen, a minimized game, or a game that disables capture may not produce frames; the reader reports that condition without capturing the desktop instead. If the game is elevated and cannot be discovered, run the companion at the same privilege level. HDR or color filters can affect the optical palette; use SDR when diagnosing decoding failures.

`-snapshot` captures only the selected game window:

```powershell
.\foreverdubbed.exe -mute
.\foreverdubbed.exe -snapshot capture.png
.\foreverdubbed.exe -image capture.png
```

Normal captures stay in memory. Only an explicit `-snapshot` writes a PNG. Cover the game with another application while testing a snapshot to verify that the other application's content is excluded.

## Build and test

Follow the [Windows build instructions](../native/README.md#build) to prepare dependencies and package this backend, including from Linux with MinGW-w64. Capture requires cgo and a Windows C/C++ compiler, even for a system-voice-only build. No extra capture DLL, Windows SDK download, C++/WinRT package, or Go module is needed. The small WinRT ABI declarations in `internal/platform/wgc_abi_windows.h` let the code compile with MinGW.

For an opt-in live regression test on Windows with the game open:

```powershell
$env:FDB_TEST_CAPTURE_APP = "WoWB.exe"
$env:CGO_ENABLED = "1"
go test ./internal/platform -run TestWindowsGameWindowCapture -count=1
```

Also check closing/reopening, minimizing/restoring, resizing, and moving the game between monitors. Linux builds and portable tests validate compilation, process matching, and recovery logic; they cannot exercise the Windows compositor or a live game.
