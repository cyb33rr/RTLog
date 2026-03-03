package extract

import (
	"testing"
)

func TestExtractCredsInline(t *testing.T) {
	result := ExtractCreds("DOMAIN/admin:P@ssw0rd@10.10.10.5", "")
	if !result.Users.Has("admin") {
		t.Errorf("expected user admin, got %v", result.Users)
	}
	if !result.Passwords.Has("P@ssw0rd") {
		t.Errorf("expected password P@ssw0rd, got %v", result.Passwords)
	}
}

func TestExtractCredsNxc(t *testing.T) {
	result := ExtractCreds("nxc smb 10.10.10.5 -u admin -p 'P@ss'", "nxc")
	if !result.Users.Has("admin") {
		t.Errorf("expected user admin, got %v", result.Users)
	}
	if !result.Passwords.Has("P@ss") {
		t.Errorf("expected password P@ss, got %v", result.Passwords)
	}
}

func TestExtractCredsLongFlags(t *testing.T) {
	result := ExtractCreds("tool --username admin --password secret", "")
	if !result.Users.Has("admin") {
		t.Errorf("expected user admin, got %v", result.Users)
	}
	if !result.Passwords.Has("secret") {
		t.Errorf("expected password secret, got %v", result.Passwords)
	}
}

func TestExtractCredsHash(t *testing.T) {
	hash := "aad3b435b51404eeaad3b435b51404ee:31d6cfe0d16ae931b73c59d7e0c089c0"
	result := ExtractCreds("nxc smb 10.10.10.5 -u admin -H "+hash, "nxc")
	if !result.Users.Has("admin") {
		t.Errorf("expected user admin, got %v", result.Users)
	}
	if !result.Hashes.Has(hash) {
		t.Errorf("expected hash %s, got %v", hash, result.Hashes)
	}
}

func TestExtractCredsLongHash(t *testing.T) {
	hash := "aad3b435b51404eeaad3b435b51404ee:31d6cfe0d16ae931b73c59d7e0c089c0"
	result := ExtractCreds("tool --hashes "+hash, "")
	if !result.Hashes.Has(hash) {
		t.Errorf("expected hash %s, got %v", hash, result.Hashes)
	}
}

func TestExtractCredsSetvar(t *testing.T) {
	result := ExtractCreds("set USERNAME admin", "msfconsole")
	if !result.Users.Has("admin") {
		t.Errorf("expected user admin, got %v", result.Users)
	}
}

func TestExtractCredsSetvarPassword(t *testing.T) {
	result := ExtractCreds("set PASSWORD secret123", "msfconsole")
	if !result.Passwords.Has("secret123") {
		t.Errorf("expected password secret123, got %v", result.Passwords)
	}
}

func TestExtractCredsEvilWinrm(t *testing.T) {
	result := ExtractCreds("evil-winrm -i 10.10.10.5 -u admin -p 'P@ssword'", "evil-winrm")
	if !result.Users.Has("admin") {
		t.Errorf("expected user admin, got %v", result.Users)
	}
	if !result.Passwords.Has("P@ssword") {
		t.Errorf("expected password P@ssword, got %v", result.Passwords)
	}
}

func TestExtractCredsQuotedValues(t *testing.T) {
	result := ExtractCreds(`nxc smb 10.10.10.5 -u "admin" -p "P@ss"`, "nxc")
	if !result.Users.Has("admin") {
		t.Errorf("expected user admin, got %v", result.Users)
	}
	if !result.Passwords.Has("P@ss") {
		t.Errorf("expected password P@ss, got %v", result.Passwords)
	}
}

func TestExtractCredsFilePathNotCred(t *testing.T) {
	result := ExtractCreds("nxc smb 10.10.10.5 -u /path/to/users.txt -p /path/to/pass.txt", "nxc")
	if result.Users.Has("/path/to/users.txt") {
		t.Errorf("file paths should not be extracted as users")
	}
	if result.Passwords.Has("/path/to/pass.txt") {
		t.Errorf("file paths should not be extracted as passwords")
	}
}

func TestExtractCredsSSH(t *testing.T) {
	result := ExtractCreds("ssh -l admin 10.10.10.5", "ssh")
	if !result.Users.Has("admin") {
		t.Errorf("expected user admin, got %v", result.Users)
	}
}

func TestExtractCredsGobuster(t *testing.T) {
	result := ExtractCreds("gobuster dir -u http://target -U admin -P secret", "gobuster")
	if !result.Users.Has("admin") {
		t.Errorf("expected user admin, got %v", result.Users)
	}
	if !result.Passwords.Has("secret") {
		t.Errorf("expected password secret, got %v", result.Passwords)
	}
}

func TestExtractCredsUnknownTool(t *testing.T) {
	// Unknown tool should only use long flags and inline patterns
	result := ExtractCreds("unknowntool -u admin -p pass", "unknowntool")
	// -u/-p are short flags, should NOT be picked up for unknown tools
	if result.Users.Has("admin") {
		t.Errorf("short flags should not be used for unknown tools")
	}
}

func TestExtractCredsSetvarNotForNonMsf(t *testing.T) {
	result := ExtractCreds("set USERNAME admin", "nxc")
	if result.Users.Has("admin") {
		t.Errorf("set-variable patterns should only match for msfconsole or empty tool")
	}
}

func TestExtractCredsRusthoundLongFlags(t *testing.T) {
	result := ExtractCreds("rusthound --ldapusername admin --ldappassword secret", "rusthound")
	if !result.Users.Has("admin") {
		t.Errorf("expected user admin, got %v", result.Users)
	}
	if !result.Passwords.Has("secret") {
		t.Errorf("expected password secret, got %v", result.Passwords)
	}
}
