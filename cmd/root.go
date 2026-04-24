package cmd

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"
	"github.com/mattias-fjellstrom/terraform-provider-query/internal/tui"
)

var rootCmd = &cobra.Command{
	Use:   "tpq",
	Short: "Interactive TUI to browse Terraform provider versions",
	Long:  `tpq opens an interactive TUI to browse and query provider information from the Terraform registry.`,
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		m := tui.New()
		p := tea.NewProgram(m, tea.WithAltScreen())
		if _, err := p.Run(); err != nil {
			return fmt.Errorf("tui error: %w", err)
		}
		return nil
	},
}

// Execute runs the root command.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
