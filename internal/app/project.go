package app

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
)

var rootMarkers = []string{
	".git",
	"package.json",
	"Cargo.toml",
	"go.mod",
	"pyproject.toml",
	"Dockerfile",
	"Makefile",
}

const maxProjectSlugLength = 80

func ProjectRoot(start string) (string, error) {
	if start == "" {
		var err error
		start, err = os.Getwd()
		if err != nil {
			return "", err
		}
	}

	abs, err := filepath.Abs(start)
	if err != nil {
		return "", err
	}
	info, err := os.Stat(abs)
	if err == nil && !info.IsDir() {
		abs = filepath.Dir(abs)
	}
	resolved, err := filepath.EvalSymlinks(abs)
	if err == nil {
		abs = resolved
	}

	for _, marker := range rootMarkers {
		for current := abs; ; current = filepath.Dir(current) {
			if _, err := os.Stat(filepath.Join(current, marker)); err == nil {
				resolved, resolveErr := filepath.EvalSymlinks(current)
				if resolveErr == nil {
					return resolved, nil
				}
				return current, nil
			}
			next := filepath.Dir(current)
			if next == current {
				break
			}
		}
	}

	return abs, nil
}

func WorkspaceID(root string) string {
	sum := sha256.Sum256([]byte(root))
	return hex.EncodeToString(sum[:])
}

func ProjectImageRepository(root, workspaceIDShort string) string {
	name := strings.ToLower(filepath.Base(filepath.Clean(root)))
	var slug strings.Builder
	separator := false
	for _, char := range name {
		if char >= 'a' && char <= 'z' || char >= '0' && char <= '9' {
			slug.WriteRune(char)
			separator = false
			continue
		}
		if slug.Len() > 0 && !separator {
			slug.WriteByte('-')
			separator = true
		}
	}
	project := strings.Trim(slug.String(), "-")
	if project == "" {
		project = "project"
	}
	if len(project) > maxProjectSlugLength {
		project = strings.TrimRight(project[:maxProjectSlugLength], "-")
	}
	return "nvim-sandbox/" + project + "-" + workspaceIDShort
}

func ProjectContext(start string) (Context, error) {
	root, err := ProjectRoot(start)
	if err != nil {
		return Context{}, err
	}
	id := WorkspaceID(root)
	short := id[:6]
	return Context{
		ProjectRoot:      root,
		WorkspaceID:      id,
		WorkspaceIDShort: short,
		ContainerName:    "sandbox-" + short,
	}, nil
}

func HasDockerfile(root string, cfg Config) bool {
	_, err := os.Stat(filepath.Join(root, cfg.Dockerfile.Filename))
	return err == nil
}

func DetectInstallCommand(imageName string) string {
	if imageName == "" {
		return ""
	}
	name := strings.ToLower(imageName)
	if index := strings.IndexAny(name, ":@"); index >= 0 {
		name = name[:index]
	}
	if slash := strings.LastIndex(name, "/"); slash >= 0 {
		name = name[slash+1:]
	}
	managers := map[string]string{
		"ubuntu": "apt-get",
		"debian": "apt-get",
		"alpine": "apk",
		"fedora": "dnf",
		"centos": "yum",
		"rhel":   "dnf",
		"rocky":  "dnf",
		"alma":   "dnf",
		"arch":   "pacman",
	}
	for base, command := range managers {
		if strings.HasPrefix(name, base) {
			return command
		}
	}
	return ""
}

func DefaultInstallArguments(installCommand string) string {
	switch strings.TrimSpace(installCommand) {
	case "apt", "apt-get":
		return "-y --no-install-recommends"
	case "apk":
		return "--no-cache"
	case "dnf", "yum":
		return "-y"
	default:
		return ""
	}
}
