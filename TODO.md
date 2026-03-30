# TODO

- [ ] Should release-please also update the versions for go packages and webapp/docs_site package.json? Does it matter?
- [ ] API endpoint to check for new releases

# Multi-Agent Support (Codex CLI)

Add OpenAI Codex CLI as a second supported agent, making Warden agent-agnostic.

No backwards compatibility is required for this work.

## Architecture

**JSONL session file parser is the primary, unified data source for all agents.** Each agent writes a session JSONL file (Claude at `~/.claude/projects/<path>/<session>.jsonl`, Codex at `~/.codex/sessions/YYYY/MM/DD/rollout-*.jsonl`). Both are on the host via bind-mounted config directories. The Go server watches these files directly on the host filesystem using fsnotify — no docker exec, no in-container watcher binary.

**Hooks are a supplementary channel for real-time state not yet in the JSONL.** Currently that's just attention/notification state (permission_prompt, idle_prompt, elicitation). For Claude, hooks provide this today. For Codex, it's a known upstream gap ([#14813](https://github.com/openai/codex/issues/14813), [#11808](https://github.com/openai/codex/issues/11808), [#6024](https://github.com/openai/codex/issues/6024)). When either agent formalizes these events into JSONL, we move parsing into the main parser and drop the hook.

**Cost estimation is unified.** Every agent reports tokens in JSONL. Warden estimates cost from tokens using a per-agent pricing table (`pricing.go`). For Claude, actual cost from `.claude.json` can optionally upgrade the estimate. The pricing module exists for all agents even if it's a no-op — avoids conditionals.

**The JSONL parsers are importable Go packages.** `agent/claudecode` and `agent/codex` are top-level packages that external Go consumers can import to parse Claude Code and Codex session files in their own tools — no dependency on the full Warden engine. This is a developer-facing feature worth highlighting in docs and README.

**Config directories are mandatory bind mounts.** `~/.claude/` and `~/.codex/` must be mounted for both config passthrough and JSONL parsing. Users can still customize mount paths, but some mount must exist. Warden is a security boundary, not a privacy boundary.

### Data flow

```
Agent writes JSONL inside container
  → File appears on host via bind mount
  → Go server watches with fsnotify (host-side)
  → Agent-specific parser maps to typed Go structs
  → Feeds into eventbus/store directly
  → SSE to frontend / TUI
```

### What JSONL provides (both agents)

- Session start/end, tool use events, user prompts
- Token usage per turn, model info, session ID
- Git info (branch, commit), working directory

### What JSONL does NOT provide (supplementary channel needed)

- Attention state: permission_prompt, idle_prompt, elicitation_dialog
- Claude: solved via Notification hook today
- Codex: unsolved upstream — documented limitation

### Package structure

```
agent/
  types.go           — Warden's internal event types (agent-agnostic)
  provider.go         — Common parser interface
  registry.go         — Provider registry (selects parser by agent type)

agent/claudecode/
  types.go           — Go structs matching Claude's JSONL schema
  parser.go          — JSONL → Warden events
  parser_test.go     — Snapshot tests with real fixtures
  pricing.go         — Token-to-cost (fallback; .claude.json has actual cost)
  provider.go        — StatusProvider implementation (existing, adapts)

agent/codex/
  types.go           — Go structs matching Codex's JSONL schema
  parser.go          — JSONL → Warden events
  parser_test.go     — Snapshot tests with real fixtures
  pricing.go         — Token-to-cost (OpenAI model pricing table)
  provider.go        — StatusProvider implementation
```

### Key design decisions

| Decision                          | Rationale                                                                                                                                                                                                                                                                                   |
| --------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Host-side JSONL parsing           | Files are on host via bind mount. No docker exec, no in-container binary needed.                                                                                                                                                                                                            |
| Typed Go structs for JSONL schema | Type-safe access throughout backend. Snapshot tests catch upstream format changes.                                                                                                                                                                                                          |
| `pricing.go` for all agents       | Uniform interface. Claude's is a fallback (actual cost from .claude.json). No conditionals.                                                                                                                                                                                                 |
| Config dirs are mandatory mounts  | Required for JSONL parsing. Config passthrough is also needed for both agents.                                                                                                                                                                                                              |
| Single container image, both CLIs | `WARDEN_AGENT_TYPE` env var controls which launches. Hooks baked at build-time. CLIs bundled because JSONL schema is a contract — CLI and parser must be tested together. Dev runtimes (Python, Go, etc.) can be installed at creation time since they have no schema contract with Warden. |
| Docker layer optimization         | Split Dockerfile so CLI installs are separate layers. Order: system deps → abduco/gosu → Claude CLI → Codex CLI → Warden scripts (last, changes most often). Unchanged layers cached on pull. Most Warden releases only re-download the small scripts layer.                                |
| Separate event scripts per agent  | `scripts/claude/`, `scripts/codex/` subdirectories. Extensible for future agents.                                                                                                                                                                                                           |
| `ClaudeStatus` → `AgentStatus`    | Rename throughout. Grep for old names to confirm.                                                                                                                                                                                                                                           |
| `DockerClient` → rename           | Current name is misleading (handles Podman too). Address during this work.                                                                                                                                                                                                                  |

### Impacts on existing architecture

| Component                  | Impact                                                                                                                               |
| -------------------------- | ------------------------------------------------------------------------------------------------------------------------------------ |
| **Web SPA**                | Add `agentType` to types. Agent selector in create form. Hide/show worktree UI per agent. No architectural change — still HTTP only. |
| **TUI**                    | Display agent type. Agent selector in container form. No Client interface changes needed — it's already agent-agnostic.              |
| **`client/` package**      | Add `agentType` field to relevant request/response types. Stays in sync with `api.ts`.                                               |
| **Reference architecture** | Preserved. JSONL parsing is backend-only (engine/service layer). SPA uses HTTP, TUI uses Client interface.                           |
| **Container scripts**      | Agent-specific subdirectories. `create-terminal.sh` branches on `WARDEN_AGENT_TYPE`.                                                 |
| **Dockerfile**             | Install both CLIs. Write both hook configs.                                                                                          |
| **Event directory**        | Still needed for hook-based events (Claude Notification). Not needed for JSONL-sourced events.                                       |

### Session file discovery

The Go server needs to find the active session JSONL file for each running project:

- **Claude**: `~/.claude/projects/<encoded-container-path>/<session-id>.jsonl` — the directory name encodes the **container-side** path (e.g., `-home-dev-warden--claude-worktrees-audit`), not the host path. The Go server reconstructs this from the known container workspace dir. Watch for new `.jsonl` files appearing in the project directory.
- **Codex**: `~/.codex/sessions/YYYY/MM/DD/rollout-<timestamp>-<ulid>.jsonl` — date-based directories. Also has `~/.codex/session_index.jsonl` as an append-only index. Watch the index file for new entries, then tail the referenced JSONL path.

### Codex hooks feature flag

Codex hooks are behind a feature flag (`codex_hooks = true` in config.toml). Even though we use JSONL as primary, if/when Codex adds notification events to hooks, we'd need this flag enabled. Consider setting it in the container's Codex config at build time.

### Codex-specific notes

- No native `--worktree` flag. Warden creates/tracks/deletes worktrees manually, launches Codex with `-C <worktree-path>`.
- Needs `--no-alt-screen` to disable TUI (required for abduco terminal embedding).
- Permission skip: `--full-auto` (equivalent to Claude's `--dangerously-skip-permissions`).
- Auth: `CODEX_API_KEY` env var (vs Claude's `ANTHROPIC_API_KEY`).
- Custom instructions: `AGENTS.md` (vs `CLAUDE.md`).
- Hooks config: `~/.codex/hooks.json` — only needed for Notification equivalent when/if Codex adds it.
- Cost: always estimated from tokens (no actual cost source like `.claude.json`).
- Codex session JSONL has no explicit session-end marker — detect via file going quiet + process exit.

### Estimated cost naming

Current "estimated" cost means two different things:

- Claude subscription: actual cost given by Claude, user just isn't paying it directly
- Codex: computed from tokens, inherently approximate

Need separate terminology. Proposed: `isSubscriptionCost` (subscription) vs `isEstimatedCost` (token-derived).

Codex has a subscription as well, so that needs to fall under the same split as Claude.

The breakdown is:

- Claude API: not subscription and estimated (but also has a final from .claude.json)
- Claude Subscription: is subscription and estimated (but also has a final from .claude.json)
- Codex API: not subscription and estimated (no final)
- Codex Subscription: is subscription and estimated (no final)

The key to making this work is finding a way to determine if a Codex user is on a subscription or not.

Codex session JSONL `event_msg` entries with `type: "token_count"` include `rate_limits.credits` with `has_credits`, `unlimited`, `balance` fields, plus a `plan_type` field. These likely indicate subscription vs API key usage. Needs verification with actual subscription vs API key sessions to confirm which values map to which billing mode.

## Steps

### Step 0: Research & Plan (this document)

- [x] Research Codex CLI capabilities, hooks, session files
- [x] Deep dive Codex source code (session JSONL format, hooks implementation)
- [x] Analyze Claude session JSONL for parity
- [x] Design unified JSONL parser architecture
- [x] Document decisions and plan

### Step 1: Go-Side Multi-Agent Abstraction

Decouple the engine from Claude Code so it can support any agent. No new agent yet — just make the existing code agent-agnostic.

- [x] **Database**: Add `agent_type TEXT NOT NULL DEFAULT 'claude-code'` column to `projects` table in `db/db.go` schema. No migration needed (fresh schema). Add `AgentType string` field to `ProjectRow` in `db/entry.go`. Update `projectColumns`, `InsertProject`, `scanProjectRow` in `db/store.go`.

- [x] **API/engine types**: Add `AgentType string` to `CreateContainerRequest` (`engine/types.go`), `Project` (`engine/types.go`), and any API response wrappers in `api/types.go`. Add to `ContainerConfig` for the edit form.

- [x] **Agent registry**: Create `agent/registry.go` with `Registry` struct — `map[string]StatusProvider` with `Register(name, provider)`, `Get(name) (StatusProvider, bool)`, and `Default() StatusProvider` (returns "claude-code"). In `warden.go`, create registry, register `claudecode.NewProvider()`, pass registry to engine client.

- [x] **Rename `DockerClient`**: It handles both Docker and Podman, so the name is misleading. Rename to `ContainerClient` or `EngineClient` in `engine/client.go`. Update all references. The `Client` interface in `engine/types.go` is already well-named.

- [x] **Rename `ClaudeStatus` → `AgentStatus`**: In `engine/types.go`, rename the type, constants (`ClaudeStatusIdle` → `AgentStatusIdle`, etc.), and the `ClaudeStatus` field on `Project`. Rename `checkClaudeStatus()` → `checkAgentStatus()` in `engine/client.go` — make it read `WARDEN_AGENT_TYPE` from the container's env vars (already cached by `workspaceDir()`) and pgrep for the correct process name (`claude` vs `codex`). **Verification**: grep entire codebase for `ClaudeStatus`, `checkClaudeStatus`, `claudeStatus` to confirm all renamed. Frontend JSON field `claudeStatus` should change to `agentStatus`.

- [x] **Cost terminology**: In `engine/agent_status.go`, the current `IsEstimatedCost` (line 143) checks Claude's `oauthAccount.billingType == "stripe_subscription"`. This means "subscription user, cost is real but not billed to them." For Codex, "estimated" means "computed from tokens." These are different concepts. Rename the existing field to `isSubscriptionCost` and add a new `isEstimatedCost` for token-derived costs. Update `AgentCostResult`, the event bus stop callback, and the frontend display logic. The `isEstimatedCostFromConfig` function moves into the Claude provider (it parses Claude-specific JSON). _(Note: isSubscriptionCost rename deferred — requires subscription data verification with real Codex sessions)_

- [x] **Container env var**: In `engine/containers.go` `CreateContainer()`, add `WARDEN_AGENT_TYPE` to the container's env vars (read from `CreateContainerRequest.AgentType`, default `"claude-code"`). This joins the existing `WARDEN_WORKSPACE_DIR`, `WARDEN_EVENT_DIR`, `WARDEN_CONTAINER_NAME`, `WARDEN_PROJECT_ID` env vars.

- [x] **Mandatory config dir mount**: In `service/host.go`, the `userMounts` slice currently has `{".claude", ...}` which is only included if `~/.claude` exists on the host. Make this mandatory (always include, create the dir if needed). Add `{".codex", containerHomeDir + "/.codex", readOnly: false}` with the same behavior. These are required for JSONL parsing — without them, the parser has no data. The `GetDefaults()` response should indicate these are required mounts.

- [x] **Engine client constructor**: Change `engine.NewClient(socket, runtime string, provider agent.StatusProvider)` to accept `*agent.Registry` instead. The `readAgentConfigRaw()` method resolves the correct provider per-container by reading `WARDEN_AGENT_TYPE` from the container's env.

- [x] Tests pass, lint clean. Existing behavior unchanged — Claude Code still works exactly as before, just through the registry.
- [x] Run `/simplify` to review changes, then update affected codemaps and docs.

### Step 2: JSONL Parser Infrastructure

Build the host-side JSONL parsing pipeline for Claude Code. This creates the infrastructure that Codex will also use.

- [x] **Parser interface**: Define in `agent/provider.go` (or a new `agent/parser.go`):

  ```go
  type SessionParser interface {
      // ParseLine parses a single JSONL line into zero or more Warden events.
      ParseLine(line []byte) []ParsedEvent
      // SessionDir returns the host-side directory to watch for session files.
      // Called with the host home dir and project metadata.
      SessionDir(homeDir string, project ProjectInfo) string
  }
  ```

  `ParsedEvent` is an agent-agnostic struct that maps to Warden's internal event types (tool_use, session_start, stop, user_prompt, token_update, etc.). Define these in `agent/types.go`.

- [x] **Relationship to existing StatusProvider**: The JSONL parser does NOT replace `StatusProvider` — they coexist. `StatusProvider` reads a config file for point-in-time status (cost, tokens). The parser tails the JSONL for real-time events. Eventually the parser may subsume StatusProvider's role, but for now they serve different purposes: parser = streaming events, provider = snapshot queries. The existing `ReadAgentStatus` / `ReadAgentCostAndBillingType` methods on the engine remain unchanged.

- [x] **Claude JSONL typed structs**: Create `agent/claudecode/types.go` with Go structs matching Claude's session JSONL format. Key types based on real session analysis:
  - `SessionEntry` — top-level JSONL line: `{type, parentUuid, message, timestamp, sessionId, cwd, ...}`
  - `AssistantMessage` — `{model, usage, stop_reason, content[]}`
  - `ContentBlock` — `{type: "tool_use"|"text"|"thinking", name, input}`
  - `UsageInfo` — `{input_tokens, output_tokens, cache_read_input_tokens, cache_creation_input_tokens}`
  - `SystemEntry` — `{subtype: "stop_hook_summary"|"turn_duration", ...}`
  - `ProgressEntry` — `{data: {type: "hook_progress", hookEvent, hookName, command}}`
  - `WorktreeState` — `{worktreeSession: {worktreeName, worktreeBranch, sessionId}}`

  All fields use `json` tags. Unknown fields are silently ignored by `json.Unmarshal`.

- [x] **Claude JSONL parser**: `agent/claudecode/parser.go` implements `SessionParser`. `ParseLine()` deserializes a JSONL line into the typed struct, then maps it to `ParsedEvent`:
  - `type: "assistant"` with tool_use content → `ParsedEvent{Type: ToolUse, ToolName: ..., Model: ..., Tokens: ...}`
  - `type: "assistant"` with `stop_reason: "end_turn"` → `ParsedEvent{Type: TurnComplete}`
  - `type: "user"` with text content → `ParsedEvent{Type: UserPrompt, Prompt: ...}`
  - `type: "system"` with `subtype: "turn_duration"` → `ParsedEvent{Type: TurnDuration, DurationMs: ...}`
  - `type: "worktree-state"` → `ParsedEvent{Type: SessionStart, SessionID: ..., WorktreeID: ...}`
  - Every `type: "assistant"` entry has `.message.usage` with token counts → accumulate in parser state.

- [x] **Claude pricing.go**: `agent/claudecode/pricing.go` with `EstimateCost(model string, usage UsageInfo) float64`. Maps Claude model IDs to per-token prices. Returns estimated cost from tokens. This is a **fallback** — the preferred source is actual cost from `.claude.json` via `StatusProvider`. The pricing function still exists (no-op-safe) so the calling code doesn't need conditionals.

- [x] **Claude session file discovery**: `SessionDir()` implementation. Claude stores session JSONL at `~/.claude/projects/<encoded-path>/<session-id>.jsonl`. The path encoding replaces `/` with `-` in the **container-side** workspace path. Example: container workspace `/home/dev/warden` with worktree `audit` → container CWD `/home/dev/warden/.claude/worktrees/audit` → encoded as `-home-dev-warden--claude-worktrees-audit`. The Go server knows the container workspace dir (from `engine.workspaceDir()`). So: `SessionDir = ~/.claude/projects/ + encode(containerWorkspaceDir)`. Watch this directory for new `.jsonl` files. The most recently modified `.jsonl` is the active session.

- [x] **Standardized test prompt**: Design a single prompt that exercises all JSONL event types both parsers need:

  ```
  Read the file README.md, then create a file called /tmp/warden-test.txt with "hello", then delete it
  ```

  This triggers: session start (worktree-state), Read tool (auto-approved tool_use), Write tool (permission prompt), Bash tool (permission prompt, rm), text responses, stop events, token counts on every assistant message. Run in **default permission mode** (not bypass) to capture permission flow.

  This could also be multiple prompts if the CLI allows it

- [x] **Test fixtures**: Run the test prompt locally against Claude Code. Copy the resulting JSONL from `~/.claude/projects/<path>/<session>.jsonl`. Anonymize it (strip API keys, session IDs → placeholder UUIDs, paths → generic). Check into repo at `agent/claudecode/testdata/session.jsonl`.

- [x] **Parser unit tests**: `agent/claudecode/parser_test.go`. Read the fixture, parse every line, assert:
  - Correct number of `ToolUse` events (3: Read, Write, Bash)
  - Correct number of `UserPrompt` events
  - Token totals are positive and accumulate
  - Model field is populated
  - Session start event has worktree info
  - Unknown/new fields don't cause parse errors (forward compatibility)

- [x] **Host-side fsnotify watcher**: A `SessionWatcher` struct (in engine or service layer, TBD during package audit) that:
  1. Takes a `SessionParser`, a session directory path, and a callback `func(ParsedEvent)`
  2. Uses `fsnotify` to watch the session directory for new `.jsonl` files
  3. When a new file appears, opens it and tails it line-by-line using a `bufio.Scanner` in a goroutine
  4. On each new line, calls `parser.ParseLine()` and invokes the callback with each `ParsedEvent`
  5. Handles file rotation (new session starts → new file appears → switch to tailing the new file)
  6. Has `Start(ctx)` and `Stop()` lifecycle methods
  7. Lifecycle: created per-project when a container starts, stopped when the container stops. The service layer manages this — similar to how `Watcher.WatchContainerDir()` works for event directories today.

- [x] **Wire into eventbus**: The callback converts `ParsedEvent` → `eventbus.Event` and calls `store.HandleEvent()`. This is the same entry point that hook-based events use today. The parsed events flow through the same pipeline: store → broker → SSE → frontend. Cost events update the session_costs table via the existing `PersistSessionCost` gateway.

- [x] Tests pass, lint clean. At this point, Claude Code projects get events from **both** the JSONL parser (tool use, prompts, tokens, session lifecycle) and hooks (Notification for attention state). They coexist — the parser provides richer data, hooks provide attention state that isn't in the JSONL.
- [x] Run `/simplify` to review changes, then update affected codemaps and docs.

### Step 3: Codex Provider & Parser

Add Codex as a second agent. Uses the same infrastructure from Step 2.

- [x] **Codex JSONL typed structs**: `agent/codex/types.go`. Based on the Codex source code analysis, the JSONL format uses these top-level types:
  - `RolloutItem` — each line: `{timestamp, type: "session_meta"|"response_item"|"turn_context"|"event_msg", payload}`
  - `SessionMeta` — `{id, cwd, cli_version, model_provider, git: {commit_hash, branch, repository_url}}`
  - `ResponseItem` — `{type: "function_call"|"function_call_output"|"message", name, call_id, arguments, role, content}`
  - `TurnContext` — `{model, approval_policy, sandbox_policy, cwd}`
  - `EventMsg` — `{type: "token_count", info: {total_token_usage: {input_tokens, output_tokens, ...}}}`

- [x] **Codex JSONL parser**: `agent/codex/parser.go` implements `SessionParser`. Mapping:
  - `session_meta` → `ParsedEvent{Type: SessionStart, SessionID, Model, GitBranch, ...}`
  - `response_item` with `type: "function_call"` → `ParsedEvent{Type: ToolUse, ToolName, ...}`
  - `response_item` with `type: "message", role: "user"` → `ParsedEvent{Type: UserPrompt, ...}`
  - `event_msg` with `type: "token_count"` → `ParsedEvent{Type: TokenUpdate, Tokens: ...}`
  - `turn_context` → update model/approval policy in parser state
  - End of file (no new writes + process exited) → `ParsedEvent{Type: SessionEnd}`

- [x] **Codex pricing.go**: `agent/codex/pricing.go` with `EstimateCost(model string, usage TokenUsage) float64`. Maps OpenAI model IDs to per-token prices (input/output separately). Cost is always estimated — Codex has no actual-cost source. Mark `isEstimatedCost: true` on all cost events. Pricing tables are here: <https://developers.openai.com/api/docs/pricing> . Codex uses 5.4 as well as the -codex models.

- [x] **Codex session file discovery**: `SessionDir()` returns `~/.codex/sessions/`. Unlike Claude (one directory per project), Codex stores all sessions in date-based subdirectories (`YYYY/MM/DD/`). The watcher needs to:
  1. Watch `~/.codex/session_index.jsonl` for new entries (append-only index with thread ID and timestamp)
  2. When a new entry appears, extract the session file path
  3. Tail that JSONL file
     Alternatively, recursively watch `~/.codex/sessions/` for new `.jsonl` files and pick the most recent one. The session_index approach is more reliable.

- [x] **Codex StatusProvider**: `agent/codex/provider.go`. The existing `StatusProvider` interface requires `ConfigFilePath()` and `ExtractStatus()`. For Codex, `ConfigFilePath()` returns a path that the watcher writes accumulated cost data to (e.g., `/tmp/warden/codex-status.json`). Or, the StatusProvider is simplified to work with the parser's accumulated state directly. **Decision**: since the JSONL parser is now primary, the StatusProvider for Codex can return the parser's running totals rather than reading a separate file. The interface may need a minor adaptation — add an optional `StatusFromParser` method that returns cached state, falling back to file reading.

- [x] **Register in warden.go**: Add `registry.Register("codex", codex.NewProvider())`.

- [x] **Test fixtures**: Run the standardized test prompt against Codex CLI locally. Copy the session JSONL from `~/.codex/sessions/YYYY/MM/DD/rollout-*.jsonl`. Anonymize and check into `agent/codex/testdata/session.jsonl`.

- [x] **Parser unit tests**: Same structure as Claude tests. Assert correct event types, token accumulation, model parsing. Test that unknown Codex JSONL fields don't break parsing.

- [x] **Mandatory mount**: Add `{".codex", containerHomeDir + "/.codex", readOnly: false}` to `userMounts` in `service/host.go`. Same mandatory behavior as `~/.claude`.

- [x] Tests pass, lint clean.
- [x] Run `/simplify` to review changes, then update affected codemaps and docs.

### Step 4: Container Changes

Update the container image to support both agents.

- [x] **Split install-tools.sh**: Break into composable sub-scripts that can be used independently:
  - `install-system-deps.sh` — apt packages including gh, Node.js, git, curl, jq, iptables, etc. (everything currently in the apt-get block)
  - `install-user.sh` — dev user creation (the UID 1000 logic)
  - `install-claude.sh` — `curl -fsSL https://claude.ai/install.sh | bash` + managed-settings.json
  - `install-codex.sh` — `npm install -g @openai/codex` + `~/.codex/hooks.json` + enable `codex_hooks` feature flag in config.toml
  - `install-warden.sh` — copy scripts to `/usr/local/bin/`, set permissions, create workspace dir

  The existing `install-tools.sh` becomes a wrapper that calls all of them in order (for the devcontainer feature path). The Dockerfile calls each as a separate `RUN` instruction for layer caching.

- [x] **Dockerfile layer order**: Rewrite the runtime stage:

  ```dockerfile
  FROM ubuntu:24.04
  COPY --from=builder /usr/local/bin/abduco /usr/local/bin/abduco
  COPY --from=builder /usr/local/bin/gosu /usr/local/bin/gosu
  COPY scripts/install-system-deps.sh /tmp/
  RUN /tmp/install-system-deps.sh           # Layer: system deps (rarely changes)
  COPY scripts/install-user.sh /tmp/
  RUN /tmp/install-user.sh                  # Layer: dev user (rarely changes)
  COPY scripts/install-claude.sh /tmp/
  RUN /tmp/install-claude.sh                # Layer: Claude CLI (changes on Claude releases)
  COPY scripts/install-codex.sh /tmp/
  RUN /tmp/install-codex.sh                 # Layer: Codex CLI (changes on Codex releases)
  COPY scripts/ /tmp/warden-scripts/
  RUN /tmp/warden-scripts/install-warden.sh # Layer: Warden scripts (changes every release)
  ```

  Most Warden releases only invalidate the last layer. CLI updates only invalidate from that CLI layer onward.

- [x] **Script subdirectories**: Reorganize `container/scripts/` into:

  ```
  scripts/
    shared/           — entrypoint.sh, user-entrypoint.sh, create-terminal.sh (agent-aware),
                        kill-worktree.sh, disconnect-terminal.sh, warden-write-event.sh,
                        warden-push-event.sh, warden-heartbeat.sh, setup-network-isolation.sh
    claude/           — warden-event-claude.sh (simplified: notification/attention events only)
    codex/            — warden-event-codex.sh (placeholder for when Codex adds notification hooks)
    install-*.sh      — composable install scripts (see above)
  ```

- [x] **Agent-aware create-terminal.sh**: The key script change. Currently hardcodes `claude` command (line 65). Change to:

  ```bash
  AGENT_TYPE="${WARDEN_AGENT_TYPE:-claude-code}"
  case "$AGENT_TYPE" in
    codex)
      AGENT_CMD="codex --no-alt-screen"
      if [ "$SKIP_PERMISSIONS" = "--skip-permissions" ]; then
        AGENT_CMD="codex --no-alt-screen --full-auto"
      fi
      # Codex doesn't have --worktree. For non-main worktrees,
      # Warden has already created the git worktree via docker exec.
      # Set WORK_DIR to the worktree path.
      if [ "$IS_GIT_REPO" = true ] && [ "$WORKTREE_ID" != "main" ]; then
        WORKTREE_PATH="${WORKSPACE_DIR}/.claude/worktrees/${WORKTREE_ID}"
        if [ -d "$WORKTREE_PATH" ]; then
          WORK_DIR="$WORKTREE_PATH"
        fi
      fi
      ;;
    *)
      AGENT_CMD="claude"
      if [ "$SKIP_PERMISSIONS" = "--skip-permissions" ]; then
        AGENT_CMD="claude --dangerously-skip-permissions"
      fi
      if [ "$IS_GIT_REPO" = true ] && [ "$WORKTREE_ID" != "main" ]; then
        AGENT_CMD="${AGENT_CMD} --worktree '${WORKTREE_ID}'"
      fi
      ;;
  esac
  INNER_CMD="cd '${WORK_DIR}' && ${AGENT_CMD}; ..."
  ```

- [x] **Codex worktree management**: Codex has no `--worktree` flag, so Warden must manage worktrees itself. The existing `engine.CreateWorktree()` already runs `git worktree add` via docker exec — this part works for both agents. The difference is in terminal creation: Claude gets `--worktree <id>` flag, Codex gets `cd <worktree-path>` + bare `codex` command. The worktree path is at `<workspace>/.claude/worktrees/<id>` (created by git worktree add). Test thoroughly: create worktree, connect terminal (verify Codex launches in correct dir), kill process, reconnect, remove worktree.

- [x] **Container scripts dropping**: With the JSONL parser as primary data source and `StatusProvider` handling final cost via docker exec on the Go side, the cost-lib bash scripts are redundant for both agents. Drop them:
  - **Drop**: `warden-cost-lib.sh`, `warden-capture-cost.sh` — the JSONL parser handles streaming cost (tokens per turn), and `ReadAgentCostAndBillingType()` in Go handles final cost reconciliation from `.claude.json` via docker exec. No bash cost scripts needed for either agent.
  - **Simplify**: `warden-event-claude.sh` — keep only the `notification` case (maps to `attention` event) and `pre_tool_use` for `AskUserQuestion` detection. All other cases (session_start, session_end, stop, tool_use, etc.) are now handled by the JSONL parser. Removing them from the hook reduces latency on Claude's execution.
  - **Keep**: `warden-push-event.sh`, `warden-write-event.sh` — still needed for the remaining hook events (notification/attention).
  - **Keep**: `warden-heartbeat.sh` — still needed for container liveness detection.

- [x] Container builds successfully with both CLIs available. Verify: `docker exec <container> which claude` and `docker exec <container> which codex`.
- [x] Run `/simplify` to review changes, then update affected codemaps and docs.

### Step 4b

- [x] **Verify workflow_dispatch**: Manually trigger the container.yml workflow via GitHub Actions UI with no tag_name. Verify it builds `:latest` with current CLI versions.

- [x] **Scheduled CI job**: New `container-scheduled.yml` workflow runs on cron (`0 6 1,15 * *`). Builds image → runs test prompt against each CLI (gated on `ANTHROPIC_API_KEY`/`OPENAI_API_KEY` secrets) → validates JSONL with shared `agent.ValidateJSONL()` → pushes if validation passes → opens GitHub issue if it fails. Extracted reusable `container-build.yml` for shared build+push logic. Simplified `container.yml` to call the reusable workflow.

- [x] **Workspace consolidation under `.warden/`**: Claude Code's worktree path (`.claude/worktrees/`) is hardcoded and not configurable (confirmed via docs + upstream issues anthropics/claude-code#27282). Solution: agent-dependent paths — Claude Code keeps `.claude/worktrees/`, Codex and future agents use `.warden/worktrees/`. Consolidated all Warden workspace artifacts under `.warden/`: renamed `.warden-terminals/` → `.warden/terminals/`, moved gitignore entries (`.warden-debug*`, `.warden-user-hook*`) under single `.warden/` entry. Removed legacy `.worktrees/` support entirely. Renamed engine receiver `dc` → `ec` (leftover from DockerClient rename). Added shared `agent.ValidateJSONL()` function used by both parser tests and CI.

### Step 5: API & Frontend

Thread agent type through the API and add UI support.

- [x] **API flow**: Verified full pipeline: `CreateContainerRequest.AgentType` → `routes.go` handler → `service.CreateContainer()` → `engine.CreateContainer()` (sets `WARDEN_AGENT_TYPE` env var) → `db.InsertProject()` (stores in `agent_type` column). `Project` and list endpoints include `agentType` in responses. `claudeStatus` → `agentStatus` rename already done in Step 1.

- [x] **Web SPA agent selector**: Button-group selector at top of `project-config-form.tsx` with "Claude Code" / "OpenAI Codex". Defaults to Claude Code via `DEFAULT_AGENT_TYPE`. Read-only in edit mode. Agent-aware skip-permissions description. `agentType` flows through `ProjectConfigFormData` → `add-project-dialog.tsx` payload → API.

- [x] **TypeScript types**: `AgentType` union type, `DEFAULT_AGENT_TYPE`, `agentTypeOptions`, `agentTypeLabels` in `types.ts`. `Project.agentType` narrowed from `string` to `AgentType`. `agentType` added to `CreateContainerRequest` and `ContainerConfig`. "Claude exited" → "Agent exited" in state labels.

- [x] **Worktree UI for Codex**: No changes needed — worktree UI is agent-agnostic. "New Worktree" button works for both agents.

- [x] **TUI changes**: Agent type selector as first field in `view_container_form.go` (cycle with tab/enter, read-only in edit). "Agent" column in `view_projects.go` project list table (8 chars, uses `agent.ShortLabel()`). All references use `agent.AllTypes`, `agent.DisplayLabels` from `agent/registry.go`.

- [x] **client/ package**: Already uses `engine.CreateContainerRequest` directly (which has `AgentType`). No changes needed.

- [x] **Single source of truth**: `agent/registry.go` exports `AllTypes`, `DisplayLabels`, `ShortLabel()`. TUI references these. TS has matching `agentTypeOptions`/`agentTypeLabels` with comment pointing to `agent/registry.go`.

- [x] Run `/simplify` to review changes, then update affected codemaps and docs. Updated: supporting.md, components.md, hooks.md, views.md, api-types.md.

- [x] **E2E tests**: Not needed — agent selector is an unconditional UI component with no conditional logic. `agentType` option added to E2E API helper `createTestProject()` for future Codex project tests.

### Step 6: Documentation

See the detailed doc checklist below. Each item describes what to update and why.

#### Project docs

- [ ] `CLAUDE.md` — update "What is Warden" section to say "Supports Claude Code and OpenAI Codex" instead of just Claude Code. Add Codex-specific dev commands if any.
- [ ] `README.md` — update the supported agents line ("Claude Code — currently the only supported agent (more coming soon)" → list both).
- [ ] `CONTRIBUTING.md` — note that PRs touching agent/ should include tests for both parsers. Note the composable install scripts for container builds.
- [ ] `docs/terminology.md` — add "Agent Type" row to the core terms table. Add note that `WARDEN_AGENT_TYPE` env var controls which CLI launches. Note Codex-specific differences (no native worktree support, AGENTS.md instead of CLAUDE.md).
- [ ] Create `docs/parser.md`
- [ ] `docs/claude-hooks.md` — reframe: hooks are now a supplementary channel for attention/notification state only. JSONL session file parsing is the primary data source. List which hook events are still active (Notification only) and which are now handled by the parser.
- [ ] `docs/ux-flows.md` — add Codex-specific flows: project creation with agent selector, worktree creation (Warden-managed), terminal connection (--no-alt-screen).

#### Codemaps (must reflect code changes)

- [ ] `docs/codemaps/backend/engine.md` — renamed types (AgentStatus, engine client rename), registry pattern, JSONL watcher lifecycle, `checkAgentStatus` with per-agent process name.
- [ ] `docs/codemaps/backend/api-types.md` — new AgentType field on request/response types, cost terminology split (isSubscriptionCost vs isEstimatedCost).
- [ ] `docs/codemaps/backend/service.md` — mandatory config dir mounts, agent-aware container creation, session watcher lifecycle management.
- [ ] `docs/codemaps/backend/events.md` — JSONL parser as event source alongside hooks. Data flow: JSONL → parser → ParsedEvent → eventbus.Event → store → broker → SSE.
- [ ] `docs/codemaps/backend/database.md` — agent_type column in projects table.
- [ ] `docs/codemaps/backend/supporting.md` — agent/ package restructure: registry, per-agent subpackages (types, parser, pricing, provider), SessionParser interface.
- [ ] `docs/codemaps/container/scripts.md` — new directory structure (shared/, claude/, codex/), composable install scripts, which scripts are dropped/simplified.
- [ ] `docs/codemaps/container/image.md` — both CLIs installed, Dockerfile layer order rationale, build args.
- [ ] `docs/codemaps/container/environment.md` — WARDEN_AGENT_TYPE env var, CODEX_API_KEY (user-provided via env vars field).
- [ ] `docs/codemaps/frontend/components.md` — agent selector component, renamed status indicator (agentStatus).
- [ ] `docs/codemaps/frontend/hooks.md` — agentType in TypeScript types.
- [ ] `docs/codemaps/tui/views.md` — agent type in container form and project detail views.

#### Docs site (public-facing)

- [ ] `docs_site/.../index.mdx` — hero/intro: "Run Claude Code and OpenAI Codex in isolated containers."
- [ ] `docs_site/.../guide/installation.md` — prereqs: mention Codex API key or ChatGPT subscription for Codex projects.
- [ ] `docs_site/.../guide/getting-started.md` — show agent selector in the "create project" walkthrough. Brief Codex quick start alongside the existing Claude walkthrough.
- [ ] `docs_site/.../features/projects.md` — explain agent type field, note that each project is locked to one agent type.
- [ ] `docs_site/.../features/worktrees.md` — note that Codex worktrees are managed by Warden (git worktree add/remove), while Claude Code manages its own via --worktree flag. Functionally equivalent from the user's perspective.
- [ ] `docs_site/.../features/cost-budget.md` — explain the two cost models: actual cost (Claude API users), subscription cost (Claude Pro/Max users, real cost but not billed), estimated cost (Codex API and ChatGPT subscription, computed from tokens). Explain pricing table and its limitations.
- [ ] `docs_site/.../features/audit.md` — JSONL-sourced events are the primary audit data. Note that Codex notification/attention events are a known limitation — link upstream issues.
- [ ] `docs_site/.../comparison.md` — update comparison table to reflect multi-agent support as a feature.
- [ ] `docs_site/.../faq.md` — add: "What/Why is cost estimated?", "Can I use both agents in the same project?" (no — one agent per project).
- [ ] `docs_site/.../contributing.md` — add section: "Adding a new agent" — create subpackage in agent/, implement SessionParser and StatusProvider, add install script, register in warden.go.
- [ ] `docs_site/.../integration/architecture.md` — add JSONL parser layer to the architecture diagram. Show data flow: session file → fsnotify → parser → eventbus → SSE.
- [ ] `docs_site/.../integration/paths.md` — highlight agent/codex and agent/claudecode as importable subpackages. Developers can use the JSONL parsers independently to build their own tooling on top of Claude Code or Codex session data without importing the full Warden engine.
- [ ] `docs_site/.../integration/go-library.md` — show warden.New() with registry setup, note AgentType in Options if added.
- [ ] `docs_site/.../integration/go-client.md` — show agentType field in create/list requests.
- [ ] `docs_site/.../integration/http-api.mdx` — add agentType to endpoint schemas (auto-generated from OpenAPI, so update swagger.yaml).
- [ ] `docs_site/.../reference/go/agent.md` — auto-generated by gomarkdoc, just verify it picks up new subpackages.
- [ ] `docs_site/.../reference/go/engine.md` — auto-generated, verify renamed types appear.
- [ ] `docs_site/.../reference/go/index.md` — manually add agent/codex to the package list.
- [ ] `docs_site/generate-go-docs.sh` — add `agent/codex` to the `PACKAGES` array.

### Future (not in scope)

- [ ] Move Claude Notification hook to JSONL parser when Claude adds attention/notification events to their session JSONL format.
- [ ] Add Codex notification/attention support when upstream adds it — either via hooks ([#14813](https://github.com/openai/codex/issues/14813)) or via JSONL.
- [ ] **Investigate OSC terminal title sequences for unified attention detection.** Terminal apps set window titles via `OSC 2` escape sequences (e.g., `\033]2;✱ Claude Code\007`). WezTerm renders these — the `✱` symbol appears when Claude needs attention. If both Claude and Codex set distinct title strings for different states (waiting for permission, idle, working), the Go backend could sniff these from the PTY stream it already proxies via WebSocket. This would be a fully agent-agnostic attention detection mechanism — no hooks, no JSONL, just standard terminal escape sequences. Only works when a viewer is connected (WebSocket active). When backgrounded, fall back to JSONL/hooks. Investigate what titles each agent sets and whether they're reliable enough to replace/supplement hooks for attention state.

---

# Package Organization Audit

Separate from the multi-agent work. The agent split validated that the `agent/` subpackage pattern works — adding `agent/codex/` alongside `agent/claudecode/` is clean and the registry pattern means only `warden.go` imports all subpackages.

But the broader package structure deserves the same kind of dependency graph analysis.

We need to decide on a concrete rule that determines whether something is a top-level package or not.

## Approach

Map the actual dependency graph and ask for each package:

1. What imports it? What does it import?
2. Does adding a new feature to this package force imports into unexpected places?
3. If we added 5 more of "this kind of thing," would the structure still hold?
4. Is there a circular dependency risk or a package that's becoming a grab-bag?

## Known questions

- **Top-level vs `internal/` boundary**: Current rule is "importable by external Go consumers" → top-level. Is this consistently applied?
- **`access/`** is top-level (importable credential resolution) — does it makes sense?
- **Audit** is spread across `db.AuditWriter` + `eventbus` + `api/` types. Should it be its own package? Or is the spread correct because audit is a cross-cutting concern?
- **`eventbus/`** handles event delivery. The new JSONL watcher is also event delivery infrastructure. Does the watcher belong in `eventbus/`, in `engine/`, or in its own package? The watcher is generic (tail JSONL, call parser func), while the parser is agent-specific. Clean separation would be: generic watcher infrastructure in one place, agent-specific parsers in `agent/<name>/`.
- **`api/` vs `engine/` types**: Some types live in `api/` (request/response), some in `engine/` (Project, Worktree, CreateContainerRequest). The boundary is roughly "API contract types" vs "engine domain types," but `CreateContainerRequest` is in `engine/` even though it's also the API request body. Is this right or should it move to `api/`?
- **`db/`** has `Store`, `AuditWriter`, `Entry`, `ProjectRow`, and the schema. That's persistence + domain types + audit writing. Is that too much for one package?

## Decision boundary

"Would someone building their own integration want to import this?" If yes → top-level. If no → `internal/`.

Secondary test: "If I add 5 more of this kind of thing, does the structure still hold?" This is the test that validated `agent/` — adding more agent subpackages doesn't create pressure elsewhere.

---

# OpenShell Analysis & Warden Comparison

## What is OpenShell?

[OpenShell](https://github.com/NVIDIA/OpenShell) is NVIDIA's open-source (Apache 2.0) runtime for executing autonomous AI agents in isolated, policy-governed sandboxes. Released February 2026, it has quickly gained traction (~3,400 stars) and is currently in alpha with "single-player mode" — designed for a single developer running agents in one environment, with stated plans to evolve toward multi-tenant enterprise deployments.

The core pitch: **safely run AI agents without exposing sensitive data, credentials, or infrastructure**. Sandboxes start with minimal permissions and are selectively opened through declarative YAML policies.

## Architecture

OpenShell uses a three-tier architecture:

1. **User Layer** — CLI (`openshell`) and a k9s-inspired terminal UI (`openshell term`)
2. **Control Plane (Gateway)** — central server managing sandbox lifecycle, policy distribution, credential storage, and SSH tunneling. Communicates via gRPC and HTTP. Persists state in SQLite (default) or Postgres.
3. **Execution Layer** — Kubernetes pods (k3s) running isolated sandbox containers

The entire infrastructure — k3s, gateway, and pre-loaded images — packages into a **single Docker container**. This is the key deployment trick: users don't need to install Kubernetes themselves.

### Sandbox Internals

Each sandbox container runs two processes:

- A **privileged supervisor** that sets up isolation and fetches credentials/policies via gRPC
- A **restricted child process** running the agent as an unprivileged user

Four enforcement layers protect the sandbox:

| Layer      | Mechanism                        | Mutability                  |
| ---------- | -------------------------------- | --------------------------- |
| Filesystem | Landlock (Linux kernel)          | Static — locked at creation |
| Network    | Network namespaces + proxy       | Dynamic — hot-reloadable    |
| Process    | Seccomp syscall filtering        | Static — locked at creation |
| Inference  | Privacy router for LLM API calls | Dynamic — hot-reloadable    |

### Network Proxy

The proxy is the linchpin of the security model. Every outbound connection from the sandbox routes through it. On each request, the proxy:

1. Identifies the requesting program via Linux process inspection
2. Verifies program integrity using SHA256 hash (trust-on-first-use)
3. Evaluates the request against OPA/Rego policies
4. Blocks connections to internal IPs (SSRF prevention)
5. Performs L7 HTTP inspection for configured endpoints

This gives per-binary, per-HTTP-method granularity — e.g., allow `git` to `GET github.com` but block `POST`.

### Inference Routing (Privacy Router)

A dedicated component intercepts LLM API calls. Agents send requests to `https://inference.local`, and the router:

- TLS-terminates the connection
- Strips agent-visible credentials
- Injects backend credentials from the gateway
- Rewrites auth headers and model fields
- Forwards to the configured provider (OpenAI, Anthropic, NVIDIA)

The sandbox never sees real API keys. This is a credential isolation pattern that also enables centralized model routing and cost control.

### Credential Management

"Providers" bundle credentials as named collections. The system auto-discovers environment variables for known agents (e.g., `ANTHROPIC_API_KEY` for Claude Code). Credentials are stored on the gateway and injected as runtime environment variables — never written to the sandbox filesystem.

## Tech Stack

| Component               | Technology                            |
| ----------------------- | ------------------------------------- |
| Core runtime            | Rust                                  |
| CLI / SDK               | Python (PyPI installable)             |
| Policy engine           | OPA / Rego                            |
| Internal comms          | gRPC / Protocol Buffers               |
| Container orchestration | k3s (embedded Kubernetes)             |
| Container runtime       | Docker                                |
| Kernel isolation        | Landlock, Seccomp, network namespaces |

## Supported Agents

- Claude Code (via `ANTHROPIC_API_KEY`)
- OpenCode (via `OPENAI_API_KEY` or `OPENROUTER_API_KEY`)
- Codex (via `OPENAI_API_KEY`)
- GitHub Copilot CLI (via `GITHUB_TOKEN`)
- Community: OpenClaw, Ollama

## Key Features

- **Declarative YAML policies** with hot-reload for network and inference layers
- **GPU passthrough** (experimental) for local inference and fine-tuning
- **SSH tunneling** into sandboxes through the gateway (no direct pod exposure)
- **Custom sandboxes** via Dockerfile or community catalog (BYOC)
- **Terminal UI** (k9s-inspired) for real-time cluster monitoring
- **Agent skills system** — 18+ built-in skills for CLI guidance, debugging, policy generation, issue triage, PR workflows
- **Agent-first development model** — the project itself is built using agent-driven workflows

---

## Comparison with Warden

### Similarities

| Aspect                | Warden                                 | OpenShell                                  |
| --------------------- | -------------------------------------- | ------------------------------------------ |
| Core goal             | Isolated containers for AI agents      | Isolated sandboxes for AI agents           |
| Primary agent         | Claude Code                            | Claude Code (+ others)                     |
| Container runtime     | Docker / Podman                        | Docker (via k3s)                           |
| Network isolation     | iptables-based (full/restricted/none)  | Network namespaces + proxy (policy-driven) |
| Terminal access       | WebSocket via Go proxy → abduco        | SSH tunnel via gateway → pod               |
| Credential injection  | Env vars via `docker run -e`           | Env vars via supervisor gRPC fetch         |
| Agent status tracking | Reads `~/.claude.json` via docker exec | Not documented (likely similar)            |
| Open source           | Apache 2.0                             | Apache 2.0                                 |

### Key Differences

| Dimension                | Warden                                                                        | OpenShell                                                                    |
| ------------------------ | ----------------------------------------------------------------------------- | ---------------------------------------------------------------------------- |
| **Philosophy**           | Engine-first library + reference UIs                                          | Platform-first with CLI/TUI                                                  |
| **Complexity**           | Lightweight — Docker containers, no orchestrator                              | Heavy — embeds k3s Kubernetes cluster                                        |
| **UI**                   | Web dashboard (React SPA) + TUI (Bubble Tea)                                  | k9s-style TUI only (no web UI)                                               |
| **Go library**           | Yes — `warden.New()` importable by external consumers                         | No — Rust crates, Python SDK                                                 |
| **Policy model**         | Simple 3-mode network isolation (full/restricted/none) with domain allowlists | Rich OPA/Rego policies with per-binary, per-HTTP-method granularity          |
| **Filesystem isolation** | Docker bind mounts + symlink resolution                                       | Landlock kernel-level enforcement                                            |
| **Process isolation**    | Docker defaults                                                               | Seccomp syscall filtering                                                    |
| **Inference routing**    | Not present — agent uses its own API key directly                             | Privacy router strips/injects credentials, enables centralized model routing |
| **Multi-agent**          | Multiple worktrees per project, multiple projects                             | One agent per sandbox (multi-tenant planned)                                 |
| **Worktree model**       | First-class git worktrees with UI management                                  | Not present — sandboxes are independent                                      |
| **Attention tracking**   | Hook events → event bus → SSE to frontend                                     | Not documented                                                               |
| **Custom images**        | Devcontainer feature injection                                                | BYOC via Dockerfile or catalog                                               |
| **GPU support**          | Not present                                                                   | Experimental GPU passthrough                                                 |
| **Cost tracking**        | Reads Claude Code metrics, displays per-project                               | Not present                                                                  |
| **State persistence**    | Filesystem (config file, event log)                                           | SQLite or Postgres via gateway                                               |

### Where OpenShell is Stronger

1. **Security depth** — Landlock + Seccomp + OPA policies + process integrity verification is significantly more granular than Warden's iptables-based network modes. OpenShell can enforce per-binary, per-endpoint, per-HTTP-method rules.

2. **Credential isolation** — The privacy router ensures agents never see real API keys. Warden passes keys directly as env vars, meaning a compromised agent has the raw credential.

3. **Policy expressiveness** — OPA/Rego policies are a well-understood, auditable standard. Warden's three network modes are simple but coarse.

4. **Agent agnosticism** — OpenShell supports multiple agent CLIs out of the box. Warden is tightly coupled to Claude Code (provider abstraction exists but only one implementation).

5. **GPU support** — Enables local inference workloads, which is increasingly relevant as models get smaller.

## Feature Ideas for Warden

### Reaching Parity

These features would close the most impactful gaps with OpenShell:

#### Medium Complexity

1. **Granular network policies**
   - Move beyond the 3-mode model to support per-domain, per-port rules configurable from the UI
   - Allow HTTP method restrictions (e.g., allow GET but block POST to a domain)
   - Support policy hot-reload without container restart (currently requires restart for domain changes)

2. **Policy-as-code**
   - Support declarative YAML/JSON policy files that can be version-controlled
   - Allow policy templates (e.g., "web-development" preset with npm registry + GitHub access)

#### Needs Investigation

1. **Package organization audit**
   - Current boundary: top-level = importable by external Go consumers, `internal/` = HTTP/UI plumbing.
   - Question: Is this consistently applied? e.g., `access/` is top-level (importable credential resolution), but audit functionality is spread across `db.AuditWriter` + `eventbus` + `api/` types rather than being its own package. Should audit be consolidated? Should `agent/` contain the JSONL parser or should parsing be a separate package?
   - Decision boundary needs clarifying: "Would someone building their own integration want to import this?" If yes → top-level. If no → `internal/`.
   - Not blocking — just needs a pass to ensure consistency as new packages (like a JSONL parser) are added.

### Standing Out (Beyond Parity)

These features would leverage Warden's unique strengths to differentiate:

#### Medium Complexity

1. **Project templates and presets**
   - One-click project setup with pre-configured network policies, environment variables, and CLAUDE.md files
   - Community-shareable templates (e.g., "Next.js + Supabase" preset)
   - Import/export project configurations
     t - Allow user to choose some tools to install at container creation time.

2. **Intelligent network policy suggestions**
   - Monitor agent network requests in permissive mode
   - Suggest a minimal policy based on observed traffic
   - "Learning mode" → "enforcement mode" workflow
   - OpenShell has a skill for generating policies from natural language, but traffic-based suggestion is more precise
