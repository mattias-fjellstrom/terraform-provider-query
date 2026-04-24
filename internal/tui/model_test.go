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

func TestRenderTerraformSnippetContainsVersionAndSource(t *testing.T) {
	out := renderTerraformSnippet("integrations", "github", "6.2.1", 80)
	for _, want := range []string{"github", "integrations/github", "6.2.1", "required_providers"} {
		if !strings.Contains(out, want) {
			t.Errorf("rendered snippet missing %q.\nfull output:\n%s", want, out)
		}
	}
}
