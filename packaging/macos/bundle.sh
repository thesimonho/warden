#!/usr/bin/env bash
# Assembles a macOS .app bundle from the warden-desktop binary.
# Usage: bash packaging/macos/bundle.sh [version]
#
# Expects bin/warden-desktop to already be built.
# Produces bin/Warden.app/

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "${SCRIPT_DIR}/../.." && pwd)"
VERSION="${1:-$(git -C "${ROOT_DIR}" describe --tags --always 2>/dev/null || echo "0.0.0")}"

APP_DIR="${ROOT_DIR}/bin/Warden.app"
CONTENTS="${APP_DIR}/Contents"

rm -rf "${APP_DIR}"
mkdir -p "${CONTENTS}/MacOS"
mkdir -p "${CONTENTS}/Resources"

# Binary
cp "${ROOT_DIR}/bin/warden-desktop" "${CONTENTS}/MacOS/warden"
chmod +x "${CONTENTS}/MacOS/warden"

# Info.plist with version substituted (strip leading 'v' for CFBundleVersion)
PLIST_VERSION="${VERSION#v}"
sed "s/__VERSION__/${PLIST_VERSION}/g" "${SCRIPT_DIR}/Info.plist" > "${CONTENTS}/Info.plist"

# Icon — convert from iconset on macOS, or use pre-built .icns if available
if [ -f "${SCRIPT_DIR}/warden.icns" ]; then
    cp "${SCRIPT_DIR}/warden.icns" "${CONTENTS}/Resources/warden.icns"
elif [ -d "${SCRIPT_DIR}/warden.iconset" ] && command -v iconutil >/dev/null 2>&1; then
    iconutil -c icns -o "${CONTENTS}/Resources/warden.icns" "${SCRIPT_DIR}/warden.iconset"
fi

echo "Built ${APP_DIR} (${VERSION})"
