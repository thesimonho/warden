package main

import _ "embed"

// iconData is the tray icon (64x64 RGBA PNG, black on transparent).
// Generated from icon.svg by packaging/generate-icons.sh (run
// automatically by the icons CI workflow on SVG changes).
//
//go:embed icon.png
var iconData []byte
