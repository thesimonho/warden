package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
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
	shutdownPath = "/api/v1/shutdown"
	stateRunning = "running"
)

// serverClient is a minimal HTTP client for the Warden API.
// It only uses the endpoints the tray needs, avoiding any
// dependency on the main warden module.
type serverClient struct {
	baseURL string
	http    *http.Client
}

// project is the minimal subset of engine.Project the tray needs.
type project struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	AgentType string `json:"agentType"`
	State     string `json:"state"`
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

// runningContainerCount returns how many projects have a running container.
func (s *serverClient) runningContainerCount() int {
	projects, err := s.listProjects()
	if err != nil {
		return 0
	}
	var count int
	for _, p := range projects {
		if p.State == stateRunning {
			count++
		}
	}
	return count
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
