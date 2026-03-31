#!/usr/bin/env bash
set -euo pipefail

# -------------------------------------------------------------------
# Install Claude Code CLI and configure managed settings (hooks).
#
# Installs the CLI via the official installer and writes the managed
# settings JSON that registers Claude Code hooks for event tracking.
#
# With the JSONL session parser as the primary data source, only three
# hooks remain active:
#   - Notification — attention state (not in JSONL)
#   - PreToolUse — AskUserQuestion detection for attention state
#   - UserPromptSubmit — attention_clear for real-time state
#
# All other events (session lifecycle, tool use, cost, etc.) are now
# parsed from the JSONL session file by the Go backend.
#
# Idempotent: skips CLI install if claude is already available.
# -------------------------------------------------------------------

if ! su - warden -c "which claude" >/dev/null 2>&1; then
  su - warden -c "curl -fsSL https://claude.ai/install.sh | bash"
fi

# -------------------------------------------------------------------
# Claude Code managed settings — hooks for real-time attention state.
# Uses /etc/claude-code/ (Linux managed settings path) so hooks merge
# with user/project settings without overwriting.
#
# NOTE: WorktreeCreate is NOT hooked — it replaces Claude Code's
# default git worktree creation and doesn't work reliably inside
# abduco terminal sessions.
# -------------------------------------------------------------------
mkdir -p /etc/claude-code
cat > /etc/claude-code/managed-settings.json <<'MANAGED_EOF'
{
  "hooks": {
    "Notification": [
      {
        "matcher": "",
        "hooks": [
          {
            "type": "command",
            "command": "/usr/local/bin/warden-event-claude.sh notification"
          }
        ]
      }
    ],
    "UserPromptSubmit": [
      {
        "matcher": "",
        "hooks": [
          {
            "type": "command",
            "command": "/usr/local/bin/warden-event-claude.sh user_prompt_submit"
          }
        ]
      }
    ],
    "PreToolUse": [
      {
        "matcher": "",
        "hooks": [
          {
            "type": "command",
            "command": "/usr/local/bin/warden-event-claude.sh pre_tool_use"
          }
        ]
      }
    ]
  }
}
MANAGED_EOF
