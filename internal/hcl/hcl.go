// Package hcl generates Terraform configuration for a set of providers.
package hcl

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"

	"github.com/mattias-fjellstrom/terraform-provider-query/internal/registry"
)

// Provider is a provider pinned to a specific version.
type Provider struct {
	Namespace string
	Name      string
	Version   string
}

// LatestVersionFunc returns the latest version of the given provider.
type LatestVersionFunc func(namespace, name string) (string, error)

// Resolve parses provider arguments (e.g. "aws" or "integrations/github") and
// looks up the latest version of each concurrently. The returned providers
// keep the order of args. Duplicate providers are ignored, and two different
// providers that share the same local name are rejected.
func Resolve(args []string, latest LatestVersionFunc) ([]Provider, error) {
	var providers []Provider
	seen := make(map[string]string)
	for _, arg := range args {
		arg = strings.ToLower(strings.TrimSpace(arg))
		ns, name := registry.ParseProvider(arg)
		if ns == "" || name == "" || strings.Contains(name, "/") {
			return nil, fmt.Errorf("invalid provider %q: expected NAME or NAMESPACE/NAME", arg)
		}
		source := ns + "/" + name
		if prev, ok := seen[name]; ok {
			if prev == source {
				continue
			}
			return nil, fmt.Errorf("providers %q and %q share the local name %q", prev, source, name)
		}
		seen[name] = source
		providers = append(providers, Provider{Namespace: ns, Name: name})
	}
	if len(providers) == 0 {
		return nil, errors.New("no providers specified")
	}

	errs := make([]error, len(providers))
	var wg sync.WaitGroup
	for i := range providers {
		wg.Add(1)
		go func(p *Provider, i int) {
			defer wg.Done()
			v, err := latest(p.Namespace, p.Name)
			if err != nil {
				errs[i] = fmt.Errorf("%s/%s: %w", p.Namespace, p.Name, err)
				return
			}
			p.Version = v
		}(&providers[i], i)
	}
	wg.Wait()

	if err := errors.Join(errs...); err != nil {
		return nil, err
	}
	return providers, nil
}

// Generate renders a terraform block with required_providers for each
// provider, followed by an empty provider block for each.
func Generate(providers []Provider) string {
	var b strings.Builder
	b.WriteString("terraform {\n")
	b.WriteString("  required_providers {\n")
	for _, p := range providers {
		fmt.Fprintf(&b, "    %s = {\n", p.Name)
		fmt.Fprintf(&b, "      source  = %q\n", p.Namespace+"/"+p.Name)
		fmt.Fprintf(&b, "      version = %q\n", p.Version)
		b.WriteString("    }\n")
	}
	b.WriteString("  }\n")
	b.WriteString("}\n")
	for _, p := range providers {
		fmt.Fprintf(&b, "\nprovider %q {\n", p.Name)
		b.WriteString("  # add configuration here\n")
		b.WriteString("}\n")
	}
	return b.String()
}

// GenerateJSON renders the same configuration as Generate using Terraform's
// JSON configuration syntax (suitable for a .tf.json file). Provider order is
// preserved.
func GenerateJSON(providers []Provider) string {
	q := func(s string) string {
		out, _ := json.Marshal(s)
		return string(out)
	}
	var b strings.Builder
	b.WriteString("{\n")
	b.WriteString("  \"terraform\": {\n")
	b.WriteString("    \"required_providers\": {\n")
	for i, p := range providers {
		fmt.Fprintf(&b, "      %s: {\n", q(p.Name))
		fmt.Fprintf(&b, "        \"source\": %s,\n", q(p.Namespace+"/"+p.Name))
		fmt.Fprintf(&b, "        \"version\": %s\n", q(p.Version))
		b.WriteString("      }")
		b.WriteString(sep(i, len(providers)))
	}
	b.WriteString("    }\n")
	b.WriteString("  },\n")
	b.WriteString("  \"provider\": {\n")
	for i, p := range providers {
		fmt.Fprintf(&b, "    %s: {\n", q(p.Name))
		b.WriteString("      \"//\": \"add configuration here\"\n")
		b.WriteString("    }")
		b.WriteString(sep(i, len(providers)))
	}
	b.WriteString("  }\n")
	b.WriteString("}\n")
	return b.String()
}

func sep(i, n int) string {
	if i < n-1 {
		return ",\n"
	}
	return "\n"
}
