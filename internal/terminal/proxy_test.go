package terminal

import (
	"bufio"
	"context"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/coder/websocket"
	dtypes "github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"

	"github.com/thesimonho/warden/constants"
	"github.com/thesimonho/warden/engine"
)

// hijackedPipe creates a HijackedResponse backed by an in-memory pipe.
// Writes to the returned writer appear as PTY output in the proxy.
// Reads from the returned reader capture what the proxy writes as PTY input.
func hijackedPipe() (dtypes.HijackedResponse, io.WriteCloser, io.ReadCloser) {
	// PTY output: server writes → proxy reads
	outReader, outWriter := io.Pipe()
	// PTY input: proxy writes → server reads
	inReader, inWriter := io.Pipe()

	conn := &pipeConn{
		Reader: outReader,
		Writer: inWriter,
	}

	hijacked := dtypes.HijackedResponse{
		Conn:   conn,
		Reader: bufio.NewReader(outReader),
	}

	return hijacked, outWriter, inReader
}

// pipeConn wraps an io.Reader and io.Writer as a net.Conn for the hijacked response.
type pipeConn struct {
	io.Reader
	io.Writer
}

func (p *pipeConn) Close() error {
	var errs []error
	if c, ok := p.Reader.(io.Closer); ok {
		errs = append(errs, c.Close())
	}
	if c, ok := p.Writer.(io.Closer); ok {
		errs = append(errs, c.Close())
	}
	for _, err := range errs {
		if err != nil {
			return err
		}
	}
	return nil
}

func (p *pipeConn) LocalAddr() net.Addr                { return pipeAddr{} }
func (p *pipeConn) RemoteAddr() net.Addr               { return pipeAddr{} }
func (p *pipeConn) SetDeadline(_ time.Time) error      { return nil }
func (p *pipeConn) SetReadDeadline(_ time.Time) error  { return nil }
func (p *pipeConn) SetWriteDeadline(_ time.Time) error { return nil }

type pipeAddr struct{}

func (pipeAddr) Network() string { return "pipe" }
func (pipeAddr) String() string  { return "pipe" }

// mockExecAPI implements ExecAPI for testing.
// The first exec create/attach pair is consumed by scrollback capture
// (returns an empty pipe that closes immediately). The second pair is
// the actual tmux attach (uses the provided hijacked pipe).
type mockExecAPI struct {
	mu          sync.Mutex
	hijacked    dtypes.HijackedResponse
	resizes     []container.ResizeOptions
	lastCreate  container.ExecOptions
	createFn    func(ctx context.Context, containerID string, opts container.ExecOptions) (container.ExecCreateResponse, error)
	created     chan struct{} // closed after the attach exec create (second call)
	createdOnce sync.Once
	createCount int
	attachCount int
}

func (m *mockExecAPI) ContainerExecCreate(_ context.Context, _ string, opts container.ExecOptions) (container.ExecCreateResponse, error) {
	m.mu.Lock()
	m.createCount++
	count := m.createCount
	m.lastCreate = opts
	m.mu.Unlock()

	if m.createFn != nil {
		return m.createFn(context.Background(), "", opts)
	}

	// Second create call is the actual attach — signal readiness.
	if count >= 2 && m.created != nil {
		m.createdOnce.Do(func() { close(m.created) })
	}

	return container.ExecCreateResponse{ID: "test-exec-id"}, nil
}

func (m *mockExecAPI) getLastCreate() container.ExecOptions {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.lastCreate
}

func (m *mockExecAPI) ContainerExecAttach(_ context.Context, _ string, _ container.ExecStartOptions) (dtypes.HijackedResponse, error) {
	m.mu.Lock()
	m.attachCount++
	count := m.attachCount
	m.mu.Unlock()

	// First attach is scrollback capture — return an empty pipe that closes immediately.
	if count == 1 {
		r, w := io.Pipe()
		_ = w.Close()
		emptyConn := &pipeConn{Reader: r, Writer: io.Discard}
		return dtypes.HijackedResponse{
			Conn:   emptyConn,
			Reader: bufio.NewReader(r),
		}, nil
	}

	// Second attach is the actual tmux session.
	return m.hijacked, nil
}

func (m *mockExecAPI) ContainerExecResize(_ context.Context, _ string, opts container.ResizeOptions) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.resizes = append(m.resizes, opts)
	return nil
}

func (m *mockExecAPI) getResizes() []container.ResizeOptions {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]container.ResizeOptions(nil), m.resizes...)
}

// dialProxy starts an httptest server with the proxy handler and returns a WebSocket client.
func dialProxy(t *testing.T, proxy *Proxy, containerID, worktreeID string) (*websocket.Conn, *httptest.Server) {
	t.Helper()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		proxy.ServeWS(w, r, containerID, worktreeID)
	}))

	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	conn, _, err := websocket.Dial(ctx, wsURL, nil)
	if err != nil {
		t.Fatalf("dial websocket: %v", err)
	}

	return conn, srv
}

func TestProxyBridgesBinaryData(t *testing.T) {
	hijacked, ptyWriter, ptyReader := hijackedPipe()
	mock := &mockExecAPI{hijacked: hijacked}
	proxy := NewProxy(mock)

	conn, srv := dialProxy(t, proxy, "test-container", "test-worktree")
	defer srv.Close()
	defer conn.CloseNow() //nolint:errcheck

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// PTY → browser: write data to the PTY side, read it from the WebSocket.
	ptyOutput := []byte("hello from PTY\r\n")
	go func() {
		if _, err := ptyWriter.Write(ptyOutput); err != nil {
			t.Log("pty write error:", err)
		}
	}()

	msgType, data, err := conn.Read(ctx)
	if err != nil {
		t.Fatalf("ws read: %v", err)
	}
	if msgType != websocket.MessageBinary {
		t.Errorf("expected binary message, got %v", msgType)
	}
	if string(data) != string(ptyOutput) {
		t.Errorf("expected %q, got %q", ptyOutput, data)
	}

	// Browser → PTY: write data from the WebSocket, read it from the PTY side.
	browserInput := []byte("ls -la\n")
	err = conn.Write(ctx, websocket.MessageBinary, browserInput)
	if err != nil {
		t.Fatalf("ws write: %v", err)
	}

	buf := make([]byte, len(browserInput))
	_, err = io.ReadFull(ptyReader, buf)
	if err != nil {
		t.Fatalf("pty read: %v", err)
	}
	if string(buf) != string(browserInput) {
		t.Errorf("expected %q, got %q", browserInput, buf)
	}

	// Cleanup: close the PTY writer to trigger EOF → proxy closes.
	ptyWriter.Close() //nolint:errcheck
}

func TestProxyHandlesResize(t *testing.T) {
	hijacked, ptyWriter, _ := hijackedPipe()
	mock := &mockExecAPI{hijacked: hijacked}
	proxy := NewProxy(mock)

	conn, srv := dialProxy(t, proxy, "test-container", "test-worktree")
	defer srv.Close()
	defer conn.CloseNow() //nolint:errcheck

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Send a resize control message (text frame).
	resizeJSON := `{"type":"resize","cols":120,"rows":40}`
	err := conn.Write(ctx, websocket.MessageText, []byte(resizeJSON))
	if err != nil {
		t.Fatalf("ws write resize: %v", err)
	}

	// Give the proxy time to process the resize.
	time.Sleep(100 * time.Millisecond)

	resizes := mock.getResizes()
	if len(resizes) == 0 {
		t.Fatal("expected at least one resize call")
	}
	if resizes[0].Width != 120 || resizes[0].Height != 40 {
		t.Errorf("expected 120x40, got %dx%d", resizes[0].Width, resizes[0].Height)
	}

	ptyWriter.Close() //nolint:errcheck
}

func TestProxyClosesOnBrowserDisconnect(t *testing.T) {
	hijacked, ptyWriter, _ := hijackedPipe()
	mock := &mockExecAPI{hijacked: hijacked}
	proxy := NewProxy(mock)

	conn, srv := dialProxy(t, proxy, "test-container", "test-worktree")
	defer srv.Close()

	// Close the browser side.
	conn.Close(websocket.StatusNormalClosure, "bye") //nolint:errcheck

	// The PTY writer should eventually see a write error or the proxy
	// should stop reading. Give it a moment to propagate.
	time.Sleep(200 * time.Millisecond)
	ptyWriter.Close() //nolint:errcheck
}

func TestProxyClosesOnPTYExit(t *testing.T) {
	hijacked, ptyWriter, _ := hijackedPipe()
	mock := &mockExecAPI{hijacked: hijacked}
	proxy := NewProxy(mock)

	conn, srv := dialProxy(t, proxy, "test-container", "test-worktree")
	defer srv.Close()
	defer conn.CloseNow() //nolint:errcheck

	// Close the PTY side (simulates tmux session ending).
	ptyWriter.Close() //nolint:errcheck

	// The WebSocket should receive a close or error.
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	_, _, err := conn.Read(ctx)
	if err == nil {
		t.Fatal("expected error after PTY closed, got nil")
	}
}

func TestProxyIgnoresMalformedControlMessages(t *testing.T) {
	hijacked, ptyWriter, _ := hijackedPipe()
	mock := &mockExecAPI{hijacked: hijacked}
	proxy := NewProxy(mock)

	conn, srv := dialProxy(t, proxy, "test-container", "test-worktree")
	defer srv.Close()
	defer conn.CloseNow() //nolint:errcheck

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Send invalid JSON — should be silently ignored.
	err := conn.Write(ctx, websocket.MessageText, []byte("not json"))
	if err != nil {
		t.Fatalf("ws write: %v", err)
	}

	// Send valid JSON but wrong type — should be ignored.
	err = conn.Write(ctx, websocket.MessageText, []byte(`{"type":"unknown"}`))
	if err != nil {
		t.Fatalf("ws write: %v", err)
	}

	// Send resize with zero dimensions — should be ignored.
	err = conn.Write(ctx, websocket.MessageText, []byte(`{"type":"resize","cols":0,"rows":0}`))
	if err != nil {
		t.Fatalf("ws write: %v", err)
	}

	time.Sleep(100 * time.Millisecond)

	resizes := mock.getResizes()
	if len(resizes) != 0 {
		t.Errorf("expected no resize calls, got %d", len(resizes))
	}

	ptyWriter.Close() //nolint:errcheck
}

// TestProxyExecRunsAsContainerUser verifies that the tmux attach exec runs as
// the warden user, not root. Without this, Docker exec defaults to root which can't
// access the tmux server socket owned by the warden user.
func TestProxyExecRunsAsContainerUser(t *testing.T) {
	hijacked, ptyWriter, _ := hijackedPipe()
	created := make(chan struct{})
	mock := &mockExecAPI{hijacked: hijacked, created: created}
	proxy := NewProxy(mock)

	conn, srv := dialProxy(t, proxy, "test-container", "test-worktree")
	defer srv.Close()
	defer conn.CloseNow() //nolint:errcheck

	// Wait for ServeWS to call ContainerExecCreate before inspecting the options.
	select {
	case <-created:
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for ContainerExecCreate")
	}

	opts := mock.getLastCreate()
	if opts.User != engine.ContainerUser {
		t.Errorf("expected exec User %q, got %q", engine.ContainerUser, opts.User)
	}

	ptyWriter.Close() //nolint:errcheck
}

// TestProxyStripsAltScreenSequences verifies that the proxy's PTY→WS path
// strips alternate screen escape sequences so xterm.js scrollback works.
func TestProxyStripsAltScreenSequences(t *testing.T) {
	hijacked, ptyWriter, _ := hijackedPipe()
	mock := &mockExecAPI{hijacked: hijacked}
	proxy := NewProxy(mock)

	conn, srv := dialProxy(t, proxy, "test-container", "test-worktree")
	defer srv.Close()
	defer conn.CloseNow() //nolint:errcheck

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Write PTY output with alt-screen enter, content, and alt-screen exit.
	go func() {
		if _, err := ptyWriter.Write([]byte("\x1b[?1049h")); err != nil {
			return
		}
		if _, err := ptyWriter.Write([]byte("visible content")); err != nil {
			return
		}
		if _, err := ptyWriter.Write([]byte("\x1b[?1049l")); err != nil {
			return
		}
		ptyWriter.Close() //nolint:errcheck
	}()

	// Read all WebSocket messages until the connection closes.
	var received []byte
	for {
		_, data, err := conn.Read(ctx)
		if err != nil {
			break
		}
		received = append(received, data...)
	}

	got := string(received)
	if got != "visible content" {
		t.Errorf("expected %q, got %q", "visible content", got)
	}
}

// shellMockExecAPI extends mockExecAPI so we can observe the bootstrap exec.
// In ModeShell the proxy issues three exec creates in order:
//  1. create-shell.sh bootstrap (drain stdout/stderr, no TTY)
//  2. tmux capture-pane scrollback (TTY)
//  3. tmux attach-session (TTY, bridged to the WebSocket)
type shellMockExecAPI struct {
	mu          sync.Mutex
	hijacked    dtypes.HijackedResponse
	createCmds  [][]string
	createCount int
	attachCount int
	attached    chan struct{}
	once        sync.Once
}

func (m *shellMockExecAPI) ContainerExecCreate(_ context.Context, _ string, opts container.ExecOptions) (container.ExecCreateResponse, error) {
	m.mu.Lock()
	m.createCount++
	m.createCmds = append(m.createCmds, append([]string(nil), opts.Cmd...))
	m.mu.Unlock()
	return container.ExecCreateResponse{ID: "exec"}, nil
}

func (m *shellMockExecAPI) ContainerExecAttach(_ context.Context, _ string, _ container.ExecStartOptions) (dtypes.HijackedResponse, error) {
	m.mu.Lock()
	m.attachCount++
	count := m.attachCount
	m.mu.Unlock()

	// #1: bootstrap — empty pipe, drained by io.Copy.
	// #2: scrollback capture — empty pipe that closes immediately.
	if count == 1 || count == 2 {
		r, w := io.Pipe()
		_ = w.Close()
		return dtypes.HijackedResponse{
			Conn:   &pipeConn{Reader: r, Writer: io.Discard},
			Reader: bufio.NewReader(r),
		}, nil
	}

	// #3: real tmux attach — signal the test that ServeWSMode is wired up.
	m.once.Do(func() {
		if m.attached != nil {
			close(m.attached)
		}
	})
	return m.hijacked, nil
}

func (m *shellMockExecAPI) ContainerExecResize(_ context.Context, _ string, _ container.ResizeOptions) error {
	return nil
}

func (m *shellMockExecAPI) getCreateCmds() [][]string {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([][]string, len(m.createCmds))
	for i, c := range m.createCmds {
		out[i] = append([]string(nil), c...)
	}
	return out
}

// TestProxyShellModeBootstrapsThenAttaches verifies that ServeShellWS runs
// create-shell.sh before the scrollback capture + tmux attach, and that the
// attach exec targets the warden-shell-<wid> session name (not the agent one).
func TestProxyShellModeBootstrapsThenAttaches(t *testing.T) {
	hijacked, ptyWriter, _ := hijackedPipe()
	attached := make(chan struct{})
	mock := &shellMockExecAPI{hijacked: hijacked, attached: attached}
	proxy := NewProxy(mock)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		proxy.ServeShellWS(w, r, "test-container", "feat-foo")
	}))
	defer srv.Close()

	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	conn, _, err := websocket.Dial(ctx, wsURL, nil)
	if err != nil {
		t.Fatalf("dial websocket: %v", err)
	}
	defer conn.CloseNow() //nolint:errcheck

	select {
	case <-attached:
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for tmux attach exec")
	}

	cmds := mock.getCreateCmds()
	if len(cmds) < 3 {
		t.Fatalf("expected at least 3 exec creates (bootstrap, scrollback, attach), got %d: %v", len(cmds), cmds)
	}

	// #1 must be the bootstrap.
	if len(cmds[0]) < 2 || cmds[0][0] != constants.CreateShellScript || cmds[0][1] != "feat-foo" {
		t.Errorf("first exec should be %q feat-foo, got %v", constants.CreateShellScript, cmds[0])
	}

	// #3 must be tmux attach-session against the shell session name.
	attach := cmds[2]
	if len(attach) < 5 || attach[0] != "tmux" || attach[2] != "attach-session" {
		t.Errorf("third exec should be tmux attach-session, got %v", attach)
	}
	if want := "warden-shell-feat-foo"; attach[4] != want {
		t.Errorf("attach target %q, want %q", attach[4], want)
	}

	ptyWriter.Close() //nolint:errcheck
}

func TestProxyExecCreateError(t *testing.T) {
	mock := &mockExecAPI{
		createFn: func(_ context.Context, _ string, _ container.ExecOptions) (container.ExecCreateResponse, error) {
			return container.ExecCreateResponse{}, io.ErrUnexpectedEOF
		},
	}
	proxy := NewProxy(mock)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		proxy.ServeWS(w, r, "bad-container", "bad-worktree")
	}))
	defer srv.Close()

	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http")
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	conn, _, err := websocket.Dial(ctx, wsURL, nil)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.CloseNow() //nolint:errcheck

	// The server should close with an internal error.
	_, _, err = conn.Read(ctx)
	if err == nil {
		t.Fatal("expected close error")
	}
	if websocket.CloseStatus(err) != websocket.StatusInternalError {
		t.Errorf("expected StatusInternalError, got %v", websocket.CloseStatus(err))
	}
}
