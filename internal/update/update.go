package update

import (
	"encoding/json"
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
	// GitHubRepo is the GitHub repository path for API calls.
	GitHubRepo = "cyb33rr/rtlog"
	// GitHubAPIURL is the base URL for checking the latest release.
	GitHubAPIURL = "https://api.github.com/repos/" + GitHubRepo + "/releases/latest"
	// AssetPrefix is the prefix for release binary assets.
	AssetPrefix = "rtlog-"
)

// Release represents a GitHub release.
type Release struct {
	TagName string  `json:"tag_name"`
	Assets  []Asset `json:"assets"`
}

// Asset represents a release asset (binary).
type Asset struct {
	Name        string `json:"name"`
	DownloadURL string `json:"browser_download_url"`
}

// FetchLatestRelease fetches the latest release from the given URL.
// Uses a 3-second timeout.
func FetchLatestRelease(url string) (*Release, error) {
	client := &http.Client{Timeout: 3 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return nil, fmt.Errorf("fetching release: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GitHub API returned %d", resp.StatusCode)
	}
	var rel Release
	if err := json.NewDecoder(resp.Body).Decode(&rel); err != nil {
		return nil, fmt.Errorf("parsing release JSON: %w", err)
	}
	return &rel, nil
}

// FindAssetURL finds the download URL for the matching OS/arch asset.
func FindAssetURL(assets []Asset, goos, goarch string) (string, error) {
	target := AssetPrefix + goos + "-" + goarch
	for _, a := range assets {
		if a.Name == target {
			return a.DownloadURL, nil
		}
	}
	var names []string
	for _, a := range assets {
		names = append(names, a.Name)
	}
	return "", fmt.Errorf("no asset found for %s-%s; available: %s", goos, goarch, strings.Join(names, ", "))
}

func rtDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join(".", ".rt")
	}
	dir := filepath.Join(home, ".rt")
	os.MkdirAll(dir, 0700)
	return dir
}

// ShouldCheck returns true if a version check should be performed
// (no check in the last 24 hours).
func ShouldCheck() bool {
	path := filepath.Join(rtDir(), "last-update-check")
	data, err := os.ReadFile(path)
	if err != nil {
		return true
	}
	ts, err := strconv.ParseInt(strings.TrimSpace(string(data)), 10, 64)
	if err != nil {
		return true
	}
	return time.Since(time.Unix(ts, 0)) > 24*time.Hour
}

// WriteLastCheck writes the current timestamp to last-update-check.
func WriteLastCheck() {
	path := filepath.Join(rtDir(), "last-update-check")
	os.WriteFile(path, []byte(fmt.Sprintf("%d", time.Now().Unix())), 0644)
}

// ReadUpdateAvailable reads the version from update-available, or "" if none.
func ReadUpdateAvailable() string {
	path := filepath.Join(rtDir(), "update-available")
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}

// WriteUpdateAvailable writes the available version to update-available.
func WriteUpdateAvailable(version string) {
	path := filepath.Join(rtDir(), "update-available")
	os.WriteFile(path, []byte(version), 0644)
}

// ClearUpdateAvailable removes the update-available file.
func ClearUpdateAvailable() {
	path := filepath.Join(rtDir(), "update-available")
	os.Remove(path)
}

// CompareVersions compares two semver strings.
// Returns -1 if current < latest, 0 if equal, 1 if current > latest.
func CompareVersions(current, latest string) int {
	parse := func(v string) []int {
		v = strings.TrimPrefix(v, "v")
		parts := strings.Split(v, ".")
		nums := make([]int, 3)
		for i := 0; i < len(parts) && i < 3; i++ {
			n, _ := strconv.Atoi(parts[i])
			nums[i] = n
		}
		return nums
	}
	c := parse(current)
	l := parse(latest)
	for i := 0; i < 3; i++ {
		if c[i] < l[i] {
			return -1
		}
		if c[i] > l[i] {
			return 1
		}
	}
	return 0
}

// IsDevVersion returns true if the version string indicates a local dev build.
func IsDevVersion(version string) bool {
	return version == "dev"
}

// IsGoInstalled checks if the binary path is inside GOPATH/bin or GOBIN.
func IsGoInstalled(binPath, gopath, gobin string) bool {
	if gobin != "" && filepath.Dir(binPath) == gobin {
		return true
	}
	if gopath != "" {
		gopathBin := filepath.Join(gopath, "bin")
		if filepath.Dir(binPath) == gopathBin {
			return true
		}
	}
	return false
}

// DownloadBinary downloads a file from url to destPath.
func DownloadBinary(url, destPath string) error {
	client := &http.Client{Timeout: 120 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return fmt.Errorf("downloading binary: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download returned %d", resp.StatusCode)
	}
	f, err := os.Create(destPath)
	if err != nil {
		return fmt.Errorf("creating temp file: %w", err)
	}
	defer f.Close()
	if _, err := io.Copy(f, resp.Body); err != nil {
		return fmt.Errorf("writing binary: %w", err)
	}
	return nil
}

// VerifyBinary checks that the file at path is a valid ELF or Mach-O executable.
func VerifyBinary(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("stat: %w", err)
	}
	if info.Size() == 0 {
		return fmt.Errorf("downloaded file is empty")
	}
	f, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("open: %w", err)
	}
	defer f.Close()
	header := make([]byte, 4)
	if _, err := f.Read(header); err != nil {
		return fmt.Errorf("reading header: %w", err)
	}
	if header[0] == 0x7f && header[1] == 'E' && header[2] == 'L' && header[3] == 'F' {
		return nil
	}
	if header[0] == 0xcf && header[1] == 0xfa && header[2] == 0xed && header[3] == 0xfe {
		return nil
	}
	if header[0] == 0xce && header[1] == 0xfa && header[2] == 0xed && header[3] == 0xfe {
		return nil
	}
	return fmt.Errorf("not a valid ELF or Mach-O binary (header: %x)", header)
}

// ReplaceBinary atomically replaces the binary at destPath with srcPath.
// Preserves the permissions of the original binary.
func ReplaceBinary(srcPath, destPath string) error {
	info, err := os.Stat(destPath)
	if err != nil {
		return fmt.Errorf("stat current binary: %w", err)
	}
	if err := os.Chmod(srcPath, info.Mode().Perm()); err != nil {
		return fmt.Errorf("chmod: %w", err)
	}
	if err := os.Rename(srcPath, destPath); err != nil {
		return fmt.Errorf("rename: %w", err)
	}
	return nil
}

// BackgroundCheck checks for a new version and writes state files.
// Designed to be called synchronously in tests or as a goroutine in production.
func BackgroundCheck(currentVersion, apiURL string) {
	rel, err := FetchLatestRelease(apiURL)
	if err != nil {
		return
	}
	cmp := CompareVersions(currentVersion, rel.TagName)
	if cmp < 0 {
		WriteUpdateAvailable(rel.TagName)
	} else {
		ClearUpdateAvailable()
	}
	WriteLastCheck()
}
