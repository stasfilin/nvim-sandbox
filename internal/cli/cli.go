package cli

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/stasfilin/nvim-sandbox/internal/app"
)

var interactiveRunner = app.RunInteractive

func Run(args []string, stdin io.Reader, stdout io.Writer, stderr io.Writer) int {
	opts := parse(args)
	if opts.parseError != "" {
		return respond(stderr, opts, 2, map[string]any{"error": "usage", "message": opts.parseError}, opts.parseError)
	}
	if message := opts.positionalError(); message != "" {
		return respond(stderr, opts, 2, map[string]any{"error": "usage", "message": message}, message)
	}
	if opts.command == "version" || opts.command == "--version" || opts.command == "-v" {
		info := currentVersion()
		return respond(stdout, opts, 0, map[string]any{
			"action":  "version",
			"version": info.Version,
			"commit":  info.Commit,
			"date":    info.Date,
			"dirty":   info.Dirty,
		}, versionText(info))
	}
	cfg := app.DefaultConfig()
	if opts.discovery != "" {
		cfg.Discovery.Mode = opts.discovery
	}
	service, err := app.NewService(cfg)
	if err != nil {
		return fail(stderr, opts, err)
	}
	switch opts.command {
	case "", "dashboard":
		if opts.command == "" && len(opts.rest) > 0 {
			return connect(service, opts, stdout, stderr)
		}
		return statusOrDashboard(service, opts, stdout, stderr)
	case "help", "--help", "-h":
		return respond(stdout, opts, 0, map[string]any{"action": "help"}, helpText(len(opts.rest) > 0 && opts.rest[0] == "all"))
	case "create":
		return create(service, opts, stdout, stderr)
	case "create-dockerfile":
		opts.source = "dockerfile"
		return create(service, opts, stdout, stderr)
	case "open", "attach":
		return open(service, opts, stdout, stderr)
	case "connect":
		return connect(service, opts, stdout, stderr)
	case "shell":
		opts.rest = []string{"/bin/sh", "-lc", "exec /bin/bash 2>/dev/null || exec /bin/sh"}
		return connect(service, opts, stdout, stderr)
	case "exec":
		return execCommand(service, opts, stdout, stderr)
	case "stop":
		return action(service.Stop, opts, stdout, stderr, "sandbox stopped")
	case "restart":
		return action(service.Restart, opts, stdout, stderr, "sandbox restarted")
	case "destroy":
		return destroy(service, opts, stdout, stderr)
	case "enable":
		opts.source = "default-image"
		return create(service, opts, stdout, stderr)
	case "disable":
		return disable(service, opts, stdout, stderr)
	case "reset":
		return action(service.Reset, opts, stdout, stderr, "sandbox decision reset")
	case "status":
		return status(service, opts, stdout, stderr)
	case "logs":
		return logs(service, opts, stdout, stderr)
	case "network":
		return network(service, opts, stdout, stderr)
	case "images":
		return images(service, opts, stdout, stderr)
	case "runtime":
		return respond(stdout, opts, 0, service.Runtime(), runtimeText(service.Runtime()))
	case "update":
		return updateCommand(service.State.Base(), opts, stdout, stderr)
	default:
		return respond(stderr, opts, 2, map[string]any{"error": "usage", "message": usage()}, usage())
	}
}

func updateCommand(stateBase string, opts options, stdout io.Writer, stderr io.Writer) int {
	current := currentVersion().Version
	latest, available, err := updateNowChecker(current, stateBase)
	if err != nil {
		return fail(stderr, opts, fmt.Errorf("update check failed: %w", err))
	}
	payload := map[string]any{
		"action":           "update",
		"current_version":  current,
		"latest_version":   latest.LatestVersion,
		"update_available": available,
		"release_url":      latest.ReleaseURL,
	}
	if !available {
		return respond(stdout, opts, 0, payload, "nvim-sandbox v"+current+" is up to date.")
	}
	payload["command"] = "brew upgrade nvim-sandbox"
	text := fmt.Sprintf(
		"Update available: v%s (current v%s)\nRun `brew upgrade nvim-sandbox`.",
		latest.LatestVersion,
		current,
	)
	if latest.ReleaseURL != "" {
		text += "\nRelease notes: " + latest.ReleaseURL
	}
	return respond(stdout, opts, 0, payload, text)
}

func statusOrDashboard(service *app.Service, opts options, stdout io.Writer, stderr io.Writer) int {
	statusValue, err := service.Status()
	if err != nil {
		return fail(stderr, opts, err)
	}
	if opts.format == "text" && !opts.noInteractive && isTerminal() {
		result, ok, err := runDashboard(service.Config, statusValue, service.State.Base(), opts.pathDisplay)
		if err != nil {
			return fail(stderr, opts, err)
		}
		if !ok || result.action == "cancel" {
			fmt.Fprintln(stdout, "Cancelled.")
			return 130
		}
		switch result.action {
		case "connect":
			return connect(service, opts, stdout, stderr)
		case "create":
			createOpts := result.create
			if statusValue.Metadata != nil {
				createOpts.ForceRecreate = true
			}
			return createWithOptions(service, opts, createOpts, stdout, stderr)
		case "recreate":
			return recreateExisting(service, opts, stdout, stderr)
		}
	}
	text := humanStatus(statusValue, opts.pathDisplay)
	if statusValue.Metadata == nil && opts.format != "json" && opts.noInteractive {
		text += "\n\nRun `nvim-sandbox create` to create a sandbox for this project."
	}
	return respond(stdout, opts, 0, payloadFrom(statusValue), text)
}

func recreateExisting(service *app.Service, opts options, stdout io.Writer, stderr io.Writer) int {
	progress := newProgressDisplay(stdout, true)
	metadata, err := service.Recreate(progress.Update)
	progress.Close()
	if err != nil {
		return fail(stderr, opts, err)
	}
	if metadata.PluginInstallCommand != "" {
		if code, err := installPlugins(service, metadata.PluginInstallCommand, stdout); err != nil {
			fmt.Fprintln(stderr, err.Error())
			return code
		}
	}
	ctx, err := service.Context()
	if err != nil {
		return fail(stderr, opts, err)
	}
	connectNow, err := runConnectPrompt(ctx)
	if err != nil {
		return fail(stderr, opts, err)
	}
	if !connectNow {
		return respond(stdout, opts, 0, map[string]any{"action": "recreated", "metadata": metadata}, "sandbox ready: "+metadata.ContainerName)
	}
	args, err := service.ConnectArgs([]string{"nvim"})
	if err != nil {
		return fail(stderr, opts, err)
	}
	return runConnection(service, args, app.ShouldStopOnExit(metadata), stdout, stderr)
}

func installPlugins(service *app.Service, command string, stdout io.Writer) (int, error) {
	fmt.Fprintln(stdout, "→ Installing plugins...")
	args, err := service.ConnectArgs([]string{"/bin/sh", "-lc", command})
	if err != nil {
		return 1, err
	}
	code := interactiveRunner(args)
	if code != 0 {
		return code, fmt.Errorf("plugin install failed (exit %d)", code)
	}
	fmt.Fprintln(stdout, "ok Plugins installed.")
	return 0, nil
}

func create(service *app.Service, opts options, stdout io.Writer, stderr io.Writer) int {
	createOpts := opts.createOptions()
	if opts.format == "text" && !opts.noInteractive && !opts.explicitCreateFlags && isTerminal() {
		ctx, err := service.Context()
		if err != nil {
			return fail(stderr, opts, err)
		}
		existing, _ := service.State.ReadProject(ctx.WorkspaceID)
		wizardOpts, ok, err := runWizard(service.Config, ctx, existing, opts.pathDisplay)
		if err != nil {
			return fail(stderr, opts, err)
		}
		if !ok {
			fmt.Fprintln(stdout, "Cancelled.")
			return 130
		}
		createOpts = wizardOpts
	}
	if opts.forceRecreate {
		createOpts.ForceRecreate = true
	}
	return createWithOptions(service, opts, createOpts, stdout, stderr)
}

func createWithOptions(service *app.Service, opts options, createOpts app.CreateOptions, stdout io.Writer, stderr io.Writer) int {
	progress := newProgressDisplay(stdout, opts.format == "text" && isTerminal())
	defer progress.Close()
	createOpts.Progress = progress.Update
	if opts.format == "json" {
		createOpts.Progress = func(string, bool) {}
	}
	metadata, err := service.Create(createOpts)
	if err != nil {
		progress.Close()
		return fail(stderr, opts, err)
	}
	if createOpts.PluginInstallCommand != "" && opts.format == "text" {
		if code, err := installPlugins(service, createOpts.PluginInstallCommand, stdout); err != nil {
			fmt.Fprintln(stderr, err.Error())
			return code
		}
	}
	if createOpts.Connect {
		command := opts.rest
		if len(command) == 0 {
			command = []string{"nvim"}
		}
		args, err := service.ConnectArgs(command)
		if err != nil {
			return fail(stderr, opts, err)
		}
		if opts.format == "json" {
			return respond(stdout, opts, 0, map[string]any{
				"action":       "created-and-connect",
				"metadata":     metadata,
				"connect":      map[string]any{"args": args},
				"stop_on_exit": app.ShouldStopOnExit(metadata),
			}, "")
		}
		return runConnection(service, args, app.ShouldStopOnExit(metadata), stdout, stderr)
	}
	return respond(stdout, opts, 0, map[string]any{"action": "created", "metadata": metadata}, "sandbox ready: "+metadata.ContainerName)
}

func open(service *app.Service, opts options, stdout io.Writer, stderr io.Writer) int {
	result, err := service.Open()
	if err != nil {
		if appErr, ok := err.(*app.Error); ok && appErr.Kind == "approval-required" {
			result["error"] = appErr.Kind
			result["message"] = appErr.Message
			return respond(stdout, opts, appErr.Code, result, appErr.Message)
		}
		return fail(stderr, opts, err)
	}
	text := fmt.Sprint(result["action"])
	if metadata, ok := result["metadata"].(*app.Metadata); ok {
		text = "sandbox ready: " + metadata.ContainerName
	}
	return respond(stdout, opts, 0, result, text)
}

func connect(service *app.Service, opts options, stdout io.Writer, stderr io.Writer) int {
	command := opts.rest
	if len(command) == 0 {
		command = []string{"nvim"}
	}
	args, err := service.ConnectArgs(command)
	if err != nil {
		return fail(stderr, opts, err)
	}
	stopOnExit, err := service.StopOnExitEnabled()
	if err != nil {
		return fail(stderr, opts, err)
	}
	if opts.command == "shell" {
		stopOnExit = false
	}
	if opts.format == "json" {
		return respond(stdout, opts, 0, map[string]any{"action": "connect", "connect": map[string]any{"args": args}, "stop_on_exit": stopOnExit}, "")
	}
	return runConnection(service, args, stopOnExit, stdout, stderr)
}

func runConnection(service *app.Service, args []string, stopOnExit bool, stdout io.Writer, stderr io.Writer) int {
	flushTerminalInput()
	code := interactiveRunner(args)
	flushTerminalInput()
	if !stopOnExit {
		return code
	}
	fmt.Fprintln(stdout, "→ Stopping sandbox after editor exit")
	if _, err := service.Stop(); err != nil {
		fmt.Fprintln(stderr, "Sandbox stop failed: "+err.Error())
		if code == 0 {
			return 1
		}
		return code
	}
	fmt.Fprintln(stdout, "ok Sandbox stopped")
	return code
}

func execCommand(service *app.Service, opts options, stdout io.Writer, stderr io.Writer) int {
	if len(opts.rest) == 0 {
		return respond(stderr, opts, 2, map[string]any{"error": "usage", "message": "usage: nvim-sandbox exec -- <command>"}, "usage: nvim-sandbox exec -- <command>")
	}
	result, err := service.Exec(strings.Join(opts.rest, " "))
	if err != nil {
		return fail(stderr, opts, err)
	}
	text := "Command completed."
	if output, ok := result["output"].(string); ok && output != "" {
		text = output
	}
	return respond(stdout, opts, 0, result, text)
}

func disable(service *app.Service, opts options, stdout io.Writer, stderr io.Writer) int {
	decision, err := service.Disable()
	if err != nil {
		return fail(stderr, opts, err)
	}
	return respond(stdout, opts, 0, map[string]any{"action": "disabled", "decision": decision}, "sandbox disabled for this project")
}

func destroy(service *app.Service, opts options, stdout io.Writer, stderr io.Writer) int {
	confirmed := opts.yes
	if !confirmed && opts.format == "text" && !opts.noInteractive && isTerminal() {
		status, err := service.Status()
		if err != nil {
			return fail(stderr, opts, err)
		}
		if status.Metadata != nil {
			confirmed, err = runDestroyPrompt(status, opts.pathDisplay)
			if err != nil {
				return fail(stderr, opts, err)
			}
			if !confirmed {
				fmt.Fprintln(stdout, "Cancelled.")
				return 130
			}
		}
	}
	result, err := service.Destroy(confirmed)
	if err != nil {
		return fail(stderr, opts, err)
	}
	return respond(stdout, opts, 0, result, "sandbox destroyed")
}

func status(service *app.Service, opts options, stdout io.Writer, stderr io.Writer) int {
	result, err := service.Status()
	if err != nil {
		return fail(stderr, opts, err)
	}
	return respond(stdout, opts, 0, payloadFrom(result), humanStatus(result, opts.pathDisplay))
}

func logs(service *app.Service, opts options, stdout io.Writer, stderr io.Writer) int {
	result, err := service.Logs()
	if err != nil {
		return fail(stderr, opts, err)
	}
	text := "No logs found for this project."
	if logs, ok := result["logs"].(string); ok && logs != "" {
		text = logs
	}
	return respond(stdout, opts, 0, result, text)
}

func network(service *app.Service, opts options, stdout io.Writer, stderr io.Writer) int {
	action, name, port, err := parseNetwork(opts.rest)
	if err != nil {
		return fail(stderr, opts, err)
	}
	progress := func(message string, done bool) {
		if opts.format == "text" && action != "status" {
			prefix := "-> "
			if done {
				prefix = "ok "
			}
			fmt.Fprintln(stdout, prefix+message)
		}
	}
	result, err := service.Network(action, name, port, progress)
	if err != nil {
		return fail(stderr, opts, err)
	}
	text := humanNetwork(result)
	if result["action"] == "network-updated" {
		text = "network updated\n\n" + text
	}
	return respond(stdout, opts, 0, result, text)
}

func parseNetwork(rest []string) (string, string, string, error) {
	action := "status"
	name := ""
	port := ""
	if len(rest) == 0 {
		return action, name, port, nil
	}
	switch rest[0] {
	case "status":
		if len(rest) != 1 {
			return "", "", "", networkUsageError()
		}
	case "enable":
		if len(rest) > 2 {
			return "", "", "", networkUsageError()
		}
		action = "enable"
		if len(rest) > 1 {
			name = rest[1]
		}
	case "disable":
		if len(rest) != 1 {
			return "", "", "", networkUsageError()
		}
		action = "disable"
	case "port":
		if len(rest) == 1 {
			action = "status"
			break
		}
		if rest[1] == "list" {
			if len(rest) != 2 {
				return "", "", "", networkUsageError()
			}
			action = "status"
			break
		}
		if rest[1] == "add" {
			action = "port-add"
		} else if rest[1] == "remove" || rest[1] == "rm" {
			action = "port-remove"
		} else {
			return "", "", "", networkUsageError()
		}
		if len(rest) != 3 {
			return "", "", "", networkUsageError()
		}
		port = rest[2]
	default:
		return "", "", "", networkUsageError()
	}
	return action, name, port, nil
}

func networkUsageError() error {
	return &app.Error{Kind: "invalid-usage", Message: "Invalid network command. Run `nvim-sandbox help all` for usage.", Code: 2}
}

func images(service *app.Service, opts options, stdout io.Writer, stderr io.Writer) int {
	result, err := service.Images()
	if err != nil {
		return fail(stderr, opts, err)
	}
	return respond(stdout, opts, 0, result, humanImages(result, opts.pathDisplay))
}

func action(fn func() (map[string]any, error), opts options, stdout io.Writer, stderr io.Writer, text string) int {
	result, err := fn()
	if err != nil {
		return fail(stderr, opts, err)
	}
	return respond(stdout, opts, 0, result, text)
}

func runtimeText(value map[string]any) string {
	if runtimeName, ok := value["runtime"].(string); ok && runtimeName != "" {
		return "Runtime: " + runtimeName
	}
	return "Runtime: none"
}

func isTerminal() bool {
	info, err := os.Stdin.Stat()
	return err == nil && (info.Mode()&os.ModeCharDevice) != 0
}
