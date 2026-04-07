package main

import (
	"fmt"
	"log"
	"os/exec"
	"runtime"
	"sync"
	"sync/atomic"
	"time"

	"github.com/energye/systray"
)

const pollInterval = 5 * time.Second

// trayState holds mutable state shared between the poll loop and
// menu click handlers.
type trayState struct {
	srv          *serverClient
	shuttingDown atomic.Bool
	menuStatus   *systray.MenuItem
	mu           sync.Mutex
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
	systray.SetTemplateIcon(iconData, iconData)
	systray.SetTooltip("Warden")

	systray.SetOnClick(func(_ systray.IMenu) { t.openDashboard() })
	systray.SetOnRClick(func(menu systray.IMenu) { _ = menu.ShowMenu() })

	mOpen := systray.AddMenuItem("Open Dashboard", "Open the Warden dashboard in a browser")
	mOpen.Click(func() { t.openDashboard() })

	systray.AddSeparator()

	t.menuStatus = systray.AddMenuItem("Checking...", "")
	t.menuStatus.Disable()

	systray.AddSeparator()

	mQuit := systray.AddMenuItem("Quit Warden", "Shut down the Warden server")
	mQuit.Click(func() { t.handleQuit() })

	// Update container count immediately, then poll.
	t.updateStatus()
	go t.pollLoop()
}

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

// updateStatusFromProjects refreshes the container count menu item.
func (t *trayState) updateStatusFromProjects(projects []project) {
	var count int
	for _, p := range projects {
		if p.State == stateRunning {
			count++
		}
	}

	t.mu.Lock()
	defer t.mu.Unlock()

	switch count {
	case 0:
		t.menuStatus.SetTitle("No containers running")
	case 1:
		t.menuStatus.SetTitle("1 container running")
	default:
		t.menuStatus.SetTitle(fmt.Sprintf("%d containers running", count))
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
