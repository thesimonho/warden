# Terminals

A **terminal** is the connection into a worktree's process. It is a disposable viewer -- closing the terminal does not kill the agent. The agent keeps running inside its tmux session, and you can reconnect later and pick up where you left off.

Terminals are not a separate resource you create and manage. They are an aspect of a worktree's lifecycle. You connect to a worktree, and the terminal exists for the duration of that connection.

## Process architecture

Each worktree runs a tmux session inside the container:

```
tmux (holds the PTY alive across disconnects)
 +-- bash
      +-- claude / codex (or just bash if the agent exited)
```

The tmux session is the critical component. Killing it kills the agent and bash underneath. The WebSocket connection is disposable -- it is just a viewer into the tmux session. Multiple disconnects and reconnects are normal and expected.

tmux is configured with: `status off` (no status bar), `mouse off` (events pass through to xterm.js), `history-limit 50000` (scrollback buffer), `window-size latest` (resizes to the most recently attached client). Agents run with the `TMUX` env var unset so they do not detect the tmux wrapper.

## Terminal lifecycle

The lifecycle has two distinct phases:

1. **Connect** -- starts the tmux session and launches the agent inside it. This is an HTTP POST.
2. **Attach** -- opens a WebSocket to stream PTY I/O to and from the browser. This is a WebSocket upgrade.

These are separate steps because connect is about starting the process, while attach is about viewing it. A worktree can be connected (running in background) without anyone attached.

```
                          HTTP POST
   stopped -------- /connect ---------> connected (background)
                                             |
                                     WebSocket upgrade
                                        /ws/{wid}
                                             |
                                             v
                                    connected (attached)
                                             |
                          +------------------+------------------+
                          |                  |                  |
                    close WebSocket    POST /kill         POST /reset
                          |                  |                  |
                          v                  v                  v
                     background           stopped           stopped
                                                        (session cleared)
```

### Connect is idempotent

Calling connect on a worktree that already has a running tmux session is safe. It returns the same response without restarting the agent. This means reconnecting after a disconnect is simply: POST connect, then open a new WebSocket.

### Disconnect vs Kill vs Reset

| Action         | HTTP call      | tmux session | Agent process | Session files | Exit code |
| -------------- | -------------- | ------------ | ------------- | ------------- | --------- |
| **Disconnect** | POST /disconnect | Kept alive | Keeps running | Preserved     | N/A       |
| **Kill**       | POST /kill       | Destroyed  | Killed        | Preserved     | Written   |
| **Reset**      | POST /reset      | Destroyed  | Killed        | Deleted       | Deleted   |

**Disconnect** is non-destructive. The agent continues working. Use it when the user navigates away.

**Kill** terminates the process but preserves session files and writes an exit code. The next connect will auto-resume the conversation.

**Reset** terminates the process and clears all session state. The next connect starts a completely fresh conversation.

## Auto-resume

When connecting to a worktree after the agent has exited, Warden automatically resumes the previous conversation instead of starting fresh. This happens transparently -- the agent launches with `--continue` (Claude Code) or `resume --last` (Codex).

Auto-resume triggers when an `exit_code` file exists in the terminal tracking directory. This file is written:

- When the **kill** endpoint is called (writes `exit_code=137`)
- When the **container restarts** (writes `exit_code=137` for orphaned terminal dirs)
- When the **agent exits normally** (writes the actual exit code)

Auto-resume does NOT trigger when:

- The worktree is **reset** (session files and exit code are deleted)
- The worktree is **removed** (entire terminal directory is deleted)

This is the key behavioral difference between kill and reset. Kill preserves the ability to resume; reset guarantees a fresh start.

## WebSocket protocol

Terminal I/O streams over WebSocket at:

```
GET /api/v1/projects/{projectId}/{agentType}/ws/{wid}
```

Replace `{wid}` with the worktree ID (e.g., `main` or `fix-auth-bug`).

### Frame types

| Direction       | Frame type | Content                                     |
| --------------- | ---------- | ------------------------------------------- |
| Server to client | Binary    | PTY output (terminal content)               |
| Client to server | Binary    | PTY input (keystrokes, paste)               |
| Client to server | Text      | JSON control messages (resize)              |
| Server to client | Ping      | Heartbeat every 30 seconds                  |
| Client to server | Pong      | Response to ping (handled by WebSocket lib) |

### Resize messages

When the terminal dimensions change, send a text frame with:

```json
{"type": "resize", "cols": 120, "rows": 40}
```

Both `cols` and `rows` must be positive integers. Messages with zero values are ignored.

### Scrollback replay

On WebSocket connect, the server captures the tmux scrollback buffer (up to 5000 lines) via `tmux capture-pane` and sends it as the first binary frame before attaching the live stream. This fills the gap between the user's last disconnect and now.

For fresh sessions, the scrollback is empty. For reconnects after a period of background work, it contains all the output the agent produced while no viewer was attached.

The scrollback capture has a 5-second timeout. If it fails (e.g., slow container), the connection proceeds without replay and a warning is logged server-side.

### Connection lifecycle

The server sends ping frames every 30 seconds. If a pong is not received within 10 seconds, the connection is closed. This detects dead browser tabs and dropped networks.

The server also strips alternate screen escape sequences from the PTY output. This forces applications like Claude Code (which uses Ink/React for rendering) to render in the normal buffer where xterm.js scrollback works correctly.

### Conceptual WebSocket example

```javascript
// Connect the terminal first (idempotent)
await fetch(`http://localhost:8090/api/v1/projects/${projectId}/claude-code/worktrees/${wid}/connect`, {
  method: 'POST'
});

// Open WebSocket for PTY I/O
const ws = new WebSocket(`ws://localhost:8090/api/v1/projects/${projectId}/claude-code/ws/${wid}`);

// First binary message is scrollback replay (may be empty)
ws.binaryType = 'arraybuffer';

ws.onmessage = (event) => {
  if (event.data instanceof ArrayBuffer) {
    // PTY output -- write to xterm.js
    terminal.write(new Uint8Array(event.data));
  }
};

// Send keystrokes as binary frames
terminal.onData((data) => {
  ws.send(new TextEncoder().encode(data));
});

// Send resize as text frame
terminal.onResize(({ cols, rows }) => {
  ws.send(JSON.stringify({ type: 'resize', cols, rows }));
});
```

## Clipboard

Warden provides clipboard support for pasting images into the agent's context.

### Image upload

Upload an image file via multipart form data. The image is staged in the container's clipboard directory, and the agent's xclip shim serves it when the agent reads the clipboard.

```bash
curl -X POST http://localhost:8090/api/v1/projects/a1b2c3d4e5f6/claude-code/clipboard \
  -F "file=@screenshot.png"
```

Response:

```json
{
  "path": "/home/warden/.local/share/warden/clipboard/screenshot.png"
}
```

The maximum file size is 10 MB. Exceeding this returns `413 Request Entity Too Large`.

For Claude Code, the xclip shim intercepts clipboard read calls and returns the staged file. For Codex, the agent receives the file path directly.

### Text clipboard

Text clipboard is handled via OSC 52 escape sequences in the terminal stream. The agent writes an OSC 52 sequence to stdout, xterm.js intercepts it, and the browser copies the text to the system clipboard. No separate API endpoint is needed -- this flows through the WebSocket connection automatically.

## API patterns

All terminal endpoints are scoped to a specific worktree:

```
/api/v1/projects/{projectId}/{agentType}/worktrees/{wid}/...
```

### Connect terminal

Starts a tmux session and launches the agent. Safe to call on an already-running worktree.

```bash
curl -X POST http://localhost:8090/api/v1/projects/a1b2c3d4e5f6/claude-code/worktrees/main/connect
```

Response (201 Created):

```json
{
  "projectId": "a1b2c3d4e5f6",
  "worktreeId": "main"
}
```

### Disconnect terminal

Closes the WebSocket viewer. The tmux session and agent continue running in the background. The worktree transitions from `connected` to `background`.

```bash
curl -X POST http://localhost:8090/api/v1/projects/a1b2c3d4e5f6/claude-code/worktrees/main/disconnect
```

Response:

```json
{
  "projectId": "a1b2c3d4e5f6",
  "worktreeId": "main"
}
```

### Kill worktree process

Destroys the tmux session and all child processes. Writes an exit code so the next connect auto-resumes.

```bash
curl -X POST http://localhost:8090/api/v1/projects/a1b2c3d4e5f6/claude-code/worktrees/main/kill
```

Response:

```json
{
  "projectId": "a1b2c3d4e5f6",
  "worktreeId": "main"
}
```

### Reset worktree

Kills the process and clears all session state. The next connect starts a fresh conversation with no auto-resume.

```bash
curl -X POST http://localhost:8090/api/v1/projects/a1b2c3d4e5f6/claude-code/worktrees/main/reset
```

Response:

```json
{
  "projectId": "a1b2c3d4e5f6",
  "worktreeId": "main"
}
```

## Edge cases and important behaviors

- **WebSocket without connect**: opening a WebSocket to a worktree that has no tmux session will fail. Always POST to `/connect` first.
- **Multiple WebSocket connections**: only one viewer should be attached at a time. Opening a second WebSocket to the same worktree may produce undefined behavior.
- **Container not running**: connect returns `404` if the project's container is not running. Start the container first via the container endpoints.
- **Scrollback size**: the server captures the last 5000 lines of scrollback. For long-running agents that produce extensive output, earlier content is lost. The agent's session files (JSONL) are the authoritative record.
- **Agent exit detection**: when the agent process exits inside tmux, bash remains as the shell. The worktree transitions to the `shell` state. The exit code is captured and included in the list worktrees response.
