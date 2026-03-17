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
		want    int
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
	currentPath := filepath.Join(tmpDir, "rtlog")
	os.WriteFile(currentPath, []byte("old"), 0755)
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
	info, _ := os.Stat(currentPath)
	if info.Mode().Perm() != 0755 {
		t.Errorf("expected 0755 permissions, got %o", info.Mode().Perm())
	}
	if _, err := os.Stat(newPath); !os.IsNotExist(err) {
		t.Error("expected temp file to be cleaned up")
	}
}

func TestVerifyBinary_Valid(t *testing.T) {
	tmpDir := t.TempDir()
	elf := filepath.Join(tmpDir, "elf")
	os.WriteFile(elf, []byte("\x7fELFrest"), 0644)
	if err := VerifyBinary(elf); err != nil {
		t.Errorf("ELF should be valid: %v", err)
	}
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
	BackgroundCheck("v1.0.0", server.URL)
	if !ShouldCheck() {
		t.Error("expected ShouldCheck to remain true after API error")
	}
}

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
	WriteLastCheck()
	if ShouldCheck() {
		BackgroundCheck("v1.0.0", server.URL)
	}
	if called {
		t.Error("expected API not to be called when ShouldCheck returns false")
	}
}
