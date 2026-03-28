package tui

import (
	"bytes"
	"io"
	"os"
	"os/signal"
	"sync"

	"github.com/thesimonho/warden/client"
	"golang.org/x/term"
)

// TerminalExecCmd bridges stdin/stdout to a [client.TerminalConnection].
// It satisfies the interface required by tea.Exec, which temporarily
// yields the terminal from Bubble Tea to this command. When Run()
// returns (e.g. the user presses the disconnect key), Bubble Tea resumes
// and re-renders the TUI.
//
// The SetStdin/SetStdout/SetStderr methods are called by tea.Exec
// before Run() — they provide the raw terminal file descriptors.
type TerminalExecCmd struct {
	conn          client.TerminalConnection
	disconnectKey byte // control character that triggers detach
	stdin         io.Reader
	stdout        io.Writer
	stderr        io.Writer
}

// SetStdin sets the stdin reader for the exec command.
func (t *TerminalExecCmd) SetStdin(r io.Reader) { t.stdin = r }

// SetStdout sets the stdout writer for the exec command.
func (t *TerminalExecCmd) SetStdout(w io.Writer) { t.stdout = w }

// SetStderr sets the stderr writer for the exec command.
func (t *TerminalExecCmd) SetStderr(w io.Writer) { t.stderr = w }

// Run bridges stdin/stdout to the terminal connection until the
// connection closes or the user presses the detach key.
func (t *TerminalExecCmd) Run() error {
	// Put the terminal into raw mode so escape sequences (arrow keys,
	// ctrl combos) are passed through to the remote PTY instead of being
	// processed locally. tea.Exec restores cooked mode before calling Run.
	if f, ok := t.stdin.(*os.File); ok {
		oldState, err := term.MakeRaw(int(f.Fd()))
		if err == nil {
			defer func() { _ = term.Restore(int(f.Fd()), oldState) }()
		}
	}

	// Send initial terminal size.
	t.sendCurrentSize()

	// Handle terminal resize signals (SIGWINCH on Unix, no-op on Windows).
	sigCh := make(chan os.Signal, 1)
	notifyResize(sigCh)
	defer signal.Stop(sigCh)

	done := make(chan struct{})
	var ioWg sync.WaitGroup

	// Goroutine: conn → stdout (PTY output to screen).
	ioWg.Add(1)
	go func() {
		defer ioWg.Done()
		buf := make([]byte, 32*1024)
		for {
			n, err := t.conn.Read(buf)
			if n > 0 {
				_, _ = t.stdout.Write(buf[:n])
			}
			if err != nil {
				return
			}
		}
	}()

	// Goroutine: stdin → conn (keyboard input to PTY).
	// Intercepts the detach key to trigger a clean disconnect.
	ioWg.Add(1)
	go func() {
		defer ioWg.Done()
		buf := make([]byte, 32*1024)
		for {
			n, err := t.stdin.Read(buf)
			if n > 0 {
				// Check for the detach key in the input.
				if t.disconnectKey != 0 && bytes.ContainsRune(buf[:n], rune(t.disconnectKey)) {
					// Close the connection to trigger shutdown.
					_ = t.conn.Close()
					return
				}
				if _, writeErr := t.conn.Write(buf[:n]); writeErr != nil {
					return
				}
			}
			if err != nil {
				return
			}
		}
	}()

	// Goroutine: SIGWINCH → resize.
	var sigWg sync.WaitGroup
	sigWg.Add(1)
	go func() {
		defer sigWg.Done()
		for {
			select {
			case <-sigCh:
				t.sendCurrentSize()
			case <-done:
				return
			}
		}
	}()

	// Wait for I/O goroutines to exit (connection closed / detach key),
	// then signal the SIGWINCH goroutine to stop.
	ioWg.Wait()
	close(done)
	sigWg.Wait()

	return nil
}

// sendCurrentSize reads the terminal dimensions and sends a resize.
func (t *TerminalExecCmd) sendCurrentSize() {
	cols, rows, err := term.GetSize(int(os.Stdout.Fd()))
	if err != nil {
		return
	}
	_ = t.conn.Resize(uint(cols), uint(rows))
}
