package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/stasfilin/nvim-sandbox/internal/app"
)

type payload map[string]any

func respond(w io.Writer, opts options, code int, value map[string]any, text string) int {
	if opts.format == "json" {
		if value == nil {
			value = map[string]any{}
		}
		value["ok"] = code == 0
		data, _ := json.Marshal(value)
		fmt.Fprintln(w, string(data))
		return code
	}
	if text != "" {
		fmt.Fprintln(w, text)
		return code
	}
	if message, ok := value["message"].(string); ok && message != "" {
		fmt.Fprintln(w, message)
		return code
	}
	if errText, ok := value["error"].(string); ok && errText != "" {
		fmt.Fprintln(w, errText)
	}
	return code
}

func fail(w io.Writer, opts options, err error) int {
	code := 1
	kind := "error"
	message := err.Error()
	if appErr, ok := err.(*app.Error); ok {
		kind = appErr.Kind
		message = appErr.Message
		if appErr.Code != 0 {
			code = appErr.Code
		}
	}
	return respond(w, opts, code, map[string]any{
		"error":   kind,
		"message": message,
	}, message)
}

func humanStatus(status app.Status) string {
	lines := []string{"nvim-sandbox status", ""}
	lines = append(lines, "Project:    "+tilde(status.Context.ProjectRoot))
	lines = append(lines, "Decision:   "+status.Decision)
	if status.Metadata == nil {
		if status.Decision == "enabled" {
			lines = append(lines, "Status:     recovery needed")
			lines = append(lines, "Next:       nvim-sandbox create --recreate")
		} else {
			lines = append(lines, "Status:     no sandbox")
		}
		return strings.Join(lines, "\n")
	}
	lines = append(lines, "Runtime:    "+fallback(status.Runtime, "-"))
	lines = append(lines, "Container:  "+status.ContainerName)
	lines = append(lines, "Image:      "+fallback(status.Image, "-"))
	if status.Metadata.BaseImage != "" {
		lines = append(lines, "Base:       "+status.Metadata.BaseImage)
	}
	packages := []string{}
	if status.Metadata.NeovimVersion != "" {
		packages = append(packages, "neovim@"+status.Metadata.NeovimVersion)
	}
	for _, pkg := range status.Metadata.InstallPackages {
		if pkg != "neovim" {
			packages = append(packages, pkg)
		}
	}
	if len(packages) > 0 {
		lines = append(lines, "Packages:   "+strings.Join(packages, ", "))
	}
	if status.Metadata.InstallEditorTools {
		tools := status.Metadata.EditorTools
		if len(tools) == 0 {
			tools = []string{"gopls", "lua_ls", "rust_analyzer", "terraformls"}
		}
		lines = append(lines, "Tools:      "+strings.Join(tools, ", "))
	}
	source := status.Source
	if source == "default-image" {
		source = "default"
	}
	if source == "dockerfile" {
		source = "Dockerfile"
	}
	lines = append(lines, "Source:     "+fallback(source, "-"))
	if status.Dockerfile != "" {
		lines = append(lines, "Dockerfile: "+status.Dockerfile)
	}
	lines = append(lines, "Status:     "+status.Status)
	lines = append(lines, "Workspace:  "+status.Workspace)
	lines = append(lines, "Mount:      "+status.Mount)
	lines = append(lines, "Stop Exit:  "+yesNo(status.StopOnExit))
	network := status.Metadata.Network
	networkText := "default"
	if !network.Enabled {
		networkText = "disabled"
	} else if network.Name != "" {
		networkText = network.Name
	}
	lines = append(lines, "Network:    "+networkText)
	if len(network.Ports) > 0 {
		lines = append(lines, "Ports:      "+strings.Join(network.Ports, ", "))
	}
	lines = append(lines, "Created At: "+fallback(status.CreatedAt, "-"))
	lines = append(lines, "Last Used:  "+fallback(status.LastUsedAt, "-"))
	return strings.Join(lines, "\n")
}

func yesNo(value bool) string {
	if value {
		return "yes"
	}
	return "no"
}

func humanNetwork(result map[string]any) string {
	network := mapFromAny(result["network"])
	enabled := true
	if value, ok := network["enabled"].(bool); ok {
		enabled = value
	}
	name, _ := network["name"].(string)
	ports := stringSliceFromAny(network["ports"])
	lines := []string{"nvim-sandbox network", ""}
	if enabled {
		lines = append(lines, "Enabled: yes")
	} else {
		lines = append(lines, "Enabled: no")
	}
	if name == "" {
		name = "default"
	}
	lines = append(lines, "Network: "+name)
	if len(ports) > 0 {
		lines = append(lines, "Ports:   "+strings.Join(ports, ", "))
	} else {
		lines = append(lines, "Ports:   none")
	}
	if requires, ok := result["requires_sandbox"].(bool); ok && requires {
		lines = append(lines, "", "No sandbox exists for this project. Run `nvim-sandbox create` first.")
	}
	return strings.Join(lines, "\n")
}

func humanImages(result map[string]any) string {
	images, ok := result["images"].([]app.ImageSummary)
	if !ok {
		if raw, ok := result["images"].([]any); ok {
			for _, item := range raw {
				if imageMap, ok := item.(map[string]any); ok {
					images = append(images, app.ImageSummary{
						Image:           stringFromMap(imageMap, "image"),
						Source:          stringFromMap(imageMap, "source"),
						BaseImage:       stringFromMap(imageMap, "base_image"),
						Runtime:         stringFromMap(imageMap, "runtime"),
						InstallPackages: stringSliceFromAny(imageMap["install_packages"]),
						NeovimVersion:   stringFromMap(imageMap, "neovim_version"),
						Projects:        stringSliceFromAny(imageMap["projects"]),
					})
				}
			}
		}
	}
	if len(images) == 0 {
		return "No managed images found."
	}
	lines := []string{"nvim-sandbox images", ""}
	for _, image := range images {
		lines = append(lines, image.Image)
		if image.BaseImage != "" {
			lines = append(lines, "  base:     "+image.BaseImage)
		}
		packages := []string{}
		if image.NeovimVersion != "" {
			packages = append(packages, "neovim@"+image.NeovimVersion)
		}
		for _, pkg := range image.InstallPackages {
			if pkg != "neovim" {
				packages = append(packages, pkg)
			}
		}
		if len(packages) > 0 {
			lines = append(lines, "  packages: "+strings.Join(packages, ", "))
		}
		if image.Source == "dockerfile" {
			lines = append(lines, "  source:   Dockerfile")
		}
		for index, project := range image.Projects {
			prefix := "             "
			if index == 0 {
				prefix = "  projects: "
			}
			lines = append(lines, prefix+project)
		}
		lines = append(lines, "")
	}
	return strings.Join(lines, "\n")
}

func fallback(value, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}

func tilde(path string) string {
	home, err := os.UserHomeDir()
	if err == nil && home != "" && strings.HasPrefix(path, home) {
		return "~" + strings.TrimPrefix(path, home)
	}
	return path
}

func payloadFrom(value any) map[string]any {
	data, _ := json.Marshal(value)
	var out map[string]any
	_ = json.Unmarshal(data, &out)
	if out == nil {
		out = map[string]any{}
	}
	return out
}

func mapFromAny(value any) map[string]any {
	if value == nil {
		return map[string]any{}
	}
	if mapped, ok := value.(map[string]any); ok {
		return mapped
	}
	data, _ := json.Marshal(value)
	var out map[string]any
	_ = json.Unmarshal(data, &out)
	if out == nil {
		out = map[string]any{}
	}
	return out
}

func stringSliceFromAny(value any) []string {
	switch typed := value.(type) {
	case []string:
		return typed
	case []any:
		result := []string{}
		for _, item := range typed {
			if text, ok := item.(string); ok {
				result = append(result, text)
			}
		}
		return result
	default:
		return nil
	}
}

func stringFromMap(values map[string]any, key string) string {
	value, _ := values[key].(string)
	return value
}

func usage() string {
	return "usage: nvim-sandbox <command> [--format json]\nRun `nvim-sandbox help` to list commands."
}

func helpText(all bool) string {
	lines := []string{
		"nvim-sandbox",
		"",
		"Usage:",
		"  nvim-sandbox                 Show current project status",
		"  nvim-sandbox <command>       Run a command",
		"  nvim-sandbox <command> --format json",
		"",
		"Commands:",
		"  create               Create a sandbox for the current project.",
		"  status               Show project sandbox status.",
		"  shell                Open an interactive shell inside the sandbox.",
		"  exec -- <cmd>        Run a command inside the sandbox.",
		"  logs                 Show sandbox logs.",
		"  network              Manage network access and published ports.",
		"  connect [-- <cmd>]   Connect with nvim or a custom interactive command.",
		"  update               Check for a newer nvim-sandbox release.",
		"  version              Show CLI version and VCS information.",
	}
	if all {
		lines = append(lines,
			"",
			"Lifecycle:",
			"  open                 Open/start an existing sandbox, or report approval needed.",
			"  attach               Attach/open an existing sandbox.",
			"  stop                 Stop the sandbox container.",
			"  restart              Restart the sandbox container.",
			"  destroy              Destroy the sandbox container.",
			"",
			"Config:",
			"  enable               Mark this project enabled and create a sandbox.",
			"  disable              Ignore this project.",
			"  reset                Remove the saved decision for this project.",
			"",
			"Advanced:",
			"  runtime              Show detected runtime.",
			"  images               List managed sandbox images.",
			"",
			"Create options:",
			"  --source <source>    default-image or dockerfile.",
			"  --dockerfile         Shorthand for --source dockerfile.",
			"  --image <image>      Custom base image.",
			"  --runtime <runtime>  auto, apple-container, docker, or podman.",
			"  --install <a,b,c>    Additional packages.",
			"  --install-command    Package manager command (for example: dnf).",
			"  --install-args       Package manager flags (for example: -y).",
			"  --install-lsp        Install common editor tools.",
			"  --attach-local-vim-config",
			"                       Mount local editor config read-only.",
			"  --connect            Connect after creation.",
			"  --stop-on-exit       Stop the container when the editor exits (default).",
			"  --keep-running       Keep the container running after the editor exits.",
			"  --recreate           Recreate with the supplied profile.",
			"  --no-interactive     Disable interactive UI.",
			"  --format json        Emit machine-readable output.",
			"",
			"Dashboard:",
			"  --path short|full    Project path display (default: short).",
			"",
			"Network:",
			"  network status",
			"  network enable [name]",
			"  network disable",
			"  network port add <host:container>",
			"  network port remove <host:container>",
		)
	} else {
		lines = append(lines, "", "Run `nvim-sandbox help all` for lifecycle, config, and advanced commands.")
	}
	lines = append(lines,
		"",
		"Examples:",
		"  nvim-sandbox create",
		"  nvim-sandbox create --install-lsp --attach-local-vim-config",
		"  nvim-sandbox create --image fedora:latest --install git,rg --install-command dnf --install-args '-y'",
		"  nvim-sandbox create --runtime docker --image ubuntu:24.04",
		"  nvim-sandbox create --recreate --install-lsp --attach-local-vim-config --connect",
		"  nvim-sandbox status --format json",
		"  nvim-sandbox exec -- cargo test",
		"  nvim-sandbox shell",
		"  nvim-sandbox update",
	)
	return strings.Join(lines, "\n")
}
