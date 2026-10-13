package main

import (
	"flag"
	"io"
	"slices"
	"testing"
)

func TestParseArgs(t *testing.T) {
	tests := []struct {
		name     string
		args     []string
		wantPos  []string
		wantHCL  bool
		wantJSON bool
	}{
		{"none", nil, nil, false, false},
		{"positional only", []string{"azurerm", "tfe"}, []string{"azurerm", "tfe"}, false, false},
		{"flag first", []string{"--json", "azurerm", "tfe"}, []string{"azurerm", "tfe"}, false, true},
		{"flag last", []string{"azurerm", "tfe", "--hcl"}, []string{"azurerm", "tfe"}, true, false},
		{"flag between", []string{"azurerm", "-json", "tfe"}, []string{"azurerm", "tfe"}, false, true},
		{"terminator", []string{"aws", "--", "--json"}, []string{"aws", "--json"}, false, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fs := flag.NewFlagSet("tpq", flag.ContinueOnError)
			fs.SetOutput(io.Discard)
			hcl := fs.Bool("hcl", false, "")
			json := fs.Bool("json", false, "")
			got, err := parseArgs(fs, tt.args)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !slices.Equal(got, tt.wantPos) {
				t.Errorf("positional: got %q, want %q", got, tt.wantPos)
			}
			if *hcl != tt.wantHCL || *json != tt.wantJSON {
				t.Errorf("flags: got hcl=%v json=%v, want hcl=%v json=%v", *hcl, *json, tt.wantHCL, tt.wantJSON)
			}
		})
	}
}

func TestParseArgsUnknownFlag(t *testing.T) {
	fs := flag.NewFlagSet("tpq", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	if _, err := parseArgs(fs, []string{"aws", "--nope"}); err == nil {
		t.Error("expected error for unknown flag")
	}
}
