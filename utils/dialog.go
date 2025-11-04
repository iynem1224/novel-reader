package utils

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"novel_reader/lang"
)

var (
	ErrDialogCancelled   = errors.New("folder selection cancelled")
	ErrDialogUnavailable = errors.New("folder selection dialog unavailable")
)

// SelectFolderDialog opens a system-specific dialog for choosing a folder and returns
// the selected absolute path. If the dialog is cancelled, ErrDialogCancelled is returned.
func SelectFolderDialog(start string) (string, error) {
	if start == "" {
		if home, err := os.UserHomeDir(); err == nil {
			start = home
		}
	}

	switch runtime.GOOS {
	case "darwin":
		return selectFolderDarwin(start)
	case "windows":
		return selectFolderWindows(start)
	default:
		return selectFolderLinux(start)
	}
}

func selectFolderDarwin(start string) (string, error) {
	// Resolve a safe default dir (must exist)
	cleanStart := filepath.Clean(start)
	if info, err := os.Stat(cleanStart); err != nil || !info.IsDir() {
		if home, err := os.UserHomeDir(); err == nil {
			cleanStart = home
		} else {
			cleanStart = "/"
		}
	}
	// AppleScript with fallback if default location is invalid
	prompt := lang.Active().Dialog.SelectFolderPrompt
	script := fmt.Sprintf(`
        set _prompt to "%s"
        set _p to ""
        try
            set _p to POSIX path of (choose folder with prompt _prompt default location POSIX file "%s")
        on error errMsg number errNum
            if errNum is -128 then error number -128 -- user cancelled, propagate
            set _p to POSIX path of (choose folder with prompt _prompt)
        end try
        return _p
    `, escapeAppleScriptString(prompt), escapeAppleScriptString(cleanStart))

	// IMPORTANT: capture only stdout (stderr contains IMKClient logs)
	cmd := exec.Command("osascript", "-e", script)
	out, err := cmd.Output()
	if err != nil {
		// Treat user cancel as cancellation
		if ee, ok := err.(*exec.ExitError); ok && len(ee.Stderr) == 0 {
			return "", ErrDialogCancelled
		}
		return "", fmt.Errorf("osascript: %v", err)
	}

	// Pick the first absolute path line (in case anything sneaks into stdout)
	chosen := firstAbsoluteLine(string(out))
	if chosen == "" {
		return "", ErrDialogCancelled
	}
	chosen = strings.ReplaceAll(strings.TrimSpace(chosen), "\r", "")
	chosen = filepath.Clean(chosen)

	// Validate the directory
	if info, err := os.Stat(chosen); err != nil || !info.IsDir() {
		return "", fmt.Errorf("not a directory: %s", chosen)
	}
	return chosen, nil
}

func firstAbsoluteLine(s string) string {
	for _, ln := range strings.Split(s, "\n") {
		ln = strings.TrimSpace(ln)
		if strings.HasPrefix(ln, "/") {
			return ln
		}
	}
	return ""
}

func selectFolderWindows(start string) (string, error) {
	escaped := escapePowerShellString(filepath.Clean(start))
	description := escapePowerShellString(lang.Active().Dialog.SelectFolderPrompt)
	script := fmt.Sprintf(`[System.Reflection.Assembly]::LoadWithPartialName('System.windows.forms') | Out-Null;
$dialog = New-Object System.Windows.Forms.FolderBrowserDialog;
$dialog.Description = '%s';
$dialog.SelectedPath = '%s';
$dialog.ShowNewFolderButton = $true;
if ($dialog.ShowDialog() -eq 'OK') { Write-Output $dialog.SelectedPath }`, description, escaped)

	cmd := exec.Command("powershell", "-NoProfile", "-Command", script)
	out, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok && len(exitErr.Stderr) == 0 {
			return "", ErrDialogCancelled
		}
		return "", err
	}

	path := strings.TrimSpace(string(out))
	if path == "" {
		return "", ErrDialogCancelled
	}

	return filepath.Clean(path), nil
}

func selectFolderLinux(start string) (string, error) {
	title := lang.Active().Dialog.SelectFolderPrompt
	candidates := [][]string{
		{"zenity", "--file-selection", "--directory", "--title", title, "--filename", ensureTrailingSeparator(filepath.Clean(start))},
		{"kdialog", "--getexistingdirectory", filepath.Clean(start), "--title", title},
	}

	for _, args := range candidates {
		cmd := exec.Command(args[0], args[1:]...)
		out, err := cmd.Output()
		if err != nil {
			if errors.Is(err, exec.ErrNotFound) {
				continue
			}
			if exitErr, ok := err.(*exec.ExitError); ok {
				if exitErr.ExitCode() == 1 {
					return "", ErrDialogCancelled
				}
			}
			return "", err
		}

		path := strings.TrimSpace(string(out))
		if path == "" {
			return "", ErrDialogCancelled
		}

		return filepath.Clean(path), nil
	}

	return "", ErrDialogUnavailable
}

func escapeAppleScriptString(s string) string {
	replacer := strings.NewReplacer("\\", "\\\\", "\"", "\\\"")
	return replacer.Replace(s)
}

func escapePowerShellString(s string) string {
	return strings.ReplaceAll(s, "'", "''")
}

func ensureTrailingSeparator(path string) string {
	if strings.HasSuffix(path, string(os.PathSeparator)) {
		return path
	}
	return path + string(os.PathSeparator)
}
