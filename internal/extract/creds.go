package extract

import (
	"regexp"
	"sort"
	"strings"
)

// TOOL_CRED_MAP maps tool names to their credential flag definitions.
// Each inner map maps flag -> role ("user", "pass", or "hash").
var TOOL_CRED_MAP = map[string]map[string]string{
	"nxc":            {"-u": "user", "-p": "pass", "-H": "hash"},
	"crackmapexec":   {"-u": "user", "-p": "pass", "-H": "hash"},
	"evil-winrm":     {"-u": "user", "-p": "pass", "-H": "hash"},
	"enum4linux":     {"-u": "user", "-p": "pass"},
	"enum4linux-ng":  {"-u": "user", "-p": "pass", "-H": "hash"},
	"smbmap":         {"-u": "user", "-p": "pass"},
	"bloodhound":     {"-u": "user", "-p": "pass"},
	"ldapdomaindump": {"-u": "user", "-p": "pass"},
	"ldeep":          {"-u": "user", "-p": "pass", "-H": "hash"},
	"windapsearch":   {"-u": "user", "-p": "pass"},
	"adidnsdump":     {"-u": "user", "-p": "pass"},
	"bloodyAD":       {"-u": "user", "-p": "pass"},
	"rusthound":      {"-u": "user", "-p": "pass", "--ldapusername": "user", "--ldappassword": "pass"},
	"rdesktop":       {"-u": "user", "-p": "pass"},
	"ssh":            {"-l": "user"},
	"plink":          {"-l": "user", "-pw": "pass"},
	"gobuster":       {"-U": "user", "-P": "pass"},
}

// Long flags — unambiguous across all tools, always applied.
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

	// RE_CRED_INLINE matches inline domain/user:pass@host patterns.
	RE_CRED_INLINE = regexp.MustCompile(
		`(?:^|\s)(?:([A-Za-z0-9._-]+)/)?([A-Za-z0-9._-]+):(\S+)@([A-Za-z0-9._-]+)(?:\s|$)`,
	)

	// RE_SETVAR_CRED matches Metasploit set-variable credential patterns.
	// Groups: 1=set-var, 2=env-var, 3=value
	RE_SETVAR_CRED = regexp.MustCompile(
		`(?i)(?:^|\s)(?:set\s+(USERNAME|PASSWORD|PASS|SMBUser|SMBPass)|(USERNAME|PASSWORD|PASS|SMBUser|SMBPass)=)\s*(\S+)`,
	)
)

func init() {
	RE_LONG_USER = BuildFlagRegex(longUserFlags)
	RE_LONG_PASS = BuildFlagRegex(longPassFlags)

	// Build RE_LONG_HASH with hex-value pattern instead of generic \S+.
	sorted := make([]string, len(longHashFlags))
	copy(sorted, longHashFlags)
	sort.Slice(sorted, func(i, j int) bool {
		return len(sorted[i]) > len(sorted[j])
	})
	escaped := make([]string, len(sorted))
	for i, f := range sorted {
		escaped[i] = regexp.QuoteMeta(f)
	}
	pattern := `(?:^|\s)(?:` + strings.Join(escaped, "|") + `)(?:\s+|=)` +
		`([A-Fa-f0-9]{16,64}(?::[A-Fa-f0-9]{16,64})?)`
	RE_LONG_HASH = regexp.MustCompile(pattern)
}

// getToolCredFlags returns the credential flag map for a tool.
func getToolCredFlags(tool string) map[string]string {
	if tool == "" {
		return nil
	}
	return TOOL_CRED_MAP[tool]
}

// CredResult holds extracted credentials.
type CredResult struct {
	Users     StringSet
	Passwords StringSet
	Hashes    StringSet
}

// ExtractCreds extracts credentials from a command string.
// Uses tool-aware short-flag matching to avoid false positives.
func ExtractCreds(cmd, tool string) *CredResult {
	result := &CredResult{
		Users:     NewStringSet(),
		Passwords: NewStringSet(),
		Hashes:    NewStringSet(),
	}

	// Pass 1: Inline domain/user:pass@host (highest confidence, tool-agnostic)
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

	// Pass 2: Long-flag usernames (unambiguous, always applied)
	for _, m := range RE_LONG_USER.FindAllStringSubmatch(cmd, -1) {
		val := StripQuotes(m[1])
		if len(val) > 1 && !IsFileLike(val) {
			result.Users.Add(val)
		}
	}

	// Pass 3: Long-flag passwords (unambiguous, always applied)
	for _, m := range RE_LONG_PASS.FindAllStringSubmatch(cmd, -1) {
		val := StripQuotes(m[1])
		if len(val) > 1 && !IsFileLike(val) {
			result.Passwords.Add(val)
		}
	}

	// Pass 4: Long-flag hashes (unambiguous, always applied)
	for _, m := range RE_LONG_HASH.FindAllStringSubmatch(cmd, -1) {
		result.Hashes.Add(m[1])
	}

	// Pass 5: Per-tool short flags (only for known tools)
	toolFlags := getToolCredFlags(tool)
	if len(toolFlags) > 0 {
		roleFlags := map[string][]string{"user": {}, "pass": {}, "hash": {}}
		for flag, role := range toolFlags {
			roleFlags[role] = append(roleFlags[role], flag)
		}

		for role, flags := range roleFlags {
			if len(flags) == 0 {
				continue
			}
			if role == "hash" {
				sorted := make([]string, len(flags))
				copy(sorted, flags)
				sort.Slice(sorted, func(i, j int) bool {
					return len(sorted[i]) > len(sorted[j])
				})
				escaped := make([]string, len(sorted))
				for i, f := range sorted {
					escaped[i] = regexp.QuoteMeta(f)
				}
				pattern := `(?:^|\s)(?:` + strings.Join(escaped, "|") + `)(?:\s+|=)` +
					`([A-Fa-f0-9]{16,64}(?::[A-Fa-f0-9]{16,64})?)`
				rx := regexp.MustCompile(pattern)
				for _, m := range rx.FindAllStringSubmatch(cmd, -1) {
					result.Hashes.Add(m[1])
				}
			} else {
				rx := BuildFlagRegex(flags)
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
