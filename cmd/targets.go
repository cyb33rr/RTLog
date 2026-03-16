package cmd

import (
	"fmt"
	"os"
	"sort"

	"github.com/cyb33rr/rtlog/internal/display"
	"github.com/cyb33rr/rtlog/internal/extract"
	"github.com/cyb33rr/rtlog/internal/logfile"
	"github.com/spf13/cobra"
)

var targetsCmd = &cobra.Command{
	Use:   "targets",
	Short: "Extract unique IPs, CIDRs, hostnames, ports, and credentials",
	Long:  "Parse all cmd fields to find IP addresses, CIDR ranges, hostnames, ports, and credentials, grouped by type.",
	Run:   runTargets,
}

func init() {
	rootCmd.AddCommand(targetsCmd)
}

// targetCreds tracks per-target credential sets.
type targetCreds struct {
	Users     extract.StringSet
	Passwords extract.StringSet
	Hashes    extract.StringSet
}

func newTargetCreds() *targetCreds {
	return &targetCreds{
		Users:     extract.NewStringSet(),
		Passwords: extract.NewStringSet(),
		Hashes:    extract.NewStringSet(),
	}
}

func runTargets(cmd *cobra.Command, args []string) {
	path, err := logfile.GetLogPath(engagementFlag)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return
	}

	d, err := openEngagementDB()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return
	}
	defer d.Close()

	entries, err := d.LoadAll()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading entries: %v\n", err)
		return
	}

	if len(entries) == 0 {
		fmt.Printf("No entries in %s\n", logfile.EngagementName(path))
		return
	}

	// Maps target -> last-seen entry index (higher = more recent)
	totalIPs := make(map[string]int)
	totalCIDRs := make(map[string]int)
	totalHosts := make(map[string]int)
	totalPorts := make(map[string]int)

	// Per-target credential sets
	tCreds := make(map[string]*targetCreds)

	for i, entry := range entries {
		result := extract.ExtractTargets(entry.Cmd, entry.Tool)
		for ip := range result.IPs {
			totalIPs[ip] = i
		}
		for cidr := range result.CIDRs {
			totalCIDRs[cidr] = i
		}
		for host := range result.Hosts {
			totalHosts[host] = i
		}
		for port := range result.Ports {
			totalPorts[port] = i
		}

		creds := extract.ExtractCreds(entry.Cmd, entry.Tool)
		hasCreds := creds.Users.Len() > 0 || creds.Passwords.Len() > 0 || creds.Hashes.Len() > 0
		if hasCreds {
			// Associate creds with every IP/hostname from the same command
			for target := range result.IPs {
				tc := getOrCreateCreds(tCreds, target)
				mergeCreds(tc, creds)
			}
			for target := range result.Hosts {
				tc := getOrCreateCreds(tCreds, target)
				mergeCreds(tc, creds)
			}
		}
	}

	hasAny := len(totalIPs) > 0 || len(totalCIDRs) > 0 || len(totalHosts) > 0 || len(totalPorts) > 0
	if !hasAny {
		fmt.Println("No targets extracted.")
		return
	}

	fmt.Println(display.Colorize(fmt.Sprintf("--- Targets in %s ---", logfile.EngagementName(path)), display.Bold))

	printTargetSection("IP Addresses", totalIPs, tCreds)
	printTargetSection("CIDR Ranges", totalCIDRs, tCreds)
	printTargetSection("Hostnames", totalHosts, tCreds)
	printTargetSection("Host:Port", totalPorts, tCreds)
}

func getOrCreateCreds(m map[string]*targetCreds, target string) *targetCreds {
	tc, ok := m[target]
	if !ok {
		tc = newTargetCreds()
		m[target] = tc
	}
	return tc
}

func mergeCreds(tc *targetCreds, creds *extract.CredResult) {
	for u := range creds.Users {
		tc.Users.Add(u)
	}
	for p := range creds.Passwords {
		tc.Passwords.Add(p)
	}
	for h := range creds.Hashes {
		tc.Hashes.Add(h)
	}
}

// byRecency sorts targets by their last-seen index descending.
func byRecency(targets map[string]int) []string {
	result := make([]string, 0, len(targets))
	for t := range targets {
		result = append(result, t)
	}
	sort.Slice(result, func(i, j int) bool {
		return targets[result[i]] > targets[result[j]]
	})
	return result
}

func formatCreds(target string, tCreds map[string]*targetCreds) []string {
	tc, ok := tCreds[target]
	if !ok {
		return nil
	}

	var lines []string
	users := tc.Users.Sorted()
	passwords := tc.Passwords.Sorted()
	hashes := tc.Hashes.Sorted()

	if len(users) > 0 {
		for _, u := range users {
			for _, p := range passwords {
				lines = append(lines, u+" : "+p)
			}
			for _, h := range hashes {
				lines = append(lines, u+" : "+h)
			}
			if len(passwords) == 0 && len(hashes) == 0 {
				lines = append(lines, u)
			}
		}
	} else {
		for _, p := range passwords {
			lines = append(lines, p)
		}
		for _, h := range hashes {
			lines = append(lines, h)
		}
	}

	return lines
}

func printTargetSection(title string, targets map[string]int, tCreds map[string]*targetCreds) {
	if len(targets) == 0 {
		return
	}
	fmt.Println()
	fmt.Println(display.Colorize("  "+title, display.Bold))
	for _, target := range byRecency(targets) {
		fmt.Printf("    %s\n", target)
		for _, credLine := range formatCreds(target, tCreds) {
			fmt.Printf("      %s\n", credLine)
		}
	}
}
