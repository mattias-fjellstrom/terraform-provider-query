package hcl

import (
	"errors"
	"strings"
	"testing"
)

func fakeLatest(versions map[string]string) LatestVersionFunc {
	return func(ns, name string) (string, error) {
		if v, ok := versions[ns+"/"+name]; ok {
			return v, nil
		}
		return "", errors.New("not found")
	}
}

func TestResolve(t *testing.T) {
	latest := fakeLatest(map[string]string{
		"hashicorp/aws":       "6.15.0",
		"hashicorp/azurerm":   "4.47.0",
		"integrations/github": "6.6.0",
	})

	got, err := Resolve([]string{"aws", "integrations/github", "AzureRM", "hashicorp/aws"}, latest)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := []Provider{
		{"hashicorp", "aws", "6.15.0"},
		{"integrations", "github", "6.6.0"},
		{"hashicorp", "azurerm", "4.47.0"},
	}
	if len(got) != len(want) {
		t.Fatalf("got %d providers, want %d: %+v", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("provider %d: got %+v, want %+v", i, got[i], want[i])
		}
	}
}

func TestResolveErrors(t *testing.T) {
	latest := fakeLatest(map[string]string{"hashicorp/aws": "6.15.0"})

	tests := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{"empty", nil, "no providers specified"},
		{"invalid", []string{"a/b/c"}, "invalid provider"},
		{"missing namespace", []string{"/aws"}, "invalid provider"},
		{"name clash", []string{"aws", "other/aws"}, "share the local name"},
		{"lookup failure", []string{"aws", "nope"}, "hashicorp/nope: not found"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Resolve(tt.args, latest)
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("got error %v, want it to contain %q", err, tt.wantErr)
			}
		})
	}
}

func TestGenerate(t *testing.T) {
	got := Generate([]Provider{
		{"hashicorp", "aws", "6.15.0"},
		{"hashicorp", "tfe", "0.70.0"},
	})
	want := `terraform {
  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "6.15.0"
    }
    tfe = {
      source  = "hashicorp/tfe"
      version = "0.70.0"
    }
  }
}

provider "aws" {
  # add configuration here
}

provider "tfe" {
  # add configuration here
}
`
	if got != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
}
