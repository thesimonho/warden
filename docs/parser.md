# JSONL Session Parser

The JSONL session parser is Warden's primary data source for agent events. Both Claude Code and Codex write session activity to JSONL files — Warden tails these files in real-time and converts them into agent-agnostic events for the SSE/audit pipeline.

## Architecture

```
Container writes JSONL session line
  → Host-side SessionWatcher polls every 2s
    → SessionParser.FindSessionFiles() discovers active session files
    → SessionParser.ParseLine() converts to []ParsedEvent
  → SessionEventToContainerEvent() bridges to eventbus.ContainerEvent
  → eventbus pipeline broadcasts SSE + writes audit
```

Each project has one `SessionWatcher` (started when the container starts, stopped when it stops). The watcher uses agent-specific discovery to find active JSONL session files, tails them for new lines, and polls for both new files and new content in existing files.

## SessionParser Interface

```go
type SessionParser interface {
    // ParseLine parses a single JSONL line into zero or more Warden events.
    ParseLine(line []byte) []ParsedEvent

    // SessionDir returns the host-side directory to watch for session files.
    SessionDir(homeDir string, project ProjectInfo) string

    // FindSessionFiles returns the absolute paths of active JSONL session
    // files belonging to the given project.
    FindSessionFiles(homeDir string, project ProjectInfo) []string
}
```

Parsers are **stateful** — they accumulate token counts across lines within a session. Create a new parser per session file. `FindSessionFiles()` is agent-specific: Claude Code scans a per-project directory; Codex reads shell snapshots to filter by project ID and globs for matching JSONL files.

## ParsedEvent Types

| Type                 | Meaning                  | Key Fields                       |
| -------------------- | ------------------------ | -------------------------------- |
| `session_start`      | Agent session began      | SessionID, Timestamp, WorktreeID |
| `session_end`        | Agent session ended      | SessionID                        |
| `tool_use`           | Agent invoked a tool     | ToolName, ToolInput (truncated)  |
| `tool_use_failure`   | Tool execution failed    | ToolName, ErrorContent           |
| `user_prompt`        | User sent a message      | Prompt                           |
| `turn_complete`      | Agent finished a turn    | SessionID                        |
| `turn_duration`      | Turn timing data         | DurationMs                       |
| `token_update`       | Cumulative token usage   | Tokens, EstimatedCostUSD         |
| `stop_failure`       | Turn ended due to error  | ErrorContent                     |
| `permission_request` | Agent needs approval     | ToolName                         |
| `elicitation`        | MCP server needs input   | ServerName                       |
| `subagent_stop`      | Subagent(s) terminated   | Content                          |
| `api_metrics`        | API performance data     | TTFTMs, OutputTokensPerSec       |
| `permission_grant`   | Permission granted       | Commands                         |
| `context_compact`    | Context window compacted | CompactTrigger, PreCompactTokens |
| `system_info`        | Informational system msg | Subtype, Content                 |

## Agent Implementations

### Claude Code (`agent/claudecode/`)

- **Session dir:** `~/.claude/projects/<sanitized-path>/`
- **JSONL format:** `SessionEntry` with `type` field (assistant, user, system, file-history-snapshot, etc.)
- **Session start:** Synthesized when the parser detects a new `sessionId` (Claude JSONL has no dedicated session_start entry). Resets cumulative token counts and tool name tracking.
- **Token source:** `cacheCreationInputTokens`, `cacheReadInputTokens`, `inputTokens`, `outputTokens` in `usage` blocks
- **Cost:** Estimated from token counts via per-model pricing table (`pricing.go`)
- **Tool extraction:** From `tool_use` content blocks in assistant messages
- **Queue operations:** `queue-operation` entries with `operation=enqueue` are parsed as `user_prompt` events (prompts submitted while Claude is still working)
- **System subtypes:** All 14 parsed — turn_duration, api_error, agents_killed, api_metrics, permission_retry, compact_boundary, microcompact_boundary, and 7 informational subtypes mapped to `system_info`

### Codex (`agent/codex/`)

- **Session dir:** `~/.codex/sessions/YYYY/MM/DD/`
- **Discovery:** Shell snapshots at `~/.codex/shell_snapshots/` filtered by `WARDEN_PROJECT_ID`
- **JSONL format:** `RolloutItem` with `type` field (session_meta, response_item, event_msg, turn_context, compacted)
- **Token source:** Cumulative `total_token_usage` in `token_count` events
- **Cost:** Estimated from token counts via OpenAI model pricing table (`pricing.go`)
- **Tool extraction:** From `response_item` entries — `function_call`, `local_shell_call`, `web_search_call`, `custom_tool_call`, `image_generation_call`, `tool_search_call`
- **Persistence policy:** Codex filters which events land in JSONL. Limited mode (CLI default) persists core events; extended mode (app-server only) adds errors and tool end events. See `docs/events_codex.md` for details.

## Hooks vs JSONL

JSONL is the **primary** data source for all event types: session lifecycle, tool use, cost, prompts, and turn completion. Claude Code hooks are a **supplementary** channel used only for attention/notification state (permission prompts, idle state, elicitation dialogs). Codex does not support hooks — attention tracking for Codex is a known upstream gap.

| Data                            | Source                                   |
| ------------------------------- | ---------------------------------------- |
| Session lifecycle               | JSONL                                    |
| Tool use / failures             | JSONL                                    |
| Cost / tokens                   | JSONL                                    |
| User prompts                    | JSONL                                    |
| Turn completion / duration      | JSONL                                    |
| Context compaction              | JSONL                                    |
| API metrics, system info        | JSONL (Claude only)                      |
| Permission grant                | JSONL (Claude only)                      |
| Permission request, elicitation | Hooks (Claude) / app-server only (Codex) |
| Attention state (Claude)        | Hooks (supplementary)                    |
| Attention state (Codex)         | Not available (upstream gap)             |

## Validation

`agent.ValidateJSONL(parser, reader)` validates JSONL lines and returns event counts. `agent.ParseAllEvents(parser, reader)` returns all parsed events (shared test helper). Used by:

- Parser unit tests (`agent/claudecode/parser_test.go`, `agent/codex/parser_test.go`)
- CI scheduled validation (`TestValidateLive`, env-gated via `VALIDATE_JSONL`) — runs biweekly in `.github/workflows/container-scheduled.yml`, builds the container image, runs both CLIs with a test prompt, validates JSONL output, and opens an issue on failure

## Adding a New Agent

1. Create `agent/<name>/` subpackage
2. Implement `SessionParser` (parser.go) with `ParseLine()`, `SessionDir()`, and `FindSessionFiles()` (agent-specific discovery logic)
3. Implement `StatusProvider` (provider.go) with `Name()`, `ProcessName()`, `ConfigFilePath()`, `ExtractStatus()`, `NewSessionParser()`
4. Add pricing table if cost estimation is needed
5. Add install script in `container/scripts/install-<name>.sh`
6. Add event handler in `container/scripts/<name>/`
7. Register in `agent/registry.go` and wire in `warden.go`
