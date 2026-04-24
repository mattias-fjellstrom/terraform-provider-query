package cmd

import (
	"bytes"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

// executeCommand runs the root command with the given arguments and returns
// combined stdout output and any error.
func executeCommand(args ...string) (string, error) {
	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(buf)
	rootCmd.SetArgs(args)
	err := rootCmd.Execute()
	return buf.String(), err
}

func TestRootCmdHelp(t *testing.T) {
	// --help should succeed and mention the tool name
	output, err := executeCommand("--help")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(output, "tpq") {
		t.Errorf("help output does not contain 'tpq': %s", output)
	}
}

func TestRootCmdUsage(t *testing.T) {
	if rootCmd.Use == "" {
		t.Error("root command Use field must not be empty")
	}
	if rootCmd.Short == "" {
		t.Error("root command Short description must not be empty")
	}
}

func init() {
	// Ensure cobra doesn't call os.Exit during tests
	rootCmd.SilenceUsage = true
	rootCmd.SilenceErrors = true
	// Replace Execute with a no-op version that doesn't call os.Exit
	_ = cobra.Command{}
}
