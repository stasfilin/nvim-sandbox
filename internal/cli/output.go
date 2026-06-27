package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"path/filepath"
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

func humanStatus(status app.Status, pathDisplay string) string {
	lines := []string{"nvim-sandbox status", ""}
	lines = append(lines, "Project:    "+displayPath(status.Context.ProjectRoot, pathDisplay))
	lines = append(lines, "Decision:   "+status.Decision)
	if status.Metadata == nil {
		if status.Decision == "enabled" {
			lines = append(lines, "Status:     recovery needed")
			lines = append(lines, "Next:       nvim-sandbox create --recreate")
		} else if status.Decision == "ignored" {
			lines = append(lines, "Status:     disabled")
			lines = append(lines, "Next:       nvim-sandbox reset")
		} else {
			lines = append(lines, "Status:     no sandbox")
			lines = append(lines, "Next:       nvim-sandbox create")
		}
		return strings.Join(lines, "\n")
	}
	lines = append(lines, "Runtime:    "+fallback(runtimeDisplayName(status.Runtime), fallback(status.Runtime, "-")))
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
		lines = append(lines, "Dockerfile: "+displayPath(status.Dockerfile, pathDisplay))
	}
	lines = append(lines, "Status:     "+status.Status)
	lines = append(lines, "Workspace:  "+status.Workspace)
	lines = append(lines, "Mount:      "+status.Mount)
	lines = append(lines, "Stop on Exit: "+yesNo(status.StopOnExit))
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
	lines = append(lines, "Next:       nvim-sandbox connect")
	return strings.Join(lines, "\n")
}

func readyText(metadata *app.Metadata) string {
	if metadata == nil {
		return "sandbox ready"
	}
	lines := []string{
		"sandbox ready: " + metadata.ContainerName,
		"Runtime:       " + fallback(runtimeDisplayName(metadata.Runtime), fallback(metadata.Runtime, "-")),
		"Image:         " + fallback(metadata.Image, "-"),
		"Next:          nvim-sandbox connect",
	}
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

func humanImages(result map[string]any, pathDisplay string) string {
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
			lines = append(lines, prefix+displayPath(project, pathDisplay))
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

func displayPath(path string, pathDisplay string) string {
	if path == "" {
		return path
	}
	if pathDisplay == "full" {
		return path
	}
	return filepath.Base(filepath.Clean(path))
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
	return "usage: nvim-sandbox [command] [options]\nRun `nvim-sandbox help` to list commands."
}

func helpText(all bool) string {
	lines := []string{
		"nvim-sandbox",
		"",
		"Usage:",
		"  nvim-sandbox                       Open the dashboard in a terminal; otherwise print status.",
		"  nvim-sandbox <command> [options]   Run a command.",
		"  nvim-sandbox help <command>        Show command-specific help.",
		"",
		"Commands:",
		"  create               Create a sandbox for the current project.",
		"  status               Show project sandbox status.",
		"  shell                Open an interactive shell inside the sandbox.",
		"  exec -- <cmd>        Run a command inside the sandbox.",
		"  logs                 Show sandbox logs.",
		"  network              Manage network access and published ports.",
		"  doctor               Check runtime, state, project, and dev binary health.",
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
			"  --lsp-tools <list>   Install selected built-in or npm LSP tools.",
			"  --attach-local-vim-config",
			"                       Mount local editor config read-only.",
			"  --connect            Connect after creation.",
			"  --stop-on-exit       Stop the container when the editor exits (default).",
			"  --keep-running       Keep the container running after the editor exits.",
			"  --recreate           Recreate with the supplied profile.",
			"  --no-interactive     Disable interactive UI.",
			"  --format json        Emit machine-readable output.",
			"",
			"Output:",
			"  --path short|full    Display project paths as names or absolute paths (default: short).",
			"  --json               Shorthand for --format json.",
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

type commandReference struct {
	name     string
	summary  string
	usage    []string
	options  []string
	examples []string
	aliases  []string
}

var commandReferences = []commandReference{
	{
		name:    "create",
		summary: "Create or recreate the current project's sandbox.",
		usage: []string{
			"nvim-sandbox create [options]",
			"nvim-sandbox create --connect [-- <cmd>]",
		},
		options: []string{
			"--runtime auto|apple-container|docker|podman",
			"--dockerfile",
			"--image <image>",
			"--install <a,b,c>",
			"--install-command <cmd>",
			"--install-args <args>",
			"--install-lsp",
			"--lsp-tools <list>",
			"--attach-local-vim-config",
			"--connect",
			"--keep-running",
			"--recreate",
			"--no-interactive",
		},
		examples: []string{
			"nvim-sandbox create",
			"nvim-sandbox create --runtime docker --image ubuntu:24.04",
			"nvim-sandbox create --lsp-tools pyright,bash-language-server,npm:@tailwindcss/language-server",
			"nvim-sandbox create --dockerfile --install-lsp --connect",
		},
		aliases: []string{"create-dockerfile", "enable"},
	},
	{
		name:    "dashboard",
		summary: "Open the interactive project dashboard in a terminal.",
		usage:   []string{"nvim-sandbox", "nvim-sandbox dashboard"},
		examples: []string{
			"nvim-sandbox",
			"nvim-sandbox dashboard --path full",
		},
	},
	{
		name:    "connect",
		summary: "Open Neovim, or run another interactive command, inside the sandbox.",
		usage: []string{
			"nvim-sandbox connect",
			"nvim-sandbox connect -- <cmd>",
			"nvim-sandbox shell",
		},
		examples: []string{
			"nvim-sandbox connect",
			"nvim-sandbox connect -- nvim +'checkhealth'",
			"nvim-sandbox shell",
		},
		aliases: []string{"open", "attach", "shell"},
	},
	{
		name:    "stop",
		summary: "Stop the current project's sandbox container.",
		usage:   []string{"nvim-sandbox stop"},
	},
	{
		name:    "restart",
		summary: "Restart the current project's sandbox container.",
		usage:   []string{"nvim-sandbox restart"},
	},
	{
		name:    "disable",
		summary: "Mark the current project ignored by nvim-sandbox.",
		usage:   []string{"nvim-sandbox disable"},
	},
	{
		name:    "reset",
		summary: "Remove the saved project decision.",
		usage:   []string{"nvim-sandbox reset"},
	},
	{
		name:    "exec",
		summary: "Run a non-interactive command in /workspace.",
		usage:   []string{"nvim-sandbox exec -- <cmd>"},
		examples: []string{
			"nvim-sandbox exec -- go test ./...",
			"nvim-sandbox exec -- npm test",
		},
	},
	{
		name:    "status",
		summary: "Show saved metadata and live runtime state for the current project.",
		usage:   []string{"nvim-sandbox status [--json] [--path short|full]"},
		examples: []string{
			"nvim-sandbox status",
			"nvim-sandbox status --json",
			"nvim-sandbox status --path full",
		},
	},
	{
		name:    "network",
		summary: "Show or change network access and published ports.",
		usage: []string{
			"nvim-sandbox network status",
			"nvim-sandbox network enable [name]",
			"nvim-sandbox network disable",
			"nvim-sandbox network port add <host:container>",
			"nvim-sandbox network port remove <host:container>",
		},
		examples: []string{
			"nvim-sandbox network enable",
			"nvim-sandbox network port add 3000:3000",
			"nvim-sandbox network port remove 3000:3000",
		},
	},
	{
		name:    "destroy",
		summary: "Remove the current project's sandbox container and metadata.",
		usage:   []string{"nvim-sandbox destroy [--yes]"},
		options: []string{
			"--yes, -y",
			"--no-interactive",
		},
		examples: []string{
			"nvim-sandbox destroy",
			"nvim-sandbox destroy --yes",
		},
	},
	{
		name:    "doctor",
		summary: "Check runtime discovery, state writability, current project metadata, and local dev binary freshness.",
		usage:   []string{"nvim-sandbox doctor [--json]"},
		examples: []string{
			"nvim-sandbox doctor",
			"nvim-sandbox doctor --json",
		},
	},
	{
		name:    "runtime",
		summary: "Show the selected runtime and every detected runtime.",
		usage:   []string{"nvim-sandbox runtime [--json]"},
		examples: []string{
			"nvim-sandbox runtime",
			"nvim-sandbox runtime --json",
		},
	},
	{
		name:    "images",
		summary: "List managed images referenced by saved projects.",
		usage:   []string{"nvim-sandbox images [--path short|full]"},
	},
	{
		name:    "logs",
		summary: "Show sandbox logs for the current project.",
		usage:   []string{"nvim-sandbox logs"},
	},
	{
		name:    "update",
		summary: "Check GitHub Releases for a newer nvim-sandbox version.",
		usage:   []string{"nvim-sandbox update [--json]"},
	},
	{
		name:    "version",
		summary: "Show the CLI version and VCS information.",
		usage:   []string{"nvim-sandbox version [--json]"},
		aliases: []string{"--version", "-v"},
	},
}

func commandHelpText(topic string) (string, bool) {
	ref, ok := commandReferenceFor(topic)
	if !ok {
		return "", false
	}
	lines := []string{"nvim-sandbox " + ref.name, "", ref.summary, "", "Usage:"}
	for _, usageLine := range ref.usage {
		lines = append(lines, "  "+usageLine)
	}
	if len(ref.aliases) > 0 {
		lines = append(lines, "", "Aliases:")
		for _, alias := range ref.aliases {
			lines = append(lines, "  "+alias)
		}
	}
	if len(ref.options) > 0 {
		lines = append(lines, "", "Options:")
		for _, option := range ref.options {
			lines = append(lines, "  "+option)
		}
	}
	if len(ref.examples) > 0 {
		lines = append(lines, "", "Examples:")
		for _, example := range ref.examples {
			lines = append(lines, "  "+example)
		}
	}
	lines = append(lines, "", "Global options:", "  --format json, --json", "  --path short|full")
	return strings.Join(lines, "\n"), true
}

func commandReferenceFor(topic string) (commandReference, bool) {
	for _, ref := range commandReferences {
		if ref.name == topic {
			return ref, true
		}
		for _, alias := range ref.aliases {
			if alias == topic {
				return ref, true
			}
		}
	}
	return commandReference{}, false
}

func unknownCommandUsage(command string) string {
	lines := []string{"unknown command: " + command}
	if suggestion := suggestCommand(command); suggestion != "" {
		lines = append(lines, "Did you mean `nvim-sandbox "+suggestion+"`?")
	}
	lines = append(lines, "", usage())
	return strings.Join(lines, "\n")
}

func unknownHelpTopicUsage(topic string) string {
	lines := []string{"unknown help topic: " + topic}
	if suggestion := suggestCommand(topic); suggestion != "" {
		lines = append(lines, "Did you mean `nvim-sandbox help "+suggestion+"`?")
	}
	lines = append(lines, "", "Run `nvim-sandbox help` to list commands.")
	return strings.Join(lines, "\n")
}

func suggestCommand(input string) string {
	best := ""
	bestDistance := 4
	for _, candidate := range knownCommands() {
		distance := levenshtein(input, candidate)
		if distance < bestDistance {
			best = candidate
			bestDistance = distance
		}
	}
	return best
}

func knownCommands() []string {
	commands := []string{"help"}
	for _, ref := range commandReferences {
		commands = append(commands, ref.name)
		commands = append(commands, ref.aliases...)
	}
	return commands
}

func levenshtein(a string, b string) int {
	if a == b {
		return 0
	}
	if a == "" {
		return len(b)
	}
	if b == "" {
		return len(a)
	}
	previous := make([]int, len(b)+1)
	current := make([]int, len(b)+1)
	for j := range previous {
		previous[j] = j
	}
	for i := 1; i <= len(a); i++ {
		current[0] = i
		for j := 1; j <= len(b); j++ {
			cost := 0
			if a[i-1] != b[j-1] {
				cost = 1
			}
			current[j] = min(previous[j]+1, current[j-1]+1, previous[j-1]+cost)
		}
		previous, current = current, previous
	}
	return previous[len(b)]
}
