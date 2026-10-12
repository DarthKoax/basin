package clipboard

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/atotto/clipboard"
)

const clipboardTimeout = 3 * time.Second

func isWSL() bool {
	if os.Getenv("WSL_DISTRO_NAME") != "" {
		return true
	}
	data, err := os.ReadFile("/proc/version")
	if err != nil {
		return false
	}
	return strings.Contains(strings.ToLower(string(data)), "microsoft")
}

func writeWSL(text string) error {
	done := make(chan error, 1)
	go func() {
		cmd := exec.Command("clip.exe")
		cmd.Stdin = strings.NewReader(text)
		done <- cmd.Run()
	}()

	select {
	case err := <-done:
		return err
	case <-time.After(clipboardTimeout):
		return fmt.Errorf("clipboard write timed out after %v", clipboardTimeout)
	}
}

func readWSL() (string, error) {
	done := make(chan struct {
		out []byte
		err error
	}, 1)
	go func() {
		cmd := exec.Command("powershell.exe", "-NoProfile", "-Command", "Get-Clipboard")
		var out bytes.Buffer
		cmd.Stdout = &out
		err := cmd.Run()
		done <- struct {
			out []byte
			err error
		}{out.Bytes(), err}
	}()

	select {
	case result := <-done:
		if result.err != nil {
			return "", result.err
		}
		return strings.TrimRight(string(result.out), "\r\n"), nil
	case <-time.After(clipboardTimeout):
		return "", fmt.Errorf("clipboard read timed out after %v", clipboardTimeout)
	}
}

func WriteAll(text string) error {
	if isWSL() {
		return writeWSL(text)
	}

	if os.Getenv("WAYLAND_DISPLAY") == "" && os.Getenv("DISPLAY") == "" {
		return fmt.Errorf("no display server found: DISPLAY and WAYLAND_DISPLAY are not set")
	}

	done := make(chan error, 1)
	go func() {
		done <- clipboard.WriteAll(text)
	}()

	select {
	case err := <-done:
		return err
	case <-time.After(clipboardTimeout):
		return fmt.Errorf("clipboard write timed out after %v (display server may not be reachable)", clipboardTimeout)
	}
}

func Write(text string) error {
	return WriteAll(text)
}

func ReadAll() (string, error) {
	if isWSL() {
		return readWSL()
	}

	if os.Getenv("WAYLAND_DISPLAY") == "" && os.Getenv("DISPLAY") == "" {
		return "", fmt.Errorf("no display server found: DISPLAY and WAYLAND_DISPLAY are not set")
	}

	done := make(chan struct {
		out string
		err error
	}, 1)
	go func() {
		out, err := clipboard.ReadAll()
		done <- struct {
			out string
			err error
		}{out, err}
	}()

	select {
	case result := <-done:
		return result.out, result.err
	case <-time.After(clipboardTimeout):
		return "", fmt.Errorf("clipboard read timed out after %v (display server may not be reachable)", clipboardTimeout)
	}
}

func DetectClipboardTool() (string, error) {
	if isWSL() {
		if _, err := exec.LookPath("clip.exe"); err == nil {
			return "clip.exe (WSL2 Windows)", nil
		}
		return "", fmt.Errorf("WSL2 detected but clip.exe not found in PATH")
	}

	hasDisplay := os.Getenv("WAYLAND_DISPLAY") != "" || os.Getenv("DISPLAY") != ""

	if os.Getenv("WAYLAND_DISPLAY") != "" {
		if _, err := exec.LookPath("wl-copy"); err == nil {
			return "wl-copy (Wayland)", nil
		}
	}
	if hasDisplay {
		if _, err := exec.LookPath("xclip"); err == nil {
			return "xclip (X11)", nil
		}
		if _, err := exec.LookPath("xsel"); err == nil {
			return "xsel (X11)", nil
		}
	}

	var installed []string
	var missing []string
	for _, tool := range []string{"xclip", "xsel", "wl-copy"} {
		if _, err := exec.LookPath(tool); err == nil {
			installed = append(installed, tool)
		} else {
			missing = append(missing, tool)
		}
	}

	if len(installed) > 0 {
		return "", fmt.Errorf("clipboard tools installed (%s) but no display server available (DISPLAY and WAYLAND_DISPLAY are unset)", strings.Join(installed, ", "))
	}
	return "", fmt.Errorf("no clipboard tools found. Install one of: %s", strings.Join(missing, ", "))
}
