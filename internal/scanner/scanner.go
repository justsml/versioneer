package scanner

import (
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

// skipDirs are directory names that should never be descended into.
var skipDirs = map[string]struct{}{
	"node_modules": {}, ".git": {}, "vendor": {}, ".venv": {},
	"venv": {}, "__pycache__": {}, ".tox": {}, "target": {},
	".gradle": {}, "build": {}, "dist": {}, ".idea": {},
	".vscode": {}, ".next": {}, ".nuxt": {},
}

// Scan walks root in parallel and returns all discovered projects.
func Scan(root string, logger *log.Logger) (*model.ScanResult, error) {
	start := time.Now()
	manifests := parser.ManifestFiles()

	// Phase 1: walk the tree and collect manifest paths.
	var paths []string
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil // skip unreadable entries
		}
		if d.IsDir() {
			name := d.Name()
			if strings.HasPrefix(name, ".") && name != "." {
				return filepath.SkipDir
			}
			if _, skip := skipDirs[name]; skip {
				return filepath.SkipDir
			}
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
