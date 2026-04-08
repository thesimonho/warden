package main

import _ "embed"

// iconData is the 256x256 RGBA PNG tray icon (black on transparent).
// Used on macOS (template icon) and Linux/Windows light themes.
// Regenerated from icon.svg by packaging/generate-icons.sh.
//
//go:embed icon.png
var iconData []byte

// iconDataLight is the 256x256 RGBA PNG tray icon (white on transparent).
// Used on Linux/Windows dark themes where the black icon is invisible.
// Regenerated from icon.svg by packaging/generate-icons.sh.
//
//go:embed icon_light.png
var iconDataLight []byte

// iconDataAttention is the dark icon with an orange attention dot overlay.
// Used when any project needs user attention (e.g. permission prompt).
//
//go:embed icon_attention.png
var iconDataAttention []byte

// iconDataAttentionLight is the light icon with an orange attention dot overlay.
// Used on dark themes when any project needs user attention.
//
//go:embed icon_attention_light.png
var iconDataAttentionLight []byte
