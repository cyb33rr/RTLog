package match

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
)

var wrappers = map[string]bool{
	"sudo": true, "nohup": true, "time": true, "env": true,
	"command": true, "exec": true, "nice": true, "ionice": true,
	"strace": true, "ltrace": true, "proxychains": true,
	"proxychains4": true, "tsocks": true,
}

type Matcher struct {
	exact map[string]bool
	globs []string
}

func LoadTools(path string) (*Matcher, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	m := &Matcher{exact: make(map[string]bool)}
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.ContainsAny(line, "*?") {
			m.globs = append(m.globs, line)
		} else {
			m.exact[line] = true
		}
	}
	return m, scanner.Err()
}

func DefaultToolsConf() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".rt", "tools.conf")
}

func (m *Matcher) Match(tool string) bool {
	if m.exact[tool] {
		return true
	}
	for _, pat := range m.globs {
		if matched, _ := filepath.Match(pat, tool); matched {
			return true
		}
	}
	return false
}

func ExtractTool(cmd string) string {
	fields := strings.Fields(cmd)
	for len(fields) > 0 {
		word := fields[0]
		if strings.Contains(word, "=") && !strings.HasPrefix(word, "-") {
			fields = fields[1:]
			continue
		}
		base := filepath.Base(word)
		if wrappers[base] {
			fields = fields[1:]
			if base == "sudo" {
				for len(fields) > 0 && strings.HasPrefix(fields[0], "-") {
					flag := fields[0]
					fields = fields[1:]
					if len(flag) == 2 && strings.ContainsAny(string(flag[1]), "ugCDRT") && len(fields) > 0 {
						fields = fields[1:]
					}
				}
			}
			continue
		}
		return base
	}
	return ""
}
