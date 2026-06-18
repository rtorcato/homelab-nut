package tui

import (
	"os/exec"
	"runtime"
)

// openURL spawns the platform's default URL opener in a detached
// process. Fire-and-forget — we don't wait on the browser and we
// don't surface errors, since this is a best-effort delight action
// (e.g. the `o` keybinding that opens the project page).
func openURL(url string) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	default:
		cmd = exec.Command("xdg-open", url)
	}
	_ = cmd.Start()
}
