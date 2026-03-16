package extract

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"regexp"
	"strings"
)

// ToolExtractConfig defines extraction behavior for a specific tool.
type ToolExtractConfig struct {
	TargetFlags    []string          // Short flags for target extraction (e.g., "-u", "-h")
	PositionalArgs bool              // Bare IPs/FQDNs are targets
	CredFlags      map[string]string // Short cred flags: flag -> role (user/pass/hash)
	NoExtract      bool              // Skip all extraction for this tool
}

// toolConfigs is the unified config map for all known tools.
var toolConfigs map[string]*ToolExtractConfig

// toolTargetRegexes holds per-tool compiled target flag regexes.
var toolTargetRegexes map[string]*regexp.Regexp

func init() {
	toolConfigs = make(map[string]*ToolExtractConfig)
	toolTargetRegexes = make(map[string]*regexp.Regexp)
	toolCredRegexes = make(map[string]map[string]*regexp.Regexp)
}

// compileToolRegexes builds per-tool target and credential regexes from toolConfigs.
func compileToolRegexes() {
	// Build per-tool target regexes
	toolTargetRegexes = make(map[string]*regexp.Regexp)
	for tool, cfg := range toolConfigs {
		if len(cfg.TargetFlags) > 0 {
			escaped := sortAndEscapeFlags(cfg.TargetFlags)
			pattern := `(?:^|\s)(?:` + strings.Join(escaped, "|") + `)(?:\s+|=)` +
				`(?:https?://)?(` + hostOrIP + `)` +
				`(?::(\d{1,5}))?`
			toolTargetRegexes[tool] = regexp.MustCompile(pattern)
		}
	}

	// Build per-tool credential regexes
	toolCredRegexes = make(map[string]map[string]*regexp.Regexp)
	for tool, cfg := range toolConfigs {
		if len(cfg.CredFlags) == 0 {
			continue
		}
		roleFlags := map[string][]string{"user": {}, "pass": {}, "hash": {}}
		for flag, role := range cfg.CredFlags {
			roleFlags[role] = append(roleFlags[role], flag)
		}
		rxMap := make(map[string]*regexp.Regexp)
		for role, fl := range roleFlags {
			if len(fl) == 0 {
				continue
			}
			if role == "hash" {
				rxMap[role] = BuildHashFlagRegex(fl)
			} else {
				rxMap[role] = BuildFlagRegex(fl)
			}
		}
		toolCredRegexes[tool] = rxMap
	}
}

// GetToolConfig returns the extraction config for a tool.
// Returns (config, true) for known tools, (nil, false) for unknown tools.
// Impacket tools (impacket-*) return an empty config.
func GetToolConfig(tool string) (*ToolExtractConfig, bool) {
	if tool == "" {
		return nil, false
	}
	if cfg, ok := toolConfigs[tool]; ok {
		return cfg, true
	}
	if strings.HasPrefix(tool, "impacket-") {
		return &ToolExtractConfig{}, true
	}
	return nil, false
}

// IsKnownTool returns true if the tool has a config entry or is an impacket tool.
func IsKnownTool(tool string) bool {
	_, known := GetToolConfig(tool)
	return known
}

// LoadConfigBytes parses extraction config from raw bytes and populates toolConfigs.
// This is used for both the embedded default config and user overrides.
func LoadConfigBytes(data []byte) error {
	scanner := bufio.NewScanner(bytes.NewReader(data))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		fields := strings.Fields(line)
		toolName := fields[0]
		cfg := &ToolExtractConfig{}

		for _, field := range fields[1:] {
			switch {
			case field == "positional":
				cfg.PositionalArgs = true
			case field == "noextract":
				cfg.NoExtract = true
			case strings.HasPrefix(field, "target:"):
				flags := strings.Split(strings.TrimPrefix(field, "target:"), ",")
				for _, f := range flags {
					f = strings.TrimSpace(f)
					if f != "" {
						cfg.TargetFlags = append(cfg.TargetFlags, f)
					}
				}
			case strings.HasPrefix(field, "cred:"):
				if cfg.CredFlags == nil {
					cfg.CredFlags = make(map[string]string)
				}
				pairs := strings.Split(strings.TrimPrefix(field, "cred:"), ",")
				for _, pair := range pairs {
					parts := strings.SplitN(pair, "=", 2)
					if len(parts) == 2 {
						flag := strings.TrimSpace(parts[0])
						role := strings.TrimSpace(parts[1])
						if flag != "" && role != "" {
							switch role {
							case "user", "pass", "hash":
								cfg.CredFlags[flag] = role
							default:
								fmt.Fprintf(os.Stderr, "extract config: unknown cred role %q for flag %q in tool %q (want user/pass/hash), skipping\n", role, flag, toolName)
							}
						}
					}
				}
			}
		}

		toolConfigs[toolName] = cfg
	}

	if err := scanner.Err(); err != nil {
		return err
	}

	compileToolRegexes()
	return nil
}

// LoadUserConfig reads a user config file and merges entries into toolConfigs.
// Missing file is not an error. User entries override built-in config entirely.
func LoadUserConfig(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	return LoadConfigBytes(data)
}
