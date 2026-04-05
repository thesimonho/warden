package main

import (
	"fmt"

	"github.com/ncruces/zenity"
)

// quitAction represents the user's choice in the quit confirmation dialog.
type quitAction int

const (
	quitActionCancel      quitAction = iota
	quitActionQuit                   // Quit server, leave containers running.
	quitActionStopAndQuit            // Stop all containers, then quit server.
)

// showQuitDialog displays a native confirmation dialog when the user
// quits with running containers. Returns the chosen action.
//
// Dialog buttons:
//   - "Stop & Quit" — stops all containers, then shuts down
//   - "Quit" — shuts down server, containers keep running
//   - "Cancel" — abort the quit
func showQuitDialog(containerCount int) quitAction {
	msg := fmt.Sprintf(
		"%d container(s) are still running.\n\nStop them before quitting?",
		containerCount,
	)

	err := zenity.Question(
		msg,
		zenity.Title("Quit Warden"),
		zenity.OKLabel("Stop & Quit"),
		zenity.CancelLabel("Cancel"),
		zenity.ExtraButton("Quit"),
	)

	switch err {
	case nil:
		// OK button ("Stop & Quit") was clicked.
		return quitActionStopAndQuit
	case zenity.ErrExtraButton:
		// Extra button ("Quit") was clicked.
		return quitActionQuit
	default:
		// Cancel or dialog closed.
		return quitActionCancel
	}
}
