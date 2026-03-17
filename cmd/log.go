package cmd

import (
	"fmt"
	"os"
	"os/user"
	"time"

	"github.com/spf13/cobra"

	"github.com/cyb33rr/rtlog/internal/db"
	"github.com/cyb33rr/rtlog/internal/logfile"
	"github.com/cyb33rr/rtlog/internal/match"
	"github.com/cyb33rr/rtlog/internal/state"
)

var (
	logCmdCmd     string
	logCmdExit    int
	logCmdDur     float64
	logCmdOut     string
	logCmdTool    string
	logCmdCwd     string
	logCmdTag     string
	logCmdNote    string
	logCmdOutFile string
	logCmdTTY     string
)

var logCmd = &cobra.Command{
	Use:    "log",
	Hidden: true,
	Short:  "Programmatically log a command entry",
	Long: `Log a command entry directly to the active engagement's SQLite database.

Intended for integration with non-interactive callers (Claude Code hooks,
scripts, automation). The command is matched against tools.conf unless
--tool is explicitly provided.

Exits silently (code 0) if logging is disabled, no engagement is set,
or the command does not match any tracked tool.`,
	Args: cobra.NoArgs,
	Run:  runLog,
}

func init() {
	logCmd.Flags().StringVar(&logCmdCmd, "cmd", "", "full command line (required)")
	logCmd.Flags().IntVar(&logCmdExit, "exit", 0, "command exit code")
	logCmd.Flags().Float64Var(&logCmdDur, "dur", 0, "command duration in seconds")
	logCmd.Flags().StringVar(&logCmdOut, "out", "", "captured stdout/stderr")
	logCmd.Flags().StringVar(&logCmdTool, "tool", "", "tool name (auto-extracted from --cmd if omitted)")
	logCmd.Flags().StringVar(&logCmdCwd, "cwd", "", "working directory (defaults to current)")
	logCmd.Flags().StringVar(&logCmdTag, "tag", "", "override tag (defaults to state file)")
	logCmd.Flags().StringVar(&logCmdNote, "note", "", "override note (defaults to state file)")
	logCmd.Flags().StringVar(&logCmdOutFile, "out-file", "", "read output from file instead of --out")
	logCmd.Flags().StringVar(&logCmdTTY, "tty", "", "TTY device (default: auto-detect or noninteractive)")
	logCmd.MarkFlagRequired("cmd")
	rootCmd.AddCommand(logCmd)
}

func runLog(cmd *cobra.Command, args []string) {
	st := state.ReadState()
	debug := os.Getenv("RTLOG_DEBUG") != ""

	if st[state.KeyEnabled] != "1" {
		if debug {
			fmt.Fprintln(os.Stderr, "[rtlog log] skipped: logging disabled")
		}
		return
	}
	engagement := st[state.KeyEngagement]
	if engagement == "" {
		if debug {
			fmt.Fprintln(os.Stderr, "[rtlog log] skipped: no engagement set")
		}
		return
	}

	tool := logCmdTool
	if tool == "" {
		tool = match.ExtractTool(logCmdCmd)
	}
	if tool == "" {
		if debug {
			fmt.Fprintf(os.Stderr, "[rtlog log] skipped: no tool in cmd %q\n", logCmdCmd)
		}
		return
	}

	if logCmdTool == "" {
		confPath := match.DefaultToolsConf()
		if confPath == "" {
			return
		}
		m, err := match.LoadTools(confPath)
		if err != nil {
			if debug {
				fmt.Fprintf(os.Stderr, "[rtlog log] skipped: cannot load tools.conf: %v\n", err)
			}
			return
		}
		if !m.Match(tool) {
			if debug {
				fmt.Fprintf(os.Stderr, "[rtlog log] skipped: tool %q not in tools.conf\n", tool)
			}
			return
		}
	}

	if debug {
		fmt.Fprintf(os.Stderr, "[rtlog log] logging tool=%q cmd=%q eng=%s\n", tool, logCmdCmd, engagement)
	}

	now := time.Now().UTC()
	cwd := logCmdCwd
	if cwd == "" {
		cwd, _ = os.Getwd()
	}
	tag := logCmdTag
	if !cmd.Flags().Changed("tag") {
		tag = st[state.KeyTag]
	}
	note := logCmdNote
	if !cmd.Flags().Changed("note") {
		note = st[state.KeyNote]
	}

	hostname, _ := os.Hostname()
	username := ""
	if u, err := user.Current(); err == nil {
		username = u.Username
	}

	entry := logfile.LogEntry{
		Ts:    now.Format(time.RFC3339),
		Epoch: now.Unix(),
		User:  username,
		Host:  hostname,
		TTY:   logCmdTTY,
		Cwd:   cwd,
		Tool:  tool,
		Cmd:   logCmdCmd,
		Exit:  logCmdExit,
		Dur:   logCmdDur,
		Tag:   tag,
		Note:  note,
		Out:   logCmdOut,
	}

	if entry.TTY == "" {
		entry.TTY = "noninteractive"
	}

	if logCmdOutFile != "" {
		data, err := os.ReadFile(logCmdOutFile)
		if err != nil {
			fmt.Fprintf(os.Stderr, "warning: could not read out-file: %v\n", err)
		} else {
			entry.Out = string(data)
			os.Remove(logCmdOutFile)
		}
	}

	logDir := os.Getenv("RTLOG_DIR")
	if logDir == "" {
		logDir = logfile.LogDir()
	}
	os.MkdirAll(logDir, 0755)
	d, err := db.Open(logDir, engagement)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	defer d.Close()

	if err := d.Insert(entry); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	// Clear one-shot note if it was used from state
	if !cmd.Flags().Changed("note") && note != "" {
		if _, err := state.UpdateState(map[string]string{state.KeyNote: ""}); err != nil && debug {
			fmt.Fprintf(os.Stderr, "[rtlog log] failed to clear note: %v\n", err)
		}
	}
}
