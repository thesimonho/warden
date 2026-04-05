package main

import _ "embed"

// iconData is the tray icon (256x256 PNG). Source of truth is
// packaging/linux/icons/256x256/warden.png — kept in sync by
// the `just generate-icons` recipe.
//
//go:embed icon.png
var iconData []byte
