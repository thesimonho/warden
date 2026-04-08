package main

import (
	"fmt"
	"log"
	"os/exec"
	"runtime"
	"sync"
	"sync/atomic"
	"time"

	"github.com/godbus/dbus/v5"
	"fyne.io/systray"
)

const pollInterval = 5 * time.Second

// trayState holds mutable state shared between the poll loop and
// menu click handlers.
type trayState struct {
	srv          *serverClient
	shuttingDown atomic.Bool
	menuStatus   *systray.MenuItem
	mu           sync.Mutex

	// needsAttention tracks whether any project needs user input.
	// Protected by mu.
	needsAttention bool
}

// runTray initializes the system tray and blocks until exit.
// This must be called from the main goroutine on macOS.
func runTray(srv *serverClient) {
	state := &trayState{srv: srv}

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

	// Update container count immediately, then poll.
	t.updateStatus()
	go t.pollLoop()
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

// pollLoop periodically checks server health (via the projects endpoint)
// and updates the container count. A single HTTP call per tick serves
// both purposes — if it fails, the server is unhealthy.
func (t *trayState) pollLoop() {
	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()

	for range ticker.C {
		projects, err := t.srv.listProjects()
		if err != nil {
			if t.shuttingDown.Load() {
				systray.Quit()
				return
			}
			log.Println("warden server is no longer reachable, exiting tray")
			systray.Quit()
			return
		}
		t.updateStatusFromProjects(projects)
	}
}

// updateStatus fetches projects and refreshes the container count.
func (t *trayState) updateStatus() {
	projects, _ := t.srv.listProjects()
	t.updateStatusFromProjects(projects)
}

// updateStatusFromProjects refreshes the container count menu item and
// swaps the tray icon when any project needs user attention.
func (t *trayState) updateStatusFromProjects(projects []project) {
	var running int
	var attention bool
	for _, p := range projects {
		if p.State == stateRunning {
			running++
		}
		if p.NeedsInput && p.NotificationType == "permission_prompt" {
			attention = true
		}
	}

	t.mu.Lock()
	changed := t.needsAttention != attention
	t.needsAttention = attention
	t.mu.Unlock()

	globalAttention.Store(attention)

	if changed {
		applyIcon(attention)
	}

	t.mu.Lock()
	defer t.mu.Unlock()

	switch running {
	case 0:
		t.menuStatus.SetTitle("No containers running")
	case 1:
		t.menuStatus.SetTitle("1 container running")
	default:
		t.menuStatus.SetTitle(fmt.Sprintf("%d containers running", running))
	}
}

// openDashboard opens the server URL in the default browser.
func (t *trayState) openDashboard() {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", t.srv.baseURL)
	case "windows":
		cmd = exec.Command("cmd", "/c", "start", t.srv.baseURL)
	default:
		cmd = exec.Command("xdg-open", t.srv.baseURL)
	}
	if err := cmd.Start(); err != nil {
		log.Printf("could not open browser: %v", err)
	}
}

// handleQuit runs the quit flow: check containers, ask user,
// stop containers if requested, then shut down the server.
func (t *trayState) handleQuit() {
	projects, err := t.srv.listProjects()
	if err != nil {
		// Can't reach server — just quit.
		t.shuttingDown.Store(true)
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
	t.srv.shutdown()
	// The poll loop will detect the server is gone and call systray.Quit().
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
