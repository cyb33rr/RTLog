# Code Review Fix Plan — 34 Issues

All items are checkable. Group by file to minimize context switches.
Each item has the exact change needed so implementation requires no re-reading.

---

## hook-noninteractive.bash

- [ ] **#1 (P0)** Remove `local` from brace groups (lines 19, 42). Bash `local` is only valid inside a function. The two `{ }` blocks at top-level use `local key val` (line 19) and `local line` (line 42). Remove the `local` keyword from both — just use plain `key val` and `line` assignments. The variables are already scoped enough since the blocks are top-level.
- [ ] **#10 (P1)** Replace `date +%s.%N` with a portable fallback at line 103 (`_rtlog_ni_pending_start`) and line 130 (duration calculation). `%N` is not available on macOS (BSD date). Pattern: `date +%s.%N 2>/dev/null || date +%s`. For the awk duration: gracefully handle if start is an integer (`awk "BEGIN {printf \"%.1f\", $(date +%s 2>/dev/null) - $_rtlog_ni_pending_start}"` as fallback).
- [ ] **#30 (P3)** Replace predictable temp file name `_rtlog_ni_outfile="/tmp/.rtlog_ni_out.$$"` (line 71) with `_rtlog_ni_outfile=$(mktemp /tmp/.rtlog_ni_out.XXXXXXXX)` or create it lazily in the debug handler via `mktemp`.

---

## hook.bash

- [ ] **#10 (P1)** Replace `date +%s.%N` at line 139 (`_rtlog_pending_start`) and line 174 (`$(date +%s.%N)` in awk duration) with portable fallback: `date +%s.%N 2>/dev/null || date +%s`.
- [ ] **#11 (P1)** Tee race condition at line 146-148: after `exec > >(tee …)`, restore fds in `_rtlog_save_rc` (line 157) needs a brief sleep after closing the tee pipe so tee flushes before `rtlog log` reads the file. In `_rtlog_save_rc`, after the fd restore block, add `command sleep 0.05 2>/dev/null` (same pattern already in `hook-noninteractive.bash:123`).
- [ ] **#30 (P3)** Replace `_rtlog_tmpfile="/tmp/.rtlog_out.$$"` (line 62) with `_rtlog_tmpfile=$(mktemp /tmp/.rtlog_out.XXXXXXXX)` set once at source time, or lazily per capture via mktemp.

---

## hook.zsh

- [ ] **#11 (P1)** Tee race condition at lines 141-143: same as hook.bash. After fd restore in `_rtlog_save_rc` (lines 151-157), add `command sleep 0.05 2>/dev/null`.
- [ ] **#30 (P3)** Replace `_rtlog_tmpfile="/tmp/.rtlog_out.$$"` (line 59) with `_rtlog_tmpfile=$(mktemp /tmp/.rtlog_out.XXXXXXXX)`.

---

## extract.conf

- [ ] **#3 (P1)** xfreerdp cred flags at line 39 use `/u:=user,/p:=pass` format. The colon is part of the flag itself (xfreerdp uses `/u:value` not `/u value`). The current `BuildFlagRegex` pattern is `(?:\s+|=)(\S+)` which matches space or `=` separator — it never matches `:value`. Add a colon-joined variant to the pattern or change the config to use a flag format that already works. The fix is in `extract.conf` line 39: change cred spec to use `/u` and `/p` as flags (strip the trailing `:` from flag names) AND update `BuildFlagRegex` in `regex.go` to also match `:` as a separator (`:` between flag and value). Specifically:
  - In `extract.conf` line 39: `xfreerdp positional cred:/u:=user,/p:=pass` → change flag names to `/u:` and `/p:` (keep colon as part of flag identifier) OR strip colon and rely on `:` separator in regex.
  - In `internal/extract/regex.go` `BuildFlagRegex`: change pattern from `(?:\s+|=)(\S+)` to `(?:\s+|=|:)(\S+)` — but this is risky for other flags. Better approach: add a separate `BuildColonFlagRegex` for xfreerdp-style `/flag:value` flags, OR handle by including the trailing `:` in the escaped flag name so the regex literal matches `/u:` followed by `(\S+)`.
  - Cleanest: keep `extract.conf` as `/u:` and `/p:` (flag already ends in `:`), and change the `BuildFlagRegex` pattern `(?:\s+|=)` to `(?:\s+|=|(?<=:))` — not valid in RE2. Instead: change separator to `(?:\s+|=|:?)` so that `:` is an optional separator after the flag.
  - **Actual fix**: In `regex.go` `BuildFlagRegex` line 99, change `(?:\s+|=)` to `(?:\s+|=|:)` in the variant used for xfreerdp, implemented as a new `BuildColonFlagRegex(flags []string)` function. Update `config.go` `compileToolRegexes` to call the colon variant when flags contain `:` suffix.
  - Simplest working fix: In `regex.go`, add a new exported function `BuildColonFlagRegex` that uses `(?:\s+|=|:)(\S+)` separator. In `config.go:compileToolRegexes`, detect flags ending in `:` and route them through `BuildColonFlagRegex` instead. In `extract.conf`, keep `/u:` and `/p:` as-is (flag names with colon suffix).

---

## internal/extract/regex.go

- [ ] **#3 (P1)** Add `BuildColonFlagRegex(flags []string) *regexp.Regexp` function. Pattern: same as `BuildFlagRegex` but uses `(?:\s+|=|:)(\S+)` as separator. The caller (`config.go:compileToolRegexes`) will select this when any flag ends with `:`.

---

## internal/extract/targets.go

- [ ] **#4 (P1)** Fix position tracker calls in passes 4-6. In each of the following call sites, the end position arg uses `m[1]` (total match end) instead of `m[3]` (capture group 1 end). The correct end for the claimed range should be `m[3]` (end of the host string, not end of full regex match which includes port group). Fix all 8 call sites:
  - Pass 4 (RE_URL_HOST loop), lines 320 and 322: `addIP(host, portStr, "", m[2], m[1])` → `addIP(host, portStr, "", m[2], m[3])` and `addHost(host, portStr, m[2], m[1])` → `addHost(host, portStr, m[2], m[3])`
  - Pass 5a (RE_GLOBAL_FLAG_HOST loop), lines 334 and 336: same fix pattern.
  - Pass 5b (toolTargetRegexes loop), lines 349 and 351: same fix pattern.
  - Pass 6 (RE_SETVAR_HOST loop), lines 365 and 367: same fix pattern.
- [ ] **#29 (P3)** `isVersionContext` at line 135-140: the `/` check incorrectly filters `--flag=/value` (e.g. `--output=/tmp/result`). Currently, when `prev == '/'`, it checks if the text before is non-space, which makes `--flag=/value` match as version context. Narrow the check: only return true if `prev == '/'` AND the text before the slash does NOT end with `=` (i.e. not a flag assignment). A simpler guard: only fire the `/` branch if there is no `=` immediately before the `/`. Change the condition to: first check `cmd[matchStart-2] != '='` (or walk back to find the char before `/`).

---

## internal/extract/config.go

- [ ] **#28 (P3)** Validate role values in `LoadConfigBytes` (line 128-131). Currently, `flag != "" && role != ""` passes any non-empty role string, treating unknown roles as password. Add explicit validation: `if role != "user" && role != "pass" && role != "hash" { log.Printf("warn: unknown cred role %q for flag %q in tool %q", role, flag, toolName); continue }`. Use `fmt.Fprintf(os.Stderr, …)` rather than `log` to avoid importing log.

---

## internal/extract/creds.go

- [ ] **#13 (P2)** `RE_CRED_INLINE` (line 22-24) drops `user:pass@host:port`. The current pattern ends with `([A-Za-z0-9._-]+)(?:\s|$)` which does not allow a port after the host. Change to `([A-Za-z0-9._-]+)(?::\d{1,5})?(?:\s|$)` — add optional `(?::\d{1,5})?` before the final `(?:\s|$)`.

---

## internal/db/db.go

- [ ] **#6 (P1)** Add `PRAGMA busy_timeout = 5000` after WAL mode pragma (after line 59). Insert: `if _, err := conn.Exec("PRAGMA busy_timeout = 5000"); err != nil { conn.Close(); return nil, fmt.Errorf("set busy_timeout: %w", err) }`.
- [ ] **#14 (P2)** Wrap schema init in a transaction (lines 72-81). Replace the two bare `conn.Exec(schemaSQL)` and `conn.Exec("PRAGMA user_version = …")` calls with: begin tx, exec schema, exec version, commit. Pattern: `tx, err := conn.Begin(); if err != nil { … }` then `tx.Exec(schemaSQL)`, `tx.Exec("PRAGMA user_version = …")`, `tx.Commit()`.
- [ ] **#16 (P2)** Detect schema version mismatch (line 64-70). After reading `version`, add: `if version > schemaVersion { fmt.Fprintf(os.Stderr, "warning: database schema version %d > expected %d; some features may not work correctly\n", version, schemaVersion) }`. Do not error-exit — just warn.
- [ ] **#18 (P2)** `Clear` (line 153) doesn't reset AUTOINCREMENT counter. After `DELETE FROM entries`, also exec `DELETE FROM sqlite_sequence WHERE name = 'entries'`. Wrap both in a single transaction or just add the second exec after the first.
- [ ] **#19 (P3)** `os.MkdirAll(dir, 0755)` at line 48 — change to `0700` since `~/.rt/` is a private directory containing logs and credentials.

---

## cmd/log.go

- [ ] **#5 (P1)** `os.Remove(logCmdOutFile)` at line 160 runs unconditionally even when `os.ReadFile` returned an error. Move the `os.Remove` inside the `else` branch (success path only). After the fix, the file is only deleted if reading succeeded and the data was stored in `entry.Out`. Pattern:
  ```go
  if logCmdOutFile != "" {
      data, err := os.ReadFile(logCmdOutFile)
      if err != nil {
          fmt.Fprintf(os.Stderr, "warning: could not read out-file: %v\n", err)
      } else {
          entry.Out = string(data)
          os.Remove(logCmdOutFile)
      }
  }
  ```

---

## cmd/importcmd.go

- [ ] **#2 (P0)** Path traversal via engagement name (line 33). After `eng := strings.TrimSuffix(base, ".jsonl")`, validate `eng` using the existing `logfile.ValidateEngagementName(eng)` function. If validation fails, print error to stderr and return early. The `logfile.ValidateEngagementName` is already exported and handles all bad name patterns (dots, slashes, etc.). Note: `filepath.Base()` is already called via `base := filepath.Base(path)` on line 32, so the base isolation is done; the missing step is name validation.
- [ ] **#15 (P2)** `scanner.Err()` not checked after the scan loop (line 81). After the `for scanner.Scan()` loop, add: `if err := scanner.Err(); err != nil { fmt.Fprintf(os.Stderr, "warning: read error in %s: %v\n", path, err) }`.
- [ ] **#25 (P3)** Dedup key uses `|` separator (line 94) which could collide if cmd/tool/cwd contain `|`. Change to null byte separator: `fmt.Sprintf("%d\x00%s\x00%s\x00%s", e.Epoch, e.Cmd, e.Tool, e.Cwd)`. Alternatively use a hash: `fmt.Sprintf("%d|%x", e.Epoch, sha256.Sum256([]byte(e.Cmd+e.Tool+e.Cwd)))`. Null byte is simpler and safe since Go strings can contain nulls but shell-sourced JSONL values cannot.

---

## cmd/tail.go

- [ ] **#8 (P1)** `logfile.GetLogPath(engagementFlag)` at line 24 calls `os.Exit(1)` on failure, bypassing deferred cleanup. Replace with `openEngagementDB()` (already used in other cmd files) which returns `(dir, eng, error)` or restructure: use the same `openEngagementDB()` helper that already exists in the codebase for consistent error handling. Look at how other commands handle this — `runTargets` at cmd/targets.go line 42 uses `openEngagementDB()`. Refactor `tail.go` to use `openEngagementDB()` and print error + return instead of letting GetLogPath call os.Exit. If `openEngagementDB()` is not accessible (same package), inline the pattern: call `logfile.AvailableEngagements()` to check empty list first.
  - **Concrete change**: replace lines 24-26 with the `openEngagementDB()` call pattern used in other commands, handle error gracefully. Also remove the unused `logPath` variable or refactor so the engagement name comes from `openEngagementDB`.

---

## cmd/targets.go

- [ ] **#17 (P2)** Errors printed to stdout instead of stderr (lines 44, 51). Change `fmt.Printf("error: %v\n", err)` → `fmt.Fprintf(os.Stderr, "error: %v\n", err)` and `fmt.Printf("Error loading entries: %v\n", err)` → `fmt.Fprintf(os.Stderr, "Error loading entries: %v\n", err)`.

---

## cmd/setup.go

- [ ] **#26 (P3)** PATH export check at line 344 uses `strings.Contains(trimmed, ...)` which would match commented lines like `# export PATH="$HOME/.local/bin:$PATH"`. Change to `strings.HasPrefix(trimmed, "export PATH=")` — this correctly requires the line to start with the export keyword (after TrimSpace already strips leading whitespace, so commented lines starting with `#` won't match).

---

## internal/logfile/logfile.go

- [ ] **#7 (P2)** `GetLogPath` calls `os.Exit(1)` (lines 121, 128-129). Change signature to `GetLogPath(engagement string) (string, error)` and return errors instead. Replace all `os.Exit(1)` with `return "", fmt.Errorf(…)`. Update all callers that currently use `logfile.GetLogPath(engagementFlag)` without checking errors — they will need to handle the returned error and call `os.Exit(1)` themselves. Callers: `cmd/tail.go` (already covered by #8), `cmd/targets.go` (line 40, but note `openEngagementDB` is used after — check actual usage), and any other cmd files. Search all callers with `grep -r "GetLogPath"` before implementing.
- [ ] **#21 (P3)** Silent fallback to relative path in `LogDir()` (line 57-60). When `os.UserHomeDir()` fails, log a warning before returning the fallback: `fmt.Fprintf(os.Stderr, "warning: cannot determine home directory (%v), using relative path\n", err)`.

---

## internal/state/state.go

- [ ] **#20 (P3)** `os.MkdirAll(dir, 0755)` at line 71 — change to `0700` since `~/.rt/` contains private state, logs, and credentials.

---

## internal/match/match.go

- [ ] **#9 (P2)** `sudo --user=root` returns wrong tool name. The sudo flag-skipping loop at line 76-82 only handles short flags with argument (`-u`, `-g`, `-C`, `-D`, `-R`, `-T`) but does not skip long flags like `--user=root` or `--group=sudo`. Add a loop branch: if `fields[0]` starts with `--`, consume the word (long flags with `=` are self-contained, long flags without `=` need to consume next arg). Specifically after the current short-flag inner loop, before `continue`, add handling: if the remaining first token starts with `--`, skip it (it either has `=` embedded, so self-contained, or takes a value — skip both to be safe).
- [ ] **#22 (P3)** Incomplete sudo flag list (line 80): add `-p`, `-r`, `-t`, `-U` to the `ContainsAny` character set. Change `"ugCDRT"` to `"ugCDRTprtU"`.
- [ ] **#23 (P3)** Malformed glob patterns silently ignored (line 36-38). In `LoadTools`, when a glob pattern is added, validate it with `filepath.Match(line, "")` — if it returns an error (malformed pattern), print a warning and skip. Pattern: `if _, err := filepath.Match(line, ""); err != nil { fmt.Fprintf(os.Stderr, "warning: invalid glob pattern %q in tools.conf: %v\n", line, err); continue }`.

---

## internal/timeutil/parse.go

- [ ] **#32 (P3)** RFC3339 is redundant before RFC3339Nano (line 10). `time.RFC3339Nano` matches all strings that `time.RFC3339` matches (it falls back to no nanoseconds). Remove `time.RFC3339` from the layouts slice. The slice should be: `[]string{time.RFC3339Nano, "2006-01-02T15:04:05"}`.
- [ ] **#24 (P3)** No-timezone layout `"2006-01-02T15:04:05"` parsed as UTC (line 12). Use `time.ParseInLocation` with `time.Local` instead of `time.Parse` for this layout only. Change the parse loop to detect which layout is being tried and use `ParseInLocation` for the no-tz layout.

---

## internal/export/markdown.go

- [ ] **#12 (P2)** `tag` (line 26) and `tool` (line 20) not pipe-escaped. `note` newlines not converted to `<br>` (line 27 does `escapePipe` but not `strings.ReplaceAll(note, "\n", "<br>")`). Fix:
  - Line 20: `tool := escapePipe(e.Tool)` (instead of bare `tool := e.Tool`)
  - Line 26: `tag := escapePipe(e.Tag)` (instead of bare `tag := e.Tag`)
  - Line 27: `note := escapePipe(strings.ReplaceAll(e.Note, "\n", "<br>"))` (add newline→br before pipe-escape, or after — order doesn't matter since `<br>` contains no pipes)

---

## internal/display/color.go

- [ ] **#31 (P2)** `RE_ANSI` regex (line 26) does not handle OSC sequences with ST terminator `\x1b\\` (only handles BEL `\x07`). Change pattern `\x1b\].*?\x07` to `\x1b\].*?(?:\x07|\x1b\\)` to cover both BEL and String Terminator variants. Also the current pattern does not handle OSC sequences that may span with `\x9c` (C1 ST). The minimal fix: `\x1b\].*?(?:\x07|\x1b\\)`.

---

## internal/display/selector.go

- [ ] **#27 (P3)** Footer shows `1/0` when entries list is empty (line 215). `s.cursor+1` is 1 when cursor=0 even if `len(s.entries)` is 0. Change: `fmt.Sprintf(" %d/%d …", s.cursor+1, len(s.entries), …)` → use a conditional: if `len(s.entries) == 0`, use `0/0`; otherwise `s.cursor+1/len(s.entries)`. Pattern: `curPos := s.cursor + 1; if len(s.entries) == 0 { curPos = 0 }`.
- [ ] **#31 (P2)** `truncateVisible` (line 118-145) has the same OSC/ST issue as `RE_ANSI` — the ANSI skip logic in `truncateVisible` only handles CSI sequences (ESC followed by `[`), not OSC sequences (ESC followed by `]`). Extend the `inEsc` logic to also handle OSC: when `r == ']'` (after ESC), enter a separate OSC mode that reads until BEL (`\x07`) or ST (`ESC \`). Add an `inOSC bool` state variable that is set when ESC+`]` is seen, and cleared when `\x07` or the second char of `\x1b\\` is seen.

---

## cmd/log_test.go

- [ ] **#33 (P3)** Redundant `defer os.Setenv` at line 37. `t.Setenv("HOME", tmpDir)` on line 36 already handles restore on test cleanup. Remove the manual `defer os.Setenv("HOME", origHome)` at line 37. Also remove `origHome := os.Getenv("HOME")` at line 35 since it's only used in the now-removed defer. (Same pattern in the second test at lines 82-84.)
- [ ] **#34 (P3)** `rootCmd.Execute()` error not checked at line 40. Change to `if err := rootCmd.Execute(); err != nil { t.Fatalf("Execute() failed: %v", err) }`. (Same at line 87 in the second test.)

---

## Summary by priority

| Priority | Count | Items |
|----------|-------|-------|
| P0 Critical | 2 | #1 (bash local), #2 (path traversal) |
| P1 High | 9 | #3 (xfreerdp regex), #4 (position tracker), #5 (Remove on error), #6 (busy_timeout), #8 (tail exit), #10 (date %N), #11 (tee race) |
| P2 Medium | 12 | #7 (GetLogPath), #9 (sudo long flags), #12 (markdown escape), #13 (inline creds port), #14 (schema tx), #15 (scanner.Err), #16 (version warn), #17 (stderr), #18 (AUTOINCREMENT), #31 (OSC escapes) |
| P3 Low | 13 | #19-#32, #33, #34 |

## Implementation order (recommended)

1. P0: #1, #2
2. P1: #4, #5, #6, #3+regex.go+extract.conf (together), #8, #10, #11
3. P2: #7+callers (large refactor), #14, #16, #18, #12, #13, #15, #17, #9, #31
4. P3: #19, #20, #21, #22, #23, #24, #25, #26, #27, #28, #29, #30, #32, #33, #34

## Cross-cutting notes

- **#7 (GetLogPath refactor)**: Before implementing, run `grep -rn "GetLogPath" /home/cyb3r/rtlog/cmd/` to find all callers. They are: `cmd/tail.go:24`, `cmd/targets.go:40`, and potentially others. Each caller must switch to error-return handling. `cmd/targets.go:40` calls `GetLogPath` then immediately calls `openEngagementDB()` — the `GetLogPath` call at line 40 seems leftover/unused (it captures `path` used only for name display). Verify whether `path` is needed at all or if `openEngagementDB()` already provides the engagement name.
- **#3 + #regex.go**: These two items must be implemented together. The extract.conf change alone does nothing without the regex change.
- **#4 (position tracker)**: Eight specific lines. After the fix, `m[3]` is the end of the first capture group (the host string), which is the correct end position to claim. The current `m[1]` (total match end) causes over-claiming and prevents legitimate subsequent matches.
- **#30 (mktemp)**: All three hook files (hook.bash, hook.zsh, hook-noninteractive.bash) need this change. In hook-noninteractive.bash, the outfile is also used as a persistent name (checked in the exit handler), so mktemp at init time is correct. Note mktemp creates the file — ensure it is truncated before use (`: > "$_rtlog_ni_outfile"` already does this).
