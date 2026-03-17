# Auto-Update Feature Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a self-update mechanism to rtlog that checks GitHub Releases for new versions and can download + replace the binary in-place.

**Architecture:** Two components — (1) a background version checker that runs as a fire-and-forget goroutine on interactive commands and writes state for the next invocation to display, and (2) an `rtlog update` command that fetches and replaces the binary. All logic lives in `internal/update/update.go`, wired via `cmd/update.go` and `cmd/root.go`.

**Tech Stack:** Go standard library only (`net/http`, `encoding/json`, `os`, `runtime`, `path/filepath`, `strconv`, `strings`, `time`, `fmt`, `bufio`). No new dependencies.

**Spec:** `docs/superpowers/specs/2026-03-17-auto-update-design.md`

---

### File Structure

| File | Action | Responsibility |
|------|--------|----------------|
| `internal/update/update.go` | Create | Version comparison, GitHub API client, binary download, self-replace, Go-install detection, state file I/O |
| `internal/update/update_test.go` | Create | Unit tests for all update package functions |
| `cmd/update.go` | Create | Cobra command wiring for `rtlog update` with `--force` flag |
| `cmd/root.go` | Modify | Append background check goroutine to `PersistentPreRunE`, add `PersistentPostRunE` for notification display |

---

### Task 1: Version Comparison

**Files:**
- Create: `internal/update/update.go`
- Create: `internal/update/update_test.go`

- [ ] **Step 1: Write failing tests for version comparison**

Create `internal/update/update_test.go`:

```go
package update

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"
)

func setupTestHome(t *testing.T) (string, func()) {
	t.Helper()
	tmpDir := t.TempDir()
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	os.MkdirAll(filepath.Join(tmpDir, ".rt"), 0700)
	return tmpDir, func() { os.Setenv("HOME", origHome) }
}

func TestCompareVersions(t *testing.T) {
	tests := []struct {
		name    string
		current string
		latest  string
		want    int // -1 = older, 0 = equal, 1 = newer
	}{
		{"equal", "1.0.0", "1.0.0", 0},
		{"patch behind", "1.0.0", "1.0.1", -1},
		{"minor behind", "1.0.0", "1.1.0", -1},
		{"major behind", "1.0.0", "2.0.0", -1},
		{"ahead", "1.1.0", "1.0.0", 1},
		{"v prefix both", "v1.0.0", "v1.0.1", -1},
		{"v prefix one", "v1.0.0", "1.0.1", -1},
		{"two segments", "1.0", "1.0.1", -1},
		{"two segments equal", "1.0", "1.0.0", 0},
		{"malformed", "abc", "1.0.0", -1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CompareVersions(tt.current, tt.latest)
			if got != tt.want {
				t.Errorf("CompareVersions(%q, %q) = %d, want %d", tt.current, tt.latest, got, tt.want)
			}
		})
	}
}

func TestIsDevVersion(t *testing.T) {
	if !IsDevVersion("dev") {
		t.Error("expected dev to be dev version")
	}
	if IsDevVersion("1.0.0") {
		t.Error("expected 1.0.0 to not be dev version")
	}
	if IsDevVersion("v1.0.0") {
		t.Error("expected v1.0.0 to not be dev version")
	}
}
```

**Note:** This file contains ALL imports and helpers upfront. Subsequent tasks append only test functions to this file — no additional `import` blocks.

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd /home/cyb3r/RTLog && go test ./internal/update/...`
Expected: compilation failure — package doesn't exist yet.

- [ ] **Step 3: Implement version comparison**

Create `internal/update/update.go`:

```go
package update

import (
	"strconv"
	"strings"
)

const (
	// GitHubRepo is the GitHub repository path for API calls.
	GitHubRepo = "cyb33rr/rtlog"
	// GitHubAPIURL is the base URL for checking the latest release.
	GitHubAPIURL = "https://api.github.com/repos/" + GitHubRepo + "/releases/latest"
	// AssetPrefix is the prefix for release binary assets.
	AssetPrefix = "rtlog-"
)

// CompareVersions compares two semver strings.
// Returns -1 if current < latest, 0 if equal, 1 if current > latest.
// Strips "v" prefix if present. Handles 2 or 3 segment versions.
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
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd /home/cyb3r/RTLog && go test ./internal/update/... -v`
Expected: all tests PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/update/update.go internal/update/update_test.go
git commit -m "feat(update): add version comparison logic"
```

---

### Task 2: State File I/O (last-update-check, update-available)

**Files:**
- Modify: `internal/update/update.go`
- Modify: `internal/update/update_test.go`

- [ ] **Step 1: Write failing tests for state file operations**

Append these test functions to `internal/update/update_test.go` (imports and helpers are already defined in Task 1):

```go
func TestShouldCheck_NoFile(t *testing.T) {
	_, cleanup := setupTestHome(t)
	defer cleanup()

	if !ShouldCheck() {
		t.Error("expected ShouldCheck to return true when no last-update-check file exists")
	}
}

func TestShouldCheck_RecentCheck(t *testing.T) {
	_, cleanup := setupTestHome(t)
	defer cleanup()

	WriteLastCheck()

	if ShouldCheck() {
		t.Error("expected ShouldCheck to return false after recent check")
	}
}

func TestShouldCheck_OldCheck(t *testing.T) {
	home, cleanup := setupTestHome(t)
	defer cleanup()

	// Write a timestamp from 25 hours ago
	old := time.Now().Add(-25 * time.Hour).Unix()
	path := filepath.Join(home, ".rt", "last-update-check")
	os.WriteFile(path, []byte(strconv.FormatInt(old, 10)), 0644)

	if !ShouldCheck() {
		t.Error("expected ShouldCheck to return true for 25-hour-old check")
	}
}

func TestUpdateAvailable_ReadWrite(t *testing.T) {
	_, cleanup := setupTestHome(t)
	defer cleanup()

	// No file yet
	if v := ReadUpdateAvailable(); v != "" {
		t.Errorf("expected empty, got %q", v)
	}

	WriteUpdateAvailable("v1.2.0")
	if v := ReadUpdateAvailable(); v != "v1.2.0" {
		t.Errorf("expected v1.2.0, got %q", v)
	}

	ClearUpdateAvailable()
	if v := ReadUpdateAvailable(); v != "" {
		t.Errorf("expected empty after clear, got %q", v)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd /home/cyb3r/RTLog && go test ./internal/update/... -v`
Expected: compilation failure — functions not defined.

- [ ] **Step 3: Implement state file I/O**

Add to `internal/update/update.go`:

```go
import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// rtDir returns the path to ~/.rt/, creating it if needed.
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
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd /home/cyb3r/RTLog && go test ./internal/update/... -v`
Expected: all tests PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/update/update.go internal/update/update_test.go
git commit -m "feat(update): add state file I/O for version check throttling"
```

---

### Task 3: GitHub API Client (fetch latest release)

**Files:**
- Modify: `internal/update/update.go`
- Modify: `internal/update/update_test.go`

- [ ] **Step 1: Write failing test for GitHub release parsing**

Append these test functions to `internal/update/update_test.go` (imports already defined in Task 1):

```go
func TestParseRelease(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{
			"tag_name": "v1.2.0",
			"assets": [
				{"name": "rtlog-linux-amd64", "browser_download_url": "https://example.com/rtlog-linux-amd64"},
				{"name": "rtlog-linux-arm64", "browser_download_url": "https://example.com/rtlog-linux-arm64"},
				{"name": "rtlog-darwin-amd64", "browser_download_url": "https://example.com/rtlog-darwin-amd64"},
				{"name": "rtlog-darwin-arm64", "browser_download_url": "https://example.com/rtlog-darwin-arm64"}
			]
		}`)
	}))
	defer server.Close()

	rel, err := FetchLatestRelease(server.URL)
	if err != nil {
		t.Fatalf("FetchLatestRelease failed: %v", err)
	}
	if rel.TagName != "v1.2.0" {
		t.Errorf("tag_name: got %q, want %q", rel.TagName, "v1.2.0")
	}
	if len(rel.Assets) != 4 {
		t.Errorf("assets: got %d, want 4", len(rel.Assets))
	}
}

func TestFindAsset(t *testing.T) {
	assets := []Asset{
		{Name: "rtlog-linux-amd64", DownloadURL: "https://example.com/rtlog-linux-amd64"},
		{Name: "rtlog-darwin-arm64", DownloadURL: "https://example.com/rtlog-darwin-arm64"},
	}

	url, err := FindAssetURL(assets, "linux", "amd64")
	if err != nil {
		t.Fatalf("FindAssetURL failed: %v", err)
	}
	if url != "https://example.com/rtlog-linux-amd64" {
		t.Errorf("got %q, want linux-amd64 URL", url)
	}

	_, err = FindAssetURL(assets, "windows", "amd64")
	if err == nil {
		t.Error("expected error for missing windows asset")
	}
}

func TestFetchLatestRelease_Timeout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(5 * time.Second)
	}))
	defer server.Close()

	_, err := FetchLatestRelease(server.URL)
	if err == nil {
		t.Error("expected timeout error")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd /home/cyb3r/RTLog && go test ./internal/update/... -v`
Expected: compilation failure — types and functions not defined.

- [ ] **Step 3: Implement GitHub API client**

Add to `internal/update/update.go`:

```go
import (
	"encoding/json"
	"net/http"
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
// Returns an error listing available assets if no match is found.
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
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd /home/cyb3r/RTLog && go test ./internal/update/... -v -timeout 10s`
Expected: all tests PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/update/update.go internal/update/update_test.go
git commit -m "feat(update): add GitHub API client for fetching releases"
```

---

### Task 4: Go-Install Detection

**Files:**
- Modify: `internal/update/update.go`
- Modify: `internal/update/update_test.go`

- [ ] **Step 1: Write failing tests for Go-install detection**

Append to `internal/update/update_test.go`:

```go
func TestIsGoInstalled(t *testing.T) {
	tests := []struct {
		name     string
		binPath  string
		gopath   string
		gobin    string
		expected bool
	}{
		{"gopath bin", "/home/user/go/bin/rtlog", "/home/user/go", "", true},
		{"gobin", "/custom/bin/rtlog", "", "/custom/bin", true},
		{"rt dir", "/home/user/.rt/rtlog", "/home/user/go", "", false},
		{"usr local", "/usr/local/bin/rtlog", "/home/user/go", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsGoInstalled(tt.binPath, tt.gopath, tt.gobin)
			if got != tt.expected {
				t.Errorf("IsGoInstalled(%q, %q, %q) = %v, want %v",
					tt.binPath, tt.gopath, tt.gobin, got, tt.expected)
			}
		})
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd /home/cyb3r/RTLog && go test ./internal/update/... -v -run TestIsGoInstalled`
Expected: compilation failure.

- [ ] **Step 3: Implement Go-install detection**

Add to `internal/update/update.go`:

```go
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
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd /home/cyb3r/RTLog && go test ./internal/update/... -v -run TestIsGoInstalled`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/update/update.go internal/update/update_test.go
git commit -m "feat(update): add Go-install detection"
```

---

### Task 5: Binary Download and Self-Replace

**Files:**
- Modify: `internal/update/update.go`
- Modify: `internal/update/update_test.go`

- [ ] **Step 1: Write failing tests for download and replace**

Append to `internal/update/update_test.go`:

```go
func TestDownloadBinary(t *testing.T) {
	content := []byte("\x7fELFtestbinary")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write(content)
	}))
	defer server.Close()

	tmpDir := t.TempDir()
	dest := filepath.Join(tmpDir, "rtlog-new")

	err := DownloadBinary(server.URL, dest)
	if err != nil {
		t.Fatalf("DownloadBinary failed: %v", err)
	}

	data, err := os.ReadFile(dest)
	if err != nil {
		t.Fatalf("reading downloaded file: %v", err)
	}
	if string(data) != string(content) {
		t.Errorf("content mismatch: got %q, want %q", data, content)
	}
}

func TestReplaceBinary(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a "current" binary
	currentPath := filepath.Join(tmpDir, "rtlog")
	os.WriteFile(currentPath, []byte("old"), 0755)

	// Create a "new" binary
	newPath := filepath.Join(tmpDir, "rtlog-new")
	os.WriteFile(newPath, []byte("new"), 0644)

	err := ReplaceBinary(newPath, currentPath)
	if err != nil {
		t.Fatalf("ReplaceBinary failed: %v", err)
	}

	data, _ := os.ReadFile(currentPath)
	if string(data) != "new" {
		t.Errorf("expected 'new', got %q", data)
	}

	// Check permissions were preserved
	info, _ := os.Stat(currentPath)
	if info.Mode().Perm() != 0755 {
		t.Errorf("expected 0755 permissions, got %o", info.Mode().Perm())
	}

	// New file should be cleaned up
	if _, err := os.Stat(newPath); !os.IsNotExist(err) {
		t.Error("expected temp file to be cleaned up")
	}
}

func TestVerifyBinary_Valid(t *testing.T) {
	tmpDir := t.TempDir()

	// ELF header
	elf := filepath.Join(tmpDir, "elf")
	os.WriteFile(elf, []byte("\x7fELFrest"), 0644)
	if err := VerifyBinary(elf); err != nil {
		t.Errorf("ELF should be valid: %v", err)
	}

	// Mach-O header (64-bit)
	macho := filepath.Join(tmpDir, "macho")
	os.WriteFile(macho, []byte{0xcf, 0xfa, 0xed, 0xfe, 0x00}, 0644)
	if err := VerifyBinary(macho); err != nil {
		t.Errorf("Mach-O should be valid: %v", err)
	}
}

func TestVerifyBinary_Invalid(t *testing.T) {
	tmpDir := t.TempDir()
	bad := filepath.Join(tmpDir, "bad")
	os.WriteFile(bad, []byte("not a binary"), 0644)
	if err := VerifyBinary(bad); err == nil {
		t.Error("expected error for non-binary file")
	}
}

func TestVerifyBinary_Empty(t *testing.T) {
	tmpDir := t.TempDir()
	empty := filepath.Join(tmpDir, "empty")
	os.WriteFile(empty, []byte{}, 0644)
	if err := VerifyBinary(empty); err == nil {
		t.Error("expected error for empty file")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd /home/cyb3r/RTLog && go test ./internal/update/... -v -run "TestDownload|TestReplace|TestVerify"`
Expected: compilation failure.

- [ ] **Step 3: Implement download, verify, and replace**

Add to `internal/update/update.go`:

```go
import "io"

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

	// ELF: 0x7f 'E' 'L' 'F'
	if header[0] == 0x7f && header[1] == 'E' && header[2] == 'L' && header[3] == 'F' {
		return nil
	}
	// Mach-O 64-bit: 0xcf 0xfa 0xed 0xfe
	if header[0] == 0xcf && header[1] == 0xfa && header[2] == 0xed && header[3] == 0xfe {
		return nil
	}
	// Mach-O 32-bit: 0xce 0xfa 0xed 0xfe
	if header[0] == 0xce && header[1] == 0xfa && header[2] == 0xed && header[3] == 0xfe {
		return nil
	}

	return fmt.Errorf("not a valid ELF or Mach-O binary (header: %x)", header)
}

// ReplaceBinary atomically replaces the binary at destPath with srcPath.
// Preserves the permissions of the original binary.
func ReplaceBinary(srcPath, destPath string) error {
	// Get current binary permissions
	info, err := os.Stat(destPath)
	if err != nil {
		return fmt.Errorf("stat current binary: %w", err)
	}

	// Set permissions on new binary to match
	if err := os.Chmod(srcPath, info.Mode().Perm()); err != nil {
		return fmt.Errorf("chmod: %w", err)
	}

	// Atomic rename
	if err := os.Rename(srcPath, destPath); err != nil {
		return fmt.Errorf("rename: %w", err)
	}
	return nil
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd /home/cyb3r/RTLog && go test ./internal/update/... -v -run "TestDownload|TestReplace|TestVerify"`
Expected: all PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/update/update.go internal/update/update_test.go
git commit -m "feat(update): add binary download, verify, and replace logic"
```

---

### Task 6: Background Check Function

**Files:**
- Modify: `internal/update/update.go`
- Modify: `internal/update/update_test.go`

- [ ] **Step 1: Write failing test for background check**

Append to `internal/update/update_test.go`:

```go
func TestBackgroundCheck_NewVersionAvailable(t *testing.T) {
	_, cleanup := setupTestHome(t)
	defer cleanup()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"tag_name": "v2.0.0", "assets": []}`)
	}))
	defer server.Close()

	BackgroundCheck("v1.0.0", server.URL)

	if v := ReadUpdateAvailable(); v != "v2.0.0" {
		t.Errorf("expected update-available to be v2.0.0, got %q", v)
	}
	if ShouldCheck() {
		t.Error("expected ShouldCheck to return false after background check")
	}
}

func TestBackgroundCheck_AlreadyUpToDate(t *testing.T) {
	_, cleanup := setupTestHome(t)
	defer cleanup()

	// Pre-populate a stale update-available
	WriteUpdateAvailable("v0.9.0")

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"tag_name": "v1.0.0", "assets": []}`)
	}))
	defer server.Close()

	BackgroundCheck("v1.0.0", server.URL)

	if v := ReadUpdateAvailable(); v != "" {
		t.Errorf("expected update-available to be cleared, got %q", v)
	}
}

func TestBackgroundCheck_APIError(t *testing.T) {
	_, cleanup := setupTestHome(t)
	defer cleanup()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(500)
	}))
	defer server.Close()

	// Should not panic or crash
	BackgroundCheck("v1.0.0", server.URL)

	// last-update-check should NOT be written on error
	if !ShouldCheck() {
		t.Error("expected ShouldCheck to remain true after API error")
	}
}
```

```go
func TestBackgroundCheck_ThrottledByLastCheck(t *testing.T) {
	_, cleanup := setupTestHome(t)
	defer cleanup()

	called := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"tag_name": "v2.0.0", "assets": []}`)
	}))
	defer server.Close()

	// Simulate that ShouldCheck() would return false (recent check)
	WriteLastCheck()

	// Caller should check ShouldCheck() before calling BackgroundCheck.
	// This test verifies the pattern used in root.go:
	if ShouldCheck() {
		BackgroundCheck("v1.0.0", server.URL)
	}

	if called {
		t.Error("expected API not to be called when ShouldCheck returns false")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd /home/cyb3r/RTLog && go test ./internal/update/... -v -run TestBackgroundCheck`
Expected: compilation failure.

- [ ] **Step 3: Implement BackgroundCheck function**

Add to `internal/update/update.go`:

```go
// BackgroundCheck checks for a new version and writes state files.
// Designed to be called synchronously in tests or as a goroutine in production.
// apiURL is parameterized for testing; pass GitHubAPIURL in production.
func BackgroundCheck(currentVersion, apiURL string) {
	rel, err := FetchLatestRelease(apiURL)
	if err != nil {
		return // silent failure
	}

	cmp := CompareVersions(currentVersion, rel.TagName)
	if cmp < 0 {
		WriteUpdateAvailable(rel.TagName)
	} else {
		ClearUpdateAvailable()
	}
	WriteLastCheck()
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd /home/cyb3r/RTLog && go test ./internal/update/... -v -run TestBackgroundCheck`
Expected: all PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/update/update.go internal/update/update_test.go
git commit -m "feat(update): add background version check function"
```

---

### Task 7: `rtlog update` Cobra Command

**Files:**
- Create: `cmd/update.go`

- [ ] **Step 1: Implement the update command**

Create `cmd/update.go`:

```go
package cmd

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/cyb33rr/rtlog/internal/update"
	"github.com/spf13/cobra"
)

var updateForce bool

var updateCmd = &cobra.Command{
	Use:   "update",
	Short: "Update rtlog to the latest version",
	Long: `Check GitHub Releases for the latest version and update the binary in-place.

If rtlog was installed via 'go install', prints the appropriate command instead.
Use --force to re-download even if the current version matches.`,
	Args: cobra.NoArgs,
	RunE: runUpdate,
}

func init() {
	updateCmd.Flags().BoolVar(&updateForce, "force", false, "re-download even if up to date")
	rootCmd.AddCommand(updateCmd)
}

func runUpdate(cmd *cobra.Command, args []string) error {
	fmt.Printf("Current version: %s\n", Version)
	fmt.Println("Checking for updates...")

	rel, err := update.FetchLatestRelease(update.GitHubAPIURL)
	if err != nil {
		return fmt.Errorf("failed to check for updates: %w", err)
	}

	// Dev build confirmation
	if update.IsDevVersion(Version) {
		fmt.Printf("Current version is 'dev' (local build). Update will replace with %s.\n", rel.TagName)
		fmt.Print("Continue? [y/N] ")
		reader := bufio.NewReader(os.Stdin)
		answer, _ := reader.ReadString('\n')
		if strings.TrimSpace(strings.ToLower(answer)) != "y" {
			fmt.Println("Update cancelled.")
			return nil
		}
	} else if !updateForce && update.CompareVersions(Version, rel.TagName) >= 0 {
		fmt.Printf("Already up to date (%s).\n", Version)
		update.ClearUpdateAvailable()
		update.WriteLastCheck()
		return nil
	}

	// Resolve binary path
	self, err := os.Executable()
	if err != nil {
		return fmt.Errorf("cannot determine binary path: %w", err)
	}
	self, err = filepath.EvalSymlinks(self)
	if err != nil {
		return fmt.Errorf("cannot resolve binary path: %w", err)
	}

	// Check if installed via go install
	gopath := os.Getenv("GOPATH")
	if gopath == "" {
		home, _ := os.UserHomeDir()
		gopath = filepath.Join(home, "go")
	}
	gobin := os.Getenv("GOBIN")

	if update.IsGoInstalled(self, gopath, gobin) {
		fmt.Println("You installed rtlog via 'go install'. Run:")
		fmt.Println("  go install github.com/cyb33rr/rtlog@latest")
		return nil
	}

	// Find matching asset
	assetURL, err := update.FindAssetURL(rel.Assets, runtime.GOOS, runtime.GOARCH)
	if err != nil {
		return err
	}

	fmt.Printf("Downloading %s...\n", rel.TagName)

	// Download to temp file in same directory (for atomic rename)
	tmpPath := self + ".update"
	if err := update.DownloadBinary(assetURL, tmpPath); err != nil {
		os.Remove(tmpPath)
		return err
	}

	// Verify
	if err := update.VerifyBinary(tmpPath); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("downloaded file is invalid: %w", err)
	}

	// Replace
	if err := update.ReplaceBinary(tmpPath, self); err != nil {
		os.Remove(tmpPath)
		if os.IsPermission(err) {
			return fmt.Errorf("%w\nTry: sudo rtlog update", err)
		}
		return err
	}

	// Cleanup state
	update.ClearUpdateAvailable()
	update.WriteLastCheck()

	fmt.Printf("Updated rtlog: %s → %s\n", Version, rel.TagName)
	return nil
}
```

- [ ] **Step 2: Build and verify it compiles**

Run: `cd /home/cyb3r/RTLog && go build ./...`
Expected: no errors.

- [ ] **Step 3: Verify help output**

Run: `cd /home/cyb3r/RTLog && go run . update --help`
Expected: shows usage with `--force` flag.

- [ ] **Step 4: Commit**

```bash
git add cmd/update.go
git commit -m "feat: add rtlog update command"
```

---

### Task 8: Background Check Integration in root.go

**Files:**
- Modify: `cmd/root.go`

- [ ] **Step 1: Add background check to PersistentPreRunE and notification to PersistentPostRunE**

Modify `cmd/root.go`. The existing `PersistentPreRunE` in `init()` needs to be extended, and a new `PersistentPostRunE` added.

Add the import:

```go
import (
	"github.com/cyb33rr/rtlog/internal/display"
	"github.com/cyb33rr/rtlog/internal/update"
)
```

Inside `init()`, after `rootCmd.CompletionOptions.HiddenDefaultCmd = true`, append to the existing `PersistentPreRunE` — replace the current assignment (lines 34-53) with:

```go
rootCmd.PersistentPreRunE = func(cmd *cobra.Command, args []string) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil // non-fatal
	}
	configPath := filepath.Join(home, ".rt", "extract.conf")

	// Auto-create from embedded default if missing
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		if data, e := embeddedFS.ReadFile("extract.conf"); e == nil {
			rtDir := filepath.Join(home, ".rt")
			_ = os.MkdirAll(rtDir, 0755)
			_ = os.WriteFile(configPath, data, 0644)
		}
	}

	// Load extraction config (primary source)
	_ = extract.LoadUserConfig(configPath)

	// Background version check: read state from *previous* run before
	// launching goroutine, so notification reflects prior state only.
	if shouldRunUpdateCheck(cmd) {
		pendingUpdate = update.ReadUpdateAvailable()
		if update.ShouldCheck() {
			go update.BackgroundCheck(Version, update.GitHubAPIURL)
		}
	}

	return nil
}

rootCmd.PersistentPostRunE = func(cmd *cobra.Command, args []string) error {
	if pendingUpdate != "" {
		fmt.Fprintf(os.Stderr, "\nUpdate available: %s (current: %s). Run 'rtlog update' to upgrade.\n", pendingUpdate, Version)
	}
	return nil
}
```

Add the helper function outside `init()`:

```go
// pendingUpdate holds the update-available version read from a *previous* run.
// Captured in PersistentPreRunE before the goroutine launches, displayed in PersistentPostRunE.
var pendingUpdate string

// shouldRunUpdateCheck returns true if the background update check should run.
// Skipped for: log command, update command, non-TTY, dev builds, opt-out env var.
func shouldRunUpdateCheck(cmd *cobra.Command) bool {
	if os.Getenv("RTLOG_NO_UPDATE_CHECK") == "1" {
		return false
	}
	if update.IsDevVersion(Version) {
		return false
	}
	if !display.IsTTY {
		return false
	}
	name := cmd.Name()
	if name == "log" || name == "update" {
		return false
	}
	return true
}
```

- [ ] **Step 2: Build and verify it compiles**

Run: `cd /home/cyb3r/RTLog && go build ./...`
Expected: no errors.

- [ ] **Step 3: Run existing tests to verify nothing is broken**

Run: `cd /home/cyb3r/RTLog && go test ./...`
Expected: all tests PASS.

- [ ] **Step 4: Commit**

```bash
git add cmd/root.go
git commit -m "feat: integrate background version check into root command"
```

---

### Task 9: Final Integration Test and Cleanup

**Files:**
- All created/modified files

- [ ] **Step 1: Run full test suite**

Run: `cd /home/cyb3r/RTLog && go test ./... -v`
Expected: all tests PASS, including new update tests.

- [ ] **Step 2: Run vet and build**

Run: `cd /home/cyb3r/RTLog && go vet ./... && go build -o rtlog .`
Expected: no warnings, binary builds.

- [ ] **Step 3: Smoke test — verify update help**

Run: `cd /home/cyb3r/RTLog && ./rtlog update --help`
Expected: shows update command help with `--force` flag.

- [ ] **Step 4: Smoke test — verify update command with current dev build**

Run: `cd /home/cyb3r/RTLog && ./rtlog update`
Expected: prompts about dev build replacement (since local build has `Version=dev`).

- [ ] **Step 5: Verify opt-out env var suppresses update notification**

Run: `cd /home/cyb3r/RTLog && RTLOG_NO_UPDATE_CHECK=1 ./rtlog status 2>&1`
Expected: no update notification in output.

- [ ] **Step 6: Final commit**

```bash
git add -A
git commit -m "feat: complete auto-update feature implementation"
```
