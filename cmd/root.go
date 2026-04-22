package cmd

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"
	"github.com/mattias-fjellstrom/terraform-provider-query/internal/registry"
	"github.com/mattias-fjellstrom/terraform-provider-query/internal/tui"
)

var hclFlag bool

var rootCmd = &cobra.Command{
	Use:   "tpq [provider]",
	Short: "Query the Terraform registry for provider versions",
	Long: `tpq queries the Terraform registry for provider information.

With no arguments it opens an interactive TUI.
With a provider name it prints the latest version.
Use --hcl to output an HCL required_providers block.`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) == 0 {
			// Launch TUI mode
			m := tui.New()
			p := tea.NewProgram(m, tea.WithAltScreen())
			if _, err := p.Run(); err != nil {
				return fmt.Errorf("tui error: %w", err)
			}
			return nil
		}

		providerArg := args[0]
		namespace, providerName := registry.ParseProvider(providerArg)
		version, source, err := registry.GetLatestVersion(namespace, providerName)
		if err != nil {
			return err
		}

		if hclFlag {
			fmt.Printf(`terraform {
  required_providers {
    %s = {
      source  = "%s"
      version = "%s"
    }
  }
}
`, providerName, source, version)
		} else {
			fmt.Println(version)
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

func init() {
	rootCmd.Flags().BoolVar(&hclFlag, "hcl", false, "Output HCL required_providers block")
}
