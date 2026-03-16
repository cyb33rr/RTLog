# RTLog — Red Team Operation Logger

A CLI tool that automatically captures and logs shell commands during penetration testing engagements. Built in Go, it hooks into your shell (zsh or bash) to silently track red team tool usage with full metadata — timestamps, exit codes, duration, tags, notes, and optionally captured output.

## Features

- **Automatic logging** — shell hook intercepts commands matching known red team tools (zsh and bash)
- **Engagement management** — organize logs by project/target
- **Phase tagging** — annotate commands with operational phases (`recon`, `exploitation`, `privesc`, etc.)
- **Output capture** — optionally capture full stdout/stderr for each command
- **Target extraction** — automatically extract IPs, CIDRs, hostnames, ports, and credentials from logged commands with tool-aware parsing to avoid false positives
- **Search & analysis** — search logs, view timelines, get tool usage stats
- **Export** — generate Markdown or CSV reports
- **SQLite storage** — logs stored in per-engagement SQLite databases with WAL mode for concurrent access
- **JSONL import** — migrate legacy JSONL logs to SQLite with deduplication

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

`rtlog setup` creates `~/.rt/`, installs shell hooks and config files, and configures `~/.zshrc` and/or `~/.bashrc`.

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
rtlog tag                # Show current tag
rtlog tag --clear        # Clear the current tag
rtlog tag --list         # List all tags with entry counts
rtlog note <text>        # Attach a one-shot note to the next command
```

### Logging Control

```bash
rtlog on                 # Enable logging
rtlog off                # Disable logging
rtlog capture on         # Enable stdout/stderr capture
rtlog capture off        # Disable capture (metadata only)
rtlog capture            # Show current capture state
```

### Viewing Logs

```bash
rtlog show               # Interactive selector (navigate with ↑/↓, Enter to view output)
rtlog show --today       # Today's entries only
rtlog show --date 2026-01-15
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

### Export & Import

```bash
rtlog export md          # Markdown table (default: <engagement>.md)
rtlog export csv         # CSV file (default: <engagement>.csv)
rtlog export md -o report.md

rtlog import old.jsonl   # Import legacy JSONL log files into SQLite
```

### Programmatic Logging

The `rtlog log` command is used by shell hooks and scripts to write entries directly:

```bash
rtlog log --cmd "nmap -sV 10.10.10.5" --exit 0 --dur 12.3
rtlog log --cmd "gobuster dir -u http://target" --out "$(cat /tmp/output)"
rtlog log --cmd "ffuf -u http://target/FUZZ" --out-file /tmp/ffuf.out --tty /dev/pts/0
```

| Flag | Description |
|---|---|
| `--cmd` | Full command line (required) |
| `--exit` | Command exit code |
| `--dur` | Duration in seconds |
| `--out` | Captured stdout/stderr |
| `--out-file` | Read output from file instead of `--out` |
| `--tool` | Tool name (auto-extracted from `--cmd` if omitted) |
| `--cwd` | Working directory (defaults to current) |
| `--tag` | Override tag (defaults to state file) |
| `--note` | Override note (defaults to state file) |
| `--tty` | TTY device (default: auto-detect) |

### Cleanup

```bash
rtlog clear              # Delete all entries from the active engagement
rtlog clear -y           # Skip confirmation
```

### Global Flags

```
-e, --engagement <name>  # Specify engagement (defaults to most recent)
-v, --version            # Print version
-h, --help               # Help for any command
```

## Tracked Tools

RTLog watches for 40+ common red team tools out of the box, configured in `~/.rt/tools.conf`:

**Reconnaissance:** `nmap` `gobuster` `ffuf` `nikto` `enum4linux` `feroxbuster` `wfuzz` `searchsploit`

**Web:** `sqlmap` `curl` `wget`

**Exploitation:** `msfconsole` `msfvenom` `hydra`

**Remote Access / File Transfer:** `ssh` `scp` `sftp` `rdesktop` `xfreerdp` `telnet` `ftp` `smbclient` `rpcclient` `winexe`

**Network Utilities:** `nc` `ncat` `socat` `ldapsearch` `snmpwalk` `dig`

**Active Directory:** `nxc` `crackmapexec` `bloodhound` `kerbrute` `impacket-*` `certipy` `bloodyAD` `rusthound` `responder`

**Post-Exploitation:** `evil-winrm` `chisel` `sshuttle`

Edit `~/.rt/tools.conf` to add or remove tools. Glob patterns are supported (e.g., `impacket-*` matches `impacket-psexec`, `impacket-smbexec`, etc.).

## Extraction Config

RTLog uses tool-specific rules when extracting targets and credentials from commands. For example, `-i` means an IP for `evil-winrm` but an interface for `responder`. Built-in rules cover all tracked tools.

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

## Storage Format

Entries are stored in SQLite databases at `~/.rt/logs/<engagement>.db`. Each database uses WAL mode for concurrent read/write access.

**Schema:**

| Column | Type | Description |
|---|---|---|
| `id` | INTEGER | Auto-increment primary key |
| `ts` | TEXT | ISO 8601 timestamp |
| `epoch` | INTEGER | Unix timestamp |
| `user` | TEXT | Username |
| `host` | TEXT | Hostname |
| `tty` | TEXT | TTY device |
| `cwd` | TEXT | Working directory |
| `tool` | TEXT | Tool name |
| `cmd` | TEXT | Full command |
| `exit` | INTEGER | Exit code |
| `dur` | REAL | Duration in seconds |
| `tag` | TEXT | Phase tag |
| `note` | TEXT | User annotation |
| `out` | TEXT | Captured output |

Indices on `epoch`, `tool`, and `tag` for fast queries.

Legacy JSONL logs can be migrated with `rtlog import <file.jsonl>`.

## Environment Variables

| Variable | Default | Description |
|---|---|---|
| `RTLOG_DIR` | `~/.rt/logs` | Override log directory |
| `RTLOG_CAPTURE` | `1` | Override capture setting |
| `RTLOG_DEBUG` | unset | Enable verbose hook debug output |

## Requirements

- **zsh or bash** — shell hooks use `preexec`/`precmd` (zsh) or `bash-preexec` (bash)
- **Linux or macOS** — amd64 or arm64

## License

See [LICENSE](LICENSE).
