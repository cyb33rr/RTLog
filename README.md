# RTLog — Red Team Operation Logger

A CLI tool that automatically captures and logs shell commands during penetration testing engagements. Built in Go, it hooks into your zsh shell to silently track red team tool usage with full metadata — timestamps, exit codes, duration, tags, notes, and optionally captured output.

## Features

- **Automatic logging** — zsh hook intercepts commands matching known red team tools
- **Engagement management** — organize logs by project/target
- **Phase tagging** — annotate commands with operational phases (`recon`, `exploitation`, `privesc`, etc.)
- **Output capture** — optionally capture full stdout/stderr for each command
- **Target extraction** — automatically extract IPs, CIDRs, hostnames, ports, and credentials from logged commands with tool-aware parsing to avoid false positives
- **Search & analysis** — search logs, view timelines, get tool usage stats
- **Export** — generate Markdown or CSV reports

## Install

### Go Install

```bash
go install github.com/cyb33rr/rtlog@latest
rtlog setup
```

### Build from Source

```bash
git clone https://github.com/cyb33rr/RTLog.git
cd RTLog
make build      # produces ./rtlog
./rtlog setup
```

`rtlog setup` creates `~/.rt/`, installs the shell hook and tool list, and adds the hook source line to `~/.zshrc`.

### Uninstall

```bash
rtlog uninstall
```

## Quick Start

```bash
# Create an engagement
rtlog new htb-box

# Set a phase tag
rtlog tag recon

# Run your tools as normal — they're logged automatically
nmap -sV 10.10.10.5
gobuster dir -u http://10.10.10.5 -w /usr/share/wordlists/common.txt

# View your log
rtlog show
rtlog tail

# Extract all targets found across commands
rtlog targets

# Export for reporting
rtlog export md
```

## Example

```bash
$ RTID=RTCyb3r smbexec.py cersei.lannister:il0vejaime@10.7.30.10 -debug
[RT-ID] Active: ident=RTCyb3r
Impacket v0.14.0.dev0+20260226.31512.9d3d86ea - Copyright Fortra, LLC and its affiliated companies

[+] Impacket Library Installation Path: /home/kali/venv/lib/python3.10/site-packages/impacket
[+] StringBinding ncacn_np:10.7.30.10[\pipe\svcctl]
[+] Executing %COMSPEC% /Q /c echo cd  ^> \\%COMPUTERNAME%\C$\__output_RTCyb3r 2^>^&1 > %SYSTEMROOT%\RTCyb3r.bat & %COMSPEC% /Q /c %SYSTEMROOT%\RTCyb3r.bat & del %SYSTEMROOT%\RTCyb3r.bat
[!] Launching semi-interactive shell - Careful what you execute
C:\Windows\system32>
```

The `RTID` environment variable sets a red team identifier that Impacket tools use for service/file naming on the target, making artifacts attributable during operations.

## Usage

### Engagement Management

```bash
rtlog new <name>         # Create and switch to a new engagement
rtlog switch <name>      # Switch to an existing engagement
rtlog list               # List all engagements (* = active)
rtlog status             # Show current state (engagement, tag, logging, capture)
```

### Tagging & Notes

```bash
rtlog tag <name>         # Set operational phase tag
rtlog tag --clear        # Clear the current tag
rtlog tag --list         # List all tags with counts
rtlog note <text>        # Attach a one-shot note to the next command
```

### Logging Control

```bash
rtlog on                 # Enable logging
rtlog off                # Disable logging
rtlog capture on         # Enable stdout/stderr capture
rtlog capture off        # Disable capture (metadata only)
```

### Viewing Logs

```bash
rtlog show               # Interactive selector (navigate with ↑/↓, Enter to view output)
rtlog show --today       # Today's entries only
rtlog show --date 2025-01-15
rtlog show -a            # Print all entries with output (non-interactive)
rtlog show -e <name>     # Show a different engagement
rtlog tail               # Last 20 entries, then follow live
rtlog tail -n 50         # Customize tail count
rtlog search <keyword>   # Case-insensitive search with highlighting
```

### Analysis

```bash
rtlog timeline           # Entries grouped by date and tag
rtlog stats              # Totals, success rate, top tools breakdown
rtlog targets            # Extract IPs, hostnames, ports, credentials
```

### Export

```bash
rtlog export md          # Markdown table (default: <engagement>.md)
rtlog export csv         # CSV file (default: <engagement>.csv)
rtlog export md -o report.md
```

### Cleanup

```bash
rtlog clear              # Delete logs for the active engagement
rtlog clear -y           # Skip confirmation
```

## Tracked Tools

RTLog watches for 25+ common red team tools out of the box, configured in `~/.rt/tools.conf`:

`nmap` `gobuster` `ffuf` `nikto` `feroxbuster` `wfuzz` `sqlmap` `curl` `wget` `hydra` `hashcat` `john` `nxc` `crackmapexec` `bloodhound` `kerbrute` `impacket-*` `certipy` `bloodyAD` `rusthound` `responder` `evil-winrm` `chisel` `sshuttle` `searchsploit` `msfconsole` `msfvenom` `enum4linux`

Edit `~/.rt/tools.conf` to add or remove tools. Glob patterns are supported.

## Extraction Config

RTLog uses tool-specific rules when extracting targets and credentials from commands. For example, `-i` means an IP for `evil-winrm` but an interface for `responder`, and `hashcat --user` is a boolean flag, not a username. Built-in rules cover all tracked tools.

To add or override extraction rules for custom tools, create `~/.rt/extract.conf`:

```conf
# One tool per line. Format:
#   tool [positional] [target:flag1,flag2] [cred:flag=role,...] [noextract]
#
# positional   - bare IPs/hostnames are extracted as targets
# target:      - comma-separated short flags for target extraction
# cred:        - comma-separated flag=role pairs (roles: user, pass, hash)
# noextract    - skip all extraction for this tool

# Examples:
mytool positional target:-t cred:-u=user,-p=pass
webtool target:-u
ignoretool noextract
```

A user-defined entry completely overrides the built-in config for that tool. Tools not listed use built-in defaults. Unknown tools (not in built-in config or `extract.conf`) fall back to permissive extraction: bare IPs/hostnames, global long flags (`--target`, `--dc-ip`, etc.), inline patterns, and URL schemes.

## Log Format

Entries are stored as JSONL in `~/.rt/logs/<engagement>.jsonl`:

```json
{
  "ts": "2025-01-15T14:23:01Z",
  "user": "cyb3r",
  "host": "kali",
  "cwd": "/home/cyb3r",
  "tool": "nmap",
  "cmd": "nmap -sV 10.10.10.5",
  "exit": 0,
  "dur": 12.3,
  "tag": "recon",
  "note": "initial scan",
  "out": "..."
}
```

## Environment Variables

| Variable | Default | Description |
|---|---|---|
| `RTLOG_DIR` | `~/.rt/logs` | Override log directory |
| `RTLOG_CAPTURE` | `1` | Override capture setting |
| `RTLOG_DEBUG` | unset | Enable verbose hook debug output |

## Requirements

- **zsh** — the shell hook uses zsh's `preexec`/`precmd`
- **Linux or macOS** — amd64 or arm64

## License

See [LICENSE](LICENSE).
