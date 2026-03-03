package extract

import (
	"testing"
)

func TestExtractTargetsIPv4(t *testing.T) {
	result := ExtractTargets("ping 10.10.10.5", "")
	if !result.IPs.Has("10.10.10.5") {
		t.Errorf("expected IP 10.10.10.5, got %v", result.IPs)
	}
}

func TestExtractTargetsIPv4WithPort(t *testing.T) {
	result := ExtractTargets("connect 192.168.1.1:8080", "")
	if !result.IPs.Has("192.168.1.1") {
		t.Errorf("expected IP 192.168.1.1, got %v", result.IPs)
	}
	if !result.Ports.Has("192.168.1.1:8080") {
		t.Errorf("expected port 192.168.1.1:8080, got %v", result.Ports)
	}
}

func TestExtractTargetsCIDR(t *testing.T) {
	result := ExtractTargets("nmap 10.10.10.0/24", "nmap")
	if !result.CIDRs.Has("10.10.10.0/24") {
		t.Errorf("expected CIDR 10.10.10.0/24, got %v", result.CIDRs)
	}
}

func TestExtractTargetsUserAtHost(t *testing.T) {
	result := ExtractTargets("ssh admin@10.10.10.5", "ssh")
	if !result.IPs.Has("10.10.10.5") {
		t.Errorf("expected IP 10.10.10.5, got %v", result.IPs)
	}
}

func TestExtractTargetsUserAtHostname(t *testing.T) {
	result := ExtractTargets("ssh admin@dc01.corp.local", "ssh")
	if !result.Hosts.Has("dc01.corp.local") {
		t.Errorf("expected host dc01.corp.local, got %v", result.Hosts)
	}
}

func TestExtractTargetsUNC(t *testing.T) {
	result := ExtractTargets(`net use \\dc01.corp.local\share`, "")
	if !result.Hosts.Has("dc01.corp.local") {
		t.Errorf("expected host dc01.corp.local, got %v", result.Hosts)
	}
}

func TestExtractTargetsURL(t *testing.T) {
	result := ExtractTargets("curl https://target.example.com:443/path", "curl")
	if !result.Hosts.Has("target.example.com") {
		t.Errorf("expected host target.example.com, got %v", result.Hosts)
	}
	if !result.Ports.Has("target.example.com:443") {
		t.Errorf("expected port target.example.com:443, got %v", result.Ports)
	}
}

func TestExtractTargetsFlagHost(t *testing.T) {
	result := ExtractTargets("nxc smb --dc-ip 10.10.10.1 -u admin -p pass", "nxc")
	if !result.IPs.Has("10.10.10.1") {
		t.Errorf("expected IP 10.10.10.1, got %v", result.IPs)
	}
}

func TestExtractTargetsSetvar(t *testing.T) {
	result := ExtractTargets("set RHOSTS 10.10.10.5", "msfconsole")
	if !result.IPs.Has("10.10.10.5") {
		t.Errorf("expected IP 10.10.10.5, got %v", result.IPs)
	}
}

func TestExtractTargetsBareHostname(t *testing.T) {
	result := ExtractTargets(" dc01.corp.local ", "")
	if !result.Hosts.Has("dc01.corp.local") {
		t.Errorf("expected host dc01.corp.local, got %v", result.Hosts)
	}
}

func TestExtractTargetsInvalidIPv4(t *testing.T) {
	result := ExtractTargets("999.999.999.999", "")
	if result.IPs.Has("999.999.999.999") {
		t.Errorf("should not extract invalid IP 999.999.999.999")
	}
}

func TestExtractTargetsVersionContext(t *testing.T) {
	result := ExtractTargets("Python/3.10.5", "")
	// 3.10.5 looks like an IP fragment but is version context
	if result.IPs.Len() > 0 {
		t.Errorf("should not extract version-like strings as IPs, got %v", result.IPs)
	}
}

func TestExtractTargetsFileExtension(t *testing.T) {
	result := ExtractTargets("cat output.txt", "cat")
	if result.Hosts.Has("output.txt") {
		t.Errorf("should not extract file names as hosts")
	}
}

func TestExtractTargetsCredentialExclusion(t *testing.T) {
	// Credential values should not be extracted as hosts
	result := ExtractTargets("nxc smb 10.10.10.5 -u cersei.lannister -p 'P@ss'", "nxc")
	if result.IPs.Has("10.10.10.5") {
		// IP should be extracted
	}
	if result.Hosts.Has("cersei.lannister") {
		t.Errorf("credential username should not be extracted as host")
	}
}

func TestExtractTargetsPositionDedup(t *testing.T) {
	// Same IP should only appear once even if matched by multiple passes
	result := ExtractTargets("nmap -h 10.10.10.5 10.10.10.5", "nmap")
	if result.IPs.Len() != 1 {
		t.Errorf("expected 1 unique IP, got %d", result.IPs.Len())
	}
}

func TestExtractTargetsURLWithIP(t *testing.T) {
	result := ExtractTargets("curl http://192.168.1.100:8080/api", "curl")
	if !result.IPs.Has("192.168.1.100") {
		t.Errorf("expected IP 192.168.1.100, got %v", result.IPs)
	}
	if !result.Ports.Has("192.168.1.100:8080") {
		t.Errorf("expected port 192.168.1.100:8080, got %v", result.Ports)
	}
}

func TestExtractTargetsImpacket(t *testing.T) {
	result := ExtractTargets("DOMAIN/admin:password@10.10.10.5", "")
	if !result.IPs.Has("10.10.10.5") {
		t.Errorf("expected IP 10.10.10.5, got %v", result.IPs)
	}
}

func TestExtractTargetsSMBURL(t *testing.T) {
	result := ExtractTargets("smbclient smb://dc01.corp.local/share", "smbclient")
	if !result.Hosts.Has("dc01.corp.local") {
		t.Errorf("expected host dc01.corp.local, got %v", result.Hosts)
	}
}

func TestExtractTargetsMultipleIPs(t *testing.T) {
	result := ExtractTargets("nmap 10.10.10.1 10.10.10.2 10.10.10.3", "nmap")
	if result.IPs.Len() != 3 {
		t.Errorf("expected 3 IPs, got %d: %v", result.IPs.Len(), result.IPs)
	}
}
