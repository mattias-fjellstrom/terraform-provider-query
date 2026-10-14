// Package check finds the provider version constraints declared in a
// Terraform configuration, compares them with the latest versions available
// in the Terraform Registry, and can rewrite outdated constraints in place.
package check

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"

	version "github.com/hashicorp/go-version"
	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	hcljson "github.com/hashicorp/hcl/v2/json"
	"github.com/zclconf/go-cty/cty"
)

// defaultRegistryHost is the only registry host whose providers can be
// checked, since versions are looked up in the public Terraform Registry.
const defaultRegistryHost = "registry.terraform.io"

// Requirement is a single provider entry in a required_providers block.
type Requirement struct {
	// File is the name of the file, relative to the scanned directory.
	File string
	// Line is the line of the version constraint, or of the provider entry
	// when it has no version constraint.
	Line      int
	LocalName string
	Namespace string
	Name      string
	// Constraint is the version constraint string, if HasConstraint is set.
	Constraint    string
	HasConstraint bool
	// Err is set when the requirement can't be checked, e.g. because its
	// source is not on the public Terraform Registry.
	Err error

	// rawSource is the source address as written, if any.
	rawSource string
	// rng is the byte range of the quoted version string in File, and
	// literal the exact source text in that range.
	rng     hcl.Range
	literal string
}

// Source returns "namespace/name". If the source address could not be
// parsed, it returns the address as written, or else the local name.
func (r Requirement) Source() string {
	if r.Namespace == "" || r.Name == "" {
		if r.rawSource != "" {
			return r.rawSource
		}
		return r.LocalName
	}
	return r.Namespace + "/" + r.Name
}

// Location returns "file:line".
func (r Requirement) Location() string {
	return fmt.Sprintf("%s:%d", r.File, r.Line)
}

var (
	rootSchema = &hcl.BodySchema{
		Blocks: []hcl.BlockHeaderSchema{{Type: "terraform"}},
	}
	terraformSchema = &hcl.BodySchema{
		Blocks: []hcl.BlockHeaderSchema{{Type: "required_providers"}},
	}
)

// Scan parses the Terraform configuration files (*.tf and *.tf.json) in dir,
// not including subdirectories, and returns every provider requirement found
// in their terraform/required_providers blocks. Files are processed in name
// order and requirements are returned in source order.
func Scan(dir string) ([]Requirement, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var reqs []Requirement
	found := false
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !isConfigFile(name) {
			continue
		}
		found = true
		src, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			return nil, err
		}
		r, err := scanFile(name, src)
		if err != nil {
			return nil, err
		}
		reqs = append(reqs, r...)
	}
	if !found {
		return nil, fmt.Errorf("no Terraform configuration files (*.tf, *.tf.json) found in %s", dir)
	}
	return reqs, nil
}

// isConfigFile reports whether Terraform would load the named file as part
// of a module. Like Terraform, it skips hidden and editor backup files.
func isConfigFile(name string) bool {
	if strings.HasPrefix(name, ".") || strings.HasPrefix(name, "#") || strings.HasSuffix(name, "~") {
		return false
	}
	return strings.HasSuffix(name, ".tf") || strings.HasSuffix(name, ".tf.json")
}

func scanFile(name string, src []byte) ([]Requirement, error) {
	var (
		file  *hcl.File
		diags hcl.Diagnostics
	)
	if strings.HasSuffix(name, ".json") {
		file, diags = hcljson.Parse(src, name)
	} else {
		file, diags = hclsyntax.ParseConfig(src, name, hcl.InitialPos)
	}
	if diags.HasErrors() {
		return nil, diags
	}

	root, _, diags := file.Body.PartialContent(rootSchema)
	if diags.HasErrors() {
		return nil, diags
	}
	var reqs []Requirement
	for _, tf := range root.Blocks {
		content, _, diags := tf.Body.PartialContent(terraformSchema)
		if diags.HasErrors() {
			return nil, diags
		}
		for _, rp := range content.Blocks {
			attrs, diags := rp.Body.JustAttributes()
			if diags.HasErrors() {
				return nil, diags
			}
			sorted := make([]*hcl.Attribute, 0, len(attrs))
			for _, a := range attrs {
				sorted = append(sorted, a)
			}
			sort.Slice(sorted, func(i, j int) bool {
				return sorted[i].Range.Start.Byte < sorted[j].Range.Start.Byte
			})
			for _, a := range sorted {
				reqs = append(reqs, parseRequirement(name, src, a))
			}
		}
	}
	return reqs, nil
}

func parseRequirement(file string, src []byte, attr *hcl.Attribute) Requirement {
	req := Requirement{File: file, Line: attr.Range.Start.Line, LocalName: attr.Name}
	source := ""
	var versionExpr hcl.Expression

	if pairs, diags := hcl.ExprMap(attr.Expr); !diags.HasErrors() {
		for _, p := range pairs {
			switch exprKey(p.Key) {
			case "source":
				s, err := stringValue(p.Value)
				if err != nil {
					req.Err = fmt.Errorf("source: %w", err)
					return req
				}
				source = s
			case "version":
				versionExpr = p.Value
			}
		}
	} else {
		// Legacy (Terraform 0.12) shorthand: name = "constraint".
		versionExpr = attr.Expr
	}

	if err := req.setSource(source); err != nil {
		req.Err = err
		return req
	}
	if versionExpr == nil {
		return req
	}

	constraint, err := stringValue(versionExpr)
	if err != nil {
		req.Err = fmt.Errorf("version: %w", err)
		return req
	}
	rng := versionExpr.Range()
	if rng.End.Byte > len(src) || rng.End.Byte-rng.Start.Byte < 2 ||
		src[rng.Start.Byte] != '"' || src[rng.End.Byte-1] != '"' {
		req.Err = errors.New("version: must be a quoted string")
		return req
	}
	req.Line = rng.Start.Line
	req.Constraint = constraint
	req.HasConstraint = true
	req.rng = rng
	req.literal = string(src[rng.Start.Byte:rng.End.Byte])
	return req
}

// setSource parses a provider source address of the form [HOST/]NAMESPACE/NAME
// (or just NAME), defaulting to hashicorp/<local name> when it is empty.
func (r *Requirement) setSource(source string) error {
	r.rawSource = source
	if source == "" {
		source = r.LocalName
	}
	parts := strings.Split(strings.ToLower(source), "/")
	switch len(parts) {
	case 1:
		parts = []string{"hashicorp", parts[0]}
	case 2:
	case 3:
		if parts[0] != defaultRegistryHost {
			return fmt.Errorf("source %q: only providers from %s are supported", source, defaultRegistryHost)
		}
		parts = parts[1:]
	default:
		return fmt.Errorf("invalid source %q", source)
	}
	if parts[0] == "" || parts[1] == "" {
		return fmt.Errorf("invalid source %q", source)
	}
	r.Namespace, r.Name = parts[0], parts[1]
	return nil
}

func exprKey(expr hcl.Expression) string {
	if kw := hcl.ExprAsKeyword(expr); kw != "" {
		return kw
	}
	s, _ := stringValue(expr)
	return s
}

func stringValue(expr hcl.Expression) (string, error) {
	v, diags := expr.Value(nil)
	if diags.HasErrors() || v.IsNull() || !v.IsKnown() || v.Type() != cty.String {
		return "", errors.New("must be a literal string")
	}
	return v.AsString(), nil
}

// Status describes how a provider's version constraint relates to the
// latest available version.
type Status int

const (
	// UpToDate means the constraint pins exactly the latest version.
	UpToDate Status = iota
	// Outdated means the constraint does not allow the latest version.
	Outdated
	// Unpinned means the constraint allows the latest version but does not
	// pin it exactly.
	Unpinned
	// NoConstraint means the provider has no version constraint.
	NoConstraint
	// Failed means the provider could not be checked.
	Failed
)

func (s Status) String() string {
	switch s {
	case UpToDate:
		return "up to date"
	case Outdated:
		return "outdated"
	case Unpinned:
		return "not pinned"
	case NoConstraint:
		return "no constraint"
	default:
		return "error"
	}
}

// Upgradable reports whether Upgrade rewrites constraints with this status.
func (s Status) Upgradable() bool { return s == Outdated || s == Unpinned }

// Result is a requirement together with the latest available version.
type Result struct {
	Requirement
	Latest string
	Status Status
	// Err explains a Failed status.
	Err error
}

// LatestVersionFunc returns the latest version of the given provider.
type LatestVersionFunc func(namespace, name string) (string, error)

// Check looks up the latest version of each distinct provider concurrently
// and compares it with each requirement's constraint. Results are returned in
// the same order as reqs.
func Check(reqs []Requirement, latest LatestVersionFunc) []Result {
	type lookup struct {
		version string
		err     error
	}
	lookups := make(map[string]*lookup)
	var wg sync.WaitGroup
	for _, r := range reqs {
		if r.Err != nil {
			continue
		}
		key := r.Source()
		if _, ok := lookups[key]; ok {
			continue
		}
		l := &lookup{}
		lookups[key] = l
		wg.Add(1)
		go func(ns, name string) {
			defer wg.Done()
			l.version, l.err = latest(ns, name)
		}(r.Namespace, r.Name)
	}
	wg.Wait()

	results := make([]Result, len(reqs))
	for i, r := range reqs {
		res := Result{Requirement: r}
		switch {
		case r.Err != nil:
			res.Status, res.Err = Failed, r.Err
		case lookups[r.Source()].err != nil:
			res.Status, res.Err = Failed, lookups[r.Source()].err
		default:
			res.Latest = lookups[r.Source()].version
			res.Status, res.Err = evaluate(r, res.Latest)
		}
		results[i] = res
	}
	return results
}

func evaluate(r Requirement, latest string) (Status, error) {
	if !r.HasConstraint {
		return NoConstraint, nil
	}
	lv, err := version.NewVersion(latest)
	if err != nil {
		return Failed, fmt.Errorf("invalid latest version %q: %w", latest, err)
	}
	cs, err := version.NewConstraint(r.Constraint)
	if err != nil {
		return Failed, fmt.Errorf("invalid version constraint %q", r.Constraint)
	}
	if isExactPin(r.Constraint, lv) {
		return UpToDate, nil
	}
	if cs.Check(lv) {
		return Unpinned, nil
	}
	return Outdated, nil
}

// isExactPin reports whether constraint is "V" or "= V" with V equal to v.
func isExactPin(constraint string, v *version.Version) bool {
	s := strings.TrimSpace(constraint)
	s = strings.TrimSpace(strings.TrimPrefix(s, "="))
	if s == "" || strings.ContainsAny(s, "<>!~=, ") {
		return false
	}
	pinned, err := version.NewVersion(s)
	return err == nil && pinned.Equal(v)
}

// Change is a version constraint rewritten by Upgrade.
type Change struct {
	Requirement
	From string
	To   string
}

var safeVersion = regexp.MustCompile(`^[0-9A-Za-z.+-]+$`)

// Upgrade rewrites the version constraint of every upgradable result, in its
// file in dir, to pin the latest version exactly. Only the quoted constraint
// strings are replaced; the rest of each file is left byte-for-byte intact.
// All edits are validated before any file is written.
func Upgrade(dir string, results []Result) ([]Change, error) {
	type edit struct {
		rng     hcl.Range
		literal string
		repl    string
	}
	byFile := make(map[string][]edit)
	var files []string
	var changes []Change
	for _, r := range results {
		if !r.Status.Upgradable() {
			continue
		}
		if !safeVersion.MatchString(r.Latest) {
			return nil, fmt.Errorf("%s: refusing to write unexpected version %q", r.Source(), r.Latest)
		}
		if _, ok := byFile[r.File]; !ok {
			files = append(files, r.File)
		}
		byFile[r.File] = append(byFile[r.File], edit{r.rng, r.literal, strconv.Quote(r.Latest)})
		changes = append(changes, Change{Requirement: r.Requirement, From: r.Constraint, To: r.Latest})
	}

	updated := make(map[string][]byte, len(files))
	for _, f := range files {
		src, err := os.ReadFile(filepath.Join(dir, f))
		if err != nil {
			return nil, err
		}
		edits := byFile[f]
		sort.Slice(edits, func(i, j int) bool { return edits[i].rng.Start.Byte > edits[j].rng.Start.Byte })
		for _, e := range edits {
			start, end := e.rng.Start.Byte, e.rng.End.Byte
			if end > len(src) || string(src[start:end]) != e.literal {
				return nil, fmt.Errorf("%s changed while checking; re-run tpq check", f)
			}
			src = append(src[:start:start], append([]byte(e.repl), src[end:]...)...)
		}
		updated[f] = src
	}

	for _, f := range files {
		path := filepath.Join(dir, f)
		info, err := os.Stat(path)
		if err != nil {
			return nil, err
		}
		if err := os.WriteFile(path, updated[f], info.Mode().Perm()); err != nil {
			return nil, err
		}
	}
	return changes, nil
}
