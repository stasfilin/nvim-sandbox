package cli

import (
	"strings"

	"github.com/stasfilin/nvim-sandbox/internal/app"
)

type options struct {
	command              string
	rest                 []string
	format               string
	pathDisplay          string
	source               string
	image                string
	runtime              string
	installPackages      []string
	installCommand       string
	installArguments     string
	installEditorTools   bool
	attachLocalVimConfig bool
	connect              bool
	stopOnExit           *bool
	forceRecreate        bool
	noInteractive        bool
	yes                  bool
	discovery            string
	explicitCreateFlags  bool
	parseError           string
}

func parse(args []string) options {
	opts := options{
		format:      "text",
		pathDisplay: "short",
		source:      "default-image",
		discovery:   "",
	}
	for i := 0; i < len(args); {
		item := args[i]
		switch item {
		case "--format":
			if !hasValue(args, i) {
				opts.parseError = "missing value for --format"
				return opts
			}
			opts.format = args[i+1]
			i += 2
		case "--json":
			opts.format = "json"
			i++
		case "--path":
			if !hasValue(args, i) {
				opts.parseError = "missing value for --path"
				return opts
			}
			opts.pathDisplay = args[i+1]
			i += 2
		case "--source":
			if !hasValue(args, i) {
				opts.parseError = "missing value for --source"
				return opts
			}
			opts.source = args[i+1]
			opts.explicitCreateFlags = true
			i += 2
		case "--dockerfile":
			opts.source = "dockerfile"
			opts.explicitCreateFlags = true
			i++
		case "--image":
			if !hasValue(args, i) {
				opts.parseError = "missing value for --image"
				return opts
			}
			opts.image = args[i+1]
			opts.explicitCreateFlags = true
			i += 2
		case "--runtime":
			if !hasValue(args, i) {
				opts.parseError = "missing value for --runtime"
				return opts
			}
			opts.runtime = args[i+1]
			opts.explicitCreateFlags = true
			i += 2
		case "--install":
			if !hasValue(args, i) {
				opts.parseError = "missing value for --install"
				return opts
			}
			opts.installPackages = splitList(args[i+1])
			opts.explicitCreateFlags = true
			i += 2
		case "--install-command":
			if !hasValue(args, i) {
				opts.parseError = "missing value for --install-command"
				return opts
			}
			opts.installCommand = args[i+1]
			opts.explicitCreateFlags = true
			i += 2
		case "--install-args", "--install-arguments":
			if !hasValue(args, i) {
				opts.parseError = "missing value for " + item
				return opts
			}
			opts.installArguments = args[i+1]
			opts.explicitCreateFlags = true
			i += 2
		case "--install-editor-tools", "--install-lsp", "--install-lsps":
			opts.installEditorTools = true
			opts.explicitCreateFlags = true
			i++
		case "--attach-local-vim-config", "--attach-local-nvim-config", "--atach-local-vim-config":
			opts.attachLocalVimConfig = true
			opts.explicitCreateFlags = true
			i++
		case "--connect":
			opts.connect = true
			i++
		case "--stop-on-exit":
			value := true
			opts.stopOnExit = &value
			opts.explicitCreateFlags = true
			i++
		case "--keep-running":
			value := false
			opts.stopOnExit = &value
			opts.explicitCreateFlags = true
			i++
		case "--recreate", "--force-recreate":
			opts.forceRecreate = true
			opts.explicitCreateFlags = true
			i++
		case "--no-interactive":
			opts.noInteractive = true
			i++
		case "--yes", "-y":
			opts.yes = true
			i++
		case "--discovery":
			if !hasValue(args, i) {
				opts.parseError = "missing value for --discovery"
				return opts
			}
			opts.discovery = args[i+1]
			i += 2
		case "--":
			opts.rest = append(opts.rest, args[i+1:]...)
			i = len(args)
		default:
			if opts.command == "" {
				opts.command = item
			} else {
				opts.rest = append(opts.rest, item)
			}
			i++
		}
	}
	if opts.format != "text" && opts.format != "json" {
		opts.parseError = "invalid format: " + opts.format + " (expected text or json)"
	}
	if opts.pathDisplay != "short" && opts.pathDisplay != "full" {
		opts.parseError = "invalid path display: " + opts.pathDisplay + " (expected short or full)"
	}
	if opts.source != "default-image" && opts.source != "dockerfile" {
		opts.parseError = "invalid source: " + opts.source + " (expected default-image or dockerfile)"
	}
	if opts.runtime != "" && opts.runtime != "auto" && opts.runtime != "apple-container" && opts.runtime != "docker" && opts.runtime != "podman" {
		opts.parseError = "invalid runtime: " + opts.runtime + " (expected auto, apple-container, docker, or podman)"
	}
	if opts.discovery != "" && opts.discovery != "never" && opts.discovery != "ask" && opts.discovery != "auto" {
		opts.parseError = "invalid discovery mode: " + opts.discovery + " (expected never, ask, or auto)"
	}
	return opts
}

func hasValue(args []string, optionIndex int) bool {
	return optionIndex+1 < len(args)
}

func splitList(value string) []string {
	fields := strings.FieldsFunc(value, func(r rune) bool {
		return r == ',' || r == ' ' || r == '\t' || r == '\n'
	})
	result := []string{}
	for _, field := range fields {
		if field != "" {
			result = append(result, field)
		}
	}
	return result
}

func (o options) createOptions() app.CreateOptions {
	return app.CreateOptions{
		Source:               o.source,
		Image:                o.image,
		Runtime:              o.runtime,
		InstallPackages:      o.installPackages,
		InstallCommand:       o.installCommand,
		InstallArguments:     o.installArguments,
		InstallEditorTools:   o.installEditorTools,
		EditorTools:          nil,
		AttachLocalVimConfig: o.attachLocalVimConfig,
		Connect:              o.connect,
		StopOnExit:           o.stopOnExit,
		ForceRecreate:        o.forceRecreate,
	}
}

func (o options) positionalError() string {
	if len(o.rest) == 0 {
		return ""
	}
	switch o.command {
	case "", "create", "connect", "exec", "network", "help", "--help", "-h":
		return ""
	default:
		return "unexpected argument for " + o.command + ": " + o.rest[0]
	}
}
