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

// Stream holds a streaming scan result whose projects arrive on a channel.
type Stream struct {
	Projects <-chan model.Project
	Start    time.Time
}

// Scan walks root in parallel and returns all discovered projects.
// The context can be used to cancel or timeout the scan.
func Scan(ctx context.Context, root string, logger *log.Logger, opts ...Options) (*model.ScanResult, error) {
	stream, err := ScanStream(ctx, root, logger, opts...)
	if err != nil {
		return nil, err
	}
	projects := make([]model.Project, 0, 256)
	totalDeps := 0
	for p := range stream.Projects {
		totalDeps += len(p.Dependencies)
		projects = append(projects, p)
	}
	return &model.ScanResult{
		RootDir:      root,
		Projects:     projects,
		TotalDeps:    totalDeps,
		ScanDuration: time.Since(stream.Start),
	}, nil
}

// ScanStream walks root and emits projects on a channel as they're discovered
// and parsed. Use this to pipeline scanning with downstream processing such as
// version resolution — resolution can begin while scanning is still in progress.
func ScanStream(ctx context.Context, root string, logger *log.Logger, opts ...Options) (*Stream, error) {
	var opt Options
	if len(opts) > 0 {
		opt = opts[0]
	}
	start := time.Now()
	manifests := parser.ManifestFiles()

	var pathsCh <-chan string
	if opt.RespectGitignore {
		// Gitignore rules must be loaded parent-first, requiring sequential traversal.
		pathsCh = walkSequential(ctx, root, manifests, logger)
	} else {
		// Parallel walker: goroutine-per-directory bounded by NumCPU semaphore.
		pathsCh = walkParallel(ctx, root, manifests)
	}

	projectsCh := parseWorkers(ctx, root, pathsCh, logger)
	return &Stream{Projects: projectsCh, Start: start}, nil
}

// walkParallel discovers manifest files concurrently using goroutine-per-directory,
// bounded by a NumCPU semaphore. Faster than filepath.WalkDir on SSDs with high IOPS.
func walkParallel(ctx context.Context, root string, manifests map[string]struct{}) <-chan string {
	ch := make(chan string, 256)
	sem := make(chan struct{}, runtime.NumCPU())
	var wg sync.WaitGroup

	var walk func(string)
	walk = func(dir string) {
		defer wg.Done()
		sem <- struct{}{}
		defer func() { <-sem }()

		if ctx.Err() != nil {
			return
		}

		entries, err := os.ReadDir(dir)
		if err != nil {
			return
		}

		for _, e := range entries {
			name := e.Name()

			if e.IsDir() {
				if strings.HasPrefix(name, ".") && name != "." {
					if _, allow := allowDotDirs[name]; !allow {
						continue
					}
				}
				if _, skip := skipDirs[name]; skip {
					continue
				}
				wg.Add(1)
				go walk(filepath.Join(dir, name))
			} else if _, ok := manifests[name]; ok {
				ch <- filepath.Join(dir, name)
			}
		}
	}

	wg.Add(1)
	go walk(root)

	go func() {
		wg.Wait()
		close(ch)
	}()

	return ch
}

// walkSequential uses filepath.WalkDir for gitignore-aware scanning.
// Gitignore rules must be loaded parent-first, requiring sequential traversal.
func walkSequential(ctx context.Context, root string, manifests map[string]struct{}, logger *log.Logger) <-chan string {
	ch := make(chan string, 256)
	go func() {
		defer close(ch)
		ign := newIgnoreChecker(root, logger)
		filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
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
				ign.loadDir(path)
				if ign.isIgnored(path, true) {
					return filepath.SkipDir
				}
				return nil
			}

			if ign.isIgnored(path, false) {
				return nil
			}
			if _, ok := manifests[d.Name()]; ok {
				ch <- path
			}
			return nil
		})
	}()
	return ch
}

// parseWorkers starts NumCPU goroutines that read manifest paths, parse them,
// and emit projects to the returned channel.
func parseWorkers(ctx context.Context, root string, pathsCh <-chan string, logger *log.Logger) <-chan model.Project {
	workers := runtime.NumCPU()
	ch := make(chan model.Project, workers*2)
	var wg sync.WaitGroup

	for range workers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for path := range pathsCh {
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
					continue
				}
				deps, err := p.Parse(path, data)
				if err != nil {
					logger.Printf("parse %s: %v", path, err)
					continue
				}
				rel, _ := filepath.Rel(root, path)
				eco := ""
				if len(deps) > 0 {
					eco = deps[0].Ecosystem
				}
				ch <- model.Project{
					Path:         filepath.Dir(rel),
					Ecosystem:    eco,
					ManifestFile: rel,
					Dependencies: deps,
					ScannedAt:    time.Now(),
				}
			}
		}()
	}

	go func() {
		wg.Wait()
		close(ch)
	}()

	return ch
}

// --- gitignore support ---

type ignoreRule struct {
	dir, pattern    string
	negate, dirOnly bool
}

type ignoreChecker struct {
	logger *log.Logger
	rules  []ignoreRule
}

func newIgnoreChecker(_ string, logger *log.Logger) *ignoreChecker {
	return &ignoreChecker{logger: logger}
}

func (ic *ignoreChecker) loadDir(dir string) {
	for _, name := range []string{".gitignore", ".ignore"} {
		data, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			continue
		}
		for _, line := range strings.Split(string(data), "\n") {
			line = strings.TrimRight(line, " \t\r")
			if line == "" || line[0] == '#' {
				continue
			}
			r := ignoreRule{dir: dir}
			if line[0] == '!' {
				r.negate = true
				line = line[1:]
			}
			if strings.HasSuffix(line, "/") {
				r.dirOnly = true
				line = strings.TrimSuffix(line, "/")
			}
			r.pattern = line
			ic.rules = append(ic.rules, r)
		}
		ic.logger.Printf("loaded %s", filepath.Join(dir, name))
	}
}

func (ic *ignoreChecker) isIgnored(path string, isDir bool) bool {
	matched := false
	for _, r := range ic.rules {
		if r.dirOnly && !isDir {
			continue
		}
		rel, err := filepath.Rel(r.dir, path)
		if err != nil || strings.HasPrefix(rel, "..") {
			continue
		}
		if matchIgnore(r.pattern, rel) {
			matched = !r.negate
		}
	}
	return matched
}

func matchIgnore(pattern, rel string) bool {
	// Strip leading slash — anchors to the .gitignore's directory (already scoped).
	pat := strings.TrimPrefix(pattern, "/")
	// Flatten **/ to match at any depth via basename fallback.
	flat := strings.ReplaceAll(pat, "**/", "")
	if ok, _ := filepath.Match(flat, filepath.Base(rel)); ok {
		return true
	}
	if strings.Contains(pat, "/") {
		if ok, _ := filepath.Match(flat, rel); ok {
			return true
		}
	}
	return false
}
