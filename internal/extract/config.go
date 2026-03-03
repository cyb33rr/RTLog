package extract

import (
	"bufio"
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
	toolConfigs = map[string]*ToolExtractConfig{
		// Positional target tools
		"nmap":          {PositionalArgs: true},
		"nxc":           {PositionalArgs: true, CredFlags: map[string]string{"-u": "user", "-p": "pass", "-H": "hash"}},
		"crackmapexec":  {PositionalArgs: true, CredFlags: map[string]string{"-u": "user", "-p": "pass", "-H": "hash"}},
		"enum4linux":    {PositionalArgs: true, CredFlags: map[string]string{"-u": "user", "-p": "pass"}},
		"enum4linux-ng": {PositionalArgs: true, CredFlags: map[string]string{"-u": "user", "-p": "pass", "-H": "hash"}},
		"ssh":           {PositionalArgs: true, CredFlags: map[string]string{"-l": "user"}},
		"smbmap":        {PositionalArgs: true, CredFlags: map[string]string{"-u": "user", "-p": "pass"}},
		"rdesktop":      {PositionalArgs: true, CredFlags: map[string]string{"-u": "user", "-p": "pass"}},
		"plink":         {PositionalArgs: true, CredFlags: map[string]string{"-l": "user", "-pw": "pass"}},

		// Flag-based target tools
		"gobuster":    {TargetFlags: []string{"-u"}, CredFlags: map[string]string{"-U": "user", "-P": "pass"}},
		"ffuf":        {TargetFlags: []string{"-u"}},
		"feroxbuster": {TargetFlags: []string{"-u"}},
		"wfuzz":       {TargetFlags: []string{"-u"}},
		"sqlmap":      {TargetFlags: []string{"-u"}},
		"nikto":       {TargetFlags: []string{"-h"}},
		"evil-winrm":  {TargetFlags: []string{"-i"}, CredFlags: map[string]string{"-u": "user", "-p": "pass", "-H": "hash"}},

		// URL-scheme-only target tools (no short target flags)
		"curl": {},
		"wget": {},

		// Metasploit tools (set-variable pass handles targets/creds)
		"msfconsole": {},
		"msfvenom":   {},

		// No extraction tools
		"hashcat":      {},
		"john":         {},
		"searchsploit": {},
		"responder":    {},
		"chisel":       {},
		"sshuttle":     {},

		// Hydra uses URL-scheme pass
		"hydra": {},

		// AD tools with cred flags (targets via global long flags like --dc)
		"bloodhound":     {CredFlags: map[string]string{"-u": "user", "-p": "pass"}},
		"kerbrute":       {},
		"certipy":        {},
		"bloodyAD":       {CredFlags: map[string]string{"-u": "user", "-p": "pass"}},
		"rusthound":      {CredFlags: map[string]string{"-u": "user", "-p": "pass", "--ldapusername": "user", "--ldappassword": "pass"}},
		"ldapdomaindump": {CredFlags: map[string]string{"-u": "user", "-p": "pass"}},
		"ldeep":          {CredFlags: map[string]string{"-u": "user", "-p": "pass", "-H": "hash"}},
		"windapsearch":   {CredFlags: map[string]string{"-u": "user", "-p": "pass"}},
		"adidnsdump":     {CredFlags: map[string]string{"-u": "user", "-p": "pass"}},
	}

	compileToolRegexes()
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

// LoadUserConfig reads a user config file and merges entries into toolConfigs.
// Missing file is not an error. User entries override built-in config entirely.
func LoadUserConfig(path string) error {
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}

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
							cfg.CredFlags[flag] = role
						}
					}
				}
			}
			// Unknown options silently ignored (forward compatible)
		}

		// User entry completely overrides built-in config
		toolConfigs[toolName] = cfg
	}

	if err := scanner.Err(); err != nil {
		return err
	}

	// Recompile all regexes after config changes
	compileToolRegexes()
	return nil
}
