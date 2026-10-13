package main

import (
	"flag"
	"fmt"
	"os"
	"slices"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/mattias-fjellstrom/terraform-provider-query/internal/hcl"
	"github.com/mattias-fjellstrom/terraform-provider-query/internal/registry"
	"github.com/mattias-fjellstrom/terraform-provider-query/internal/tui"
)

// version is set at build time via ldflags.
var version = "dev"

func main() {
	showVersion := flag.Bool("version", false, "print the version and exit")
	genHCL := flag.Bool("hcl", false, "print the configuration in HCL syntax (default)")
	genJSON := flag.Bool("json", false, "print the configuration in Terraform JSON syntax")
	flag.Usage = func() {
		out := flag.CommandLine.Output()
		fmt.Fprintf(out, "Usage:\n"+
			"  tpq                         open the terminal UI\n"+
			"  tpq [--hcl|--json] PROVIDER...\n"+
			"                              print a terraform block pinning the latest versions\n"+
			"                              of the given providers, plus a provider block for\n"+
			"                              each, and exit (e.g. tpq aws azurerm integrations/github)\n"+
			"\nFlags may appear before or after the providers.\n\nFlags:\n")
		flag.PrintDefaults()
	}
	args, _ := parseArgs(flag.CommandLine, os.Args[1:])

	if *showVersion {
		fmt.Println(version)
		return
	}

	if *genHCL && *genJSON {
		fmt.Fprintln(os.Stderr, "error: --hcl and --json are mutually exclusive")
		flag.Usage()
		os.Exit(2)
	}

	if len(args) > 0 || *genHCL || *genJSON {
		providers, err := hcl.Resolve(args, registry.GetLatestVersion)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
		if *genJSON {
			fmt.Print(hcl.GenerateJSON(providers))
		} else {
			fmt.Print(hcl.Generate(providers))
		}
		return
	}

	m := tui.New()
	p := tea.NewProgram(m, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

// parseArgs parses flags in args using fs and returns the positional
// arguments. Unlike fs.Parse, flags may be interleaved with positional
// arguments (e.g. "aws --json tfe"). Everything after a "--" terminator is
// treated as positional.
func parseArgs(fs *flag.FlagSet, args []string) ([]string, error) {
	var rest []string
	if i := slices.Index(args, "--"); i >= 0 {
		args, rest = args[:i], args[i+1:]
	}
	var positional []string
	for {
		if err := fs.Parse(args); err != nil {
			return nil, err
		}
		if fs.NArg() == 0 {
			break
		}
		positional = append(positional, fs.Arg(0))
		args = fs.Args()[1:]
	}
	return append(positional, rest...), nil
}
