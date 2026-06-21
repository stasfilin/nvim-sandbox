package app

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
)

type CommandResult struct {
	Code   int
	Stdout string
	Stderr string
}

type Runner func(args []string) CommandResult

type Backend interface {
	Name() string
	Build(ContainerOptions) error
	ImageStatus(image string) (bool, error)
	Create(ContainerOptions) error
	Start(name string) error
	Stop(name string) error
	Destroy(name string) error
	Exec(ContainerOptions, []string) (string, error)
	ConnectArgs(ContainerOptions, []string) []string
	Logs(name string) (string, error)
	Status(name string) (string, error)
}

type RuntimeInfo struct {
	Name  string `json:"name"`
	Label string `json:"label"`
}

type ContainerOptions struct {
	ProjectRoot   string
	ContainerName string
	Image         string
	Workspace     string
	Readonly      bool
	Dockerfile    string
	ExtraMounts   []Mount
	Network       Network
}

var commandRunner Runner = defaultRunner

func SetRunnerForTests(r Runner) {
	if r == nil {
		commandRunner = defaultRunner
		return
	}
	commandRunner = r
}

func defaultRunner(args []string) CommandResult {
	cmd := exec.Command(args[0], args[1:]...)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	code := 0
	if err != nil {
		code = 1
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			code = exitErr.ExitCode()
		}
	}
	return CommandResult{Code: code, Stdout: stdout.String(), Stderr: stderr.String()}
}

func run(args []string) (CommandResult, error) {
	result := commandRunner(args)
	if result.Code == 0 {
		return result, nil
	}
	output := strings.TrimSpace(result.Stderr + "\n" + result.Stdout)
	if output == "" {
		output = "command failed: " + strings.Join(args, " ")
	}
	return result, errors.New(output)
}

func AvailableRuntimes() []RuntimeInfo {
	candidates := []struct {
		executable string
		name       string
		label      string
		macOSOnly  bool
	}{
		{"container", "apple-container", "Apple Container", true},
		{"docker", "docker", "Docker", false},
		{"podman", "podman", "Podman", false},
	}
	var result []RuntimeInfo
	for _, candidate := range candidates {
		if candidate.macOSOnly && runtime.GOOS != "darwin" {
			continue
		}
		if _, err := exec.LookPath(candidate.executable); err == nil {
			result = append(result, RuntimeInfo{Name: candidate.name, Label: candidate.label})
		}
	}
	return result
}

func DetectRuntime() string {
	available := AvailableRuntimes()
	if len(available) == 0 {
		return ""
	}
	return available[0].Name
}

func BackendFor(name string) Backend {
	switch name {
	case "apple-container":
		return appleContainerBackend{}
	case "docker":
		return dockerLikeBackend{binary: "docker", name: "docker"}
	case "podman":
		return dockerLikeBackend{binary: "podman", name: "podman"}
	default:
		return nil
	}
}

func shellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "'\\''") + "'"
}

func RunInteractive(args []string) int {
	cmd := exec.Command(args[0], args[1:]...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return exitErr.ExitCode()
		}
		return 1
	}
	return 0
}

func inspectStatus(text string) string {
	text = strings.TrimSpace(text)
	if text == "" {
		return ""
	}

	var list []map[string]any
	if err := json.Unmarshal([]byte(text), &list); err == nil {
		if len(list) == 0 {
			return ""
		}
		return statusFromMap(list[0])
	}

	var single map[string]any
	if err := json.Unmarshal([]byte(text), &single); err == nil {
		return statusFromMap(single)
	}

	return "exists"
}

func statusFromMap(value map[string]any) string {
	if status, ok := value["status"].(string); ok && status != "" {
		return status
	}
	if state, ok := value["State"].(map[string]any); ok {
		if status, ok := state["Status"].(string); ok && status != "" {
			return status
		}
		if running, ok := state["Running"].(bool); ok && running {
			return "running"
		}
	}
	if status, ok := value["Status"].(string); ok && status != "" {
		return status
	}
	return "exists"
}

func mountSpec(mount Mount) string {
	spec := fmt.Sprintf("type=bind,source=%s,target=%s", mount.Source, mount.Target)
	if mount.Readonly {
		spec += ",readonly"
	}
	return spec
}

func sandboxEnvCommand(command []string) []string {
	if len(command) == 0 {
		command = []string{"nvim"}
	}
	result := []string{"/usr/bin/env", "NVIM_SANDBOX=1"}
	return append(result, command...)
}
