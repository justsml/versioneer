package scanner

import (
	"context"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	ignore "github.com/sabhiram/go-gitignore"

	"github.com/justsml/versioneer/internal/model"
	"github.com/justsml/versioneer/internal/parser"
)

// allowDotDirs are dot-prefixed directories that should still be scanned.
var allowDotDirs = map[string]struct{}{
	".github": {},
}

// skipDirs are directory names that should never be descended into.
var skipDirs = map[string]struct{}{
	"node_modules": {}, ".git": {}, "vendor": {}, ".venv": {},
	"venv": {}, "__pycache__": {}, ".tox": {}, "target": {},
	".gradle": {}, "build": {}, "dist": {}, ".idea": {},
	".vscode": {}, ".next": {}, ".nuxt": {},
}

// Options controls optional Scan behaviour.
type Options struct {
	// RespectGitignore loads .gitignore and .ignore files and excludes
	// matching paths from the scan.
	RespectGitignore bool
}

// Scan walks root in parallel and returns all discovered projects.
// The context can be used to cancel or timeout the scan.
func Scan(ctx context.Context, root string, logger *log.Logger, opts ...Options) (*model.ScanResult, error) {
	var opt Options
	if len(opts) > 0 {
		opt = opts[0]
	}

	start := time.Now()
	manifests := parser.ManifestFiles()

	// Build hierarchical gitignore checker when requested.
	var ign *ignoreChecker
	if opt.RespectGitignore {
		ign = newIgnoreChecker(root, logger)
	}

	// Phase 1: walk the tree and collect manifest paths.
	var paths []string
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if err != nil {
			return nil // skip unreadable entries
		}

		if d.IsDir() {
			name := d.Name()
			if strings.HasPrefix(name, ".") && name != "." {
				if _, allow := allowDotDirs[name]; !allow {
					return filepath.SkipDir
				}
			}
			if _, skip := skipDirs[name]; skip {
				return filepath.SkipDir
			}
			// Load ignore files from this directory before descending.
			if ign != nil {
				ign.loadDir(path)
				if ign.isIgnored(path, true) {
					return filepath.SkipDir
				}
			}
			return nil
		}

		if ign != nil && ign.isIgnored(path, false) {
			return nil
		}
		if _, ok := manifests[d.Name()]; ok {
			paths = append(paths, path)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	// Phase 2: parse manifests in parallel.
	workers := runtime.NumCPU()
	if workers > len(paths) {
		workers = len(paths)
	}
	if workers == 0 {
		return &model.ScanResult{
			RootDir:      root,
			ScanDuration: time.Since(start),
		}, nil
	}

	type result struct {
		project model.Project
		err     error
	}

	ch := make(chan string, len(paths))
	results := make(chan result, len(paths))

	var wg sync.WaitGroup
	for range workers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for path := range ch {
				if ctx.Err() != nil {
					return
				}
				p := parser.ForFile(path)
				if p == nil {
					continue
				}
				data, err := os.ReadFile(path)
				if err != nil {
					logger.Printf("skip %s: %v", path, err)
					results <- result{err: err}
					continue
				}
				deps, err := p.Parse(path, data)
				if err != nil {
					logger.Printf("parse %s: %v", path, err)
					results <- result{err: err}
					continue
				}
				rel, _ := filepath.Rel(root, path)
				eco := ""
				if len(deps) > 0 {
					eco = deps[0].Ecosystem
				}
				results <- result{project: model.Project{
					Path:         filepath.Dir(rel),
					Ecosystem:    eco,
					ManifestFile: rel,
					Dependencies: deps,
					ScannedAt:    time.Now(),
				}}
			}
		}()
	}

	for _, p := range paths {
		ch <- p
	}
	close(ch)

	go func() {
		wg.Wait()
		close(results)
	}()

	var projects []model.Project
	totalDeps := 0
	for r := range results {
		if r.err != nil {
			continue // log in verbose mode later
		}
		totalDeps += len(r.project.Dependencies)
		projects = append(projects, r.project)
	}

	return &model.ScanResult{
		RootDir:      root,
		Projects:     projects,
		TotalDeps:    totalDeps,
		ScanDuration: time.Since(start),
	}, nil
}

// ignoreChecker accumulates .gitignore / .ignore patterns per directory and
// tests paths against all applicable matchers.
type ignoreChecker struct {
	root     string
	logger   *log.Logger
	matchers []scopedMatcher
}

type scopedMatcher struct {
	dir     string // absolute directory this .gitignore/.ignore lives in
	matcher *ignore.GitIgnore
}

func newIgnoreChecker(root string, logger *log.Logger) *ignoreChecker {
	return &ignoreChecker{root: root, logger: logger}
}

// loadDir reads .gitignore and .ignore from dir (if they exist) and adds
// them to the matcher list.
func (ic *ignoreChecker) loadDir(dir string) {
	for _, name := range []string{".gitignore", ".ignore"} {
		path := filepath.Join(dir, name)
		gi, err := ignore.CompileIgnoreFile(path)
		if err != nil {
			continue // file doesn't exist or unreadable — skip silently
		}
		ic.matchers = append(ic.matchers, scopedMatcher{dir: dir, matcher: gi})
		ic.logger.Printf("loaded %s", path)
	}
}

// isIgnored returns true if the path matches any applicable ignore pattern.
func (ic *ignoreChecker) isIgnored(path string, isDir bool) bool {
	for _, sm := range ic.matchers {
		rel, err := filepath.Rel(sm.dir, path)
		if err != nil || strings.HasPrefix(rel, "..") {
			continue // path not under this matcher's scope
		}
		// Append trailing separator for directories so patterns like "dir/" match.
		check := rel
		if isDir {
			check += "/"
		}
		if sm.matcher.MatchesPath(check) {
			return true
		}
	}
	return false
}
