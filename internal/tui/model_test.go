package tui

import (
	"strings"
	"testing"
)

func TestBuildTerraformBlock(t *testing.T) {
	got := buildTerraformBlock("hashicorp", "aws", "6.40.0")
	want := `terraform {
  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "6.40.0"
    }
  }
}
`
	if got != want {
		t.Fatalf("buildTerraformBlock mismatch.\n--- got ---\n%s\n--- want ---\n%s", got, want)
	}
}

func TestTrimCommonIndent(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "strips common indent",
			input: "    line1\n    line2\n      line3\n",
			want:  "line1\nline2\n  line3\n",
		},
		{
			name:  "skips blank lines when computing indent",
			input: "    line1\n\n    line2\n",
			want:  "line1\n\nline2\n",
		},
		{
			name:  "no indent returns unchanged",
			input: "line1\nline2\n",
			want:  "line1\nline2\n",
		},
		{
			name:  "empty string",
			input: "",
			want:  "",
		},
		{
			name:  "only blank lines",
			input: "\n\n\n",
			want:  "\n\n\n",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := trimCommonIndent(tt.input)
			if got != tt.want {
				t.Errorf("trimCommonIndent() =\n%q\nwant\n%q", got, tt.want)
			}
		})
	}
}

func TestRenderTerraformSnippetContainsVersionAndSource(t *testing.T) {
	out := renderTerraformSnippet("integrations", "github", "6.2.1", 80)
	for _, want := range []string{"github", "integrations/github", "6.2.1", "required_providers"} {
		if !strings.Contains(out, want) {
			t.Errorf("rendered snippet missing %q.\nfull output:\n%s", want, out)
		}
	}
}
