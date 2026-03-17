# RTLog Red Team Gap Analysis — Design Spec

**Date:** 2026-03-17
**Context:** Full red team operations (long-duration, multi-phase, stealth-focused with C2 infrastructure). Solo operator now, team collaboration as a future goal. Attack infrastructure is trusted — no OPSEC-of-tool concerns.

## Changes

### 1. Fix tools.conf / extract.conf mismatch

**Problem:** 7 tools have extraction rules in `extract.conf` but are absent from `tools.conf`. Their commands are never logged and the extraction rules never fire.

**Action:** Add the following to `tools.conf`:

```
enum4linux-ng
smbmap
plink
ldapdomaindump
ldeep
windapsearch
adidnsdump
```

**Placement in tools.conf:**
- `enum4linux-ng` — under Reconnaissance, next to `enum4linux`
- `smbmap` — new "SMB / RPC" section or under Network Utilities
- `plink` — under Remote Access / File Transfer
- `ldapdomaindump`, `ldeep`, `windapsearch`, `adidnsdump` — under Active Directory

### 2. Add missing target-interactive tools to tools.conf

**Problem:** Several common red team tools that interact with the target environment are not tracked.

**Action:** Add the following to `tools.conf`:

```
nuclei
httpx
testssl.sh
subfinder
amass
ligolo-ng
ligolo
bore
gost
```

**Placement in tools.conf:**
- `nuclei`, `httpx`, `testssl.sh` — under Web (or new "Web Recon" section)
- `subfinder`, `amass` — under Reconnaissance
- `ligolo-ng`, `ligolo` — under Post-Exploitation (alongside chisel, sshuttle)
- `bore`, `gost` — under Post-Exploitation

**extract.conf:** No new extraction rules needed for these initially. Unknown tools already fall back to global long flags (`--url`, `--host`, `--target`, etc.) and URL scheme extraction, which covers the common cases. Tool-specific extraction rules can be added later if false positives or missed targets appear in practice.

### 3. Remove non-operational tools from tools.conf

**Problem:** Some tracked tools don't interact with the target environment and generate timeline noise.

**Action:** Remove from `tools.conf`:

```
searchsploit   # local exploit-db search, no network interaction
dig            # general DNS utility, too noisy for routine use
socat          # general-purpose utility, not specifically a red team tool
```

**Note:** `dig` and `socat` removal is opinionated. If DNS recon or port forwarding logging is desired for a specific engagement, the user can re-add them to their local `~/.rt/tools.conf`. The embedded defaults should track tools that are unambiguously operational.

### 4. Add JSONL export

**Problem:** No structured export format for integration with reporting platforms (Ghostwriter, SysReptor, PlexTrac) or custom scripting pipelines. Markdown and CSV exist but aren't machine-friendly.

**Action:** Add `jsonl` as an export format.

**CLI interface:**
```
rtlog export jsonl [-o file]
```

Follows the same pattern as existing `md` and `csv` exports.

**Output format:** One JSON object per line, all fields included:

```json
{"id":1,"ts":"2026-03-17T14:30:00Z","epoch":1742222400,"user":"cyb3r","host":"attack-box","tty":"/dev/pts/0","cwd":"/opt/tools","tool":"nmap","cmd":"nmap -sV -sC 10.10.1.0/24","exit":0,"dur":45.2,"tag":"recon","note":"initial network sweep","out":""}
```

**Implementation:**
- New function in `internal/export/jsonl.go`: `ExportJSONL(entries []logfile.LogEntry) string`
- Uses `encoding/json` to marshal each entry — no external dependencies
- Update `cmd/export.go` to accept `jsonl` as a valid format alongside `md` and `csv`
- Default output filename: `<engagement>.jsonl`

### 5. Add entry-level delete and edit

**Problem:** `rtlog clear` is the only mutation — it deletes all entries. No way to remove a single accidental entry or fix a note/tag typo.

#### 5a. Delete single entry

**CLI interface:**
```
rtlog delete <id> [-y]
```

- Deletes the entry with the given ID
- Shows the entry before deletion and asks for confirmation (unless `-y` flag)
- Prints confirmation message after deletion

**Implementation:**
- New file `cmd/delete.go`
- New DB method: `func (d *DB) Delete(id int64) error`
  - `DELETE FROM entries WHERE id = ?`
- New DB method: `func (d *DB) GetByID(id int64) (*logfile.LogEntry, error)` (for showing the entry before deletion)

#### 5b. Edit entry metadata

**CLI interface:**
```
rtlog edit <id> [--note TEXT] [--tag TEXT]
```

- Only `note` and `tag` are editable — the command, timestamp, exit code, duration, and other operational fields are immutable record
- Shows the entry after editing
- At least one of `--note` or `--tag` must be provided

**Implementation:**
- New file `cmd/edit.go`
- New DB method: `func (d *DB) Update(id int64, fields map[string]string) error`
  - Builds a dynamic `UPDATE entries SET ... WHERE id = ?` from the provided fields
  - Only allows `note` and `tag` columns (validated in code, not user input in SQL)
- Reuses `GetByID` from 5a

## Out of scope

The following were evaluated and explicitly excluded:

- **C2 framework integration** — C2 commands happen outside the shell; capturing them would require per-framework plugins (Sliver BOF, Cobalt Strike aggressor scripts). Different problem, different solution.
- **Scope awareness / out-of-scope alerting** — useful but adds complexity disproportionate to value for a solo operator.
- **MITRE ATT&CK TTP mapping** — reporting concern, not a logging concern. Can be added post-hoc by external tooling consuming the JSONL export.
- **Findings/objectives tracking** — different data model (findings link to multiple commands). Better suited for a reporting tool consuming RTLog data.
- **Screenshot/evidence linking** — file management concern, not a logging concern.
- **Session/pivot tracking** — requires inferring topology from commands, which is fragile. Better done manually in notes.
- **Multi-operator / team sync** — future goal but a major architectural change. Current single-user SQLite model is correct for now.
- **At-rest encryption** — attack infra is trusted.

## Testing

- **tools.conf changes:** Verify all added tools are logged by shell hook (run a command, check `rtlog show`)
- **JSONL export:** Unit test in `internal/export/jsonl_test.go` — round-trip: create entries, export to JSONL, parse each line back, assert equality
- **Delete:** Unit test in `internal/db/db_test.go` — insert entry, delete by ID, verify count drops, verify entry gone
- **Edit:** Unit test in `internal/db/db_test.go` — insert entry, update note/tag, reload, verify changes
- **Removal:** Verify searchsploit/dig/socat commands are no longer captured after removal
