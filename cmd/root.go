package cmd

import (
	"fmt"
	"os"
	"strings"
	"sync"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"
	"github.com/mattias-fjellstrom/terraform-provider-query/internal/registry"
	"github.com/mattias-fjellstrom/terraform-provider-query/internal/tui"
)

var hclFlag bool

type providerResult struct {
	index       int
	providerArg string
	name        string
	source      string
	version     string
	err         error
}

var rootCmd = &cobra.Command{
	Use:   "tpq [provider...]",
	Short: "Query the Terraform registry for provider versions",
	Long: `tpq queries the Terraform registry for provider information.

With no arguments it opens an interactive TUI.
With one or more provider names it prints the latest version(s).
Use --hcl to output an HCL required_providers block.`,
	Args: cobra.ArbitraryArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) == 0 {
			m := tui.New()
			p := tea.NewProgram(m, tea.WithAltScreen())
			if _, err := p.Run(); err != nil {
				return fmt.Errorf("tui error: %w", err)
			}
			return nil
		}

		results := make([]providerResult, len(args))
		var wg sync.WaitGroup
		for i, arg := range args {
			wg.Add(1)
			go func(idx int, providerArg string) {
				defer wg.Done()
				namespace, name := registry.ParseProvider(providerArg)
				version, source, err := registry.GetLatestVersion(namespace, name)
				results[idx] = providerResult{
					index:       idx,
					providerArg: providerArg,
					name:        name,
					source:      source,
					version:     version,
					err:         err,
				}
			}(i, arg)
		}
		wg.Wait()

		// Check for any errors first
		for _, r := range results {
			if r.err != nil {
				return r.err
			}
		}

		if hclFlag {
			var sb strings.Builder
			sb.WriteString("terraform {\n  required_providers {\n")
			for _, r := range results {
				sb.WriteString(fmt.Sprintf("    %s = {\n      source  = \"%s\"\n      version = \"%s\"\n    }\n", r.name, r.source, r.version))
			}
			sb.WriteString("  }\n}\n")
			fmt.Print(sb.String())
		} else if len(results) == 1 {
			fmt.Println(results[0].version)
		} else {
			for _, r := range results {
				fmt.Printf("%s: %s\n", r.name, r.version)
			}
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
