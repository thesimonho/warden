package main

import _ "embed"

// iconData is the tray icon loaded from the project's existing
// packaging icon. Systray accepts PNGs of any size and scales
// to fit the platform's tray dimensions.
//
//go:embed icon.png
var iconData []byte
