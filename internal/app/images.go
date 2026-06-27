package app

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

const imageProfileVersion = "5"

type ImageProfile struct {
	Name             string
	CacheKey         string
	BaseImage        string
	Image            string
	InstallCommand   string
	InstallArguments string
	Packages         []string
	NeovimVersion    string
	EditorTools      bool
	EditorToolNames  []string
	Managed          bool
}

func NormalizeInstallPackages(installCommand string, packages []string) []string {
	aliases := map[string][]string{}
	switch installCommand {
	case "apt", "apt-get":
		aliases = map[string][]string{
			"fd":   {"fd-find"},
			"nvim": {"neovim"},
			"ping": {"iputils-ping"},
			"rg":   {"ripgrep"},
		}
	case "apk":
		aliases = map[string][]string{
			"build-essential": {"build-base"},
			"nvim":            {"neovim"},
			"ping":            {"iputils"},
			"rg":              {"ripgrep"},
		}
	case "dnf", "yum":
		aliases = map[string][]string{
			"build-essential": {"gcc", "gcc-c++", "make"},
			"nvim":            {"neovim"},
			"ping":            {"iputils"},
			"rg":              {"ripgrep"},
		}
	case "pacman":
		aliases = map[string][]string{
			"build-essential": {"base-devel"},
			"nvim":            {"neovim"},
			"ping":            {"iputils"},
			"python3":         {"python"},
			"rg":              {"ripgrep"},
		}
	}
	result := make([]string, 0, len(packages))
	for _, pkg := range packages {
		if normalized, ok := aliases[pkg]; ok {
			result = appendMissing(result, normalized...)
		} else {
			result = appendMissing(result, pkg)
		}
	}
	return result
}

func ImageProfileFor(cfg Config, image string, packages []string, installCommand string, installArguments string, neovimVersion string, editorTools bool, editorToolNames []string) ImageProfile {
	base := image
	if base == "" {
		base = cfg.Image
	}
	if installCommand == "" {
		installCommand = cfg.DefaultImage.InstallCommand
	}
	if installArguments == "" {
		installArguments = DefaultInstallArguments(installCommand)
		if installCommand == cfg.DefaultImage.InstallCommand && cfg.DefaultImage.InstallArguments != "" {
			installArguments = cfg.DefaultImage.InstallArguments
		}
	}
	editorToolNames = normalizeEditorTools(editorTools, editorToolNames)
	editorTools = len(editorToolNames) > 0
	if len(packages) == 0 && !editorTools && base == cfg.Image && cfg.DefaultImage.Enabled {
		packages = NormalizeInstallPackages(installCommand, cfg.DefaultImage.Packages)
		return ImageProfile{
			Name:             "default",
			CacheKey:         imageProfileKey(base, installCommand, installArguments, packages, neovimVersion, editorToolNames),
			BaseImage:        base,
			Image:            cfg.DefaultImage.Tag,
			InstallCommand:   installCommand,
			InstallArguments: installArguments,
			Packages:         packages,
			Managed:          true,
		}
	}
	if len(packages) == 0 && !editorTools {
		return ImageProfile{
			BaseImage:        base,
			Image:            base,
			InstallCommand:   installCommand,
			InstallArguments: installArguments,
			Packages:         []string{},
			Managed:          false,
		}
	}
	packages = NormalizeInstallPackages(installCommand, packages)
	cacheKey := imageProfileKey(base, installCommand, installArguments, packages, neovimVersion, editorToolNames)
	short := cacheKey[:12]
	return ImageProfile{
		Name:             short,
		CacheKey:         cacheKey,
		BaseImage:        base,
		Image:            "nvim-sandbox-dev:" + short,
		InstallCommand:   installCommand,
		InstallArguments: installArguments,
		Packages:         packages,
		NeovimVersion:    neovimVersion,
		EditorTools:      editorTools,
		EditorToolNames:  editorToolNames,
		Managed:          true,
	}
}

func ScopeImageProfile(profile ImageProfile, projectRoot, workspaceIDShort string) ImageProfile {
	if !profile.Managed || workspaceIDShort == "" {
		return profile
	}
	profile.Image = ProjectImageRepository(projectRoot, workspaceIDShort) + ":" + profile.Name
	return profile
}

func imageProfileKey(base, installCommand, installArguments string, packages []string, neovimVersion string, editorToolNames []string) string {
	key := strings.Join([]string{imageProfileVersion, base, installCommand, installArguments, strings.Join(packages, ","), neovimVersion, strings.Join(editorToolNames, ",")}, "|")
	sum := sha256.Sum256([]byte(key))
	return hex.EncodeToString(sum[:])
}

func EnsureImage(state *State, cfg Config, backend Backend, profile ImageProfile, progress func(string, bool)) (string, error) {
	if progress == nil {
		progress = func(string, bool) {}
	}
	if !profile.Managed {
		progress("Image profile: external image "+profile.Image, true)
		return profile.Image, nil
	}
	buildDir := filepath.Join(state.Base(), "images", profile.Name)
	marker := filepath.Join(buildDir, ".profile")
	exists, _ := backend.ImageStatus(profile.Image)
	cacheValid := profile.Name != "default"
	if data, err := os.ReadFile(marker); err == nil && strings.TrimSpace(string(data)) == profile.CacheKey {
		cacheValid = true
	}
	if exists && cacheValid {
		progress("Image profile: cached image "+profile.Image, true)
		return profile.Image, nil
	}
	progress("Image profile: "+imageBuildPlan(profile), true)
	progress("Building image "+profile.Image, false)
	if err := os.MkdirAll(buildDir, 0o755); err != nil {
		return "", err
	}
	dockerfile := filepath.Join(buildDir, "Dockerfile")
	if err := os.WriteFile(dockerfile, []byte(ImageDockerfile(profile.BaseImage, profile.InstallCommand, profile.InstallArguments, profile.Packages, cfg.Workspace, profile.NeovimVersion, profile.EditorToolNames)), 0o644); err != nil {
		return "", err
	}
	if err := backend.Build(ContainerOptions{ProjectRoot: buildDir, Dockerfile: dockerfile, Image: profile.Image}); err != nil {
		return "", err
	}
	if err := os.WriteFile(marker, []byte(profile.CacheKey+"\n"), 0o644); err != nil {
		return "", err
	}
	progress("Image built: "+profile.Image, true)
	return profile.Image, nil
}

func imageBuildPlan(profile ImageProfile) string {
	parts := []string{"base " + profile.BaseImage}
	if len(profile.Packages) > 0 {
		parts = append(parts, "packages "+strings.Join(profile.Packages, ", "))
	}
	if profile.NeovimVersion != "" {
		parts = append(parts, "neovim "+profile.NeovimVersion)
	}
	if len(profile.EditorToolNames) > 0 {
		parts = append(parts, "LSP tools "+strings.Join(profile.EditorToolNames, ", "))
	}
	return strings.Join(parts, " | ")
}

func ImageDockerfile(baseImage, installCommand, installArguments string, packages []string, workspace string, neovimVersion string, editorToolNames []string) string {
	lines := []string{"FROM " + baseImage, ""}
	lines = append(lines, installLines(installCommand, installArguments, packages, neovimVersion, editorToolNames)...)
	lines = append(lines, "", "WORKDIR "+workspace, "")
	return strings.Join(lines, "\n")
}

func installLines(installCommand, installArguments string, packages []string, neovimVersion string, editorToolNames []string) []string {
	if len(packages) == 0 && len(editorToolNames) == 0 {
		return nil
	}
	if installCommand == "apt" || installCommand == "apt-get" {
		if installArguments == "" {
			installArguments = DefaultInstallArguments(installCommand)
		}
		hasNvim := false
		aptPackages := []string{}
		for _, pkg := range packages {
			if pkg == "neovim" || pkg == "nvim" {
				hasNvim = true
			} else {
				aptPackages = append(aptPackages, pkg)
			}
		}
		if hasNvim {
			for _, required := range []string{"ca-certificates", "curl", "tar"} {
				if !slices.Contains(aptPackages, required) {
					aptPackages = append(aptPackages, required)
				}
			}
		}
		if len(editorToolNames) > 0 {
			requiredPackages := []string{"ca-certificates", "curl", "gzip", "python3", "tar"}
			if hasAnyEditorTool(editorToolNames, "gopls", "terraformls") {
				requiredPackages = append(requiredPackages, "git", "golang-go")
			}
			if hasNPMEditorTool(editorToolNames) {
				requiredPackages = append(requiredPackages, "nodejs", "npm")
			}
			for _, required := range requiredPackages {
				if !slices.Contains(aptPackages, required) {
					aptPackages = append(aptPackages, required)
				}
			}
		}
		lines := []string{
			"ENV DEBIAN_FRONTEND=noninteractive",
			"",
			"RUN apt-get update \\",
			"  && echo 'nvim-sandbox: installing packages' \\",
			"  && " + installCommand + " install " + installArguments + " " + strings.Join(aptPackages, " ") + " \\",
			"  && rm -rf /var/lib/apt/lists/*",
		}
		if hasNvim {
			if neovimVersion == "" {
				neovimVersion = "stable"
			}
			lines = append(lines,
				"",
				"RUN set -eux; \\",
				"  echo 'nvim-sandbox: installing Neovim'; \\",
				`  arch="$(uname -m)"; \`,
				`  case "$arch" in \`,
				`    x86_64) nvim_arch="x86_64" ;; \`,
				`    aarch64|arm64) nvim_arch="arm64" ;; \`,
				`    *) echo "unsupported architecture: $arch" >&2; exit 1 ;; \`,
				`  esac; \`,
				`  curl -fsSL "https://github.com/neovim/neovim/releases/download/`+neovimVersion+`/nvim-linux-${nvim_arch}.tar.gz" -o /tmp/nvim.tar.gz; \`,
				"  tar -C /opt -xzf /tmp/nvim.tar.gz; \\",
				`  ln -sf "/opt/nvim-linux-${nvim_arch}/bin/nvim" /usr/local/bin/nvim; \`,
				"  rm -f /tmp/nvim.tar.gz",
			)
		}
		lines = append(lines, editorToolInstallLines(editorToolNames)...)
		return lines
	}
	switch installCommand {
	case "apk":
		if installArguments == "" {
			installArguments = DefaultInstallArguments(installCommand)
		}
		apkPackages := slices.Clone(packages)
		if len(editorToolNames) > 0 {
			apkPackages = appendMissing(apkPackages, "ca-certificates", "curl", "gzip", "python3", "tar")
			if hasAnyEditorTool(editorToolNames, "gopls", "terraformls") {
				apkPackages = appendMissing(apkPackages, "git", "go")
			}
			if hasNPMEditorTool(editorToolNames) {
				apkPackages = appendMissing(apkPackages, "nodejs", "npm")
			}
		}
		lines := []string{"RUN echo 'nvim-sandbox: installing packages' && apk add " + installArguments + " " + strings.Join(apkPackages, " ")}
		lines = append(lines, editorToolInstallLines(editorToolNames)...)
		return lines
	case "dnf", "yum":
		if installArguments == "" {
			installArguments = DefaultInstallArguments(installCommand)
		}
		rpmPackages := slices.Clone(packages)
		if len(editorToolNames) > 0 {
			rpmPackages = appendMissing(rpmPackages, "ca-certificates", "curl", "gzip", "python3", "tar")
			if hasAnyEditorTool(editorToolNames, "gopls", "terraformls") {
				rpmPackages = appendMissing(rpmPackages, "git", "golang")
			}
			if hasNPMEditorTool(editorToolNames) {
				rpmPackages = appendMissing(rpmPackages, "nodejs", "npm")
			}
		}
		lines := []string{
			"RUN echo 'nvim-sandbox: installing packages' \\",
			"  && " + installCommand + " install " + installArguments + " " + strings.Join(rpmPackages, " ") + " \\",
			"  && " + installCommand + " clean all",
		}
		lines = append(lines, editorToolInstallLines(editorToolNames)...)
		return lines
	case "pacman":
		if installArguments == "" {
			installArguments = DefaultInstallArguments(installCommand)
		}
		pacmanPackages := slices.Clone(packages)
		if len(editorToolNames) > 0 {
			pacmanPackages = appendMissing(pacmanPackages, "ca-certificates", "curl", "gzip", "python", "tar")
			if hasAnyEditorTool(editorToolNames, "gopls", "terraformls") {
				pacmanPackages = appendMissing(pacmanPackages, "git", "go")
			}
			if hasNPMEditorTool(editorToolNames) {
				pacmanPackages = appendMissing(pacmanPackages, "nodejs", "npm")
			}
		}
		lines := []string{
			"RUN echo 'nvim-sandbox: installing packages' \\",
			"  && pacman -Sy " + installArguments + " " + strings.Join(pacmanPackages, " ") + " \\",
			"  && pacman -Scc --noconfirm",
		}
		lines = append(lines, editorToolInstallLines(editorToolNames)...)
		return lines
	default:
		if len(editorToolNames) > 0 {
			return []string{"RUN echo 'nvim-sandbox editor tools profile does not support install command: " + installCommand + "' >&2; exit 1"}
		}
		command := strings.TrimSpace(strings.Join([]string{installCommand, installArguments, strings.Join(packages, " ")}, " "))
		return []string{"RUN " + command}
	}
}

func appendMissing(values []string, additions ...string) []string {
	for _, addition := range additions {
		if !slices.Contains(values, addition) {
			values = append(values, addition)
		}
	}
	return values
}

func editorToolInstallLines(editorToolNames []string) []string {
	if len(editorToolNames) == 0 {
		return nil
	}
	lines := []string{}
	if invalid := invalidEditorTools(editorToolNames); len(invalid) > 0 {
		return []string{"RUN echo 'nvim-sandbox editor tools contain invalid package names: " + strings.Join(invalid, ", ") + "' >&2; exit 1"}
	}
	if hasEditorTool(editorToolNames, "gopls") {
		lines = append(lines,
			"",
			"RUN set -eux; \\",
			"  echo 'nvim-sandbox: installing LSP tool gopls'; \\",
			"  GOBIN=/usr/local/bin go install golang.org/x/tools/gopls@latest",
		)
	}
	if hasEditorTool(editorToolNames, "terraformls") {
		lines = append(lines,
			"",
			"RUN set -eux; \\",
			"  echo 'nvim-sandbox: installing LSP tool terraformls'; \\",
			"  GOBIN=/usr/local/bin go install github.com/hashicorp/terraform-ls@latest; \\",
			"  if command -v terraform-ls >/dev/null 2>&1; then ln -sf \"$(command -v terraform-ls)\" /usr/local/bin/terraformls; fi",
		)
	}
	if hasEditorTool(editorToolNames, "lua_ls") {
		lines = append(lines,
			"",
			"RUN set -eux; \\",
			"  echo 'nvim-sandbox: installing LSP tool lua_ls'; \\",
			"  arch=\"$(uname -m)\"; \\",
			"  case \"$arch\" in x86_64) lua_arch=\"x64\" ;; aarch64|arm64) lua_arch=\"arm64\" ;; *) echo \"unsupported architecture: $arch\" >&2; exit 1 ;; esac; \\",
			"  lua_url=\"$(LUA_ARCH=\"${lua_arch}\" python3 -c 'import json, os, urllib.request; suffix=\"linux-\"+os.environ[\"LUA_ARCH\"]+\".tar.gz\"; data=json.load(urllib.request.urlopen(\"https://api.github.com/repos/LuaLS/lua-language-server/releases/latest\")); print(next(a[\"browser_download_url\"] for a in data[\"assets\"] if a[\"name\"].endswith(suffix)))')\"; \\",
			"  mkdir -p /opt/lua-language-server; \\",
			"  curl -fsSL \"$lua_url\" -o /tmp/lua-language-server.tar.gz; \\",
			"  tar -xzf /tmp/lua-language-server.tar.gz -C /opt/lua-language-server; \\",
			"  if [ -x /opt/lua-language-server/bin/lua-language-server ]; then ln -sf /opt/lua-language-server/bin/lua-language-server /usr/local/bin/lua-language-server; ln -sf /opt/lua-language-server/bin/lua-language-server /usr/local/bin/lua_ls; fi; \\",
			"  rm -f /tmp/lua-language-server.tar.gz",
		)
	}
	if hasEditorTool(editorToolNames, "rust_analyzer") {
		lines = append(lines,
			"",
			"RUN set -eux; \\",
			"  echo 'nvim-sandbox: installing LSP tool rust_analyzer'; \\",
			"  arch=\"$(uname -m)\"; \\",
			"  case \"$arch\" in x86_64) rust_arch=\"x86_64-unknown-linux-gnu\" ;; aarch64|arm64) rust_arch=\"aarch64-unknown-linux-gnu\" ;; *) echo \"unsupported architecture: $arch\" >&2; exit 1 ;; esac; \\",
			"  curl -fsSL \"https://github.com/rust-lang/rust-analyzer/releases/latest/download/rust-analyzer-${rust_arch}.gz\" -o /tmp/rust-analyzer.gz; \\",
			"  gzip -dc /tmp/rust-analyzer.gz > /usr/local/bin/rust-analyzer; \\",
			"  chmod +x /usr/local/bin/rust-analyzer; \\",
			"  ln -sf /usr/local/bin/rust-analyzer /usr/local/bin/rust_analyzer; \\",
			"  rm -f /tmp/rust-analyzer.gz",
		)
	}
	if packages := npmEditorToolPackages(editorToolNames); len(packages) > 0 {
		quoted := []string{}
		for _, pkg := range packages {
			quoted = append(quoted, shellQuote(pkg))
		}
		lines = append(lines,
			"",
			"RUN set -eux; \\",
			"  echo 'nvim-sandbox: installing npm LSP tools'; \\",
			"  npm install -g "+strings.Join(quoted, " "),
		)
	}
	return lines
}

func normalizeEditorTools(enabled bool, names []string) []string {
	if !enabled && len(names) == 0 {
		return nil
	}
	if len(names) == 0 {
		return defaultEditorTools()
	}
	normalized := []string{}
	for _, name := range names {
		name = normalizeEditorToolName(name)
		if name != "" && !slices.Contains(normalized, name) {
			normalized = append(normalized, name)
		}
	}
	return normalized
}

func defaultEditorTools() []string {
	return []string{"gopls", "lua_ls", "rust_analyzer", "terraformls"}
}

func normalizeEditorToolName(name string) string {
	name = strings.TrimSpace(name)
	switch name {
	case "":
		return ""
	case "lua-language-server":
		return "lua_ls"
	case "rust-analyzer":
		return "rust_analyzer"
	case "terraform-ls":
		return "terraformls"
	default:
		return name
	}
}

func hasEditorTool(names []string, needle string) bool {
	return slices.Contains(names, needle)
}

func hasAnyEditorTool(names []string, needles ...string) bool {
	for _, needle := range needles {
		if hasEditorTool(names, needle) {
			return true
		}
	}
	return false
}

func hasNPMEditorTool(names []string) bool {
	return len(npmEditorToolPackages(names)) > 0
}

func npmEditorToolPackages(names []string) []string {
	packages := []string{}
	for _, name := range names {
		for _, pkg := range npmPackagesForEditorTool(name) {
			packages = appendMissing(packages, pkg)
		}
	}
	return packages
}

func npmPackagesForEditorTool(name string) []string {
	switch {
	case name == "pyright":
		return []string{"pyright"}
	case name == "bash-language-server":
		return []string{"bash-language-server"}
	case name == "typescript-language-server":
		return []string{"typescript", "typescript-language-server"}
	case name == "vscode-langservers-extracted":
		return []string{"vscode-langservers-extracted"}
	case name == "yaml-language-server":
		return []string{"yaml-language-server"}
	case strings.HasPrefix(name, "npm:"):
		pkg := strings.TrimPrefix(name, "npm:")
		if pkg == "" {
			return nil
		}
		return []string{pkg}
	default:
		if isBuiltinEditorTool(name) {
			return nil
		}
		return []string{name}
	}
}

func invalidEditorTools(names []string) []string {
	invalid := []string{}
	for _, name := range names {
		for _, pkg := range npmPackagesForEditorTool(name) {
			if !validNPMPackageSpec(pkg) {
				invalid = append(invalid, name)
				break
			}
		}
	}
	return invalid
}

func isBuiltinEditorTool(name string) bool {
	switch name {
	case "gopls", "lua_ls", "rust_analyzer", "terraformls":
		return true
	default:
		return false
	}
}

func validNPMPackageSpec(value string) bool {
	if value == "" || strings.HasPrefix(value, "-") {
		return false
	}
	for _, r := range value {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') {
			continue
		}
		switch r {
		case '@', '/', '.', '_', '-', '+', '~':
			continue
		default:
			return false
		}
	}
	return true
}
