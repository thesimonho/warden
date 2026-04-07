#!/usr/bin/env bash
# Generate all platform icons from icon.svg (square) and copy logo.svg (wide).
# Called by the icons CI workflow on icon.svg/logo.svg changes.
# Can also be run locally: bash packaging/generate-icons.sh
set -euo pipefail

# Resolve repo root relative to this script.
REPO_ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "${REPO_ROOT}"

ICON="icon.svg"
LOGO="logo.svg"
for f in "${ICON}" "${LOGO}"; do
    if [ ! -f "${f}" ]; then
        echo "Missing ${f}" >&2
        exit 1
    fi
done

# Rasterize square icon SVG to a high-res intermediate PNG for crisp downscaling.
# Uses a solid white background so the icon is readable on all platforms.
# Forces RGBA sRGB output — without this ImageMagick optimizes to 8-bit
# grayscale (since the SVG is black-on-white), which causes macOS to
# render the dock icon with a grey border instead of edge-to-edge white.
HI="$(mktemp /tmp/warden-icon-1024.XXXXXX.png)"
trap 'rm -f "${HI}"' EXIT
magick -density 384 -background white "${ICON}" -flatten -resize 1024x1024 \
    -colorspace sRGB -type TrueColorAlpha -depth 8 -define png:color-type=6 \
    "${HI}"

# All downstream resizes must also force RGBA, otherwise ImageMagick
# re-optimizes to grayscale on output.
RGBA="-define png:color-type=6"

# Linux — multi-size PNGs for hicolor icon theme (AppImage, .desktop)
# The 512px copy at packaging/linux/warden.png is kept for nfpm and
# backward compatibility; the sized variants go into icons/ for the
# AppImage's hicolor theme so desktop environments don't have to
# downscale from 512 (which produces blurry taskbar/panel icons).
for size in 16 32 48 64 128 256 512; do
    mkdir -p "packaging/linux/icons/${size}x${size}"
    magick "${HI}" -resize "${size}x${size}" ${RGBA} "packaging/linux/icons/${size}x${size}/warden.png"
done
cp "packaging/linux/icons/512x512/warden.png" packaging/linux/warden.png

# Tray icon — black on transparent for macOS menu bar template image.
# Generated directly from SVG (no white background) so systray.SetTemplateIcon
# can let macOS handle light/dark mode. 64px RGBA; the library scales as needed.
magick -density 384 -background none "${ICON}" -resize 64x64 \
    -colorspace sRGB -type TrueColorAlpha -depth 8 ${RGBA} cmd/warden-tray/icon.png

# Windows — multi-size .ico
magick "${HI}" \
    \( -clone 0 -resize 16x16 \) \
    \( -clone 0 -resize 32x32 \) \
    \( -clone 0 -resize 48x48 \) \
    \( -clone 0 -resize 64x64 \) \
    \( -clone 0 -resize 128x128 \) \
    \( -clone 0 -resize 256x256 \) \
    -delete 0 packaging/windows/warden.ico

# macOS — iconset (converted to .icns by bundle.sh on macOS)
mkdir -p packaging/macos/warden.iconset
for pair in "16x16:icon_16x16" "32x32:icon_16x16@2x" "32x32:icon_32x32" \
            "64x64:icon_32x32@2x" "128x128:icon_128x128" "256x256:icon_128x128@2x" \
            "256x256:icon_256x256" "512x512:icon_256x256@2x" "512x512:icon_512x512" \
            "1024x1024:icon_512x512@2x"; do
    size="${pair%%:*}"
    name="${pair#*:}"
    magick "${HI}" -resize "${size}" ${RGBA} "packaging/macos/warden.iconset/${name}.png"
done

# Web — favicon and PWA icons (served from web/public/)
mkdir -p web/public
magick "${HI}" -resize 32x32 web/public/favicon.ico
magick "${HI}" -resize 180x180 ${RGBA} web/public/apple-touch-icon.png
magick "${HI}" -resize 192x192 ${RGBA} web/public/favicon-192.png
magick "${HI}" -resize 512x512 ${RGBA} web/public/favicon-512.png

# Copy SVGs for direct use in web UI (transparent, theme-adapted via CSS)
cp "${ICON}" web/public/icon.svg
cp "${LOGO}" web/public/logo.svg

# Docs site — shares the same favicon as the web app
cp web/public/favicon.ico docs/site/public/favicon.ico

# Docs site — light/dark logo variants for Starlight theme
cp "${LOGO}" docs/site/src/assets/logo-light.svg
sed 's/#262626/#ffffff/g' "${LOGO}" > docs/site/src/assets/logo-dark.svg

echo "Generated icons for Linux, Windows, macOS, docs, and web"
