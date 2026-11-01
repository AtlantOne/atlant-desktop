module atlant-gui

go 1.27.2

require (
	atlant.local/remoteadmin v0.0.0
	golang.org/x/sys v0.49.0
)

require golang.org/x/crypto v0.58.0 // indirect

replace atlant.local/remoteadmin => ../../remoteadmin
