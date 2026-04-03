#!/usr/bin/env bash
set -euo pipefail

# -------------------------------------------------------------------
# Install Claude Code CLI and configure managed settings (hooks).
#
# Installs the CLI via the official installer and writes the managed
# settings JSON that registers Claude Code hooks for event tracking.
#
# The JSONL session parser is the primary data source for most events
# (tool use, cost, prompts, errors). Hooks are used for two purposes:
#
# 1. Real-time attention state (not in JSONL):
#    - Notification, PreToolUse, UserPromptSubmit
#
# 2. Audit events not available in JSONL:
#    - SessionStart, SessionEnd, PermissionRequest, ConfigChange,
#      InstructionsLoaded, TaskCompleted, Elicitation, ElicitationResult,
#      SubagentStart, SubagentStop
#
# Idempotent: skips CLI install if claude is already available.
# -------------------------------------------------------------------

if ! su - warden -c "which claude" >/dev/null 2>&1; then
  su - warden -c "curl -fsSL https://claude.ai/install.sh | bash"
fi

# -------------------------------------------------------------------
# Claude Code managed settings — hooks for attention state and audit events.
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
    ],
    "PermissionRequest": [
      {
        "matcher": "",
        "hooks": [
          {
            "type": "command",
            "command": "/usr/local/bin/warden-event-claude.sh permission_request"
          }
        ]
      }
    ],
    "ConfigChange": [
      {
        "matcher": "",
        "hooks": [
          {
            "type": "command",
            "command": "/usr/local/bin/warden-event-claude.sh config_change"
          }
        ]
      }
    ],
    "InstructionsLoaded": [
      {
        "matcher": "",
        "hooks": [
          {
            "type": "command",
            "command": "/usr/local/bin/warden-event-claude.sh instructions_loaded"
          }
        ]
      }
    ],
    "TaskCompleted": [
      {
        "matcher": "",
        "hooks": [
          {
            "type": "command",
            "command": "/usr/local/bin/warden-event-claude.sh task_completed"
          }
        ]
      }
    ],
    "SessionStart": [
      {
        "matcher": "",
        "hooks": [
          {
            "type": "command",
            "command": "/usr/local/bin/warden-event-claude.sh session_start"
          }
        ]
      }
    ],
    "SessionEnd": [
      {
        "matcher": "",
        "hooks": [
          {
            "type": "command",
            "command": "/usr/local/bin/warden-event-claude.sh session_end"
          }
        ]
      }
    ],
    "Elicitation": [
      {
        "matcher": "",
        "hooks": [
          {
            "type": "command",
            "command": "/usr/local/bin/warden-event-claude.sh elicitation"
          }
        ]
      }
    ],
    "ElicitationResult": [
      {
        "matcher": "",
        "hooks": [
          {
            "type": "command",
            "command": "/usr/local/bin/warden-event-claude.sh elicitation_result"
          }
        ]
      }
    ],
    "SubagentStart": [
      {
        "matcher": "",
        "hooks": [
          {
            "type": "command",
            "command": "/usr/local/bin/warden-event-claude.sh subagent_start"
          }
        ]
      }
    ],
    "SubagentStop": [
      {
        "matcher": "",
        "hooks": [
          {
            "type": "command",
            "command": "/usr/local/bin/warden-event-claude.sh subagent_stop"
          }
        ]
      }
    ]
  }
}
MANAGED_EOF
