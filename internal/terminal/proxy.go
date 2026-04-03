// Package terminal provides a WebSocket-to-PTY proxy that bridges browser
// terminals to tmux sessions running inside containers via the Docker exec API.
package terminal

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/coder/websocket"
	dtypes "github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"

	"github.com/thesimonho/warden/engine"
)

// pingInterval is how often the server pings the browser to detect dead connections.
const pingInterval = 30 * time.Second

// pingTimeout is how long the server waits for a pong before closing.
const pingTimeout = 10 * time.Second

// readLimit is the maximum WebSocket message size from the browser.
// Terminal input is typically tiny, but paste events can be large.
const readLimit = 128 * 1024 // 128 KB

// containerUser references the non-root user inside project containers.
var containerUser = engine.ContainerUser

// ExecAPI is the subset of the Docker client used by the proxy.
// Both Docker and Podman implement these through the same SDK.
type ExecAPI interface {
	ContainerExecCreate(ctx context.Context, container string, options container.ExecOptions) (container.ExecCreateResponse, error)
	ContainerExecAttach(ctx context.Context, execID string, options container.ExecStartOptions) (dtypes.HijackedResponse, error)
	ContainerExecResize(ctx context.Context, execID string, options container.ResizeOptions) error
}

// resizeMsg is the JSON control message sent from xterm.js when the terminal
// dimensions change.
type resizeMsg struct {
	Type string `json:"type"`
	Cols uint   `json:"cols"`
	Rows uint   `json:"rows"`
}

// Proxy bridges WebSocket connections to container PTYs via docker exec.
type Proxy struct {
	api ExecAPI
}

// NewProxy creates a terminal proxy backed by the given Docker/Podman exec API.
func NewProxy(api ExecAPI) *Proxy {
	return &Proxy{api: api}
}

// ServeWS upgrades the HTTP request to a WebSocket and bridges it to a tmux
// session inside the container. The connection stays open until the browser
// disconnects or the exec process exits.
//
// Before attaching the live stream, scrollback is captured from the tmux pane
// and sent to the client. For fresh sessions this is empty; for reconnects it
// fills the gap between the user's last disconnect and now.
//
// The caller is responsible for validating containerID and worktreeID.
func (p *Proxy) ServeWS(w http.ResponseWriter, r *http.Request, containerID, worktreeID string) {
	conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{
		// Terminal data is raw bytes, compression adds latency for negligible gain.
		CompressionMode: websocket.CompressionDisabled,
	})
	if err != nil {
		slog.Error("websocket accept failed", "err", err.Error())
		return
	}
	defer conn.CloseNow() //nolint:errcheck

	conn.SetReadLimit(readLimit)

	ctx := r.Context()

	// Capture tmux scrollback and send it before attaching the live stream.
	// For fresh sessions this returns empty; for reconnects it replays output
	// the user missed while disconnected.
	sessionName := fmt.Sprintf("warden-%s", worktreeID)
	scrollback, scrollErr := p.captureScrollback(ctx, containerID, sessionName)
	if scrollErr != nil {
		slog.Warn("scrollback capture failed", "container", containerID, "worktree", worktreeID, "err", scrollErr)
	} else if len(scrollback) > 0 {
		conn.Write(ctx, websocket.MessageBinary, scrollback) //nolint:errcheck
	}

	// Create a docker exec with TTY that attaches to the tmux session.
	// tmux attach-session is the viewer — killing it won't affect the server session.
	execResp, err := p.api.ContainerExecCreate(ctx, containerID, container.ExecOptions{
		Cmd:          []string{"tmux", "attach-session", "-t", sessionName},
		User:         containerUser,
		Env:          []string{"TERM=xterm-256color"},
		AttachStdin:  true,
		AttachStdout: true,
		AttachStderr: true,
		Tty:          true,
	})
	if err != nil {
		slog.Warn("exec create failed", "container", containerID, "worktree", worktreeID, "err", err)
		conn.Close(websocket.StatusInternalError, "failed to create exec") //nolint:errcheck
		return
	}

	hijacked, err := p.api.ContainerExecAttach(ctx, execResp.ID, container.ExecStartOptions{
		Tty: true,
	})
	if err != nil {
		slog.Warn("exec attach failed", "container", containerID, "worktree", worktreeID, "err", err)
		conn.Close(websocket.StatusInternalError, "failed to attach exec") //nolint:errcheck
		return
	}
	defer hijacked.Close()

	slog.Info("terminal websocket connected", "container", containerID, "worktree", worktreeID)

	// Bridge bidirectionally until either side closes.
	bridgeErr := p.bridge(ctx, conn, hijacked, execResp.ID)
	if bridgeErr != nil {
		slog.Debug("terminal bridge closed", "container", containerID, "worktree", worktreeID, "err", bridgeErr)
	}

	conn.Close(websocket.StatusNormalClosure, "") //nolint:errcheck
}

// bridge pipes data between the WebSocket and the hijacked exec stream.
// Returns when either side closes or errors.
func (p *Proxy) bridge(ctx context.Context, conn *websocket.Conn, hijacked dtypes.HijackedResponse, execID string) error {
	// Use a cancellable context so either goroutine can stop the other.
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	var wg sync.WaitGroup
	errCh := make(chan error, 2)

	// PTY → WebSocket: read from exec, write binary frames to browser.
	wg.Add(1)
	go func() {
		defer wg.Done()
		errCh <- p.ptyToWS(ctx, conn, hijacked.Reader)
		cancel()
	}()

	// WebSocket → PTY: read frames from browser, write to exec stdin.
	// Binary frames are terminal input. Text frames are control messages (resize).
	wg.Add(1)
	go func() {
		defer wg.Done()
		errCh <- p.wsToPTY(ctx, conn, hijacked.Conn, execID)
		cancel()
	}()

	// Start the ping loop to detect dead browser connections.
	wg.Add(1)
	go func() {
		defer wg.Done()
		p.pingLoop(ctx, conn)
		cancel()
	}()

	// Wait for the first error, then let the deferred cancel() stop the rest.
	wg.Wait()

	// Return the first non-nil error.
	close(errCh)
	for err := range errCh {
		if err != nil {
			return err
		}
	}
	return nil
}

// ptyToWS reads raw PTY output and writes it as binary WebSocket frames.
// The reader is wrapped with AltScreenFilter to strip alternate screen
// escape sequences, forcing applications (like Claude Code) to render in
// the normal buffer where xterm.js scrollback works.
func (p *Proxy) ptyToWS(ctx context.Context, conn *websocket.Conn, reader io.Reader) error {
	filtered := NewAltScreenFilter(reader)
	buf := make([]byte, 32*1024)
	for {
		n, err := filtered.Read(buf)
		if n > 0 {
			writeErr := conn.Write(ctx, websocket.MessageBinary, buf[:n])
			if writeErr != nil {
				return fmt.Errorf("ws write: %w", writeErr)
			}
		}
		if err != nil {
			if errors.Is(err, io.EOF) || errors.Is(err, context.Canceled) {
				return nil
			}
			return fmt.Errorf("pty read: %w", err)
		}
	}
}

// wsToPTY reads WebSocket frames and writes them to the exec's stdin.
// Binary frames contain terminal input (keystrokes, paste).
// Text frames contain JSON control messages (resize).
func (p *Proxy) wsToPTY(ctx context.Context, conn *websocket.Conn, writer io.Writer, execID string) error {
	for {
		msgType, data, err := conn.Read(ctx)
		if err != nil {
			if websocket.CloseStatus(err) == websocket.StatusNormalClosure ||
				websocket.CloseStatus(err) == websocket.StatusGoingAway ||
				errors.Is(err, context.Canceled) {
				return nil
			}
			return fmt.Errorf("ws read: %w", err)
		}

		switch msgType {
		case websocket.MessageBinary:
			if _, writeErr := writer.Write(data); writeErr != nil {
				return fmt.Errorf("pty write: %w", writeErr)
			}
		case websocket.MessageText:
			p.handleControlMessage(ctx, data, execID)
		}
	}
}

// handleControlMessage processes a JSON text frame from the browser.
// Currently supports resize messages.
func (p *Proxy) handleControlMessage(ctx context.Context, data []byte, execID string) {
	var msg resizeMsg
	if err := json.Unmarshal(data, &msg); err != nil {
		slog.Debug("ignoring malformed control message", "err", err)
		return
	}

	if msg.Type != "resize" || msg.Cols == 0 || msg.Rows == 0 {
		return
	}

	if err := p.api.ContainerExecResize(ctx, execID, container.ResizeOptions{
		Height: msg.Rows,
		Width:  msg.Cols,
	}); err != nil {
		slog.Debug("exec resize failed", "err", err, "cols", msg.Cols, "rows", msg.Rows)
	}
}

// scrollbackTimeout bounds how long we wait for tmux capture-pane.
// A slow container or large scrollback shouldn't delay the terminal indefinitely.
const scrollbackTimeout = 5 * time.Second

// captureScrollback runs `tmux capture-pane` inside the container to grab the
// session's scrollback buffer. Returns the raw output bytes suitable for
// writing directly to the WebSocket as a binary frame.
//
// Uses TTY mode so Docker returns raw output without multiplexing headers.
func (p *Proxy) captureScrollback(ctx context.Context, containerID, sessionName string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(ctx, scrollbackTimeout)
	defer cancel()

	execResp, err := p.api.ContainerExecCreate(ctx, containerID, container.ExecOptions{
		Cmd:          []string{"tmux", "capture-pane", "-t", sessionName, "-p", "-S", "-50000"},
		User:         containerUser,
		AttachStdout: true,
		Tty:          true,
	})
	if err != nil {
		return nil, fmt.Errorf("scrollback exec create: %w", err)
	}

	hijacked, err := p.api.ContainerExecAttach(ctx, execResp.ID, container.ExecStartOptions{
		Tty: true,
	})
	if err != nil {
		return nil, fmt.Errorf("scrollback exec attach: %w", err)
	}
	defer hijacked.Close()

	data, err := io.ReadAll(hijacked.Reader)
	if err != nil {
		return nil, fmt.Errorf("scrollback read: %w", err)
	}

	return data, nil
}

// pingLoop sends WebSocket pings at regular intervals to detect dead connections.
// Browsers don't send close frames when tabs are killed or networks drop.
func (p *Proxy) pingLoop(ctx context.Context, conn *websocket.Conn) {
	ticker := time.NewTicker(pingInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			pingCtx, cancel := context.WithTimeout(ctx, pingTimeout)
			err := conn.Ping(pingCtx)
			cancel()
			if err != nil {
				slog.Debug("ping failed, closing connection", "err", err)
				return
			}
		}
	}
}
