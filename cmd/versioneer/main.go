package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/versioneer/versioneer/internal/model"
	"github.com/versioneer/versioneer/internal/output"
	"github.com/versioneer/versioneer/internal/resolver"
	"github.com/versioneer/versioneer/internal/scanner"
)

func main() {
	format := flag.String("format", "table", "output format: json, jsonl, csv, markdown, table, summary")
	depFilter := flag.String("dep", "", "filter to projects containing this dependency (substring match)")
	ecoFilter := flag.String("eco", "", "filter to a specific ecosystem (go, npm, python, rust, ruby, java, php, dart)")
	typeFilter := flag.String("type", "", "filter to a dependency type (direct, dev, indirect, peer, optional)")
	resolve := flag.Bool("resolve", false, "resolve actual versions from lock files and node_modules")

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: versioneer [flags] [directory]\n\nScan and report project dependencies.\n\nFlags:\n")
		flag.PrintDefaults()
		fmt.Fprintf(os.Stderr, "\nExamples:\n")
		fmt.Fprintf(os.Stderr, "  versioneer .                          # scan current dir, table output\n")
		fmt.Fprintf(os.Stderr, "  versioneer -format=json ~/code        # full JSON report\n")
		fmt.Fprintf(os.Stderr, "  versioneer -dep=react -format=csv .   # find all projects using react\n")
		fmt.Fprintf(os.Stderr, "  versioneer -eco=python -format=md .   # python deps as markdown\n")
	}
	flag.Parse()

	root := "."
	if flag.NArg() > 0 {
		root = flag.Arg(0)
	}

	result, err := scanner.Scan(root)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	// Resolve actual versions from lock files / disk.
	if *resolve {
		resolver.Resolve(result)
	}

	// Apply filters.
	if *depFilter != "" || *ecoFilter != "" || *typeFilter != "" {
		result = filter(result, *depFilter, *ecoFilter, *typeFilter)
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

func filter(r *model.ScanResult, dep, eco, depType string) *model.ScanResult {
	dep = strings.ToLower(dep)
	eco = strings.ToLower(eco)
	depType = strings.ToLower(depType)

	var projects []model.Project
	totalDeps := 0

	for _, p := range r.Projects {
		var kept []model.Dependency
		for _, d := range p.Dependencies {
			if eco != "" && strings.ToLower(d.Ecosystem) != eco {
				continue
			}
			if depType != "" && strings.ToLower(d.DepType) != depType {
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
