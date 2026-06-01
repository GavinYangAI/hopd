package guiapp

import (
	_ "embed"

	"fyne.io/fyne/v2"
)

// logoPNG is the hopd app logo (the "hop" arc with a green destination dot),
// embedded so it can be shown in the dashboard header and used as the window /
// app icon. It mirrors the repo-root Icon.png used to package hopd-gui.app.
//
//go:embed logo.png
var logoPNG []byte

// logoResource is the embedded logo as a Fyne resource.
var logoResource = fyne.NewStaticResource("hopd-logo.png", logoPNG)
