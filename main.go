package main

import (
	"flag"
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/mattias-fjellstrom/terraform-provider-query/internal/hcl"
	"github.com/mattias-fjellstrom/terraform-provider-query/internal/registry"
	"github.com/mattias-fjellstrom/terraform-provider-query/internal/tui"
)

// version is set at build time via ldflags.
var version = "dev"

func main() {
	showVersion := flag.Bool("version", false, "print the version and exit")
	genHCL := flag.Bool("hcl", false, "print a terraform block pinning the latest versions of the given providers,\nplus a provider block for each, and exit (e.g. tpq --hcl aws azurerm integrations/github)")
	flag.Usage = func() {
		out := flag.CommandLine.Output()
		fmt.Fprintf(out, "Usage:\n  tpq                       open the terminal UI\n  tpq --hcl PROVIDER...     print Terraform configuration for the given providers\n\nFlags:\n")
		flag.PrintDefaults()
	}
	flag.Parse()

	if *showVersion {
		fmt.Println(version)
		return
	}

	if *genHCL {
		providers, err := hcl.Resolve(flag.Args(), registry.GetLatestVersion)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
		fmt.Print(hcl.Generate(providers))
		return
	}

	if flag.NArg() > 0 {
		fmt.Fprintf(os.Stderr, "error: unexpected arguments %v (did you mean --hcl?)\n", flag.Args())
		flag.Usage()
		os.Exit(2)
	}

	m := tui.New()
	p := tea.NewProgram(m, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}
