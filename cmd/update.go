package cmd

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/spf13/cobra"
)

var updateCmd = &cobra.Command{
	Use:   "update",
	Short: "Update rtlog to the latest version",
	Long: `Update rtlog via 'go install' and re-run setup to refresh hooks and config.

Requires Go toolchain.`,
	Args: cobra.NoArgs,
	RunE: runUpdate,
}

func init() {
	rootCmd.AddCommand(updateCmd)
}

func runUpdate(cmd *cobra.Command, args []string) error {
	fmt.Println("Updating rtlog...")
	goCmd := exec.Command("go", "install", "github.com/cyb33rr/rtlog@latest")
	goCmd.Stdout = os.Stdout
	goCmd.Stderr = os.Stderr
	if err := goCmd.Run(); err != nil {
		return fmt.Errorf("update failed: %w", err)
	}
	fmt.Println("Updated successfully.")
	fmt.Println()

	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("cannot determine home directory: %w", err)
	}

	fmt.Println("Running setup to refresh hooks and config...")
	if err := setupCore(home); err != nil {
		return fmt.Errorf("setup failed: %w", err)
	}
	fmt.Println("Setup complete.")

	return nil
}
