# Atlant Desktop

Atlant's Windows app manages subscriptions, server profiles and VPN connections
through a graphical interface. It uses [Atlant Core](https://github.com/AtlantOne/atlant-core)
for both TCP and QUIC: direct REALITY or XHTTP on the TCP path, and direct
QUIC REALITY over UDP.

QUIC starts directly over UDP with key-based authentication, without a TCP
bootstrap or a CA-issued certificate for the VPN server.
Choose a transport to suit your network, or use split transport selection for
TCP and UDP traffic. TUN mode routes traffic through the Wintun adapter.

The interface supports English, Russian and Chinese. For the Linux command-line
client, see the core repository.

## Download

Download the ZIP for your Windows architecture from [Releases](https://github.com/AtlantOne/atlant-desktop/releases).
Extract the entire archive and run `bin/atlant-gui.exe`; the matching core and Wintun DLL must remain beside it.

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
unchanged. See `THREAT_MODEL.md` for the protocol's security boundaries.
