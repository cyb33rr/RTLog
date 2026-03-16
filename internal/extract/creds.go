package extract

import (
	"regexp"
	"strings"
)

// Long flags — unambiguous across all tools.
var (
	longUserFlags = []string{"--username", "--user", "--login"}
	longPassFlags = []string{"--password", "--passwd", "--pass"}
	longHashFlags = []string{"--hashes", "-hashes", "--hash"}
)

// Compiled long-flag regexes.
var (
	RE_LONG_USER *regexp.Regexp
	RE_LONG_PASS *regexp.Regexp
	RE_LONG_HASH *regexp.Regexp

	// RE_CRED_INLINE matches inline domain/user:pass@host patterns,
	// including an optional :port suffix after the host.
	RE_CRED_INLINE = regexp.MustCompile(
		`(?:^|\s)(?:([A-Za-z0-9._-]+)/)?([A-Za-z0-9._-]+):(\S+)@([A-Za-z0-9._-]+)(?::\d{1,5})?(?:\s|$)`,
	)

	// RE_SETVAR_CRED matches Metasploit set-variable credential patterns.
	// Groups: 1=set-var, 2=env-var, 3=value
	RE_SETVAR_CRED = regexp.MustCompile(
		`(?i)(?:^|\s)(?:set\s+(USERNAME|PASSWORD|PASS|SMBUser|SMBPass)|(USERNAME|PASSWORD|PASS|SMBUser|SMBPass)=)\s*(\S+)`,
	)
)

// Pre-compiled per-tool credential regexes (keyed by tool name).
// Built from toolConfigs in config.go compileToolRegexes().
var toolCredRegexes map[string]map[string]*regexp.Regexp

func init() {
	RE_LONG_USER = BuildFlagRegex(longUserFlags)
	RE_LONG_PASS = BuildFlagRegex(longPassFlags)
	RE_LONG_HASH = BuildHashFlagRegex(longHashFlags)
}

// CredResult holds extracted credentials.
type CredResult struct {
	Users     StringSet
	Passwords StringSet
	Hashes    StringSet
}

// ExtractCreds extracts credentials from a command string.
// Uses tool-aware extraction to avoid false positives.
func ExtractCreds(cmd, tool string) *CredResult {
	result := &CredResult{
		Users:     NewStringSet(),
		Passwords: NewStringSet(),
		Hashes:    NewStringSet(),
	}

	config, toolKnown := GetToolConfig(tool)

	// Tools with NoExtract skip all extraction.
	if config != nil && config.NoExtract {
		return result
	}

	// Long credential flags apply for unknown tools or tools with cred config.
	hasCreds := config != nil && len(config.CredFlags) > 0
	applyLongFlags := !toolKnown || hasCreds

	// Pass 1: Inline domain/user:pass@host (highest confidence, always applied)
	// Groups: 1=domain, 2=user, 3=pass, 4=host
	for _, m := range RE_CRED_INLINE.FindAllStringSubmatch(cmd, -1) {
		user := m[2]
		pwd := m[3]
		if len(user) > 1 && !IsFileLike(user) {
			result.Users.Add(user)
		}
		if len(pwd) > 1 && !IsFileLike(pwd) {
			result.Passwords.Add(pwd)
		}
	}

	// Pass 2: Long-flag usernames (only for unknown tools or tools with cred flags)
	if applyLongFlags {
		for _, m := range RE_LONG_USER.FindAllStringSubmatch(cmd, -1) {
			val := StripQuotes(m[1])
			if len(val) > 1 && !IsFileLike(val) {
				result.Users.Add(val)
			}
		}
	}

	// Pass 3: Long-flag passwords (only for unknown tools or tools with cred flags)
	if applyLongFlags {
		for _, m := range RE_LONG_PASS.FindAllStringSubmatch(cmd, -1) {
			val := StripQuotes(m[1])
			if len(val) > 1 && !IsFileLike(val) {
				result.Passwords.Add(val)
			}
		}
	}

	// Pass 4: Long-flag hashes (only for unknown tools or tools with cred flags)
	if applyLongFlags {
		for _, m := range RE_LONG_HASH.FindAllStringSubmatch(cmd, -1) {
			result.Hashes.Add(m[1])
		}
	}

	// Pass 5: Per-tool short flags (only for known tools with cred config)
	if rxMap, ok := toolCredRegexes[tool]; ok {
		for role, rx := range rxMap {
			if role == "hash" {
				for _, m := range rx.FindAllStringSubmatch(cmd, -1) {
					result.Hashes.Add(m[1])
				}
			} else {
				for _, m := range rx.FindAllStringSubmatch(cmd, -1) {
					val := StripQuotes(m[1])
					if len(val) > 1 && !IsFileLike(val) {
						if role == "user" {
							result.Users.Add(val)
						} else {
							result.Passwords.Add(val)
						}
					}
				}
			}
		}
	}

	// Pass 6: Metasploit set-variable creds (only for msfconsole or unknown tool)
	if tool == "" || tool == "msfconsole" {
		// Groups: 1=set-var, 2=env-var, 3=value
		for _, m := range RE_SETVAR_CRED.FindAllStringSubmatch(cmd, -1) {
			varName := m[1]
			if varName == "" {
				varName = m[2]
			}
			val := StripQuotes(m[3])
			if len(val) <= 1 || IsFileLike(val) {
				continue
			}
			upper := strings.ToUpper(varName)
			switch upper {
			case "USERNAME", "SMBUSER":
				result.Users.Add(val)
			case "PASSWORD", "PASS", "SMBPASS":
				result.Passwords.Add(val)
			}
		}
	}

	return result
}
