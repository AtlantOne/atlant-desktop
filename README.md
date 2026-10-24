# Atlant Desktop

Windows graphical client for [Atlant Core](https://github.com/AtlantOne/atlant-core).
Linux/macOS GUI support is not claimed by this repository.

## Build on Windows

```powershell
New-Item -ItemType Directory -Force artifacts/current
Push-Location client/windows/gui
go run github.com/akavel/rsrc@v0.10.2 -arch amd64 -manifest app.manifest -o rsrc.syso
Pop-Location
go -C client/windows/gui build -trimpath -ldflags="-H=windowsgui" -o ../../../artifacts/current/atlant-gui.exe .
```

Build the Windows core from a compatible `atlant-core` checkout. Put
`atlant-win-fixed.exe` and the official `wintun.dll` next to `atlant-gui.exe`.
Do not put personal connection profiles, keys, or the Wintun binary in source
control. The GUI and core have independent build modules.

Original GUI sources are MIT licensed; see `LICENSE`. Dependency terms remain
unchanged. See `THREAT_MODEL.md` for experimental protocol limitations.
