package engine

import (
	"bufio"
	"bytes"
	"context"
	"encoding/binary"
	"net"
	"sync"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
)

// execCall records a single exec command sent to the mock API.
type execCall struct {
	ContainerID string
	Cmd         []string
	User        string
}

// execMockAPI is a minimal mock of the Docker API that captures exec calls
// and returns canned stdout responses. Only the exec-related methods are
// implemented; all others will panic via the embedded nil APIClient.
//
// Thread-safe: multiple goroutines can call exec concurrently.
type execMockAPI struct {
	client.APIClient

	mu     sync.Mutex
	calls  []execCall
	nextID int
	// responses maps an exec ID to its canned stdout response.
	responses map[string]string
	// cmdResponses maps a command fingerprint to its canned response.
	// The fingerprint is the first element of the Cmd slice.
	cmdResponses map[string]string
}

func newExecMockAPI() *execMockAPI {
	return &execMockAPI{
		responses:    make(map[string]string),
		cmdResponses: make(map[string]string),
	}
}

// onCmd registers a canned response for any exec whose first command
// element matches the given prefix.
func (m *execMockAPI) onCmd(cmdPrefix string, response string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.cmdResponses[cmdPrefix] = response
}

// getCalls returns all recorded exec calls.
func (m *execMockAPI) getCalls() []execCall {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]execCall(nil), m.calls...)
}

func (m *execMockAPI) ContainerExecCreate(_ context.Context, containerID string, cfg container.ExecOptions) (container.ExecCreateResponse, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.nextID++
	execID := "exec-" + containerID + "-" + string(rune('0'+m.nextID))

	m.calls = append(m.calls, execCall{
		ContainerID: containerID,
		Cmd:         cfg.Cmd,
		User:        cfg.User,
	})

	// Find a matching canned response by command prefix.
	for prefix, resp := range m.cmdResponses {
		if len(cfg.Cmd) > 0 && cfg.Cmd[0] == prefix {
			m.responses[execID] = resp
			break
		}
		// Also match sh -c commands by checking the shell command content.
		if len(cfg.Cmd) >= 3 && cfg.Cmd[0] == "sh" && cfg.Cmd[1] == "-c" {
			if bytes.Contains([]byte(cfg.Cmd[2]), []byte(prefix)) {
				m.responses[execID] = resp
				break
			}
		}
	}

	return container.ExecCreateResponse{ID: execID}, nil
}

func (m *execMockAPI) ContainerExecAttach(_ context.Context, execID string, _ container.ExecAttachOptions) (types.HijackedResponse, error) {
	m.mu.Lock()
	stdout := m.responses[execID]
	m.mu.Unlock()

	// Docker multiplexes exec output with an 8-byte header per frame.
	// stdcopy.StdCopy expects: [stream_type(1)][0(3)][size(4)][payload].
	var buf bytes.Buffer
	if len(stdout) > 0 {
		header := make([]byte, 8)
		header[0] = 1 // stdout stream
		binary.BigEndian.PutUint32(header[4:], uint32(len(stdout)))
		buf.Write(header)
		buf.WriteString(stdout)
	}

	// nopConn satisfies the net.Conn interface for HijackedResponse.Close().
	serverConn, clientConn := net.Pipe()
	serverConn.Close() //nolint:errcheck

	return types.HijackedResponse{
		Reader: bufio.NewReader(&buf),
		Conn:   clientConn,
	}, nil
}

func (m *execMockAPI) ContainerExecInspect(_ context.Context, _ string) (container.ExecInspect, error) {
	return container.ExecInspect{ExitCode: 0}, nil
}

func (m *execMockAPI) ContainerInspect(_ context.Context, _ string) (container.InspectResponse, error) {
	return container.InspectResponse{
		ContainerJSONBase: &container.ContainerJSONBase{
			Name: "/test-container",
		},
		Config: &container.Config{
			Labels: map[string]string{},
		},
	}, nil
}
