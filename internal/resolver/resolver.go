// Package resolver looks up actual installed/locked versions for dependencies.
// It checks lock files first (fast, no I/O per dep), then falls back to on-disk checks.
package resolver

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"sync"

	"github.com/justsml/versioneer/internal/model"
)

var supportedResolvers = map[string]bool{
	"npm": true, "go": true, "rust": true, "python": true,
}

func hasResolver(ecosystem string) bool {
	return supportedResolvers[ecosystem]
}

// lockCache caches parsed lock file data by absolute path.
// Each entry is a *lockResult computed exactly once via sync.OnceFunc.
var lockCache sync.Map // map[string]*sync.Once-guarded result

type lockResult struct {
	data map[string]string
}

func cachedReadLock(path string, parse func([]byte) map[string]string) (map[string]string, bool) {
	// Use LoadOrStore with a sync.Once to ensure each path is parsed exactly once,
	// even when multiple goroutines request the same lock file concurrently.
	type entry struct {
		once   sync.Once
		result lockResult
	}
	actual, _ := lockCache.LoadOrStore(path, &entry{})
	e := actual.(*entry)
	e.once.Do(func() {
		data, err := os.ReadFile(path)
		if err != nil {
			return
		}
		m := parse(data)
		if len(m) > 0 {
			e.result.data = m
		}
	})
	return e.result.data, e.result.data != nil
}

// Resolve fills in the Resolved field for all dependencies in the scan result.
// It processes projects in parallel, caching lock file reads across projects.
func Resolve(ctx context.Context, result *model.ScanResult, logger *log.Logger) {
	sem := make(chan struct{}, runtime.NumCPU())
	var wg sync.WaitGroup
	var unsupportedMu sync.Mutex
	unsupported := map[string]int{}
	for i := range result.Projects {
		wg.Add(1)
		go func(p *model.Project) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			if ctx.Err() != nil {
				return
			}
			if !hasResolver(p.Ecosystem) {
				unsupportedMu.Lock()
				unsupported[p.Ecosystem]++
				unsupportedMu.Unlock()
			}
			resolveProject(result.RootDir, p, logger)
		}(&result.Projects[i])
	}
	wg.Wait()

	// Surface unsupported ecosystems so users know resolution was skipped.
	if len(unsupported) > 0 {
		var ecos []string
		for eco, n := range unsupported {
			ecos = append(ecos, fmt.Sprintf("%s (%d projects)", eco, n))
		}
		logger.Printf("resolve: version resolution not supported for: %s", strings.Join(ecos, ", "))
	}
}

func resolveProject(rootDir string, p *model.Project, logger *log.Logger) {
	// Always collect timestamps, even for empty projects.
	collectTimestamps(rootDir, p)

	if len(p.Dependencies) == 0 {
		return
	}

	// Determine the absolute project directory from the manifest path.
	absManifest := p.ManifestFile
	if !filepath.IsAbs(absManifest) {
		absManifest = filepath.Join(rootDir, absManifest)
	}
	projectDir := filepath.Dir(absManifest)

	switch p.Ecosystem {
	case "npm":
		resolveNPM(projectDir, p)
	case "go":
		resolveGo(projectDir, p)
	case "rust":
		resolveRust(projectDir, p)
	case "python":
		resolvePython(projectDir, p)
	default:
		logger.Printf("resolve: no resolver for ecosystem %q (%s)", p.Ecosystem, p.ManifestFile)
	}
}

// --- npm: package-lock.json > yarn.lock > pnpm-lock.yaml > node_modules ---

func resolveNPM(dir string, p *model.Project) {
	// Walk up from the package dir to find a lock file (handles monorepos).
	for d := dir; ; {
		if resolveNPMLockV2(d, p) {
			return
		}
		if resolveYarnLock(d, p) {
			return
		}
		if resolvePnpmLock(d, p) {
			return
		}
		parent := filepath.Dir(d)
		if parent == d {
			break
		}
		// Stop walking if we hit a .git directory (repo root).
		if _, err := os.Stat(filepath.Join(d, ".git")); err == nil && d != dir {
			break
		}
		d = parent
	}
	// Fallback: walk up for node_modules too.
	for d := dir; ; {
		if resolveNodeModules(d, p) {
			return
		}
		parent := filepath.Dir(d)
		if parent == d {
			break
		}
		if _, err := os.Stat(filepath.Join(d, ".git")); err == nil && d != dir {
			break
		}
		d = parent
	}
}

// package-lock.json v2/v3 format: .packages["node_modules/<name>"].version
func resolveNPMLockV2(dir string, p *model.Project) bool {
	path := filepath.Join(dir, "package-lock.json")
	resolved, ok := cachedReadLock(path, parsePackageLock)
	if !ok {
		return false
	}
	return applyResolved(p, resolved)
}

func parsePackageLock(data []byte) map[string]string {
	var lock struct {
		Packages map[string]struct {
			Version string `json:"version"`
		} `json:"packages"`
		Dependencies map[string]struct {
			Version string `json:"version"`
		} `json:"dependencies"`
	}
	if json.Unmarshal(data, &lock) != nil {
		return nil
	}

	resolved := make(map[string]string, len(lock.Packages)+len(lock.Dependencies))
	for key, pkg := range lock.Packages {
		name := strings.TrimPrefix(key, "node_modules/")
		if name != "" && pkg.Version != "" {
			resolved[name] = pkg.Version
		}
	}
	if len(resolved) == 0 {
		for name, dep := range lock.Dependencies {
			if dep.Version != "" {
				resolved[name] = dep.Version
			}
		}
	}
	return resolved
}

// yarn.lock format: "<name>@<range>":\n  version "<version>"
var yarnVersionRe = regexp.MustCompile(`(?m)^"?(@?[^@\s"]+)@[^:]+:\s*\n\s+version\s+"([^"]+)"`)

// Also handle yarn berry (v2+) format: "<name>@npm:<range>":\n  version: <version>
var yarnBerryRe = regexp.MustCompile(`(?m)^"?(@?[^@\s"]+)@(?:npm:)?[^:]+:\s*\n\s+version:\s+(.+)`)

func resolveYarnLock(dir string, p *model.Project) bool {
	path := filepath.Join(dir, "yarn.lock")
	resolved, ok := cachedReadLock(path, parseYarnLock)
	if !ok {
		return false
	}
	return applyResolved(p, resolved)
}

func parseYarnLock(data []byte) map[string]string {
	content := string(data)
	resolved := map[string]string{}
	for _, m := range yarnVersionRe.FindAllStringSubmatch(content, -1) {
		resolved[m[1]] = m[2]
	}
	if len(resolved) == 0 {
		for _, m := range yarnBerryRe.FindAllStringSubmatch(content, -1) {
			resolved[m[1]] = strings.TrimSpace(m[2])
		}
	}
	return resolved
}

// pnpm-lock.yaml: packages/<name>/<version> or dependencies: <name>: <version>
// pnpmDepRe matches dependency entries in pnpm-lock.yaml.
// The value must start with a digit (version) to avoid matching metadata fields.
var pnpmDepRe = regexp.MustCompile(`(?m)^[ \t]+'?(@?[^':\s]+)'?:\s+(\d[^\s]*)`)


func resolvePnpmLock(dir string, p *model.Project) bool {
	path := filepath.Join(dir, "pnpm-lock.yaml")
	resolved, ok := cachedReadLock(path, parsePnpmLock)
	if !ok {
		return false
	}
	return applyResolved(p, resolved)
}

func parsePnpmLock(data []byte) map[string]string {
	resolved := map[string]string{}
	for _, m := range pnpmDepRe.FindAllStringSubmatch(string(data), -1) {
		name := m[1]
		val := m[2]
		// Strip pnpm peer-dep suffix like "1.2.3(react@18.0.0)"
		if idx := strings.Index(val, "("); idx > 0 {
			val = val[:idx]
		}
		resolved[name] = val
	}
	return resolved
}

func applyResolved(p *model.Project, resolved map[string]string) bool {
	applied := false
	for i := range p.Dependencies {
		if v, ok := resolved[p.Dependencies[i].Name]; ok {
			p.Dependencies[i].Resolved = v
			applied = true
		}
	}
	return applied
}

// Fallback: read node_modules/<name>/package.json
func resolveNodeModules(dir string, p *model.Project) bool {
	found := false
	for i := range p.Dependencies {
		if p.Dependencies[i].Resolved != "" {
			continue
		}
		pkgPath := filepath.Join(dir, "node_modules", p.Dependencies[i].Name, "package.json")
		data, err := os.ReadFile(pkgPath)
		if err != nil {
			continue
		}
		var pkg struct {
			Version string `json:"version"`
		}
		if json.Unmarshal(data, &pkg) == nil && pkg.Version != "" {
			p.Dependencies[i].Resolved = pkg.Version
			found = true
		}
	}
	return found
}

// --- Go: go.sum has exact versions ---

// resolveGo extracts versions from go.sum.
// NOTE: go.sum is a checksum database, not a lock file. It may contain stale
// entries from previously-used versions. For authoritative resolution, run
// "go list -m all". We prefer non-/go.mod entries (which represent the actual
// source tree checksum) over /go.mod-only entries.
func resolveGo(dir string, p *model.Project) {
	f, err := os.Open(filepath.Join(dir, "go.sum"))
	if err != nil {
		return
	}
	defer f.Close()

	resolved := map[string]string{}
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		parts := strings.Fields(sc.Text())
		if len(parts) < 2 {
			continue
		}
		name := parts[0]
		raw := parts[1]
		isGoMod := strings.HasSuffix(raw, "/go.mod")
		ver := strings.TrimSuffix(raw, "/go.mod")
		prev, exists := resolved[name]
		// Prefer non-/go.mod entries; if we only have a /go.mod entry, keep it
		// as a fallback but let a source-tree entry overwrite it.
		if !exists || (isGoMod && prev == "") || !isGoMod {
			resolved[name] = ver
		}
	}

	for i := range p.Dependencies {
		if v, ok := resolved[p.Dependencies[i].Name]; ok {
			p.Dependencies[i].Resolved = v
		}
	}
}

// --- Rust: Cargo.lock ---

func resolveRust(dir string, p *model.Project) {
	data, err := os.ReadFile(filepath.Join(dir, "Cargo.lock"))
	if err != nil {
		// Walk up to workspace root, stopping at .git boundary.
		parent := filepath.Dir(dir)
		for parent != dir {
			data, err = os.ReadFile(filepath.Join(parent, "Cargo.lock"))
			if err == nil {
				break
			}
			// Stop at repo root.
			if _, gitErr := os.Stat(filepath.Join(parent, ".git")); gitErr == nil {
				break
			}
			dir = parent
			parent = filepath.Dir(parent)
		}
		if err != nil {
			return
		}
	}

	resolved := map[string]string{}
	var currentName string
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "name = ") {
			currentName = strings.Trim(strings.TrimPrefix(line, "name = "), `"`)
		}
		if strings.HasPrefix(line, "version = ") && currentName != "" {
			resolved[currentName] = strings.Trim(strings.TrimPrefix(line, "version = "), `"`)
			currentName = ""
		}
	}

	for i := range p.Dependencies {
		if v, ok := resolved[p.Dependencies[i].Name]; ok {
			p.Dependencies[i].Resolved = v
		}
	}
}

// --- Python: pip freeze / venv ---

func resolvePython(dir string, p *model.Project) {
	// Check common venv locations for installed packages
	venvDirs := []string{
		filepath.Join(dir, ".venv"),
		filepath.Join(dir, "venv"),
		filepath.Join(dir, "env"),
	}

	for _, venv := range venvDirs {
		metaDir := filepath.Join(venv, "lib")
		if _, err := os.Stat(metaDir); err != nil {
			continue
		}

		// Build map from dist-info directories
		resolved := readPythonDistInfo(metaDir)
		if len(resolved) == 0 {
			continue
		}

		for i := range p.Dependencies {
			normalized := normalizePythonName(p.Dependencies[i].Name)
			if v, ok := resolved[normalized]; ok {
				p.Dependencies[i].Resolved = v
			}
		}
		return
	}
}

func readPythonDistInfo(libDir string) map[string]string {
	resolved := map[string]string{}

	// Find python3.x/site-packages
	entries, err := os.ReadDir(libDir)
	if err != nil {
		return nil
	}
	for _, e := range entries {
		if !e.IsDir() || !strings.HasPrefix(e.Name(), "python") {
			continue
		}
		sitePackages := filepath.Join(libDir, e.Name(), "site-packages")
		distEntries, err := os.ReadDir(sitePackages)
		if err != nil {
			continue
		}
		for _, de := range distEntries {
			name := de.Name()
			if !strings.HasSuffix(name, ".dist-info") {
				continue
			}
			// Format: <name>-<version>.dist-info
			trimmed := strings.TrimSuffix(name, ".dist-info")
			parts := strings.SplitN(trimmed, "-", 2)
			if len(parts) == 2 {
				normalized := normalizePythonName(parts[0])
				resolved[normalized] = parts[1]
			}
		}
	}
	return resolved
}

func normalizePythonName(name string) string {
	// PEP 503: lowercase, replace [-_.] with -
	name = strings.ToLower(name)
	name = strings.ReplaceAll(name, "_", "-")
	name = strings.ReplaceAll(name, ".", "-")
	return name
}
