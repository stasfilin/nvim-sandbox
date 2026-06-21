package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea/v2"

	"github.com/stasfilin/nvim-sandbox/internal/app"
)

func withProject(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	previous, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(root); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(previous) })
	return root
}

func TestStatusJSONForUnknownProject(t *testing.T) {
	root := withProject(t)
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := Run([]string{"status", "--format", "json"}, nil, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("code = %d stderr=%s", code, stderr.String())
	}
	var decoded map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded["ok"] != true {
		t.Fatalf("ok = %#v", decoded["ok"])
	}
	if decoded["decision"] != "unknown" {
		t.Fatalf("decision = %#v", decoded["decision"])
	}
	if decoded["status"] != "-" {
		t.Fatalf("status = %#v", decoded["status"])
	}
	context := decoded["context"].(map[string]any)
	wantRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		t.Fatal(err)
	}
	if context["project_root"] != wantRoot {
		t.Fatalf("project_root = %#v, want %q", context["project_root"], wantRoot)
	}
}

func TestCreateJSONContainsOnlyOneDocument(t *testing.T) {
	withProject(t)
	app.SetRunnerForTests(func(args []string) app.CommandResult {
		if len(args) > 1 && (args[1] == "inspect" || args[1] == "image") {
			return app.CommandResult{Code: 1, Stderr: "not found"}
		}
		return app.CommandResult{Code: 0}
	})
	t.Cleanup(func() { app.SetRunnerForTests(nil) })

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run([]string{"create", "--runtime", "apple-container", "--image", "alpine:3.20", "--no-interactive", "--format", "json"}, nil, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("code=%d stderr=%s", code, stderr.String())
	}
	var decoded map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &decoded); err != nil {
		t.Fatalf("create output is not one JSON document: %v\n%s", err, stdout.String())
	}
	if decoded["action"] != "created" {
		t.Fatalf("create payload = %#v", decoded)
	}
}

func TestVersionAliasesDoNotRequireProjectState(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", "/dev/null")
	for _, command := range []string{"version", "--version", "-v"} {
		var stdout bytes.Buffer
		var stderr bytes.Buffer
		if code := Run([]string{command}, nil, &stdout, &stderr); code != 0 {
			t.Fatalf("%s code=%d stderr=%s", command, code, stderr.String())
		}
		if !strings.HasPrefix(stdout.String(), "nvim-sandbox ") {
			t.Fatalf("%s output = %q", command, stdout.String())
		}
	}
}

func TestVersionJSON(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	if code := Run([]string{"version", "--format", "json"}, nil, &stdout, &stderr); code != 0 {
		t.Fatalf("code=%d stderr=%s", code, stderr.String())
	}
	var decoded map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded["ok"] != true || decoded["action"] != "version" || decoded["version"] == "" {
		t.Fatalf("version payload = %#v", decoded)
	}
}

func TestUpdateCommandShowsUpgradeInstructions(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	previous := updateNowChecker
	updateNowChecker = func(current string, stateBase string) (updateInfo, bool, error) {
		if current == "" || stateBase == "" {
			t.Fatalf("current=%q stateBase=%q", current, stateBase)
		}
		return updateInfo{
			LatestVersion: "0.2.0",
			ReleaseURL:    "https://github.com/stasfilin/nvim-sandbox/releases/tag/v0.2.0",
		}, true, nil
	}
	t.Cleanup(func() { updateNowChecker = previous })

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	if code := Run([]string{"update"}, nil, &stdout, &stderr); code != 0 {
		t.Fatalf("code=%d stderr=%s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "Update available: v0.2.0") || !strings.Contains(stdout.String(), "brew upgrade nvim-sandbox") {
		t.Fatalf("update output = %q", stdout.String())
	}
	if !strings.Contains(stdout.String(), "Release notes: https://github.com/stasfilin/nvim-sandbox/releases/tag/v0.2.0") {
		t.Fatalf("update output does not include release notes: %q", stdout.String())
	}
}

func TestUpdateCommandJSONReportsCurrentVersion(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	previous := updateNowChecker
	updateNowChecker = func(current string, _ string) (updateInfo, bool, error) {
		return updateInfo{LatestVersion: current}, false, nil
	}
	t.Cleanup(func() { updateNowChecker = previous })

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	if code := Run([]string{"update", "--format", "json"}, nil, &stdout, &stderr); code != 0 {
		t.Fatalf("code=%d stderr=%s", code, stderr.String())
	}
	var decoded map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded["action"] != "update" || decoded["update_available"] != false || decoded["current_version"] == "" {
		t.Fatalf("update payload = %#v", decoded)
	}
}

func TestInvalidOptionsReturnUsageError(t *testing.T) {
	tests := [][]string{
		{"status", "--format"},
		{"status", "--format", "yaml"},
		{"status", "unexpected"},
		{"create", "--source", "unknown"},
		{"create", "--runtime"},
		{"create", "--runtime", "lxc"},
		{"open", "--discovery", "sometimes"},
		{"dashboard", "--path"},
		{"dashboard", "--path", "relative"},
	}
	for _, args := range tests {
		var stdout bytes.Buffer
		var stderr bytes.Buffer
		if code := Run(args, nil, &stdout, &stderr); code != 2 {
			t.Fatalf("Run(%#v) code=%d stdout=%s stderr=%s", args, code, stdout.String(), stderr.String())
		}
		if strings.TrimSpace(stderr.String()) == "" {
			t.Fatalf("Run(%#v) returned no usage error", args)
		}
	}
}

func TestDashboardPathDisplayDefaultsToShort(t *testing.T) {
	if got := parse([]string{"dashboard"}).pathDisplay; got != "short" {
		t.Fatalf("default path display = %q, want short", got)
	}
	if got := parse([]string{"dashboard", "--path", "full"}).pathDisplay; got != "full" {
		t.Fatalf("explicit path display = %q, want full", got)
	}

	const root = "/Users/example/Developer/nvim-sandbox"
	if got := displayProjectPath(root, "short"); got != "nvim-sandbox" {
		t.Fatalf("short project path = %q", got)
	}
	if got := displayProjectPath(root, "full"); got != root {
		t.Fatalf("full project path = %q", got)
	}
}

func TestCreateOptionsSelectDockerLikeRuntime(t *testing.T) {
	for _, runtimeName := range []string{"docker", "podman"} {
		t.Run(runtimeName, func(t *testing.T) {
			opts := parse([]string{"create", "--runtime", runtimeName, "--no-interactive"})
			if opts.parseError != "" {
				t.Fatal(opts.parseError)
			}
			if got := opts.createOptions().Runtime; got != runtimeName {
				t.Fatalf("runtime = %q, want %q", got, runtimeName)
			}
		})
	}
}

func TestParseNetworkRejectsInvalidArgumentCounts(t *testing.T) {
	tests := [][]string{
		{"status", "unexpected"},
		{"enable", "network-name", "unexpected"},
		{"disable", "unexpected"},
		{"port", "list", "unexpected"},
		{"port", "add"},
		{"port", "add", "3000:3000", "8080:8080"},
		{"port", "remove"},
	}

	for _, args := range tests {
		if _, _, _, err := parseNetwork(args); err == nil {
			t.Fatalf("parseNetwork(%#v) returned no error", args)
		}
	}
}

func TestShortRevision(t *testing.T) {
	if got, want := shortRevision("1234567890abcdef"), "1234567890ab"; got != want {
		t.Fatalf("shortRevision = %q, want %q", got, want)
	}
}

func TestDashboardChoicesForExistingSandbox(t *testing.T) {
	model := dashboardModel{
		status: app.Status{
			Decision:      "enabled",
			Runtime:       "apple-container",
			ContainerName: "sandbox-123456",
			Image:         "nvim-sandbox-default:ubuntu-24.04",
			Status:        "running",
			Metadata:      &app.Metadata{},
		},
	}
	choices := model.choices()
	if len(choices) != 4 {
		t.Fatalf("choices = %#v", choices)
	}
	if choices[0].value != "connect" || choices[1].value != "recreate" || choices[2].value != "new" {
		t.Fatalf("choices = %#v", choices)
	}
}

func TestInteractiveViewsClearFinalInlineFrame(t *testing.T) {
	dashboard := dashboardModel{action: "connect"}
	if view := dashboard.View(); fmt.Sprint(view.Layer) != "" || view.AltScreen {
		t.Fatalf("completed dashboard view = %#v", view)
	}
	wizard := wizardModel{step: stepDone}
	if view := wizard.View(); fmt.Sprint(view.Layer) != "" || view.AltScreen {
		t.Fatalf("completed wizard view = %#v", view)
	}
}

func TestConnectPromptDefaultsToYes(t *testing.T) {
	model := connectPromptModel{ctx: app.Context{ProjectRoot: "/tmp/project"}}
	next, command := model.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter}))
	updated := next.(connectPromptModel)
	if !updated.done || !updated.connect || command == nil {
		t.Fatalf("connect prompt result done=%v connect=%v command=%v", updated.done, updated.connect, command)
	}
	if view := updated.View(); fmt.Sprint(view.Layer) != "" || view.AltScreen {
		t.Fatalf("completed connect prompt view = %#v", view)
	}
}

func TestConnectPromptCanDecline(t *testing.T) {
	model := connectPromptModel{}
	next, _ := model.Update(tea.KeyPressMsg(tea.Key{Text: "n"}))
	updated := next.(connectPromptModel)
	if !updated.done || updated.connect {
		t.Fatalf("connect prompt result done=%v connect=%v", updated.done, updated.connect)
	}
}

func TestConnectPromptStaysCompactForInlineRendering(t *testing.T) {
	model := connectPromptModel{ctx: app.Context{ProjectRoot: "/a/very/long/project/path"}}
	rendered := fmt.Sprint(model.View().Layer)
	if lines := strings.Count(rendered, "\n") + 1; lines > 6 {
		t.Fatalf("connect prompt uses %d lines:\n%s", lines, rendered)
	}
	if !strings.Contains(rendered, "Connect to nvim now?") || !strings.Contains(rendered, "No, return to shell") {
		t.Fatalf("connect prompt is incomplete:\n%s", rendered)
	}
}

func TestDestroyPromptDefaultsToCancel(t *testing.T) {
	model := destroyPromptModel{status: app.Status{ContainerName: "sandbox-123456"}}
	next, command := model.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter}))
	updated := next.(destroyPromptModel)
	if !updated.done || updated.confirmed || command == nil {
		t.Fatalf("destroy prompt done=%v confirmed=%v command=%v", updated.done, updated.confirmed, command)
	}
}

func TestDestroyPromptCanConfirm(t *testing.T) {
	model := destroyPromptModel{}
	next, command := model.Update(tea.KeyPressMsg(tea.Key{Text: "y"}))
	updated := next.(destroyPromptModel)
	if !updated.done || !updated.confirmed || command == nil {
		t.Fatalf("destroy prompt done=%v confirmed=%v command=%v", updated.done, updated.confirmed, command)
	}
}

func TestDestroyMissingSandboxReturnsError(t *testing.T) {
	withProject(t)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run([]string{"destroy", "--yes", "--format", "json"}, nil, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("code=%d stdout=%s stderr=%s", code, stdout.String(), stderr.String())
	}
	var decoded map[string]any
	if err := json.Unmarshal(stderr.Bytes(), &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded["error"] != "sandbox-not-found" {
		t.Fatalf("error = %#v", decoded["error"])
	}
}

func TestDashboardCreateNewSwitchesToWizard(t *testing.T) {
	previous := availableRuntimes
	availableRuntimes = func() []app.RuntimeInfo {
		return []app.RuntimeInfo{{Name: "docker", Label: "Docker"}}
	}
	t.Cleanup(func() { availableRuntimes = previous })

	model := dashboardModel{
		cfg:         app.DefaultConfig(),
		pathDisplay: "full",
		status: app.Status{
			Context:  app.Context{ProjectRoot: "/tmp/project"},
			Metadata: &app.Metadata{},
		},
	}
	next, cmd := model.applyChoice("new")
	updated := next.(dashboardModel)
	if cmd != nil {
		t.Fatal("new action should switch to wizard without quitting")
	}
	if updated.mode != "wizard" {
		t.Fatalf("mode = %q, want wizard", updated.mode)
	}
	if updated.action != "" {
		t.Fatalf("action = %q, want empty until wizard completes", updated.action)
	}
	if updated.wizard.pathDisplay != "full" {
		t.Fatalf("wizard path display = %q, want full", updated.wizard.pathDisplay)
	}
}

func TestWizardAlwaysAsksForRuntime(t *testing.T) {
	previous := availableRuntimes
	availableRuntimes = func() []app.RuntimeInfo {
		return []app.RuntimeInfo{{Name: "apple-container", Label: "Apple Container"}}
	}
	t.Cleanup(func() { availableRuntimes = previous })

	model, err := newWizardModel(app.DefaultConfig(), app.Context{ProjectRoot: "/tmp/project"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if model.step != stepRuntime {
		t.Fatalf("step = %v, want runtime selection", model.step)
	}
	if model.result.Runtime != "" {
		t.Fatalf("runtime was selected without user input: %q", model.result.Runtime)
	}
}

func TestWizardProjectPathDisplay(t *testing.T) {
	previous := availableRuntimes
	availableRuntimes = func() []app.RuntimeInfo {
		return []app.RuntimeInfo{{Name: "docker", Label: "Docker"}}
	}
	t.Cleanup(func() { availableRuntimes = previous })

	const root = "/Users/example/Developer/nvim-sandbox"
	model, err := newWizardModelWithPath(app.DefaultConfig(), app.Context{ProjectRoot: root}, nil, "full")
	if err != nil {
		t.Fatal(err)
	}
	if model.pathDisplay != "full" {
		t.Fatalf("path display = %q, want full", model.pathDisplay)
	}
	if rendered := fmt.Sprint(model.View().Layer); !strings.Contains(rendered, root) {
		t.Fatalf("wizard view does not contain full project path: %q", rendered)
	}
}

func TestDashboardChoicesForMissingSandbox(t *testing.T) {
	model := dashboardModel{
		status: app.Status{
			Decision: "unknown",
			Status:   "-",
		},
	}
	choices := model.choices()
	if len(choices) != 2 {
		t.Fatalf("choices = %#v", choices)
	}
	if choices[0].value != "create" || choices[1].value != "cancel" {
		t.Fatalf("choices = %#v", choices)
	}
}

func TestDashboardDetailsIncludeVersionAndProjectState(t *testing.T) {
	previous := availableRuntimes
	availableRuntimes = func() []app.RuntimeInfo {
		return []app.RuntimeInfo{{Name: "apple-container", Label: "Apple Container"}, {Name: "docker", Label: "Docker"}}
	}
	t.Cleanup(func() { availableRuntimes = previous })

	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "Dockerfile"), []byte("FROM scratch\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	model := dashboardModel{
		cfg: app.DefaultConfig(),
		status: app.Status{
			Context:  app.Context{ProjectRoot: root},
			Decision: "unknown",
			Status:   "-",
		},
	}
	details := map[string]string{}
	for _, detail := range model.details() {
		details[detail.label] = detail.value
	}
	if details["CLI"] == "" {
		t.Fatal("dashboard CLI version is empty")
	}
	if details["Runtime"] != "choose during creation" || details["Available"] != "Apple Container, Docker" {
		t.Fatalf("runtime details = %#v", details)
	}
	if details["Dockerfile"] != "yes" || details["Decision"] != "unknown" || details["State"] != "not created" {
		t.Fatalf("dashboard details = %#v", details)
	}
}

func TestDashboardShowsAvailableUpdate(t *testing.T) {
	model := dashboardModel{status: app.Status{Status: "-"}}
	next, command := model.Update(updateCheckMsg{info: updateInfo{LatestVersion: "0.2.0"}})
	if command != nil {
		t.Fatalf("update message returned command %#v", command)
	}
	updated := next.(dashboardModel)
	details := map[string]string{}
	for _, detail := range updated.details() {
		details[detail.label] = detail.value
	}
	if details["Update"] != "v0.2.0 available — brew upgrade nvim-sandbox" {
		t.Fatalf("update detail = %q", details["Update"])
	}
}

func TestOpenAskModeRequiresApprovalForDockerfile(t *testing.T) {
	root := withProject(t)
	if err := os.WriteFile(filepath.Join(root, "Dockerfile"), []byte("FROM ubuntu:24.04\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := Run([]string{"open", "--discovery", "ask", "--format", "json"}, nil, &stdout, &stderr)
	if code != 3 {
		t.Fatalf("code = %d stdout=%s stderr=%s", code, stdout.String(), stderr.String())
	}
	var decoded map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded["error"] != "approval-required" {
		t.Fatalf("error = %#v", decoded["error"])
	}
	if decoded["has_dockerfile"] != true {
		t.Fatalf("has_dockerfile = %#v", decoded["has_dockerfile"])
	}
}

func TestStatusHumanForEnabledProjectWithoutMetadataShowsRecovery(t *testing.T) {
	withProject(t)
	state, err := app.NewState()
	if err != nil {
		t.Fatal(err)
	}
	if err := state.Ensure(); err != nil {
		t.Fatal(err)
	}
	ctx, err := app.ProjectContext("")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := state.WriteDecision(ctx, "enabled"); err != nil {
		t.Fatal(err)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run([]string{"status"}, nil, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("code = %d stdout=%s stderr=%s", code, stdout.String(), stderr.String())
	}
	if !strings.Contains(stdout.String(), "Status:     recovery needed") {
		t.Fatalf("status did not show recovery:\n%s", stdout.String())
	}
}

func TestWizardReviewEnterQuitsWithoutCancel(t *testing.T) {
	model := wizardModel{
		cfg:  app.DefaultConfig(),
		step: stepReview,
		ctx: app.Context{
			ProjectRoot: "/tmp/project",
		},
		result: app.CreateOptions{
			Source: "default-image",
		},
	}
	next, cmd := model.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter}))
	updated := next.(wizardModel)
	if updated.cancelled {
		t.Fatal("review enter marked wizard as cancelled")
	}
	if updated.step != stepDone {
		t.Fatalf("step = %v, want %v", updated.step, stepDone)
	}
	if cmd == nil {
		t.Fatal("review enter did not return a quit command")
	}
}

func TestParseCreateRecreateFlag(t *testing.T) {
	opts := parse([]string{"create", "--recreate", "--install-lsp"})
	createOpts := opts.createOptions()
	if !createOpts.ForceRecreate {
		t.Fatal("--recreate did not set ForceRecreate")
	}
	if !createOpts.InstallEditorTools {
		t.Fatal("--install-lsp did not set InstallEditorTools")
	}
}

func TestParseInstallArgumentsFlag(t *testing.T) {
	opts := parse([]string{"create", "--install", "git,rg", "--install-command", "dnf", "--install-args", "-y --setopt=install_weak_deps=False"})
	createOpts := opts.createOptions()
	if createOpts.InstallCommand != "dnf" {
		t.Fatalf("install command = %q", createOpts.InstallCommand)
	}
	if createOpts.InstallArguments != "-y --setopt=install_weak_deps=False" {
		t.Fatalf("install arguments = %q", createOpts.InstallArguments)
	}
}

func TestParseStopOnExitFlags(t *testing.T) {
	keepRunning := parse([]string{"create", "--keep-running"}).createOptions()
	if keepRunning.StopOnExit == nil || *keepRunning.StopOnExit {
		t.Fatalf("--keep-running stop_on_exit = %#v", keepRunning.StopOnExit)
	}
	stopOnExit := parse([]string{"create", "--stop-on-exit"}).createOptions()
	if stopOnExit.StopOnExit == nil || !*stopOnExit.StopOnExit {
		t.Fatalf("--stop-on-exit stop_on_exit = %#v", stopOnExit.StopOnExit)
	}
}

func TestWizardStopOnExitDefaultsToYes(t *testing.T) {
	model := wizardModel{step: stepConnect}
	model.applyChoice()
	if model.step != stepStopOnExit {
		t.Fatalf("step = %v, want stepStopOnExit", model.step)
	}
	if choices := model.choices(); len(choices) != 2 || choices[0].value != "yes" {
		t.Fatalf("stop-on-exit choices = %#v", choices)
	}
	model.applyChoice()
	if model.step != stepReview || model.result.StopOnExit == nil || !*model.result.StopOnExit {
		t.Fatalf("stop-on-exit result step=%v value=%#v", model.step, model.result.StopOnExit)
	}
}

func TestRunConnectionStopsSandboxAfterEditorExit(t *testing.T) {
	withProject(t)
	service, err := app.NewService(app.DefaultConfig())
	if err != nil {
		t.Fatal(err)
	}
	ctx, err := service.Context()
	if err != nil {
		t.Fatal(err)
	}
	stop := true
	if _, err := service.State.WriteProject(ctx, app.Metadata{
		Runtime:       "apple-container",
		ContainerName: ctx.ContainerName,
		Workspace:     "/workspace",
		StopOnExit:    &stop,
	}); err != nil {
		t.Fatal(err)
	}
	commands := [][]string{}
	app.SetRunnerForTests(func(args []string) app.CommandResult {
		commands = append(commands, append([]string{}, args...))
		return app.CommandResult{Code: 0}
	})
	t.Cleanup(func() { app.SetRunnerForTests(nil) })
	previousRunner := interactiveRunner
	interactiveRunner = func([]string) int { return 0 }
	t.Cleanup(func() { interactiveRunner = previousRunner })

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	if code := runConnection(service, []string{"nvim"}, true, &stdout, &stderr); code != 0 {
		t.Fatalf("code = %d stderr=%s", code, stderr.String())
	}
	if len(commands) != 1 || len(commands[0]) < 2 || commands[0][1] != "stop" {
		t.Fatalf("commands = %#v, want container stop", commands)
	}
	if !strings.Contains(stdout.String(), "Sandbox stopped") {
		t.Fatalf("stdout = %q", stdout.String())
	}
}

func TestShellUsesBashWithPOSIXFallback(t *testing.T) {
	withProject(t)
	service, err := app.NewService(app.DefaultConfig())
	if err != nil {
		t.Fatal(err)
	}
	ctx, err := service.Context()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.State.WriteProject(ctx, app.Metadata{
		Runtime:       "apple-container",
		ContainerName: ctx.ContainerName,
		Image:         "dev:latest",
		Workspace:     "/workspace",
	}); err != nil {
		t.Fatal(err)
	}
	app.SetRunnerForTests(func(args []string) app.CommandResult {
		if len(args) > 1 && args[1] == "inspect" {
			return app.CommandResult{Code: 0, Stdout: `[{"status":"running"}]`}
		}
		return app.CommandResult{Code: 0}
	})
	t.Cleanup(func() { app.SetRunnerForTests(nil) })
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	if code := Run([]string{"shell", "--format", "json"}, nil, &stdout, &stderr); code != 0 {
		t.Fatalf("code=%d stderr=%s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "exec /bin/bash") || !strings.Contains(stdout.String(), "exec /bin/sh") {
		t.Fatalf("shell args do not include fallback: %s", stdout.String())
	}
}

func TestCreateDockerfileAliasUsesDockerfileSource(t *testing.T) {
	root := withProject(t)
	t.Setenv("PATH", t.TempDir())
	if err := os.WriteFile(filepath.Join(root, "Dockerfile"), []byte("FROM ubuntu:24.04\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run([]string{"create-dockerfile", "--format", "json", "--no-interactive"}, nil, &stdout, &stderr)
	if code == 0 {
		t.Fatalf("expected create to fail without a runtime fake, stdout=%s", stdout.String())
	}
	if !strings.Contains(stderr.String(), "No container runtime found") && !strings.Contains(stderr.String(), "not implemented") {
		t.Fatalf("stderr = %q", stderr.String())
	}
}

func TestWizardInputDefaultIsReplacedOnTyping(t *testing.T) {
	model := wizardModel{
		step:               stepNeovim,
		input:              "stable",
		inputDefaultActive: true,
		result: app.CreateOptions{
			Source: "default-image",
		},
	}
	next, _ := model.Update(tea.KeyPressMsg(tea.Key{Text: "n"}))
	updated := next.(wizardModel)
	if updated.input != "n" {
		t.Fatalf("input = %q, want first typed char to replace default", updated.input)
	}
}

func TestDockerfileWizardInstallsNeovimWhenAccepted(t *testing.T) {
	model := wizardModel{
		step:      stepSource,
		hasDocker: true,
		result:    app.CreateOptions{Source: "default-image"},
		cursor:    1,
	}
	model.applyChoice()
	if model.result.Source != "dockerfile" || model.step != stepNeovimInstall {
		t.Fatalf("source = %q step = %v", model.result.Source, model.step)
	}
	model.applyChoice()
	if model.step != stepNeovim || model.input != "stable" {
		t.Fatalf("step = %v version input = %q", model.step, model.input)
	}
	model.applyInput()
	if model.step != stepEditorToolsInstall || model.result.NeovimVersion != "stable" || !contains(model.result.InstallPackages, "neovim") {
		t.Fatalf("step=%v version=%q packages=%#v", model.step, model.result.NeovimVersion, model.result.InstallPackages)
	}
}

func TestWizardCanSkipNeovimInstallation(t *testing.T) {
	model := wizardModel{
		step:   stepNeovimInstall,
		cursor: 1,
		result: app.CreateOptions{
			NeovimVersion:   "stable",
			InstallPackages: []string{"neovim", "git"},
		},
	}
	model.applyChoice()
	if model.step != stepEditorToolsInstall || model.result.NeovimVersion != "" || contains(model.result.InstallPackages, "neovim") {
		t.Fatalf("step=%v version=%q packages=%#v", model.step, model.result.NeovimVersion, model.result.InstallPackages)
	}
	if summary := strings.Join(model.summaryLines(true), "\n"); strings.Contains(summary, "Neovim:") {
		t.Fatalf("review incorrectly shows Neovim installation:\n%s", summary)
	}
}

func TestWizardInputAllowsQCharacter(t *testing.T) {
	model := wizardModel{
		step:  stepCustomPackages,
		input: "r",
	}
	next, cmd := model.Update(tea.KeyPressMsg(tea.Key{Text: "q"}))
	updated := next.(wizardModel)
	if updated.cancelled {
		t.Fatal("typing q in input cancelled wizard")
	}
	if updated.input != "rq" {
		t.Fatalf("input = %q, want rq", updated.input)
	}
	if cmd != nil {
		t.Fatal("typing q in input returned quit command")
	}
}

func TestWizardNativePackPluginChoiceMountsSitePack(t *testing.T) {
	model := wizardModel{
		step: stepPluginManager,
		result: app.CreateOptions{
			AttachLocalVimConfig: true,
		},
	}
	model.applyPluginChoice(
		`nvim --headless -c "lua for _, d in ipairs(vim.fn.glob('/root/.local/share/nvim/site/pack/*/start/*/doc', false, true)) do pcall(vim.cmd, 'helptags ' .. d) end" +qa`,
		"Native pack (mount site/pack + helptags)",
	)
	if !model.result.AttachLocalNvimSite {
		t.Fatal("native pack plugin choice did not request site/pack mount")
	}
	if model.result.PluginInstallCommand == "" {
		t.Fatal("native pack plugin choice did not set install command")
	}
	if model.step != stepConnect {
		t.Fatalf("step = %v, want stepConnect", model.step)
	}
}

func TestWizardPackageChecklistKeepsDefaultsAndCustomPrompt(t *testing.T) {
	model := wizardModel{
		step: stepPackages,
		result: app.CreateOptions{
			InstallPackages: []string{"neovim"},
		},
	}
	model.initPackageSelections()
	model.packageSelections["__custom__"] = true
	model.applyMulti()
	if model.step != stepCustomPackages {
		t.Fatalf("step = %v, want stepCustomPackages", model.step)
	}
	if got := model.inputPrompt(); got != "Custom packages (comma separated)" {
		t.Fatalf("input prompt = %q", got)
	}
	for _, pkg := range []string{"neovim", "git", "ripgrep", "fd-find", "curl", "ping", "python3", "python3-pip", "build-essential"} {
		found := false
		for _, got := range model.result.InstallPackages {
			if got == pkg {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("packages = %#v, missing %s", model.result.InstallPackages, pkg)
		}
	}
}

func TestWizardEditorToolChecklistCanDisableOneTool(t *testing.T) {
	model := wizardModel{step: stepEditorTools}
	model.initEditorSelections()
	model.toggleSelection("gopls")
	model.applyMulti()
	if model.step != stepPackages {
		t.Fatalf("step = %v, want stepPackages", model.step)
	}
	if !model.result.InstallEditorTools {
		t.Fatal("expected editor tools to remain enabled")
	}
	if len(model.result.EditorTools) != 1 || model.result.EditorTools[0] != "gopls" {
		t.Fatalf("editor tools = %#v, want only gopls", model.result.EditorTools)
	}
}

func TestWizardEditorToolInstallPromptCanSkipTools(t *testing.T) {
	model := wizardModel{step: stepEditorToolsInstall, cursor: 1}
	model.applyChoice()
	if model.step != stepPackages {
		t.Fatalf("step = %v, want stepPackages", model.step)
	}
	if model.result.InstallEditorTools {
		t.Fatal("expected editor tools to be skipped")
	}
	if len(model.result.EditorTools) != 0 {
		t.Fatalf("editor tools = %#v, want none", model.result.EditorTools)
	}
}

func TestWizardCursorIsClampedAfterCustomPackages(t *testing.T) {
	model := wizardModel{
		cfg:                app.DefaultConfig(),
		step:               stepCustomPackages,
		cursor:             7,
		input:              "htop",
		defaultInstallCmd:  "apt-get",
		defaultInstallArgs: "-y --no-install-recommends",
		result: app.CreateOptions{
			InstallPackages: []string{"neovim", "git"},
		},
	}
	model.applyInput()
	if model.step != stepInstallCommand {
		t.Fatalf("step = %v, want stepInstallCommand", model.step)
	}
	if model.installCommandInput != "apt-get" || model.installArgumentsInput != "-y --no-install-recommends" {
		t.Fatalf("install inputs = %q %q", model.installCommandInput, model.installArgumentsInput)
	}
	model.applyInstallInput()
	if model.step != stepNetwork {
		t.Fatalf("step = %v, want stepNetwork", model.step)
	}
	if model.cursor >= len(model.choices()) {
		t.Fatalf("cursor = %d choices = %d", model.cursor, len(model.choices()))
	}
}

func TestWizardDetectsInstallCommandFromCustomImage(t *testing.T) {
	model := wizardModel{
		step:              stepImage,
		defaultInstallCmd: "apt-get",
		input:             "fedora:latest",
		result: app.CreateOptions{
			Source: "default-image",
		},
	}
	model.applyInput()
	if model.defaultInstallCmd != "dnf" {
		t.Fatalf("defaultInstallCmd = %q, want dnf", model.defaultInstallCmd)
	}
	model.result.InstallPackages = []string{"neovim", "git"}
	model.step = stepPackages
	model.packageSelections = map[string]bool{"git": true}
	model.applyMulti()
	if model.step != stepInstallCommand {
		t.Fatalf("step = %v, want stepInstallCommand", model.step)
	}
	if model.installCommandInput != "dnf" {
		t.Fatalf("install command input = %q, want dnf", model.installCommandInput)
	}
	if model.installArgumentsInput != "-y" {
		t.Fatalf("install arguments input = %q, want -y", model.installArgumentsInput)
	}
	model.applyInstallInput()
	if model.step != stepNetwork || model.result.InstallCommand != "dnf" || model.result.InstallArguments != "-y" {
		t.Fatalf("install result step=%v command=%q arguments=%q", model.step, model.result.InstallCommand, model.result.InstallArguments)
	}
}

func TestWizardInstallScreenEditsBothFields(t *testing.T) {
	model := wizardModel{
		step:                          stepInstallCommand,
		defaultInstallCmd:             "dnf",
		defaultInstallArgs:            "-y",
		installCommandInput:           "dnf",
		installArgumentsInput:         "-y",
		installCommandDefaultActive:   true,
		installArgumentsDefaultActive: true,
	}
	next, _ := model.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyTab}))
	model = next.(wizardModel)
	if model.installField != 1 {
		t.Fatalf("install field = %d, want arguments", model.installField)
	}
	next, _ = model.Update(tea.KeyPressMsg(tea.Key{Text: "-"}))
	model = next.(wizardModel)
	if model.installArgumentsInput != "-" || model.installArgumentsDefaultActive {
		t.Fatalf("arguments input = %q default=%v", model.installArgumentsInput, model.installArgumentsDefaultActive)
	}
	next, _ = model.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter}))
	model = next.(wizardModel)
	if model.step != stepNetwork || model.result.InstallCommand != "dnf" || model.result.InstallArguments != "-" {
		t.Fatalf("install result step=%v command=%q arguments=%q", model.step, model.result.InstallCommand, model.result.InstallArguments)
	}
}

func TestWizardInstallScreenRendersBothFields(t *testing.T) {
	model := wizardModel{
		step:                          stepInstallCommand,
		installCommandInput:           "dnf",
		installArgumentsInput:         "-y",
		installCommandDefaultActive:   true,
		installArgumentsDefaultActive: true,
	}
	rendered := model.renderInstallInput()
	for _, want := range []string{"Package installation", "Install command", "Install arguments", "dnf", "-y"} {
		if !strings.Contains(rendered, want) {
			t.Fatalf("combined install screen missing %q:\n%s", want, rendered)
		}
	}
}

func TestWizardPortInputExplainsAndValidatesMappings(t *testing.T) {
	model := wizardModel{
		step:               stepPorts,
		input:              defaultPublishedPorts,
		inputDefaultActive: true,
		result:             app.CreateOptions{Network: &app.Network{Enabled: true}},
	}
	rendered := model.renderInput()
	for _, want := range []string{"HOST:CONTAINER", "3000:3000", "8080:80"} {
		if !strings.Contains(rendered, want) {
			t.Fatalf("port screen missing %q:\n%s", want, rendered)
		}
	}
	if !strings.Contains(rendered, "(default)") {
		t.Fatalf("port defaults are not identified:\n%s", rendered)
	}
	accepted := model
	accepted.applyInput()
	if accepted.step != stepMount || !slices.Equal(accepted.result.Network.Ports, []string{"3000:3000", "8080:80"}) {
		t.Fatalf("default mappings step=%v ports=%#v", accepted.step, accepted.result.Network.Ports)
	}

	empty := model
	empty.input = ""
	empty.inputDefaultActive = false
	empty.applyInput()
	if empty.step != stepMount || len(empty.result.Network.Ports) != 0 || empty.inputError != "" {
		t.Fatalf("empty ports step=%v ports=%#v error=%q", empty.step, empty.result.Network.Ports, empty.inputError)
	}

	model.input = "invalid"
	model.inputDefaultActive = false
	model.applyInput()
	if model.step != stepPorts || model.inputError == "" {
		t.Fatalf("invalid mapping advanced wizard: step=%v error=%q", model.step, model.inputError)
	}

	model.input = "3000:3000, 8080:80"
	model.applyInput()
	if model.step != stepMount || !slices.Equal(model.result.Network.Ports, []string{"3000:3000", "8080:80"}) {
		t.Fatalf("valid mappings step=%v ports=%#v", model.step, model.result.Network.Ports)
	}
}

func TestWizardOffersNetworkAndPublishedPorts(t *testing.T) {
	rendered := wizardModel{step: stepNetwork}.renderChoices()
	for _, want := range []string{"Published ports require network access", "no host port mappings", "isolated"} {
		if !strings.Contains(rendered, want) {
			t.Fatalf("network screen missing %q:\n%s", want, rendered)
		}
	}

	model := wizardModel{step: stepNetwork}
	model.applyChoice()
	if model.step != stepMount || model.result.Network == nil || !model.result.Network.Enabled {
		t.Fatalf("enabled network step=%v network=%#v", model.step, model.result.Network)
	}
	publish := wizardModel{step: stepNetwork, cursor: 1, inputDefaultActive: true}
	publish.applyChoice()
	if publish.step != stepPorts || !publish.inputDefaultActive || publish.input != defaultPublishedPorts {
		t.Fatalf("publish step=%v input=%q default=%v", publish.step, publish.input, publish.inputDefaultActive)
	}

	disabled := wizardModel{step: stepNetwork, cursor: 2}
	disabled.applyChoice()
	if disabled.step != stepMount || disabled.result.Network == nil || disabled.result.Network.Enabled {
		t.Fatalf("disabled network step=%v network=%#v", disabled.step, disabled.result.Network)
	}
}

func TestWizardReviewAlwaysShowsPublishedPorts(t *testing.T) {
	enabled := wizardModel{result: app.CreateOptions{Network: &app.Network{Enabled: true}}}
	summary := strings.Join(enabled.summaryLines(true), "\n")
	if !strings.Contains(summary, "Network: enabled") || !strings.Contains(summary, "Published ports: none") {
		t.Fatalf("enabled network summary:\n%s", summary)
	}

	disabled := wizardModel{result: app.CreateOptions{Network: &app.Network{Enabled: false}}}
	summary = strings.Join(disabled.summaryLines(true), "\n")
	if !strings.Contains(summary, "Network: disabled") || !strings.Contains(summary, "Published ports: unavailable (network disabled)") {
		t.Fatalf("disabled network summary:\n%s", summary)
	}
}

func TestWizardReviewShowsResolvedInstallPlan(t *testing.T) {
	model := wizardModel{
		step: stepReview,
		result: app.CreateOptions{
			Image:            "fedora:latest",
			InstallCommand:   "dnf",
			InstallArguments: "-y --setopt=install_weak_deps=False",
			InstallPackages:  []string{"neovim", "git", "build-essential"},
		},
	}
	summary := strings.Join(model.summaryLines(true), "\n")
	for _, want := range []string{"Install: dnf", "Arguments: -y --setopt=install_weak_deps=False", "Packages: git, gcc, gcc-c++, make"} {
		if !strings.Contains(summary, want) {
			t.Fatalf("review summary missing %q:\n%s", want, summary)
		}
	}
	if strings.Contains(summary, "build-essential") {
		t.Fatalf("review summary contains unresolved package alias:\n%s", summary)
	}
}

func TestWrapWordsKeepsHyphenatedPackageNamesTogether(t *testing.T) {
	wrapped := strings.Join(wrapWords("git, ripgrep, fd-find, curl, python3-pip, build-essential", 24), "\n")
	if strings.Contains(wrapped, "build-\nessential") || !strings.Contains(wrapped, "build-essential") {
		t.Fatalf("hyphenated package was split:\n%s", wrapped)
	}
}
