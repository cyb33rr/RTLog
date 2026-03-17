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
	_ = json.NewDecoder
	_ = fmt.Sprintf
	_ = io.Copy
	_ = http.Get
	_ = os.Open
	_ = filepath.Join
	_ = strconv.Atoi
	_ = strings.TrimSpace
	_ = time.Now
)

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
