package client

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"strings"

	"github.com/coder/websocket"

	"github.com/thesimonho/warden/api"
	"github.com/thesimonho/warden/eventbus"
)

// SubscribeEvents opens a Server-Sent Events connection and returns a
// channel of parsed events. The channel closes when the context is
// cancelled or the connection drops. Call the returned function to
// unsubscribe and clean up the connection.
//
// Event types sent on the channel:
//   - "worktree_state": worktree attention/session state changed (Data contains containerName, worktreeId, state fields)
//   - "project_state": project cost updated (Data contains containerName, totalCost)
//   - "worktree_list_changed": worktrees were created/removed (Data contains containerName)
//   - "heartbeat": periodic keepalive (Data is empty object)
//
// Each event's Data field is a json.RawMessage — unmarshal it based on
// the Event type.
//
// Example:
//
//	ch, unsub, err := c.SubscribeEvents(ctx)
//	if err != nil { return err }
//	defer unsub()
//
//	for event := range ch {
//	    switch event.Event {
//	    case eventbus.SSEWorktreeState:
//	        // A worktree's state changed — refresh worktree list
//	    case eventbus.SSEProjectState:
//	        // Cost updated — refresh project list
//	    }
//	}
func (c *Client) SubscribeEvents(ctx context.Context) (<-chan eventbus.SSEEvent, func(), error) {
	ctx, cancel := context.WithCancel(ctx)
	ch := make(chan eventbus.SSEEvent, 64)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/api/v1/events", nil)
	if err != nil {
		cancel()
		return nil, nil, err
	}
	req.Header.Set("Accept", "text/event-stream")

	// Use a client without timeout for the long-lived SSE connection.
	sseClient := &http.Client{}
	resp, err := sseClient.Do(req)
	if err != nil {
		cancel()
		return nil, nil, fmt.Errorf("SSE connect: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		_ = resp.Body.Close()
		cancel()
		return nil, nil, fmt.Errorf("SSE connect: HTTP %d", resp.StatusCode)
	}

	go readSSEStream(ctx, resp.Body, ch)

	unsub := func() {
		cancel()
		_ = resp.Body.Close()
	}
	return ch, unsub, nil
}

// readSSEStream parses an SSE text stream and sends events to the channel.
// Closes the channel when the stream ends or the context is cancelled.
func readSSEStream(ctx context.Context, body io.ReadCloser, ch chan<- eventbus.SSEEvent) {
	defer close(ch)
	defer body.Close() //nolint:errcheck

	scanner := bufio.NewScanner(body)
	var eventType string
	var dataLines []byte

	for scanner.Scan() {
		if ctx.Err() != nil {
			return
		}

		line := scanner.Text()

		switch {
		case strings.HasPrefix(line, "event:"):
			eventType = strings.TrimSpace(strings.TrimPrefix(line, "event:"))

		case strings.HasPrefix(line, "data:"):
			data := strings.TrimPrefix(line, "data:")
			data = strings.TrimSpace(data)
			dataLines = []byte(data)

		case line == "":
			// Empty line = end of event.
			if eventType != "" && dataLines != nil {
				event := eventbus.SSEEvent{
					Event: eventbus.SSEEventType(eventType),
					Data:  json.RawMessage(dataLines),
				}
				select {
				case ch <- event:
				case <-ctx.Done():
					return
				}
			}
			eventType = ""
			dataLines = nil
		}
	}
}

// --- Terminal Attachment ---

// AttachTerminal opens a WebSocket connection to a worktree's terminal
// and returns a [TerminalConnection] for bidirectional PTY I/O.
//
// Prerequisites: [ConnectTerminal] must be called first to ensure the
// terminal process (tmux session) is running. AttachTerminal creates
// a viewer into the existing session.
//
// The terminal lifecycle:
//  1. [ConnectTerminal] — start the tmux session (idempotent)
//  2. AttachTerminal — open a viewer connection
//  3. Read/Write on the [TerminalConnection] — PTY I/O
//  4. Close the connection (or the user presses the disconnect key)
//  5. [DisconnectTerminal] — notify the server the viewer closed
//
// The tmux session survives viewer disconnects. Call
// [KillWorktreeProcess] to fully terminate the session.
func (c *Client) AttachTerminal(ctx context.Context, projectID, agentType, worktreeID string) (TerminalConnection, error) {
	wsURL := strings.Replace(c.baseURL, "http://", "ws://", 1)
	wsURL = strings.Replace(wsURL, "https://", "wss://", 1)
	wsURL += "/api/v1/projects/" + projectID + "/" + agentType + "/ws/" + worktreeID

	conn, _, err := websocket.Dial(ctx, wsURL, &websocket.DialOptions{
		CompressionMode: websocket.CompressionDisabled,
	})
	if err != nil {
		return nil, fmt.Errorf("websocket dial: %w", err)
	}

	conn.SetReadLimit(128 * 1024) // 128KB, matches proxy.go

	return &wsTerminalConn{
		conn: conn,
		ctx:  ctx,
	}, nil
}

// wsTerminalConn wraps a WebSocket connection as a TerminalConnection.
type wsTerminalConn struct {
	conn *websocket.Conn
	ctx  context.Context
}

// Read reads PTY output from the WebSocket (binary frames).
func (w *wsTerminalConn) Read(p []byte) (int, error) {
	_, data, err := w.conn.Read(w.ctx)
	if err != nil {
		return 0, err
	}
	n := copy(p, data)
	return n, nil
}

// Write sends keyboard input via the WebSocket (binary frames).
func (w *wsTerminalConn) Write(p []byte) (int, error) {
	err := w.conn.Write(w.ctx, websocket.MessageBinary, p)
	if err != nil {
		return 0, err
	}
	return len(p), nil
}

// Close closes the WebSocket connection.
func (w *wsTerminalConn) Close() error {
	return w.conn.Close(websocket.StatusNormalClosure, "")
}

// Resize sends a JSON resize command via the WebSocket (text frame).
func (w *wsTerminalConn) Resize(cols, rows uint) error {
	msg, err := json.Marshal(map[string]any{
		"type": "resize",
		"cols": cols,
		"rows": rows,
	})
	if err != nil {
		return err
	}
	return w.conn.Write(w.ctx, websocket.MessageText, msg)
}

// TerminalConnection provides raw bidirectional I/O to a container terminal.
// Read returns PTY output, Write sends keyboard input, and Resize updates
// the remote terminal dimensions.
//
// In HTTP mode, this wraps a WebSocket connection (binary frames for I/O,
// text frames for resize commands). In embedded mode, this wraps a docker
// exec session attached to the tmux session.
//
// Example:
//
//	conn, err := c.AttachTerminal(ctx, projectID, agentType, worktreeID)
//	if err != nil { return err }
//	defer conn.Close()
//
//	conn.Resize(80, 24) // set initial terminal size
//	conn.Write([]byte("ls\n"))
//
//	buf := make([]byte, 4096)
//	n, _ := conn.Read(buf)
//	fmt.Print(string(buf[:n]))
type TerminalConnection interface {
	io.ReadWriteCloser

	// Resize changes the terminal dimensions of the remote PTY.
	Resize(cols, rows uint) error
}

// --- Clipboard ---

// UploadClipboard stages an image file in the container's clipboard directory
// for the xclip shim. Returns the path where the file was written.
func (c *Client) UploadClipboard(ctx context.Context, projectID, agentType string, content []byte, mimeType string) (*api.ClipboardUploadResponse, error) {
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)

	part, err := writer.CreateFormFile("file", "paste.png")
	if err != nil {
		return nil, fmt.Errorf("creating form file: %w", err)
	}
	if _, err := part.Write(content); err != nil {
		return nil, fmt.Errorf("writing content: %w", err)
	}
	if err := writer.Close(); err != nil {
		return nil, fmt.Errorf("closing multipart writer: %w", err)
	}

	clipPath := projectPath(projectID, agentType) + "/clipboard"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+clipPath, &buf)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())

	var resp api.ClipboardUploadResponse
	if err := c.do(req, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}
