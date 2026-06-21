package cli

import (
	"fmt"
	"runtime/debug"
)

var version = "dev"

type versionInfo struct {
	Version string `json:"version"`
	Commit  string `json:"commit,omitempty"`
	Date    string `json:"date,omitempty"`
	Dirty   bool   `json:"dirty"`
}

func currentVersion() versionInfo {
	info := versionInfo{Version: version}
	build, ok := debug.ReadBuildInfo()
	if !ok {
		return info
	}
	if info.Version == "dev" && build.Main.Version != "" && build.Main.Version != "(devel)" {
		info.Version = build.Main.Version
	}
	for _, setting := range build.Settings {
		switch setting.Key {
		case "vcs.revision":
			info.Commit = shortRevision(setting.Value)
		case "vcs.time":
			info.Date = setting.Value
		case "vcs.modified":
			info.Dirty = setting.Value == "true"
		}
	}
	return info
}

func shortRevision(revision string) string {
	const shortLength = 12
	if len(revision) <= shortLength {
		return revision
	}
	return revision[:shortLength]
}

func versionText(info versionInfo) string {
	text := "nvim-sandbox " + info.Version
	if info.Commit == "" {
		return text
	}
	commit := info.Commit
	if info.Dirty {
		commit += "-dirty"
	}
	return fmt.Sprintf("%s (%s)", text, commit)
}
