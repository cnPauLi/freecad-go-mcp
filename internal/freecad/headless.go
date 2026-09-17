// Headless script execution: run a FreeCAD script in a separate freecadcmd
// process so heavy OCCT work (helical sweeps, lofts, many-tool booleans) that
// crashes OpenCascade only kills the helper process, not the GUI and its
// unsaved documents.
package freecad

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"syscall"
	"time"
)

// flatpakApp is probed when no freecadcmd binary is on PATH.
const flatpakApp = "org.freecad.FreeCAD"

// noiseLines are FreeCAD banner/progress fragments stripped from the output.
var noiseLines = []string{"%)", "Importing project files", "Postprocessing", "FreeCAD 1.", "(C) 2001", "LGPL"}

// crashReport matches FreeCAD's own signal handler output. FreeCAD can print a
// native backtrace and exit 1, hiding the signal from the OS exit status.
var crashReport = regexp.MustCompile(`(?m)^Program received signal (SIG[A-Z0-9]+),`)

// ParseCommand splits a shell-like command line, e.g.
// "flatpak run --command=freecadcmd org.freecad.FreeCAD".
func ParseCommand(value string) ([]string, error) {
	if strings.TrimSpace(value) == "" {
		return nil, nil
	}
	return splitCommand(value)
}

// DetectFreeCADCmd finds a headless FreeCAD executable: PATH first, then the
// Flatpak app.
func DetectFreeCADCmd() []string {
	for _, name := range []string{"freecadcmd", "FreeCADCmd", "freecadcmd.exe", "freecad.cmd"} {
		if path, err := exec.LookPath(name); err == nil {
			return []string{path}
		}
	}
	flatpak, err := exec.LookPath("flatpak")
	if err != nil {
		return nil
	}
	probeCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := exec.CommandContext(probeCtx, flatpak, "info", flatpakApp).Run(); err != nil {
		return nil
	}
	return []string{flatpak, "run", "--command=freecadcmd", flatpakApp}
}

// RunHeadless executes code with `command -c "exec(open(script).read())"`.
//
// The returned map reports success, returncode, the filtered output, and the
// crashed / timed_out flags, mirroring the Python headless runner.
func RunHeadless(ctx context.Context, code string, timeout float64, command []string) map[string]any {
	if math.IsNaN(timeout) || math.IsInf(timeout, 0) || timeout <= 0 {
		return map[string]any{"success": false, "error": "timeout must be a positive finite number"}
	}
	resolved := command
	if len(resolved) == 0 {
		resolved = DetectFreeCADCmd()
	}
	if len(resolved) == 0 {
		return map[string]any{
			"success": false,
			"error":   "freecadcmd not found: install FreeCAD, or pass --freecadcmd to freecad-mcp",
		}
	}

	dir, err := scriptDir()
	if err != nil {
		return map[string]any{"success": false, "error": fmt.Sprintf("could not prepare the script directory: %v", err)}
	}
	script, err := writeScript(dir, code)
	if err != nil {
		return map[string]any{"success": false, "error": fmt.Sprintf("could not write the headless script: %v", err)}
	}
	defer func() { _ = os.Remove(script) }()

	statement := fmt.Sprintf("exec(open(%s, encoding='utf-8').read())", pythonString(script))
	argv := append(append([]string{}, resolved...), "-c", statement)

	runCtx, cancel := context.WithTimeout(ctx, time.Duration(timeout*float64(time.Second)))
	defer cancel()

	var stdout, stderr bytes.Buffer
	cmd := exec.CommandContext(runCtx, argv[0], argv[1:]...)
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	runErr := cmd.Run()
	output := cleanOutput(stdout.String() + "\n" + stderr.String())

	if errors.Is(runCtx.Err(), context.DeadlineExceeded) {
		return map[string]any{
			"success":   false,
			"timed_out": true,
			"error":     fmt.Sprintf("headless FreeCAD did not finish within %s s", formatG(timeout)),
			"output":    output,
		}
	}

	returncode := 0
	if runErr != nil {
		var exitErr *exec.ExitError
		if !errors.As(runErr, &exitErr) {
			return map[string]any{"success": false, "error": fmt.Sprintf("could not start headless FreeCAD: %v", runErr)}
		}
		returncode = exitCode(exitErr)
	}

	result := map[string]any{
		"success":    returncode == 0,
		"returncode": returncode,
		"output":     output,
	}
	switch {
	case returncode < 0:
		result["crashed"] = true
		result["error"] = fmt.Sprintf(
			"headless FreeCAD crashed with %s (OCCT native crash; the GUI is unaffected)",
			signalName(-returncode),
		)
	case returncode != 0:
		if match := crashReport.FindStringSubmatch(output); match != nil {
			result["crashed"] = true
			result["error"] = fmt.Sprintf(
				"headless FreeCAD reported a crash with %s (exit code %d; the GUI is unaffected)",
				match[1], returncode,
			)
		} else {
			result["error"] = fmt.Sprintf("script failed (exit code %d)", returncode)
		}
	}
	return result
}

// scriptDir returns the directory holding temporary scripts. Flatpak sandboxes
// usually see $HOME but not /tmp, so scripts stay under the home directory.
func scriptDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(home, ".cache", "freecad-mcp", "headless")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	return dir, nil
}

func writeScript(dir, code string) (string, error) {
	file, err := os.CreateTemp(dir, "*.py")
	if err != nil {
		return "", err
	}
	if _, err := file.WriteString(code); err != nil {
		_ = file.Close()
		_ = os.Remove(file.Name())
		return "", err
	}
	if err := file.Close(); err != nil {
		_ = os.Remove(file.Name())
		return "", err
	}
	return file.Name(), nil
}

// cleanOutput drops blank lines and FreeCAD banner/progress noise.
func cleanOutput(output string) string {
	lines := make([]string, 0, 16)
	for _, line := range strings.Split(output, "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		noisy := false
		for _, noise := range noiseLines {
			if strings.Contains(line, noise) {
				noisy = true
				break
			}
		}
		if !noisy {
			lines = append(lines, line)
		}
	}
	return strings.Join(lines, "\n")
}

// exitCode converts a wait status to the negative-signal convention Python's
// subprocess uses.
func exitCode(err *exec.ExitError) int {
	if status, ok := err.Sys().(syscall.WaitStatus); ok {
		if status.Signaled() {
			return -int(status.Signal())
		}
		return status.ExitStatus()
	}
	return err.ExitCode()
}

var signalNames = map[int]string{
	1: "SIGHUP", 2: "SIGINT", 3: "SIGQUIT", 4: "SIGILL", 6: "SIGABRT",
	8: "SIGFPE", 9: "SIGKILL", 10: "SIGBUS", 11: "SIGSEGV", 13: "SIGPIPE",
	14: "SIGALRM", 15: "SIGTERM",
}

func signalName(signal int) string {
	if name, ok := signalNames[signal]; ok {
		return name
	}
	return strconv.Itoa(signal)
}

// formatG renders a float the way Python's "{:g}" format does.
func formatG(value float64) string {
	return strconv.FormatFloat(value, 'g', -1, 64)
}

// pythonString renders a Python string literal for a filesystem path.
func pythonString(path string) string {
	var b strings.Builder
	b.WriteByte('\'')
	for _, char := range path {
		switch char {
		case '\\':
			b.WriteString(`\\`)
		case '\'':
			b.WriteString(`\'`)
		case '\n':
			b.WriteString(`\n`)
		case '\r':
			b.WriteString(`\r`)
		case '\t':
			b.WriteString(`\t`)
		default:
			b.WriteRune(char)
		}
	}
	b.WriteByte('\'')
	return b.String()
}

// splitCommand implements POSIX shell word splitting (Python's shlex.split):
// single quotes are literal, double quotes honour \" \\ \$ \` escapes, and a
// backslash outside quotes escapes the next character.
func splitCommand(value string) ([]string, error) {
	var (
		fields  []string
		current strings.Builder
		started bool
	)
	runes := []rune(value)
	for i := 0; i < len(runes); i++ {
		char := runes[i]
		switch char {
		case ' ', '\t', '\n':
			if started {
				fields = append(fields, current.String())
				current.Reset()
				started = false
			}
		case '\'':
			started = true
			end := i + 1
			for end < len(runes) && runes[end] != '\'' {
				end++
			}
			if end == len(runes) {
				return nil, errors.New("no closing quotation")
			}
			current.WriteString(string(runes[i+1 : end]))
			i = end
		case '"':
			started = true
			i++
			closed := false
			for ; i < len(runes); i++ {
				if runes[i] == '\\' && i+1 < len(runes) {
					next := runes[i+1]
					if next == '"' || next == '\\' || next == '$' || next == '`' {
						current.WriteRune(next)
						i++
						continue
					}
					current.WriteRune('\\')
					continue
				}
				if runes[i] == '"' {
					closed = true
					break
				}
				current.WriteRune(runes[i])
			}
			if !closed {
				return nil, errors.New("no closing quotation")
			}
		case '\\':
			if i+1 >= len(runes) {
				return nil, errors.New("no escaped character")
			}
			i++
			started = true
			current.WriteRune(runes[i])
		default:
			started = true
			current.WriteRune(char)
		}
	}
	if started {
		fields = append(fields, current.String())
	}
	return fields, nil
}
