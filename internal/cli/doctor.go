package cli

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss/v2"

	"github.com/stasfilin/nvim-sandbox/internal/app"
)

type doctorCheck struct {
	Name    string `json:"name"`
	Status  string `json:"status"`
	Message string `json:"message"`
}

type doctorReport struct {
	Action string        `json:"action"`
	OK     bool          `json:"ok"`
	Checks []doctorCheck `json:"checks"`
}

func doctorCommand(opts options, stdout io.Writer, stderr io.Writer) int {
	report := runDoctor()
	code := 0
	if !report.OK {
		code = 1
	}
	return respond(stdout, opts, code, doctorPayload(report), humanDoctor(report, opts.format == "text" && isTerminal() && writerIsTerminal(stdout)))
}

func runDoctor() doctorReport {
	checks := []doctorCheck{}
	available := app.AvailableRuntimes()
	detected := app.DetectRuntime()
	if detected == "" {
		checks = append(checks, doctorCheck{Name: "runtime", Status: "error", Message: "No supported runtime found on PATH. Install Apple Container, Docker, or Podman."})
	} else {
		labels := []string{}
		for _, runtime := range available {
			labels = append(labels, runtime.Label)
		}
		checks = append(checks, doctorCheck{Name: "runtime", Status: "ok", Message: runtimeDisplayName(detected) + " selected"})
		checks = append(checks, doctorCheck{Name: "available", Status: "ok", Message: strings.Join(labels, ", ")})
	}

	state, err := app.NewState()
	if err != nil {
		checks = append(checks, doctorCheck{Name: "state", Status: "error", Message: err.Error()})
	} else if err := state.Ensure(); err != nil {
		checks = append(checks, doctorCheck{Name: "state", Status: "error", Message: err.Error()})
	} else if err := writableDir(state.Base()); err != nil {
		checks = append(checks, doctorCheck{Name: "state", Status: "error", Message: state.Base() + " is not writable: " + err.Error()})
	} else {
		checks = append(checks, doctorCheck{Name: "state", Status: "ok", Message: state.Base()})
	}

	ctx, err := app.ProjectContext("")
	if err != nil {
		checks = append(checks, doctorCheck{Name: "project", Status: "error", Message: err.Error()})
	} else if state != nil {
		metadata, err := state.ReadProject(ctx.WorkspaceID)
		if err != nil {
			checks = append(checks, doctorCheck{Name: "project", Status: "error", Message: "Project metadata is unreadable: " + err.Error()})
		} else if metadata == nil {
			checks = append(checks, doctorCheck{Name: "project", Status: "ok", Message: displayPath(ctx.ProjectRoot, "short") + ": no sandbox metadata yet"})
		} else if app.BackendFor(metadata.Runtime) == nil {
			checks = append(checks, doctorCheck{Name: "project", Status: "error", Message: "Saved runtime is not supported: " + metadata.Runtime})
		} else {
			checks = append(checks, doctorCheck{Name: "project", Status: "ok", Message: displayPath(ctx.ProjectRoot, "short") + ": " + metadata.ContainerName + " via " + fallback(runtimeDisplayName(metadata.Runtime), metadata.Runtime)})
		}
	}

	if check := devBinaryCheck(); check.Name != "" {
		checks = append(checks, check)
	}

	ok := true
	for _, check := range checks {
		if check.Status == "error" {
			ok = false
			break
		}
	}
	return doctorReport{Action: "doctor", OK: ok, Checks: checks}
}

func writableDir(dir string) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	file, err := os.CreateTemp(dir, ".doctor-*.tmp")
	if err != nil {
		return err
	}
	name := file.Name()
	if err := file.Close(); err != nil {
		_ = os.Remove(name)
		return err
	}
	return os.Remove(name)
}

func writerIsTerminal(writer io.Writer) bool {
	file, ok := writer.(*os.File)
	if !ok {
		return false
	}
	info, err := file.Stat()
	return err == nil && (info.Mode()&os.ModeCharDevice) != 0
}

func devBinaryCheck() doctorCheck {
	cwd, err := os.Getwd()
	if err != nil {
		return doctorCheck{}
	}
	if _, err := os.Stat(filepath.Join(cwd, "go.mod")); err != nil {
		return doctorCheck{}
	}
	if _, err := os.Stat(filepath.Join(cwd, "cmd", "nvim-sandbox", "main.go")); err != nil {
		return doctorCheck{}
	}
	binary := filepath.Join(cwd, "dist", "nvim-sandbox")
	binaryInfo, err := os.Stat(binary)
	if err != nil {
		return doctorCheck{Name: "dev-binary", Status: "warn", Message: "dist/nvim-sandbox is missing. Run `make build`."}
	}
	latestSource, err := latestGoSourceModTime(cwd)
	if err != nil {
		return doctorCheck{Name: "dev-binary", Status: "warn", Message: "Could not inspect source timestamps: " + err.Error()}
	}
	if latestSource.After(binaryInfo.ModTime().Add(time.Second)) {
		return doctorCheck{Name: "dev-binary", Status: "warn", Message: "dist/nvim-sandbox is older than Go sources. Run `make build`."}
	}
	return doctorCheck{Name: "dev-binary", Status: "ok", Message: "dist/nvim-sandbox is up to date"}
}

func latestGoSourceModTime(root string) (time.Time, error) {
	var latest time.Time
	for _, dir := range []string{"cmd", "internal"} {
		err := filepath.WalkDir(filepath.Join(root, dir), func(path string, entry os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".go") {
				return nil
			}
			info, err := entry.Info()
			if err != nil {
				return err
			}
			if info.ModTime().After(latest) {
				latest = info.ModTime()
			}
			return nil
		})
		if err != nil {
			return latest, err
		}
	}
	return latest, nil
}

func doctorPayload(report doctorReport) map[string]any {
	checks := []map[string]any{}
	for _, check := range report.Checks {
		checks = append(checks, map[string]any{
			"name":    check.Name,
			"status":  check.Status,
			"message": check.Message,
		})
	}
	return map[string]any{"action": report.Action, "checks": checks}
}

func humanDoctor(report doctorReport, styled bool) string {
	if styled {
		return styledDoctor(report)
	}
	lines := []string{"nvim-sandbox doctor", ""}
	for _, check := range report.Checks {
		lines = append(lines, fmt.Sprintf("%-10s %-5s %s", check.Name+":", check.Status, check.Message))
	}
	if report.OK {
		lines = append(lines, "", "Result: ok")
	} else {
		lines = append(lines, "", "Result: issues found")
	}
	return strings.Join(lines, "\n")
}

func styledDoctor(report doctorReport) string {
	styles := newWizardStyles(true)
	border := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#94a3b8")).
		Padding(1, 2)
	header := styles.title.Render("nvim-sandbox") + styles.muted.Render(" > ") + styles.accent.Render("doctor")
	lines := []string{header, ""}
	for _, check := range report.Checks {
		label := styles.muted.Render(fmt.Sprintf("%-11s", check.Name+":"))
		status := doctorStatusStyle(styles, check.Status).Render(fmt.Sprintf("%-5s", check.Status))
		line := label + "  " + status
		if check.Status != "ok" && check.Message != "" {
			line += "  " + styles.body.Render(compactDoctorMessage(check.Message))
		}
		lines = append(lines, line)
	}
	result := "ok"
	resultStyle := styles.accent
	if !report.OK {
		result = "issues found"
		resultStyle = styles.errorText
	}
	lines = append(lines, "", styles.muted.Render("Result: ")+resultStyle.Render(result))
	width := doctorPanelWidth(report)
	if width > 0 {
		border = border.Width(width)
	}
	return border.Render(strings.Join(lines, "\n"))
}

func doctorStatusStyle(styles wizardStyles, status string) interface{ Render(...string) string } {
	switch status {
	case "ok":
		return styles.accent
	case "warn":
		return styles.placeholder
	default:
		return styles.errorText
	}
}

func doctorPanelWidth(report doctorReport) int {
	maxLine := len("nvim-sandbox > doctor")
	for _, check := range report.Checks {
		message := ""
		if check.Status != "ok" {
			message = compactDoctorMessage(check.Message)
		}
		line := len(fmt.Sprintf("%-11s  %-5s  %s", check.Name+":", check.Status, message))
		if line > maxLine {
			maxLine = line
		}
	}
	maxLine = maxInt(maxLine, len("Result: issues found"))
	if maxLine < 72 {
		return maxLine
	}
	return 72
}

func compactDoctorMessage(message string) string {
	const limit = 56
	message = strings.TrimSpace(message)
	if len(message) <= limit {
		return message
	}
	if limit <= 3 {
		return message[:limit]
	}
	return message[:limit-3] + "..."
}
