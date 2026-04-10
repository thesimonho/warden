package main

import (
	"fmt"
	"log"
	"os/exec"
	"runtime"
	"sync"

	"github.com/godbus/dbus/v5"
)

// notificationType mirrors event.NotificationType without importing it.
type notificationType string

const (
	notifPermissionPrompt  notificationType = "permission_prompt"
	notifIdlePrompt        notificationType = "idle_prompt"
	notifElicitationDialog notificationType = "elicitation_dialog"
	notifAuthSuccess       notificationType = "auth_success"
)

// notificationMessage returns a human-readable (title, body) pair for
// the given notification type and project name.
func notificationMessage(nt notificationType, projectName string) (title, body string) {
	switch nt {
	case notifPermissionPrompt:
		return fmt.Sprintf("%s needs tool approval", projectName),
			"A worktree is waiting for permission."
	case notifElicitationDialog:
		return fmt.Sprintf("%s has a question", projectName),
			"The agent is asking a question that needs your answer."
	case notifIdlePrompt:
		return fmt.Sprintf("%s is waiting for input", projectName),
			"The agent is done and waiting for the next prompt."
	case notifAuthSuccess:
		return fmt.Sprintf("%s authentication complete", projectName),
			"Authentication was successful."
	default:
		return fmt.Sprintf("%s needs attention", projectName),
			"A worktree is waiting for your response."
	}
}

// notifier tracks notification dedup state and sends platform-native
// desktop notifications with click-to-open support.
type notifier struct {
	baseURL string

	mu sync.Mutex
	// lastNotified tracks the most recent notification type sent per
	// project key (projectId:agentType). Only cleared when needsInput
	// transitions back to false, preventing duplicate notifications
	// during the same attention cycle.
	lastNotified map[string]notificationType
}

// newNotifier creates a notifier targeting the given Warden server URL.
func newNotifier(baseURL string) *notifier {
	return &notifier{
		baseURL:      baseURL,
		lastNotified: make(map[string]notificationType),
	}
}

// notify sends a desktop notification if this is a new attention cycle
// or a different notification type than last sent for this project.
// Returns true if a notification was actually sent.
func (n *notifier) notify(projectKey, projectName, projectID, agentType string, nt notificationType) bool {
	n.mu.Lock()
	defer n.mu.Unlock()

	if prev, ok := n.lastNotified[projectKey]; ok && prev == nt {
		return false
	}
	n.lastNotified[projectKey] = nt

	title, body := notificationMessage(nt, projectName)
	deepLink := fmt.Sprintf("%s/projects/%s/%s", n.baseURL, projectID, agentType)

	go sendDesktopNotification(title, body, deepLink)
	return true
}

// clearAttention removes the dedup entry for a project, allowing
// future notifications when attention is needed again.
func (n *notifier) clearAttention(projectKey string) {
	n.mu.Lock()
	defer n.mu.Unlock()
	delete(n.lastNotified, projectKey)
}

// sendDesktopNotification dispatches a native notification with a
// click-to-open action. Platform-specific implementations:
//   - macOS:   osascript (click opens URL via AppleScript handler)
//   - Linux:   DBus org.freedesktop.Notifications with action callback
//   - Windows: PowerShell toast notification with activation URI
func sendDesktopNotification(title, body, openURL string) {
	var err error
	switch runtime.GOOS {
	case "darwin":
		err = notifyMacOS(title, body, openURL)
	case "linux":
		err = notifyLinux(title, body, openURL)
	case "windows":
		err = notifyWindows(title, body, openURL)
	default:
		log.Printf("desktop notifications not supported on %s", runtime.GOOS)
		return
	}
	if err != nil {
		log.Printf("failed to send notification: %v", err)
	}
}

// openBrowser opens a URL in the system default browser.
func openBrowser(url string) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "windows":
		cmd = exec.Command("cmd", "/c", "start", url)
	default:
		cmd = exec.Command("xdg-open", url)
	}
	if err := cmd.Start(); err != nil {
		log.Printf("failed to open browser: %v", err)
	}
}

// --- macOS ---

// notifyMacOS uses osascript to display a notification.
//
// Standard osascript notifications don't support click actions, so we
// use a Script Editor approach: display the notification and tell
// "System Events" to open the URL when the user interacts with it.
// This is a known limitation — macOS sandboxing prevents true click
// callbacks without a bundled app with NSUserNotificationCenter.
//
// As a pragmatic workaround for the app bundle, we use terminal-notifier
// if available (it supports -open), otherwise fall back to plain osascript.
func notifyMacOS(title, body, openURL string) error {
	// Prefer terminal-notifier if installed — it supports click-to-open.
	if path, err := exec.LookPath("terminal-notifier"); err == nil {
		return exec.Command(path,
			"-title", title,
			"-message", body,
			"-open", openURL,
			"-sender", "com.warden.app",
			"-group", "warden",
		).Run()
	}

	// Fallback: plain osascript (no click action).
	script := fmt.Sprintf(`display notification %q with title %q`, body, title)
	return exec.Command("osascript", "-e", script).Run()
}

// --- Linux ---

// notifyLinux sends a notification via the DBus org.freedesktop.Notifications
// interface with a "default" action. When the user clicks the notification
// body, the notification server emits ActionInvoked and we open the URL.
//
// Uses the godbus/dbus library (already a dependency for color scheme
// detection) for proper typed DBus calls instead of shelling out to gdbus.
func notifyLinux(title, body, openURL string) error {
	conn, err := dbus.SessionBus()
	if err != nil {
		return fmt.Errorf("connecting to session bus: %w", err)
	}

	obj := conn.Object(
		"org.freedesktop.Notifications",
		"/org/freedesktop/Notifications",
	)

	// Call org.freedesktop.Notifications.Notify per the freedesktop spec.
	// The "default" action key is a special action that fires when the
	// notification body is clicked (not an explicit button).
	call := obj.Call(
		"org.freedesktop.Notifications.Notify", 0,
		"Warden",                                  // app_name
		uint32(0),                                 // replaces_id
		"",                                        // app_icon
		title,                                     // summary
		body,                                      // body
		[]string{"default", "Open in browser"},    // actions
		map[string]dbus.Variant{                      // hints
			"urgency":       dbus.MakeVariant(byte(1)),  // normal
			"desktop-entry": dbus.MakeVariant("warden"), // register as Warden in KDE notification settings
		},
		int32(-1), // expire_timeout_ms (-1 = server default)
	)
	if call.Err != nil {
		return fmt.Errorf("Notify call: %w", call.Err)
	}

	// Extract the notification ID from the response.
	var notifID uint32
	if err := call.Store(&notifID); err != nil {
		return fmt.Errorf("reading notification ID: %w", err)
	}

	// Listen for the ActionInvoked signal in the background.
	// When the user clicks, we open the URL and stop listening.
	go listenForAction(conn, notifID, openURL)
	return nil
}

// listenForAction watches for an ActionInvoked DBus signal matching the
// given notification ID. Opens the URL when the "default" action fires,
// and cleans up when the notification is closed (NotificationClosed signal).
func listenForAction(conn *dbus.Conn, notifID uint32, openURL string) {
	matchOpts := []dbus.MatchOption{
		dbus.WithMatchObjectPath("/org/freedesktop/Notifications"),
		dbus.WithMatchInterface("org.freedesktop.Notifications"),
	}

	if err := conn.AddMatchSignal(matchOpts...); err != nil {
		log.Printf("failed to subscribe to notification signals: %v", err)
		return
	}
	defer func() {
		// Remove the DBus match rule to prevent accumulation.
		_ = conn.RemoveMatchSignal(matchOpts...)
	}()

	ch := make(chan *dbus.Signal, 4)
	conn.Signal(ch)
	defer conn.RemoveSignal(ch)

	for sig := range ch {
		switch sig.Name {
		case "org.freedesktop.Notifications.ActionInvoked":
			// ActionInvoked(uint32 id, string action_key)
			if len(sig.Body) >= 2 {
				id, _ := sig.Body[0].(uint32)
				action, _ := sig.Body[1].(string)
				if id == notifID && action == "default" {
					openBrowser(openURL)
					return
				}
			}
		case "org.freedesktop.Notifications.NotificationClosed":
			// NotificationClosed(uint32 id, uint32 reason)
			if len(sig.Body) >= 1 {
				id, _ := sig.Body[0].(uint32)
				if id == notifID {
					return
				}
			}
		}
	}
}

// --- Windows ---

// notifyWindows uses PowerShell to display a Windows toast notification
// with an activation URI that opens the deep link on click.
func notifyWindows(title, body, openURL string) error {
	script := fmt.Sprintf(`
[Windows.UI.Notifications.ToastNotificationManager, Windows.UI.Notifications, ContentType = WindowsRuntime] | Out-Null
[Windows.Data.Xml.Dom.XmlDocument, Windows.Data.Xml.Dom, ContentType = WindowsRuntime] | Out-Null

$template = @"
<toast activationType="protocol" launch="%s">
  <visual>
    <binding template="ToastGeneric">
      <text>%s</text>
      <text>%s</text>
    </binding>
  </visual>
</toast>
"@

$xml = New-Object Windows.Data.Xml.Dom.XmlDocument
$xml.LoadXml($template)
$toast = [Windows.UI.Notifications.ToastNotification]::new($xml)
[Windows.UI.Notifications.ToastNotificationManager]::CreateToastNotifier("Warden").Show($toast)
`, openURL, title, body)

	return exec.Command("powershell", "-NoProfile", "-Command", script).Run()
}
