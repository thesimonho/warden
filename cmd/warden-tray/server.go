package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"
)

const (
	readyTimeout   = 30 * time.Second
	readyInterval  = 500 * time.Millisecond
	requestTimeout = 5 * time.Second
	// stopTimeout must exceed the Docker stop timeout (30s) so the
	// HTTP request doesn't time out before the container stops.
	stopTimeout  = 35 * time.Second
	healthPath   = "/api/v1/health"
	projectsPath = "/api/v1/projects"
	settingsPath = "/api/v1/settings"
	eventsPath   = "/api/v1/events"
	shutdownPath = "/api/v1/shutdown"
	stateRunning = "running"
	stateExited  = "exited"

	// settingsRefreshInterval controls how often the tray re-reads
	// the notificationsEnabled setting from the server.
	settingsRefreshInterval = 60 * time.Second
)

// serverClient is a minimal HTTP client for the Warden API.
// It only uses the endpoints the tray needs, avoiding any
// dependency on the main warden module.
type serverClient struct {
	baseURL string
	http    *http.Client
}

// project is the minimal subset of engine.Project the tray needs.
// ProjectID is the stable identifier used in API paths; ID is the
// Docker container ID (not used by the tray).
type project struct {
	ProjectID        string `json:"projectId"`
	Name             string `json:"name"`
	AgentType        string `json:"agentType"`
	State            string `json:"state"`
	NeedsInput       bool   `json:"needsInput"`
	NotificationType string `json:"notificationType"`
}

// settingsResponse is the minimal subset of server settings the tray needs.
type settingsResponse struct {
	NotificationsEnabled bool `json:"notificationsEnabled"`
}

func newServerClient(baseURL string) *serverClient {
	return &serverClient{
		baseURL: baseURL,
		http:    &http.Client{Timeout: requestTimeout},
	}
}

// waitForReady polls the health endpoint until the server responds
// or the timeout elapses.
func (s *serverClient) waitForReady() bool {
	deadline := time.Now().Add(readyTimeout)
	for time.Now().Before(deadline) {
		if s.isHealthy() {
			return true
		}
		time.Sleep(readyInterval)
	}
	return false
}

// isHealthy returns true if the server health endpoint responds 200
// with the X-Warden header.
func (s *serverClient) isHealthy() bool {
	resp, err := s.http.Get(s.baseURL + healthPath) //nolint:noctx
	if err != nil {
		return false
	}
	resp.Body.Close() //nolint:errcheck
	return resp.StatusCode == http.StatusOK && resp.Header.Get("X-Warden") == "1"
}

// healthResponse is the minimal subset of the health endpoint response.
type healthResponse struct {
	Version string `json:"version"`
}

// fetchVersion returns the server version from the health endpoint.
func (s *serverClient) fetchVersion() string {
	resp, err := s.http.Get(s.baseURL + healthPath) //nolint:noctx
	if err != nil {
		return ""
	}
	defer resp.Body.Close() //nolint:errcheck

	var h healthResponse
	if err := json.NewDecoder(resp.Body).Decode(&h); err != nil {
		return ""
	}
	return h.Version
}

// listProjects returns all projects from the server.
func (s *serverClient) listProjects() ([]project, error) {
	resp, err := s.http.Get(s.baseURL + projectsPath) //nolint:noctx
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close() //nolint:errcheck

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status: %d", resp.StatusCode)
	}

	var projects []project
	if err := json.NewDecoder(resp.Body).Decode(&projects); err != nil {
		return nil, err
	}
	return projects, nil
}

// fetchNotificationsEnabled reads the server-side notification setting.
func (s *serverClient) fetchNotificationsEnabled() (bool, error) {
	resp, err := s.http.Get(s.baseURL + settingsPath) //nolint:noctx
	if err != nil {
		return false, err
	}
	defer resp.Body.Close() //nolint:errcheck

	var settings settingsResponse
	if err := json.NewDecoder(resp.Body).Decode(&settings); err != nil {
		return false, err
	}
	return settings.NotificationsEnabled, nil
}

// stopProject stops a running project's container. Uses a longer
// timeout than other requests because Docker stop waits up to 30s
// for the container to exit gracefully.
func (s *serverClient) stopProject(projectID, agentType string) error {
	url := fmt.Sprintf("%s/api/v1/projects/%s/%s/stop", s.baseURL, projectID, agentType)
	client := &http.Client{Timeout: stopTimeout}
	resp, err := client.Post(url, "", nil) //nolint:noctx
	if err != nil {
		return err
	}
	resp.Body.Close() //nolint:errcheck
	if resp.StatusCode >= 400 {
		return fmt.Errorf("stop failed: %d", resp.StatusCode)
	}
	return nil
}

// shutdown requests a graceful server shutdown.
func (s *serverClient) shutdown() {
	resp, err := s.http.Post(s.baseURL+shutdownPath, "", nil) //nolint:noctx
	if err != nil {
		log.Printf("shutdown request failed: %v", err)
		return
	}
	resp.Body.Close() //nolint:errcheck
}

// --- SSE Client ---

// sseEvent is a parsed Server-Sent Event.
type sseEvent struct {
	Event string
	Data  json.RawMessage
}

// projectStateData mirrors the SSE project_state payload.
type projectStateData struct {
	ProjectID        string `json:"projectId"`
	AgentType        string `json:"agentType"`
	ContainerName    string `json:"containerName"`
	NeedsInput       bool   `json:"needsInput"`
	NotificationType string `json:"notificationType,omitempty"`
}

// containerStateData mirrors the SSE container_state_changed payload.
type containerStateData struct {
	ProjectID     string `json:"projectId"`
	AgentType     string `json:"agentType"`
	ContainerName string `json:"containerName"`
	Action        string `json:"action"`
}

// sseCallback receives parsed SSE events from the stream.
type sseCallback func(evt sseEvent)

// connectSSE opens an SSE connection to /api/v1/events and calls the
// callback for each event. Blocks until the connection drops or the
// done channel is closed. Returns an error on connection failure.
func (s *serverClient) connectSSE(done <-chan struct{}, callback sseCallback) error {
	req, err := http.NewRequest("GET", s.baseURL+eventsPath, nil) //nolint:noctx
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "text/event-stream")

	// Use a separate client with no timeout — SSE connections are long-lived.
	sseClient := &http.Client{Timeout: 0}
	resp, err := sseClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close() //nolint:errcheck

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("SSE connect failed: %d", resp.StatusCode)
	}

	// Parse SSE stream in a goroutine so we can select on done.
	events := make(chan sseEvent, 16)
	errCh := make(chan error, 1)
	go func() {
		defer close(events)
		scanner := bufio.NewScanner(resp.Body)
		var currentEvent string
		var dataLines []string

		for scanner.Scan() {
			line := scanner.Text()

			if line == "" {
				// Empty line = end of event.
				if currentEvent != "" && len(dataLines) > 0 {
					data := strings.Join(dataLines, "\n")
					events <- sseEvent{
						Event: currentEvent,
						Data:  json.RawMessage(data),
					}
				}
				currentEvent = ""
				dataLines = nil
				continue
			}

			if strings.HasPrefix(line, "event: ") {
				currentEvent = strings.TrimPrefix(line, "event: ")
			} else if strings.HasPrefix(line, "data: ") {
				dataLines = append(dataLines, strings.TrimPrefix(line, "data: "))
			}
		}
		errCh <- scanner.Err()
	}()

	for {
		select {
		case <-done:
			return nil
		case evt, ok := <-events:
			if !ok {
				// Stream ended — check for scanner error.
				select {
				case err := <-errCh:
					return err
				default:
					return fmt.Errorf("SSE stream closed")
				}
			}
			callback(evt)
		}
	}
}
