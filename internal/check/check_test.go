package check

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
)

const versionsTF = `# Pinned providers
terraform {
  required_version = ">= 1.5"

  required_providers {
    aws = {
      source  = "hashicorp/aws" # comment
      version = "~> 5.0"
    }
    azurerm = {
      version = ">= 3.0, < 5.0"
      source  = "registry.terraform.io/hashicorp/azurerm"
    }
    github = {
      source = "integrations/github"
      version = "6.2.0"
    }
  }
}
`

const mainTF = `terraform {
  required_providers {
    random = {
      source = "hashicorp/random"
    }
    tfe = {
      "source"  = "HashiCorp/TFE"
      "version" = ">= 0.1"
    }
    null = "3.0.0"
    private = {
      source  = "app.terraform.io/acme/private"
      version = "1.0.0"
    }
  }
}

resource "random_pet" "this" {}
`

const providersJSON = `{
  "terraform": {
    "required_providers": {
      "time": {
        "source": "hashicorp/time",
        "version": "0.9.0"
      }
    }
  }
}
`

var latestVersions = map[string]string{
	"hashicorp/aws":       "6.1.0",
	"hashicorp/azurerm":   "4.2.0",
	"integrations/github": "6.2.0",
	"hashicorp/random":    "3.7.0",
	"hashicorp/tfe":       "0.70.0",
	"hashicorp/null":      "3.2.0",
	"hashicorp/time":      "0.13.0",
}

func fakeLatest(ns, name string) (string, error) {
	if v, ok := latestVersions[ns+"/"+name]; ok {
		return v, nil
	}
	return "", errors.New("not found")
}

func writeConfig(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	files := map[string]string{
		"versions.tf":         versionsTF,
		"main.tf":             mainTF,
		"providers.tf.json":   providersJSON,
		"notes.txt":           `terraform { required_providers { x = "1.0" } }`,
		".hidden.tf":          `terraform { required_providers { x = "1.0" } }`,
		"backup.tf~":          `terraform { required_providers { x = "1.0" } }`,
		"sub/nested/child.tf": `terraform { required_providers { x = "1.0" } }`,
	}
	for name, content := range files {
		path := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o640); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func TestScan(t *testing.T) {
	dir := writeConfig(t)
	reqs, err := Scan(dir)
	if err != nil {
		t.Fatal(err)
	}
	type want struct {
		loc, local, source, constraint string
		hasConstraint, err             bool
	}
	wants := []want{
		{"main.tf:3", "random", "hashicorp/random", "", false, false},
		{"main.tf:8", "tfe", "hashicorp/tfe", ">= 0.1", true, false},
		{"main.tf:10", "null", "hashicorp/null", "3.0.0", true, false},
		{"main.tf:11", "private", "app.terraform.io/acme/private", "", false, true},
		{"providers.tf.json:6", "time", "hashicorp/time", "0.9.0", true, false},
		{"versions.tf:8", "aws", "hashicorp/aws", "~> 5.0", true, false},
		{"versions.tf:11", "azurerm", "hashicorp/azurerm", ">= 3.0, < 5.0", true, false},
		{"versions.tf:16", "github", "integrations/github", "6.2.0", true, false},
	}
	if len(reqs) != len(wants) {
		t.Fatalf("got %d requirements, want %d: %+v", len(reqs), len(wants), reqs)
	}
	for i, w := range wants {
		r := reqs[i]
		if r.Location() != w.loc || r.LocalName != w.local || r.Source() != w.source ||
			r.Constraint != w.constraint || r.HasConstraint != w.hasConstraint || (r.Err != nil) != w.err {
			t.Errorf("requirement %d: got %s %s %s %q has=%v err=%v, want %+v",
				i, r.Location(), r.LocalName, r.Source(), r.Constraint, r.HasConstraint, r.Err, w)
		}
	}
}

func TestScanErrors(t *testing.T) {
	if _, err := Scan(t.TempDir()); err == nil || !strings.Contains(err.Error(), "no Terraform configuration files") {
		t.Errorf("empty dir: got %v", err)
	}

	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "bad.tf"), []byte("terraform {\n"), 0o644)
	if _, err := Scan(dir); err == nil || !strings.Contains(err.Error(), "bad.tf") {
		t.Errorf("invalid HCL: got %v", err)
	}
}

func TestCheck(t *testing.T) {
	reqs, err := Scan(writeConfig(t))
	if err != nil {
		t.Fatal(err)
	}
	var calls atomic.Int32
	latest := func(ns, name string) (string, error) {
		calls.Add(1)
		return fakeLatest(ns, name)
	}
	// Duplicate a requirement to verify lookups are deduplicated.
	reqs = append(reqs, reqs[0])
	results := Check(reqs, latest)

	want := map[string]Status{
		"random":  NoConstraint,
		"tfe":     Unpinned,
		"null":    Outdated,
		"private": Failed,
		"time":    Outdated,
		"aws":     Outdated,
		"azurerm": Unpinned,
		"github":  UpToDate,
	}
	for _, r := range results {
		if r.Status != want[r.LocalName] {
			t.Errorf("%s: got status %v (err %v), want %v", r.LocalName, r.Status, r.Err, want[r.LocalName])
		}
		if r.Status != Failed && r.Latest != latestVersions[r.Source()] {
			t.Errorf("%s: got latest %q", r.LocalName, r.Latest)
		}
	}
	if got := calls.Load(); got != 7 {
		t.Errorf("got %d lookups, want 7", got)
	}

	failing := Check(reqs[:1], func(string, string) (string, error) { return "", errors.New("boom") })
	if failing[0].Status != Failed || failing[0].Err == nil {
		t.Errorf("lookup failure: got %+v", failing[0])
	}
}

func TestEvaluate(t *testing.T) {
	tests := []struct {
		constraint, latest string
		want               Status
	}{
		{"1.2.3", "1.2.3", UpToDate},
		{"= 1.2.3", "1.2.3", UpToDate},
		{"=1.2", "1.2.0", UpToDate},
		{"1.2.2", "1.2.3", Outdated},
		{"~> 1.2", "1.9.0", Unpinned},
		{"~> 1.2", "2.0.0", Outdated},
		{"~> 1.2.0", "1.3.0", Outdated},
		{">= 1.0", "1.0.0", Unpinned},
		{">= 1.0, < 2.0", "2.0.0", Outdated},
		{"!= 1.2.3", "1.2.3", Outdated},
		{"not a version", "1.2.3", Failed},
	}
	for _, tt := range tests {
		got, _ := evaluate(Requirement{Constraint: tt.constraint, HasConstraint: true}, tt.latest)
		if got != tt.want {
			t.Errorf("evaluate(%q, %q) = %v, want %v", tt.constraint, tt.latest, got, tt.want)
		}
	}
}

func TestUpgrade(t *testing.T) {
	dir := writeConfig(t)
	reqs, err := Scan(dir)
	if err != nil {
		t.Fatal(err)
	}
	changes, err := Upgrade(dir, Check(reqs, fakeLatest))
	if err != nil {
		t.Fatal(err)
	}
	if len(changes) != 5 {
		t.Errorf("got %d changes, want 5: %+v", len(changes), changes)
	}

	wantFiles := map[string]string{
		"versions.tf": strings.NewReplacer(
			`"~> 5.0"`, `"6.1.0"`,
			`">= 3.0, < 5.0"`, `"4.2.0"`,
		).Replace(versionsTF),
		"main.tf": strings.NewReplacer(
			`"version" = ">= 0.1"`, `"version" = "0.70.0"`,
			`null = "3.0.0"`, `null = "3.2.0"`,
		).Replace(mainTF),
		"providers.tf.json": strings.Replace(providersJSON, `"0.9.0"`, `"0.13.0"`, 1),
	}
	for name, want := range wantFiles {
		path := filepath.Join(dir, name)
		got, _ := os.ReadFile(path)
		if string(got) != want {
			t.Errorf("%s:\n got:\n%s\nwant:\n%s", name, got, want)
		}
		if info, _ := os.Stat(path); info.Mode().Perm() != 0o640 {
			t.Errorf("%s: mode changed to %v", name, info.Mode().Perm())
		}
	}

	// A second pass finds nothing left to upgrade.
	reqs, _ = Scan(dir)
	if changes, err := Upgrade(dir, Check(reqs, fakeLatest)); err != nil || len(changes) != 0 {
		t.Errorf("second upgrade: got %d changes, err %v", len(changes), err)
	}
}

func TestUpgradeDetectsConcurrentEdit(t *testing.T) {
	dir := writeConfig(t)
	reqs, _ := Scan(dir)
	results := Check(reqs, fakeLatest)
	os.WriteFile(filepath.Join(dir, "versions.tf"), []byte("\n"+versionsTF), 0o640)
	if _, err := Upgrade(dir, results); err == nil {
		t.Fatal("expected error")
	}
	if got, _ := os.ReadFile(filepath.Join(dir, "main.tf")); string(got) != mainTF {
		t.Error("main.tf was modified even though validation failed")
	}
}

func TestRun(t *testing.T) {
	dir := writeConfig(t)
	var stdout, stderr bytes.Buffer
	if code := Run(dir, false, fakeLatest, &stdout, &stderr); code != 1 {
		t.Errorf("exit code %d, want 1 (private provider fails)", code)
	}
	out := strings.Join(strings.Fields(stdout.String()), " ")
	for _, s := range []string{
		"PROVIDER CONSTRAINT LATEST STATUS LOCATION",
		"hashicorp/aws ~> 5.0 6.1.0 outdated versions.tf:8",
		"integrations/github 6.2.0 6.2.0 up to date versions.tf:16",
		"hashicorp/random (none) 3.7.0 no constraint main.tf:3",
		"app.terraform.io/acme/private (none) - error main.tf:11",
		"5 of 8 providers can be upgraded",
	} {
		if !strings.Contains(out, s) {
			t.Errorf("stdout missing %q:\n%s", s, out)
		}
	}
	if !strings.Contains(stderr.String(), "app.terraform.io/acme/private") {
		t.Errorf("stderr missing unsupported source error:\n%s", stderr.String())
	}
	if got, _ := os.ReadFile(filepath.Join(dir, "versions.tf")); string(got) != versionsTF {
		t.Error("check without --upgrade modified files")
	}

	stdout.Reset()
	stderr.Reset()
	Run(dir, true, fakeLatest, &stdout, &stderr)
	if !strings.Contains(stdout.String(), `versions.tf:8: hashicorp/aws "~> 5.0" -> "6.1.0"`) ||
		!strings.Contains(stdout.String(), "Upgraded 5 providers") {
		t.Errorf("upgrade stdout:\n%s", stdout.String())
	}
	if !strings.Contains(stderr.String(), "skipped hashicorp/random (main.tf:3): no version constraint") {
		t.Errorf("upgrade stderr:\n%s", stderr.String())
	}
}
