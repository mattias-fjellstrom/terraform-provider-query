package check

import (
	"fmt"
	"github.com/hashicorp/go-version"
	"io"
	"strings"
	"text/tabwriter"
)

// Run checks the Terraform configuration in dir, writing a report to stdout
// and problems to stderr. With upgrade set, it pins every outdated or
// unpinned provider to its latest version instead of printing the report.
// It returns the process exit code: 0 on success, 1 if anything failed.
func Run(dir string, upgrade bool, latest LatestVersionFunc, stdout, stderr io.Writer) int {
	reqs, err := Scan(dir)
	if err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}
	if len(reqs) == 0 {
		fmt.Fprintln(stdout, "No providers found in required_providers blocks.")
		return 0
	}
	results := Check(reqs, latest)

	if upgrade {
		changes, err := Upgrade(dir, results)
		if err != nil {
			fmt.Fprintf(stderr, "error: %v\n", err)
			return 1
		}
		writeChanges(stdout, changes)
		for _, r := range results {
			if r.Status == NoConstraint {
				fmt.Fprintf(stderr, "skipped %s (%s): no version constraint\n", r.Source(), r.Location())
			}
		}
	} else {
		writeReport(stdout, results)
	}
	return reportErrors(stderr, results)
}

func classifyVersionLag(constraint, latest string) string {
	pinned, err := version.NewVersion(strings.TrimSpace(
		strings.TrimPrefix(strings.TrimSpace(constraint), "="),
	))
	if err != nil {
		return "outdated"
	}

	latestVersion, err := version.NewVersion(latest)
	if err != nil {
		return "outdated"
	}

	pinnedParts := pinned.Segments()
	latestParts := latestVersion.Segments()

	if pinnedParts[0] != latestParts[0] {
		return "major_lag"
	}

	if pinnedParts[1] != latestParts[1] {
		return "feature_lag"
	}

	if pinnedParts[2] != latestParts[2] {
		return "patch_lag"
	}

	return "outdated"
}

func writeReport(w io.Writer, results []Result) {
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "PROVIDER\tCONSTRAINT\tLATEST\tSTATUS\tLOCATION")
	upgradable := 0
	for _, r := range results {
		constraint := r.Constraint
		if !r.HasConstraint {
			constraint = "(none)"
		}
		latest := r.Latest
		if latest == "" {
			latest = "-"
		}
		status := r.Status.String()

		if r.Status == Outdated {
			status = classifyVersionLag(r.Constraint, r.Latest)
		}

		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\n", r.Source(), constraint, latest, status, r.Location())
		if r.Status.Upgradable() {
			upgradable++
		}
	}
	tw.Flush()

	fmt.Fprintln(w)
	if upgradable == 0 {
		fmt.Fprintln(w, "All provider versions are pinned to the latest versions.")
		return
	}
	fmt.Fprintf(w, "%d of %d %s can be upgraded. Run \"tpq check --upgrade\" to pin %s to the latest %s.\n",
		upgradable, len(results), plural(len(results), "provider", "providers"),
		plural(upgradable, "it", "them"), plural(upgradable, "version", "versions"))
}

func writeChanges(w io.Writer, changes []Change) {
	if len(changes) == 0 {
		fmt.Fprintln(w, "Nothing to upgrade; all provider versions are pinned to the latest versions.")
		return
	}
	for _, c := range changes {
		fmt.Fprintf(w, "%s: %s %q -> %q\n", c.Location(), c.Source(), c.From, c.To)
	}
	fmt.Fprintf(w, "\nUpgraded %d %s. Run \"terraform init -upgrade\" to update the dependency lock file.\n",
		len(changes), plural(len(changes), "provider", "providers"))
}

func reportErrors(w io.Writer, results []Result) int {
	code := 0
	for _, r := range results {
		if r.Status == Failed {
			fmt.Fprintf(w, "error: %s (%s): %v\n", r.Source(), r.Location(), r.Err)
			code = 1
		}
	}
	return code
}

func plural(n int, one, many string) string {
	if n == 1 {
		return one
	}
	return many
}
