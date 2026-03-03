package extract

import (
	"regexp"
	"strconv"
	"strings"
)

// hostOrIP matches either a bare IPv4 address or an FQDN hostname.
// IP branch comes first to prevent the FQDN branch from consuming digits.
const hostOrIP = `(?:\d{1,3}\.\d{1,3}\.\d{1,3}\.\d{1,3}` +
	`|[A-Za-z0-9](?:[A-Za-z0-9-]*[A-Za-z0-9])?(?:\.[A-Za-z0-9](?:[A-Za-z0-9-]*[A-Za-z0-9])?)*\.[A-Za-z]{2,})`

// Compiled regex patterns for target extraction.
var (
	// RE_IPV4_EXT matches IPv4 with optional /CIDR and :port.
	RE_IPV4_EXT = regexp.MustCompile(
		`\b(\d{1,3}\.\d{1,3}\.\d{1,3}\.\d{1,3})` +
			`(?:/(\d{1,2}))?` +
			`(?::(\d{1,5}))?\b`,
	)

	// RE_USER_AT_HOST matches user@host and domain/user@host patterns.
	RE_USER_AT_HOST = regexp.MustCompile(
		`(?:[A-Za-z0-9._-]+/)?[A-Za-z0-9._-]+@(` + hostOrIP + `)`,
	)

	// RE_UNC_HOST matches UNC paths: \\host\share and //host/share.
	RE_UNC_HOST = regexp.MustCompile(
		`(?:\\\\|//)(` + hostOrIP + `)[/\\]`,
	)

	// RE_URL_HOST matches URLs with common schemes.
	RE_URL_HOST = regexp.MustCompile(
		`(?:https?|smb|ldaps?|ftp|rdp|vnc)://` +
			`(?:[A-Za-z0-9._~:%-]+@)?` +
			`(` + hostOrIP + `)` +
			`(?::(\d{1,5}))?`,
	)

	// RE_BARE_HOSTNAME matches standalone FQDNs.
	RE_BARE_HOSTNAME = regexp.MustCompile(
		`(?:^|\s)([A-Za-z0-9](?:[A-Za-z0-9-]*[A-Za-z0-9])?(?:\.[A-Za-z0-9](?:[A-Za-z0-9-]*[A-Za-z0-9])?)*\.[A-Za-z]{2,})(?:\s|$)`,
	)

	// RE_SETVAR_HOST matches Metasploit set-variable patterns.
	RE_SETVAR_HOST = regexp.MustCompile(
		`(?i)(?:^|\s)(?:set\s+(?:RHOSTS?|LHOST|TARGET)|(?:RHOSTS?|LHOST|TARGET)=)\s*` +
			`(` + hostOrIP + `)` +
			`(?::(\d{1,5}))?`,
	)

	// RE_GLOBAL_FLAG_HOST is built from globalTargetFlags (long flags only).
	RE_GLOBAL_FLAG_HOST *regexp.Regexp

	// RE_IPV4_FULL is for fullmatch checking of IPv4 addresses.
	RE_IPV4_FULL = regexp.MustCompile(`^\d{1,3}\.\d{1,3}\.\d{1,3}\.\d{1,3}$`)
)

// globalTargetFlags are long flags that are unambiguous across all tools.
var globalTargetFlags = []string{
	"--url", "--host", "--target", "--target-ip",
	"--dc-ip", "--dc-host", "--dc", "--kdcHost",
	"-rhost", "--rhost", "--ip", "--server", "--remote-host",
}

// FILE_EXTENSIONS is the set of known file extensions to filter out.
var FILE_EXTENSIONS = map[string]struct{}{
	"py": {}, "js": {}, "ts": {}, "sh": {}, "rb": {}, "go": {}, "rs": {},
	"c": {}, "h": {}, "cpp": {}, "java": {},
	"conf": {}, "cfg": {}, "ini": {}, "yml": {}, "yaml": {}, "json": {},
	"xml": {}, "txt": {}, "log": {},
	"so": {}, "dll": {}, "exe": {}, "bin": {}, "gz": {}, "tar": {}, "zip": {},
}

func init() {
	// Build RE_GLOBAL_FLAG_HOST from globalTargetFlags sorted longest-first.
	escaped := sortAndEscapeFlags(globalTargetFlags)
	pattern := `(?:^|\s)(?:` + strings.Join(escaped, "|") + `)(?:\s+|=)` +
		`(?:https?://)?(` + hostOrIP + `)` +
		`(?::(\d{1,5}))?`
	RE_GLOBAL_FLAG_HOST = regexp.MustCompile(pattern)
}

// TargetResult holds the extracted targets from a command.
type TargetResult struct {
	IPs   StringSet
	CIDRs StringSet
	Hosts StringSet
	Ports StringSet
}

// isValidIPv4 returns true if all octets are 0-255.
func isValidIPv4(ip string) bool {
	parts := strings.Split(ip, ".")
	if len(parts) != 4 {
		return false
	}
	for _, p := range parts {
		if p == "" {
			return false
		}
		n, err := strconv.Atoi(p)
		if err != nil || n < 0 || n > 255 {
			return false
		}
	}
	return true
}

// isSchemeContext returns true if the match is preceded by :// (URL scheme).
func isSchemeContext(cmd string, matchStart int) bool {
	if matchStart >= 3 && cmd[matchStart-3:matchStart] == "://" {
		return true
	}
	// Also handle userinfo@ in URLs: scheme://user@HOST
	if matchStart >= 1 && cmd[matchStart-1] == '@' {
		return strings.Contains(cmd[:matchStart], "://")
	}
	return false
}

// isVersionContext returns true if the IP-like match is preceded by version context.
func isVersionContext(cmd string, matchStart int) bool {
	if matchStart <= 0 {
		return false
	}
	if isSchemeContext(cmd, matchStart) {
		return false
	}
	prev := cmd[matchStart-1]
	if prev == 'v' || prev == 'V' {
		return true
	}
	if prev == '/' {
		// Check if it looks like a path/version: e.g., Python/3.10.5
		before := strings.TrimRight(cmd[:matchStart], " \t")
		if len(before) > 0 && before[len(before)-1] != ' ' && before[len(before)-1] != '\t' {
			return true
		}
	}
	return false
}

// isPathContext returns true if hostname-like match is preceded by path separators.
func isPathContext(cmd string, matchStart int) bool {
	if matchStart <= 0 {
		return false
	}
	if isSchemeContext(cmd, matchStart) {
		return false
	}
	prev := cmd[matchStart-1]
	return prev == '/' || prev == '.'
}

// CredValueSpans returns a set of character positions occupied by credential
// flag values. These should be excluded from target extraction.
func CredValueSpans(cmd, tool string) *PositionTracker {
	pt := NewPositionTracker()

	config, toolKnown := GetToolConfig(tool)
	hasCreds := config != nil && len(config.CredFlags) > 0
	applyLongFlags := !toolKnown || hasCreds

	// Long credential flags (only for unknown tools or tools with cred config)
	if applyLongFlags {
		for _, rx := range []*regexp.Regexp{RE_LONG_USER, RE_LONG_PASS} {
			for _, m := range rx.FindAllStringSubmatchIndex(cmd, -1) {
				if m[2] >= 0 && m[3] >= 0 {
					pt.Mark(m[2], m[3])
				}
			}
		}

		// Long hash flags
		for _, m := range RE_LONG_HASH.FindAllStringSubmatchIndex(cmd, -1) {
			if m[2] >= 0 && m[3] >= 0 {
				pt.Mark(m[2], m[3])
			}
		}
	}

	// Per-tool short credential flags (pre-compiled)
	if rxMap, ok := toolCredRegexes[tool]; ok {
		for _, rx := range rxMap {
			for _, m := range rx.FindAllStringSubmatchIndex(cmd, -1) {
				if m[2] >= 0 && m[3] >= 0 {
					pt.Mark(m[2], m[3])
				}
			}
		}
	}

	return pt
}

// ExtractTargets runs target extraction passes over a command string.
// Pass gating is tool-specific to avoid false positives.
func ExtractTargets(cmd, tool string) *TargetResult {
	result := &TargetResult{
		IPs:   NewStringSet(),
		CIDRs: NewStringSet(),
		Hosts: NewStringSet(),
		Ports: NewStringSet(),
	}

	config, toolKnown := GetToolConfig(tool)

	// Tools with NoExtract skip all extraction.
	if config != nil && config.NoExtract {
		return result
	}

	// Positional extraction (bare IPs/FQDNs) is allowed for:
	// - Unknown tools (backward compatible)
	// - Known tools with PositionalArgs=true
	allowPositional := !toolKnown || (config != nil && config.PositionalArgs)

	// Pre-mark credential value positions so they aren't extracted as targets.
	pos := CredValueSpans(cmd, tool)

	addIP := func(ipStr, portStr, cidrStr string, start, end int) {
		if !pos.Claim(start, end) {
			return
		}
		if !isValidIPv4(ipStr) {
			return
		}
		if isVersionContext(cmd, start) {
			return
		}
		if cidrStr != "" {
			cidrVal, err := strconv.Atoi(cidrStr)
			if err == nil && cidrVal >= 0 && cidrVal <= 32 {
				result.CIDRs.Add(ipStr + "/" + cidrStr)
			}
		} else {
			result.IPs.Add(ipStr)
		}
		if portStr != "" {
			portVal, err := strconv.Atoi(portStr)
			if err == nil && portVal >= 1 && portVal <= 65535 {
				result.Ports.Add(ipStr + ":" + portStr)
			}
		}
	}

	addHost := func(hostStr, portStr string, start, end int) {
		if !pos.Claim(start, end) {
			return
		}
		hostLower := strings.ToLower(hostStr)
		parts := strings.SplitN(hostLower, ".", -1)
		if len(parts) >= 2 {
			ext := parts[len(parts)-1]
			if _, ok := FILE_EXTENSIONS[ext]; ok {
				return
			}
		}
		if isPathContext(cmd, start) {
			return
		}
		if RE_IPV4_FULL.MatchString(hostStr) {
			addIP(hostStr, portStr, "", start, end)
			return
		}
		result.Hosts.Add(hostLower)
		if portStr != "" {
			portVal, err := strconv.Atoi(portStr)
			if err == nil && portVal >= 1 && portVal <= 65535 {
				result.Ports.Add(hostLower + ":" + portStr)
			}
		}
	}

	// Pass 1: IPv4 with optional CIDR and port (only if positional allowed)
	if allowPositional {
		for _, m := range RE_IPV4_EXT.FindAllStringSubmatchIndex(cmd, -1) {
			ipStr := cmd[m[2]:m[3]]
			var cidrStr, portStr string
			if m[4] >= 0 {
				cidrStr = cmd[m[4]:m[5]]
			}
			if m[6] >= 0 {
				portStr = cmd[m[6]:m[7]]
			}
			addIP(ipStr, portStr, cidrStr, m[0], m[1])
		}
	}

	// Pass 2: user@host / domain/user@host (always - structurally specific)
	for _, m := range RE_USER_AT_HOST.FindAllStringSubmatchIndex(cmd, -1) {
		host := cmd[m[2]:m[3]]
		if isValidIPv4(host) {
			addIP(host, "", "", m[2], m[3])
		} else {
			addHost(host, "", m[2], m[3])
		}
	}

	// Pass 3: UNC paths (always - structurally specific)
	for _, m := range RE_UNC_HOST.FindAllStringSubmatchIndex(cmd, -1) {
		host := cmd[m[2]:m[3]]
		if isValidIPv4(host) {
			addIP(host, "", "", m[2], m[3])
		} else {
			addHost(host, "", m[2], m[3])
		}
	}

	// Pass 4: URL schemes (always - structurally specific)
	for _, m := range RE_URL_HOST.FindAllStringSubmatchIndex(cmd, -1) {
		host := cmd[m[2]:m[3]]
		var portStr string
		if m[4] >= 0 {
			portStr = cmd[m[4]:m[5]]
		}
		if isValidIPv4(host) {
			addIP(host, portStr, "", m[2], m[1])
		} else {
			addHost(host, portStr, m[2], m[1])
		}
	}

	// Pass 5a: Global long flags (always - unambiguous)
	for _, m := range RE_GLOBAL_FLAG_HOST.FindAllStringSubmatchIndex(cmd, -1) {
		host := cmd[m[2]:m[3]]
		var portStr string
		if m[4] >= 0 {
			portStr = cmd[m[4]:m[5]]
		}
		if isValidIPv4(host) {
			addIP(host, portStr, "", m[2], m[1])
		} else {
			addHost(host, portStr, m[2], m[1])
		}
	}

	// Pass 5b: Per-tool short target flags (only for tools with target flag config)
	if rx, ok := toolTargetRegexes[tool]; ok {
		for _, m := range rx.FindAllStringSubmatchIndex(cmd, -1) {
			host := cmd[m[2]:m[3]]
			var portStr string
			if m[4] >= 0 {
				portStr = cmd[m[4]:m[5]]
			}
			if isValidIPv4(host) {
				addIP(host, portStr, "", m[2], m[1])
			} else {
				addHost(host, portStr, m[2], m[1])
			}
		}
	}

	// Pass 6: Metasploit set-variable patterns (msfconsole or unknown tool)
	if tool == "" || tool == "msfconsole" {
		for _, m := range RE_SETVAR_HOST.FindAllStringSubmatchIndex(cmd, -1) {
			host := cmd[m[2]:m[3]]
			var portStr string
			if m[4] >= 0 {
				portStr = cmd[m[4]:m[5]]
			}
			if isValidIPv4(host) {
				addIP(host, portStr, "", m[2], m[1])
			} else {
				addHost(host, portStr, m[2], m[1])
			}
		}
	}

	// Pass 7: Bare FQDN hostnames (only if positional allowed)
	if allowPositional {
		for _, m := range RE_BARE_HOSTNAME.FindAllStringSubmatchIndex(cmd, -1) {
			host := cmd[m[2]:m[3]]
			addHost(host, "", m[2], m[3])
		}
	}

	return result
}
