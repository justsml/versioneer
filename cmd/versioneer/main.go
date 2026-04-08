package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"os/signal"
	"strings"

	"github.com/justsml/versioneer/internal/matcher"
	"github.com/justsml/versioneer/internal/model"
	"github.com/justsml/versioneer/internal/output"
	"github.com/justsml/versioneer/internal/resolver"
	"github.com/justsml/versioneer/internal/scanner"
)

func main() {
	format := flag.String("format", "table", "output format: json, jsonl, csv, markdown, table, summary")
	depFilter := flag.String("dep", "", "filter to projects containing this dependency (substring match)")
	ecoFilter := flag.String("eco", "", "filter to a specific ecosystem (go, npm, python, rust, ruby, java, php, dart)")
	typeFilter := flag.String("type", "", "filter to a dependency type (direct, dev, indirect, peer, optional)")
	resolve := flag.Bool("resolve", false, "resolve actual versions from lock files and node_modules")
	check := flag.String("check", "", "check for vulnerable packages: pkg@>=1.0,<2.0,other-pkg (comma-separated)")
	checkFile := flag.String("checkfile", "", "file with vulnerable package rules (one per line: pkg@constraint)")
	gitignore := flag.Bool("gitignore", false, "respect .gitignore and .ignore exclusion files")
	verbose := flag.Bool("verbose", false, "print diagnostic messages (skipped files, parse errors, resolution failures)")

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: versioneer [flags] [directory]\n\nScan and report project dependencies.\n\nFlags:\n")
		flag.PrintDefaults()
		fmt.Fprintf(os.Stderr, "\nExamples:\n")
		fmt.Fprintf(os.Stderr, "  versioneer .                                              # scan current dir\n")
		fmt.Fprintf(os.Stderr, "  versioneer -resolve -dep=react ~/code                     # find react with actual versions\n")
		fmt.Fprintf(os.Stderr, "  versioneer -resolve -check='axios@<1.7.0,colors' ~/code   # security sweep\n")
		fmt.Fprintf(os.Stderr, "  versioneer -resolve -checkfile=vulns.txt ~/code            # sweep from file\n")
		fmt.Fprintf(os.Stderr, "\nCheckfile format (one rule per line):\n")
		fmt.Fprintf(os.Stderr, "  axios@>=1.3.0,<1.6.4    # affected range\n")
		fmt.Fprintf(os.Stderr, "  event-stream@=3.3.6      # exact malicious version\n")
		fmt.Fprintf(os.Stderr, "  colors                   # any version (name-only)\n")
		fmt.Fprintf(os.Stderr, "  @scope/pkg@<2.0.0        # scoped packages\n")
	}
	flag.Parse()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	var logger *log.Logger
	if *verbose {
		logger = log.New(os.Stderr, "versioneer: ", 0)
	} else {
		logger = log.New(io.Discard, "", 0)
	}

	root := "."
	if flag.NArg() > 0 {
		root = flag.Arg(0)
	}

	result, err := scanner.Scan(ctx, root, logger, scanner.Options{
		RespectGitignore: *gitignore,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	// Resolve actual versions from lock files / disk.
	// Auto-enable when doing security checks — we need real versions.
	if *resolve || *check != "" || *checkFile != "" {
		resolver.Resolve(ctx, result, logger)
	}

	// Apply filters.
	if *depFilter != "" || *ecoFilter != "" || *typeFilter != "" {
		result = filter(result, *depFilter, *ecoFilter, *typeFilter)
	}

	// Security check mode: load rules and filter to matches.
	if *check != "" || *checkFile != "" {
		rules, err := loadRules(*check, *checkFile)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error loading rules: %v\n", err)
			os.Exit(1)
		}
		hits := checkVulns(result, rules)
		if hits == 0 {
			fmt.Fprintf(os.Stderr, "No matches found for %d rules across %d projects.\n",
				len(rules), len(result.Projects))
			os.Exit(0)
		}
		fmt.Fprintf(os.Stderr, "FOUND %d matching dependencies across rules.\n", hits)
		// Exit 1 when hits found (useful for CI).
		defer os.Exit(1)
	}

	formatter, err := output.Get(*format)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	if err := formatter.Format(os.Stdout, result); err != nil {
		fmt.Fprintf(os.Stderr, "error writing output: %v\n", err)
		os.Exit(1)
	}
}

func loadRules(inline, filePath string) ([]matcher.Rule, error) {
	var rules []matcher.Rule
	if inline != "" {
		r, err := matcher.ParseRulesArg(inline)
		if err != nil {
			return nil, err
		}
		rules = append(rules, r...)
	}
	if filePath != "" {
		r, err := matcher.LoadRulesFile(filePath)
		if err != nil {
			return nil, err
		}
		rules = append(rules, r...)
	}
	return rules, nil
}

// checkVulns filters the result in-place to only matching deps, returns hit count.
func checkVulns(result *model.ScanResult, rules []matcher.Rule) int {
	// Build a quick lookup by name.
	byName := map[string][]matcher.Rule{}
	for _, r := range rules {
		byName[r.Name] = append(byName[r.Name], r)
	}

	var projects []model.Project
	totalHits := 0

	unresolved := 0
	for _, p := range result.Projects {
		var hits []model.Dependency
		for _, d := range p.Dependencies {
			nameRules, ok := byName[d.Name]
			if !ok {
				continue
			}

			// Use resolved (actual) version. If unavailable, try to extract
			// a concrete version from the spec (pinned like "1.7.9"), but
			// skip range expressions (^, ~, >=, etc.) — we can't reliably
			// match those without the real installed version.
			checkVersion := d.Resolved
			if checkVersion == "" {
				checkVersion = extractConcreteVersion(d.Version)
			}

			if checkVersion == "" {
				// Name matches but version is unknown — flag as unresolved.
				d.Resolved = "UNRESOLVED"
				hits = append(hits, d)
				unresolved++
				continue
			}

			for _, rule := range nameRules {
				if rule.Match(checkVersion) {
					hits = append(hits, d)
					break
				}
			}
		}
		if len(hits) > 0 {
			totalHits += len(hits)
			p.Dependencies = hits
			projects = append(projects, p)
		}
	}

	result.Projects = projects
	result.TotalDeps = totalHits

	if unresolved > 0 {
		fmt.Fprintf(os.Stderr, "WARNING: %d dependencies matched by name but version could not be resolved — marked UNRESOLVED.\n", unresolved)
	}
	return totalHits
}

// extractConcreteVersion returns a version string only if it looks like a pinned
// version (e.g. "1.7.9", "v2.0.0"). Returns "" for range expressions.
func extractConcreteVersion(spec string) string {
	spec = strings.TrimSpace(spec)
	if spec == "" || spec == "*" {
		return ""
	}
	// Reject anything containing range/wildcard operators.
	if strings.ContainsAny(spec, "^~><=!|x*X ") {
		return ""
	}
	// Must start with a digit or 'v' followed by digit.
	if spec[0] >= '0' && spec[0] <= '9' {
		return spec
	}
	if len(spec) > 1 && spec[0] == 'v' && (spec[1] >= '0' && spec[1] <= '9') {
		return spec
	}
	return ""
}

func filter(r *model.ScanResult, dep, eco, depType string) *model.ScanResult {
	dep = strings.ToLower(dep)
	eco = strings.ToLower(eco)
	depType = strings.ToLower(depType)

	var projects []model.Project
	totalDeps := 0

	for _, p := range r.Projects {
		var kept []model.Dependency
		for _, d := range p.Dependencies {
			if eco != "" && !strings.EqualFold(d.Ecosystem, eco) {
				continue
			}
			if depType != "" && !strings.EqualFold(d.DepType, depType) {
				continue
			}
			if dep != "" && !strings.Contains(strings.ToLower(d.Name), dep) {
				continue
			}
			kept = append(kept, d)
		}
		if len(kept) > 0 {
			p.Dependencies = kept
			totalDeps += len(kept)
			projects = append(projects, p)
		}
	}

	return &model.ScanResult{
		RootDir:      r.RootDir,
		Projects:     projects,
		TotalDeps:    totalDeps,
		ScanDuration: r.ScanDuration,
	}
}
