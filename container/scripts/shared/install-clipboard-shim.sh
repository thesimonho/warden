#!/usr/bin/env bash
set -euo pipefail

# -------------------------------------------------------------------
# Install clipboard shim for web terminal image paste support.
#
# Creates an xclip wrapper at ~/.local/bin/xclip that intercepts
# clipboard read operations. When a staged image exists (uploaded by
# the web frontend via the clipboard API), the shim returns it.
# All other xclip calls pass through to the real binary.
#
# This enables transparent image paste: the browser uploads the image
# to /tmp/warden-clipboard/, then sends Ctrl+V to the PTY. The agent
# (Claude Code, Codex) calls xclip to read the clipboard and gets
# the staged image via this shim.
#
# The staging directory and shim are agent-agnostic — any tool that
# reads the clipboard via xclip will pick up staged content.
# -------------------------------------------------------------------

CLIPBOARD_DIR="/tmp/warden-clipboard"
SHIM_PATH="/home/warden/.local/bin/xclip"

# Create the staging directory.
mkdir -p "$CLIPBOARD_DIR"
chown warden:warden "$CLIPBOARD_DIR"

# Find the real xclip binary (skip our shim via PATH search).
REAL_XCLIP=""
for dir in /usr/bin /usr/local/bin; do
  if [ -x "${dir}/xclip" ]; then
    REAL_XCLIP="${dir}/xclip"
    break
  fi
done

# Write the shim script.
cat > "$SHIM_PATH" << 'SHIM'
#!/usr/bin/env bash
# Warden clipboard shim — intercepts xclip calls for web terminal
# image paste support. Staged images in /tmp/warden-clipboard/ are
# served to agents that check the clipboard via xclip.

CLIPBOARD_DIR="/tmp/warden-clipboard"
ARGS="$*"

# --- TARGETS check: report available MIME types ---
# Claude Code calls: xclip -selection clipboard -t TARGETS -o
if [[ "$ARGS" == *"-selection clipboard"*"-t TARGETS"*"-o"* ]] || \
   [[ "$ARGS" == *"-selection clipboard"*"-o"*"-t TARGETS"* ]]; then
  # Check for staged image file (newest first).
  staged=$(find "$CLIPBOARD_DIR" -maxdepth 1 -type f -name '*.png' -o -name '*.jpg' -o -name '*.jpeg' -o -name '*.gif' -o -name '*.webp' -o -name '*.bmp' 2>/dev/null | sort -r | head -1)
  if [ -n "$staged" ]; then
    ext="${staged##*.}"
    case "$ext" in
      jpg) ext="jpeg" ;;
    esac
    echo "image/${ext}"
    exit 0
  fi
  # No staged image — fall through to real xclip.
  :
fi

# --- Image read: return staged image data ---
# Claude Code calls: xclip -selection clipboard -t image/<fmt> -o
if [[ "$ARGS" == *"-selection clipboard"*"-t image/"*"-o"* ]] || \
   [[ "$ARGS" == *"-selection clipboard"*"-o"*"-t image/"* ]]; then
  staged=$(find "$CLIPBOARD_DIR" -maxdepth 1 -type f -name '*.png' -o -name '*.jpg' -o -name '*.jpeg' -o -name '*.gif' -o -name '*.webp' -o -name '*.bmp' 2>/dev/null | sort -r | head -1)
  if [ -n "$staged" ]; then
    cat "$staged"
    rm -f "$staged"
    exit 0
  fi
  # No staged image — fall through to real xclip.
  :
fi

# --- Pass through to real xclip for all other calls ---
REAL_XCLIP="__REAL_XCLIP__"
if [ -n "$REAL_XCLIP" ] && [ -x "$REAL_XCLIP" ]; then
  exec "$REAL_XCLIP" "$@"
fi

# No real xclip available — exit silently (same as missing xclip).
exit 1
SHIM

# Patch in the real xclip path (or empty string if not installed).
sed -i "s|__REAL_XCLIP__|${REAL_XCLIP}|g" "$SHIM_PATH"

chmod +x "$SHIM_PATH"
chown warden:warden "$SHIM_PATH"
