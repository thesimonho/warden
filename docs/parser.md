# JSONL Session Parser

The JSONL session parser is Warden's primary data source for agent events. Both Claude Code and Codex write session activity to JSONL files — Warden tails these files in real-time and converts them into agent-agnostic events for the SSE/audit pipeline.

## Architecture

```
Container writes JSONL session line
  → Host-side SessionWatcher detects file change (fsnotify + 2s polling)
  → SessionParser.ParseLine() converts to []ParsedEvent
  → SessionEventToContainerEvent() bridges to eventbus.ContainerEvent
  → eventbus pipeline broadcasts SSE + writes audit
```

Each project has one `SessionWatcher` (started when the container starts, stopped when it stops). The watcher monitors the host-mounted agent config directory for new JSONL lines.

## SessionParser Interface

```go
type SessionParser interface {
    // ParseLine parses a single JSONL line into zero or more Warden events.
    ParseLine(line []byte) []ParsedEvent

    // SessionDir returns the host-side directory to watch for session files.
    SessionDir(homeDir string, project ProjectInfo) string
}
```

Parsers are **stateful** — they accumulate token counts across lines within a session. Create a new parser per session file.

## ParsedEvent Types

| Type | Meaning | Key Fields |
|------|---------|------------|
| `session_start` | Agent session began | SessionID, Model, GitBranch, WorktreeID |
| `session_end` | Agent session ended | SessionID |
| `tool_use` | Agent invoked a tool | ToolName, ToolInput (truncated) |
| `user_prompt` | User sent a message | Prompt |
| `turn_complete` | Agent finished a turn | SessionID |
| `turn_duration` | Turn timing data | DurationMs |
| `token_update` | Cumulative token usage | Tokens, EstimatedCostUSD |

## Agent Implementations

### Claude Code (`agent/claudecode/`)

- **Session dir:** `~/.claude/projects/<sanitized-path>/`
- **JSONL format:** `SessionEntry` with `type` field (init, summary, user, assistant, result)
- **Token source:** `cacheCreationInputTokens`, `cacheReadInputTokens`, `inputTokens`, `outputTokens` in `usage` blocks
- **Cost:** Estimated from token counts via per-model pricing table (`pricing.go`)
- **Tool extraction:** From `tool_use` content blocks in assistant messages

### Codex (`agent/codex/`)

- **Session dir:** `~/.codex/sessions/<session-id>/`
- **JSONL format:** `RolloutItem` with `type` field (session_meta, response_item.created, event_msg)
- **Token source:** `total_tokens`, `input_tokens`, `output_tokens` in `token_count_info` and rate limit headers
- **Cost:** Estimated from token counts via OpenAI model pricing table (`pricing.go`)
- **Tool extraction:** From `function_call` response items

## Hooks vs JSONL

JSONL is the **primary** data source for all event types: session lifecycle, tool use, cost, prompts, and turn completion. Claude Code hooks are a **supplementary** channel used only for attention/notification state (permission prompts, idle state, elicitation dialogs). Codex does not support hooks — attention tracking for Codex is a known upstream gap.

| Data | Source |
|------|--------|
| Session lifecycle | JSONL |
| Tool use | JSONL |
| Cost / tokens | JSONL |
| User prompts | JSONL |
| Turn completion | JSONL |
| Attention state (Claude) | Hooks (supplementary) |
| Attention state (Codex) | Not available (upstream gap) |

## Validation

`agent.ValidateJSONL(parser, reader)` validates JSONL lines and returns event counts. Used by:

- Parser unit tests (`agent/claudecode/parser_test.go`, `agent/codex/parser_test.go`)
- CI scheduled validation (`TestValidateLive`, env-gated via `VALIDATE_JSONL`)

## Adding a New Agent

1. Create `agent/<name>/` subpackage
2. Implement `SessionParser` (parser.go) with `ParseLine()` and `SessionDir()`
3. Implement `StatusProvider` (provider.go) with `Name()`, `ProcessName()`, `ConfigFilePath()`, `ExtractStatus()`, `NewSessionParser()`
4. Add pricing table if cost estimation is needed
5. Add install script in `container/scripts/install-<name>.sh`
6. Add event handler in `container/scripts/<name>/`
7. Register in `agent/registry.go` and wire in `warden.go`
