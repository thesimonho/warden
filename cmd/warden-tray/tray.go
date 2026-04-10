package main

import (
	"encoding/json"
	"fmt"
	"log"
	"runtime"
	"sync"
	"sync/atomic"
	"time"

	"github.com/godbus/dbus/v5"
	"fyne.io/systray"
)

const (
	// sseReconnectMin is the initial delay before reconnecting after
	// an SSE connection drop.
	sseReconnectMin = 1 * time.Second
	// sseReconnectMax caps the exponential backoff for SSE reconnection.
	sseReconnectMax = 30 * time.Second

	// SSE event types (mirrors event.SSEEventType constants).
	sseEventProjectState          = "project_state"
	sseEventContainerStateChanged = "container_state_changed"
	sseEventServerShutdown        = "server_shutdown"

	// Container lifecycle actions (mirrors event.ContainerStateAction).
	containerActionCreated = "created"
	containerActionStarted = "started"
	containerActionStopped = "stopped"
	containerActionDeleted = "deleted"
)

// projectKey returns a dedup key for a project.
func projectKey(projectID, agentType string) string {
	return projectID + ":" + agentType
}

// trayState holds mutable state shared between the SSE listener,
// menu click handlers, and notification logic.
type trayState struct {
	srv          *serverClient
	notif        *notifier
	shuttingDown atomic.Bool
	menuStatus   *systray.MenuItem
	sseStop      chan struct{} // closed to stop the SSE loop

	mu sync.Mutex
	// needsAttention tracks whether any project needs user input.
	needsAttention bool
	// notificationsEnabled caches the server-side setting.
	notificationsEnabled bool
	// projects tracks the current state of all projects from the
	// initial fetch. Updated by SSE events.
	projects map[string]*projectState
}

// projectState tracks per-project data from SSE events.
type projectState struct {
	projectID        string
	agentType        string
	name             string // display name (container name)
	state            string // Docker state
	needsInput       bool
	notificationType string
}

// runTray initializes the system tray and blocks until exit.
// This must be called from the main goroutine on macOS.
func runTray(srv *serverClient) {
	state := &trayState{
		srv:      srv,
		notif:    newNotifier(srv.baseURL),
		sseStop:  make(chan struct{}),
		projects: make(map[string]*projectState),
	}

	systray.Run(func() { state.onReady() }, func() {})
}

// quitTray signals the tray to exit.
func quitTray() {
	systray.Quit()
}

// onReady is called by systray once the tray is initialized.
func (t *trayState) onReady() {
	applyIcon(false)
	if runtime.GOOS == "linux" {
		go watchColorScheme()
	}
	systray.SetTooltip("Warden")

	systray.SetOnTapped(func() { t.openDashboard() })

	versionLabel := "Warden"
	if v := t.srv.fetchVersion(); v != "" {
		versionLabel = "Warden " + v
	}
	mVersion := systray.AddMenuItem(versionLabel, "")
	mVersion.Disable()

	systray.AddSeparator()

	mOpen := systray.AddMenuItem("Open Dashboard", "Open the Warden dashboard in a browser")

	t.menuStatus = systray.AddMenuItem("Checking...", "")
	t.menuStatus.Disable()

	systray.AddSeparator()

	mQuit := systray.AddMenuItem("Quit Warden", "Shut down the Warden server")

	// Route menu item clicks via channels.
	go func() {
		for {
			select {
			case <-mOpen.ClickedCh:
				t.openDashboard()
			case <-mQuit.ClickedCh:
				t.handleQuit()
			}
		}
	}()

	// Load initial state, then start SSE.
	t.loadInitialState()
	go t.sseLoop()
	go t.settingsRefreshLoop()
}

// loadInitialState fetches the full project list and notification
// setting to seed the tray before SSE events start flowing.
func (t *trayState) loadInitialState() {
	// Fetch notification setting (default to enabled on error).
	enabled, err := t.srv.fetchNotificationsEnabled()
	if err != nil {
		enabled = true
	}

	// Fetch projects to seed the menu.
	projects, _ := t.srv.listProjects()

	t.mu.Lock()
	t.notificationsEnabled = enabled
	// Clear existing state to remove projects deleted while disconnected.
	clear(t.projects)
	for _, p := range projects {
		key := projectKey(p.ProjectID, p.AgentType)
		t.projects[key] = &projectState{
			projectID:        p.ProjectID,
			agentType:        p.AgentType,
			name:             p.Name,
			state:            p.State,
			needsInput:       p.NeedsInput,
			notificationType: p.NotificationType,
		}
	}
	t.mu.Unlock()
	t.refreshStatusFromState()
}

// sseLoop maintains a persistent SSE connection with automatic
// reconnection using exponential backoff.
func (t *trayState) sseLoop() {
	backoff := sseReconnectMin

	for {
		select {
		case <-t.sseStop:
			return
		default:
		}

		err := t.srv.connectSSE(t.sseStop, t.handleSSEEvent)
		if t.shuttingDown.Load() {
			systray.Quit()
			return
		}

		select {
		case <-t.sseStop:
			return
		default:
		}

		if err != nil {
			// Check if server is still alive.
			if !t.srv.isHealthy() {
				log.Println("warden server is no longer reachable, exiting tray")
				systray.Quit()
				return
			}
			log.Printf("SSE connection lost: %v, reconnecting in %v", err, backoff)
		}

		// Wait before reconnecting.
		timer := time.NewTimer(backoff)
		select {
		case <-t.sseStop:
			timer.Stop()
			return
		case <-timer.C:
		}

		// Exponential backoff.
		backoff *= 2
		if backoff > sseReconnectMax {
			backoff = sseReconnectMax
		}

		// Refresh full state on reconnect to catch events missed
		// during the disconnect window.
		t.loadInitialState()
		// Reset backoff on successful reconnect (will be set in next iteration).
		backoff = sseReconnectMin
	}
}

// settingsRefreshLoop periodically re-reads the notification setting
// from the server so toggling it in the UI takes effect without restart.
func (t *trayState) settingsRefreshLoop() {
	ticker := time.NewTicker(settingsRefreshInterval)
	defer ticker.Stop()

	for {
		select {
		case <-t.sseStop:
			return
		case <-ticker.C:
			if enabled, err := t.srv.fetchNotificationsEnabled(); err == nil {
				t.mu.Lock()
				t.notificationsEnabled = enabled
				t.mu.Unlock()
			}
		}
	}
}

// handleSSEEvent processes a single SSE event from the server.
func (t *trayState) handleSSEEvent(evt sseEvent) {
	switch evt.Event {
	case sseEventProjectState:
		t.handleProjectState(evt)
	case sseEventContainerStateChanged:
		t.handleContainerStateChanged(evt)
	case sseEventServerShutdown:
		t.shuttingDown.Store(true)
	}
}

// handleProjectState processes a project_state SSE event, updating
// attention tracking and sending notifications.
func (t *trayState) handleProjectState(evt sseEvent) {
	var data projectStateData
	if err := unmarshalJSON(evt.Data, &data); err != nil {
		return
	}

	key := projectKey(data.ProjectID, data.AgentType)

	t.mu.Lock()
	ps, exists := t.projects[key]
	if !exists {
		ps = &projectState{
			projectID: data.ProjectID,
			agentType: data.AgentType,
			name:      data.ContainerName,
			state:     stateRunning,
		}
		t.projects[key] = ps
	}

	wasNeedingInput := ps.needsInput
	ps.needsInput = data.NeedsInput
	ps.notificationType = data.NotificationType
	notifEnabled := t.notificationsEnabled
	projectName := ps.name
	t.mu.Unlock()

	// Handle notification logic.
	if data.NeedsInput && !wasNeedingInput && notifEnabled {
		t.notif.notify(key, projectName, data.ProjectID, data.AgentType, notificationType(data.NotificationType))
	} else if !data.NeedsInput && wasNeedingInput {
		t.notif.clearAttention(key)
	}

	t.refreshStatusFromState()
}

// handleContainerStateChanged processes a container_state_changed SSE
// event, updating the project list and menu.
func (t *trayState) handleContainerStateChanged(evt sseEvent) {
	var data containerStateData
	if err := unmarshalJSON(evt.Data, &data); err != nil {
		return
	}

	key := projectKey(data.ProjectID, data.AgentType)

	t.mu.Lock()
	switch data.Action {
	case containerActionCreated, containerActionStarted:
		ps, exists := t.projects[key]
		if !exists {
			ps = &projectState{
				projectID: data.ProjectID,
				agentType: data.AgentType,
				name:      data.ContainerName,
			}
			t.projects[key] = ps
		}
		ps.state = stateRunning
	case containerActionStopped:
		if ps, exists := t.projects[key]; exists {
			ps.state = stateExited
			ps.needsInput = false
			ps.notificationType = ""
		}
		t.notif.clearAttention(key)
	case containerActionDeleted:
		delete(t.projects, key)
		t.notif.clearAttention(key)
	}
	t.mu.Unlock()

	t.refreshStatusFromState()
}

// refreshStatusFromState updates the menu status label and tray icon
// based on current in-memory project state.
func (t *trayState) refreshStatusFromState() {
	t.mu.Lock()
	var running int
	var attention bool
	for _, ps := range t.projects {
		if ps.state == stateRunning {
			running++
		}
		if ps.needsInput {
			attention = true
		}
	}
	changed := t.needsAttention != attention
	t.needsAttention = attention
	t.mu.Unlock()

	globalAttention.Store(attention)
	if changed {
		applyIcon(attention)
	}

	switch running {
	case 0:
		t.menuStatus.SetTitle("No containers running")
	case 1:
		t.menuStatus.SetTitle("1 container running")
	default:
		t.menuStatus.SetTitle(fmt.Sprintf("%d containers running", running))
	}
}

// unmarshalJSON is a helper that unmarshals JSON data into v.
func unmarshalJSON(data []byte, v any) error {
	if err := json.Unmarshal(data, v); err != nil {
		log.Printf("failed to parse SSE data: %v", err)
		return err
	}
	return nil
}

// applyIcon sets the tray icon based on the current platform, color scheme,
// and attention state. On macOS, uses SetTemplateIcon (OS handles inversion)
// with a separate attention variant. On Linux, detects dark/light via the
// freedesktop portal. On Windows, defaults to the dark icon for now.
func applyIcon(attention bool) {
	switch runtime.GOOS {
	case "darwin":
		if attention {
			systray.SetIcon(iconDataAttention)
		} else {
			systray.SetTemplateIcon(iconData, iconData)
		}
	case "linux":
		dark := isDarkTheme()
		switch {
		case dark && attention:
			systray.SetIcon(iconDataAttentionLight)
		case dark:
			systray.SetIcon(iconDataLight)
		case attention:
			systray.SetIcon(iconDataAttention)
		default:
			systray.SetIcon(iconData)
		}
	default:
		if attention {
			systray.SetIcon(iconDataAttention)
		} else {
			systray.SetIcon(iconData)
		}
	}
}

// isDarkTheme queries the freedesktop desktop portal for the system color
// scheme. Returns true if the user prefers a dark theme. Works on KDE,
// GNOME, and other modern DEs that implement the portal. Defaults to true
// (dark panels are more common) if the portal is unavailable.
func isDarkTheme() bool {
	conn, err := dbus.SessionBus()
	if err != nil {
		return true
	}
	return readColorScheme(conn) == 1
}

// readColorScheme reads the color-scheme setting from the freedesktop portal.
// Returns 0 (no preference), 1 (prefer dark), or 2 (prefer light).
func readColorScheme(conn *dbus.Conn) uint32 {
	obj := conn.Object(
		"org.freedesktop.portal.Desktop",
		"/org/freedesktop/portal/desktop",
	)

	call := obj.Call(
		"org.freedesktop.portal.Settings.Read", 0,
		"org.freedesktop.appearance", "color-scheme",
	)
	if call.Err != nil {
		return 1 // default dark
	}

	var outer dbus.Variant
	if err := call.Store(&outer); err != nil {
		return 1
	}
	inner, ok := outer.Value().(dbus.Variant)
	if !ok {
		return 1
	}
	scheme, ok := inner.Value().(uint32)
	if !ok {
		return 1
	}
	return scheme
}

// watchColorScheme listens for freedesktop portal SettingChanged signals
// and swaps the tray icon when the color scheme changes.
func watchColorScheme() {
	conn, err := dbus.SessionBus()
	if err != nil {
		return
	}

	if err := conn.AddMatchSignal(
		dbus.WithMatchObjectPath("/org/freedesktop/portal/desktop"),
		dbus.WithMatchInterface("org.freedesktop.portal.Settings"),
		dbus.WithMatchMember("SettingChanged"),
	); err != nil {
		log.Printf("failed to watch color scheme changes: %v", err)
		return
	}

	ch := make(chan *dbus.Signal, 10)
	conn.Signal(ch)

	for sig := range ch {
		if len(sig.Body) < 3 {
			continue
		}
		namespace, _ := sig.Body[0].(string)
		key, _ := sig.Body[1].(string)
		if namespace == "org.freedesktop.appearance" && key == "color-scheme" {
			// Re-read attention state to apply correct icon variant.
			applyIcon(globalAttention.Load())
		}
	}
}

// globalAttention tracks the current attention state for the color scheme
// watcher, which runs in a separate goroutine.
var globalAttention atomic.Bool

// openDashboard opens the server URL in the default browser.
func (t *trayState) openDashboard() {
	openBrowser(t.srv.baseURL)
}

// handleQuit runs the quit flow: check containers, ask user,
// stop containers if requested, then shut down the server.
func (t *trayState) handleQuit() {
	projects, err := t.srv.listProjects()
	if err != nil {
		// Can't reach server — just quit.
		t.shuttingDown.Store(true)
		close(t.sseStop)
		systray.Quit()
		return
	}

	var running []project
	for _, p := range projects {
		if p.State == stateRunning {
			running = append(running, p)
		}
	}

	if len(running) > 0 {
		action := showQuitDialog(len(running))
		switch action {
		case quitActionCancel:
			return
		case quitActionStopAndQuit:
			t.stopAllContainers(running)
		case quitActionQuit:
			// Leave containers running.
		}
	}

	t.shuttingDown.Store(true)
	close(t.sseStop)
	t.srv.shutdown()
	// The SSE loop will detect shutdown and call systray.Quit().
}

// stopAllContainers stops containers concurrently to avoid serial
// round-trips (each Docker stop can take up to 10s).
func (t *trayState) stopAllContainers(running []project) {
	var wg sync.WaitGroup
	for _, p := range running {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := t.srv.stopProject(p.ProjectID, p.AgentType); err != nil {
				log.Printf("failed to stop %s: %v", p.Name, err)
			}
		}()
	}
	wg.Wait()
}
