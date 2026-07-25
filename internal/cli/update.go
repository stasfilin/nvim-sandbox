package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const (
	latestReleaseURL = "https://api.github.com/repos/stasfilin/nvim-sandbox/releases/latest"
	updateCacheTTL   = 24 * time.Hour
)

var updateHTTPClient = &http.Client{Timeout: 3 * time.Second}
var updateNowChecker = checkForUpdateNow

type updateInfo struct {
	LatestVersion string
	ReleaseURL    string
}

type updateCheckMsg struct {
	info updateInfo
}

type updateCache struct {
	LatestVersion string    `json:"latest_version"`
	ReleaseURL    string    `json:"release_url"`
	CheckedAt     time.Time `json:"checked_at"`
}

type githubRelease struct {
	TagName string `json:"tag_name"`
	HTMLURL string `json:"html_url"`
}

func appendUpdateNotice(text string, currentVersion string, stateBase string) string {
	info := checkForUpdate(currentVersion, stateBase)
	if info.LatestVersion == "" {
		return text
	}
	return text + fmt.Sprintf(
		"\n\nUpdate available: v%s (current v%s) — run `brew upgrade nvim-sandbox`.",
		info.LatestVersion,
		currentVersion,
	)
}

func checkForUpdate(currentVersion string, stateBase string) updateInfo {
	if updateCheckDisabled() {
		return updateInfo{}
	}
	return checkForUpdateWith(updateHTTPClient, latestReleaseURL, currentVersion, stateBase, time.Now())
}

func checkForUpdateWith(client *http.Client, endpoint string, currentVersion string, stateBase string, checkedAt time.Time) updateInfo {
	if _, ok := parseSemanticVersion(currentVersion); !ok {
		return updateInfo{}
	}

	cachePath := filepath.Join(stateBase, "update-check.json")
	cached, _ := readUpdateCache(cachePath)
	if cached != nil && cacheIsFresh(*cached, checkedAt) {
		return availableUpdate(currentVersion, cached.LatestVersion, cached.ReleaseURL)
	}

	latest, available, err := checkForUpdateNowWith(client, endpoint, currentVersion, stateBase, checkedAt)
	if err != nil {
		if cached != nil {
			return availableUpdate(currentVersion, cached.LatestVersion, cached.ReleaseURL)
		}
		return updateInfo{}
	}
	if !available {
		return updateInfo{}
	}
	return latest
}

func checkForUpdateNow(currentVersion string, stateBase string) (updateInfo, bool, error) {
	return checkForUpdateNowWith(updateHTTPClient, latestReleaseURL, currentVersion, stateBase, time.Now())
}

func checkForUpdateNowWith(client *http.Client, endpoint string, currentVersion string, stateBase string, checkedAt time.Time) (updateInfo, bool, error) {
	if _, ok := parseSemanticVersion(currentVersion); !ok {
		return updateInfo{}, false, fmt.Errorf("cannot check updates for development version %q", currentVersion)
	}
	release, err := fetchLatestRelease(client, endpoint, currentVersion)
	if err != nil {
		return updateInfo{}, false, err
	}
	latestVersion := strings.TrimPrefix(release.TagName, "v")
	if _, ok := parseSemanticVersion(latestVersion); !ok {
		return updateInfo{}, false, fmt.Errorf("latest release has invalid version %q", release.TagName)
	}

	latest := updateInfo{LatestVersion: latestVersion, ReleaseURL: release.HTMLURL}
	_ = writeUpdateCache(filepath.Join(stateBase, "update-check.json"), updateCache{
		LatestVersion: latest.LatestVersion,
		ReleaseURL:    latest.ReleaseURL,
		CheckedAt:     checkedAt.UTC(),
	})
	return latest, newerSemanticVersion(latestVersion, currentVersion), nil
}

func fetchLatestRelease(client *http.Client, endpoint string, currentVersion string) (githubRelease, error) {
	request, err := http.NewRequest(http.MethodGet, endpoint, nil)
	if err != nil {
		return githubRelease{}, err
	}
	request.Header.Set("Accept", "application/vnd.github+json")
	request.Header.Set("X-GitHub-Api-Version", "2026-03-10")
	request.Header.Set("User-Agent", "nvim-sandbox/"+currentVersion)

	response, err := client.Do(request)
	if err != nil {
		return githubRelease{}, err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return githubRelease{}, fmt.Errorf("latest release request returned %s", response.Status)
	}

	var release githubRelease
	decoder := json.NewDecoder(io.LimitReader(response.Body, 1<<20))
	if err := decoder.Decode(&release); err != nil {
		return githubRelease{}, err
	}
	if release.TagName == "" {
		return githubRelease{}, errors.New("latest release response has no tag")
	}
	return release, nil
}

func availableUpdate(currentVersion string, latestVersion string, releaseURL string) updateInfo {
	latestVersion = strings.TrimPrefix(latestVersion, "v")
	if !newerSemanticVersion(latestVersion, currentVersion) {
		return updateInfo{}
	}
	return updateInfo{LatestVersion: latestVersion, ReleaseURL: releaseURL}
}

func newerSemanticVersion(candidate string, current string) bool {
	candidateParts, candidateOK := parseSemanticVersion(candidate)
	currentParts, currentOK := parseSemanticVersion(current)
	if !candidateOK || !currentOK {
		return false
	}
	for index := range candidateParts {
		if candidateParts[index] != currentParts[index] {
			return candidateParts[index] > currentParts[index]
		}
	}
	return false
}

func parseSemanticVersion(value string) ([3]int, bool) {
	var parsed [3]int
	value = strings.TrimPrefix(strings.TrimSpace(value), "v")
	parts := strings.Split(value, ".")
	if len(parts) != len(parsed) {
		return parsed, false
	}
	for index, part := range parts {
		if part == "" || (len(part) > 1 && part[0] == '0') {
			return parsed, false
		}
		number, err := strconv.Atoi(part)
		if err != nil || number < 0 {
			return parsed, false
		}
		parsed[index] = number
	}
	return parsed, true
}

func readUpdateCache(path string) (*updateCache, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var cache updateCache
	if err := json.Unmarshal(data, &cache); err != nil {
		return nil, err
	}
	return &cache, nil
}

func writeUpdateCache(path string, cache updateCache) error {
	directory := filepath.Dir(path)
	if err := os.MkdirAll(directory, 0o755); err != nil {
		return err
	}
	data, err := json.Marshal(cache)
	if err != nil {
		return err
	}
	data = append(data, '\n')
	temporary, err := os.CreateTemp(directory, ".update-check-*.tmp")
	if err != nil {
		return err
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)
	if err := temporary.Chmod(0o644); err != nil {
		temporary.Close()
		return err
	}
	if _, err := temporary.Write(data); err != nil {
		temporary.Close()
		return err
	}
	if err := temporary.Close(); err != nil {
		return err
	}
	return os.Rename(temporaryPath, path)
}

func cacheIsFresh(cache updateCache, now time.Time) bool {
	age := now.Sub(cache.CheckedAt)
	return age >= 0 && age < updateCacheTTL
}

func updateCheckDisabled() bool {
	value := strings.TrimSpace(os.Getenv("NVIM_SANDBOX_NO_UPDATE_CHECK"))
	return value == "1" || strings.EqualFold(value, "true")
}
