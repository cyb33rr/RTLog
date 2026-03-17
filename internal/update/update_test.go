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
