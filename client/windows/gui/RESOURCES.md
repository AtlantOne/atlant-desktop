# Windows manifest

`rsrc.syso` contains the application manifest, not deployment secrets or VPN keys.
Its source is `app.manifest`. Regenerate the resource before building after a
manifest change:

```powershell
go run github.com/akavel/rsrc@v0.10.2 -arch amd64 -manifest app.manifest -o rsrc.syso
```

The GUI requests administrator privileges for TUN setup. Unit tests using file
lists in `scripts/check.ps1` intentionally omit the executable resource.
