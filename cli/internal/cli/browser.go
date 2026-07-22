package cli

import (
	"fmt"
	"os/exec"
	"runtime"
)

// openBrowser tries to open url in the default browser. Failure is expected
// on headless machines and must be treated as non-fatal by callers.
func openBrowser(url string) error {
	switch runtime.GOOS {
	case "darwin":
		return exec.Command("open", url).Start()
	case "linux":
		return exec.Command("xdg-open", url).Start()
	case "windows":
		return exec.Command("rundll32", "url.dll,FileProtocolHandler", url).Start()
	default:
		return fmt.Errorf("unsupported platform %s", runtime.GOOS)
	}
}
