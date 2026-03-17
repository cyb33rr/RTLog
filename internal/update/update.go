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
	GitHubRepo   = "cyb33rr/rtlog"
	GitHubAPIURL = "https://api.github.com/repos/" + GitHubRepo + "/releases/latest"
	AssetPrefix  = "rtlog-"
)

// Suppress unused import warnings - these are used in later tasks
var (
	_ = io.Copy
)

type Release struct {
	TagName string  `json:"tag_name"`
	Assets  []Asset `json:"assets"`
}

type Asset struct {
	Name        string `json:"name"`
	DownloadURL string `json:"browser_download_url"`
}

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

func WriteLastCheck() {
	path := filepath.Join(rtDir(), "last-update-check")
	os.WriteFile(path, []byte(fmt.Sprintf("%d", time.Now().Unix())), 0644)
}

func ReadUpdateAvailable() string {
	path := filepath.Join(rtDir(), "update-available")
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}

func WriteUpdateAvailable(version string) {
	path := filepath.Join(rtDir(), "update-available")
	os.WriteFile(path, []byte(version), 0644)
}

func ClearUpdateAvailable() {
	path := filepath.Join(rtDir(), "update-available")
	os.Remove(path)
}

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

func IsDevVersion(version string) bool {
	return version == "dev"
}

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
