// Package resolver looks up actual installed/locked versions for dependencies.
// It checks lock files first (fast, no I/O per dep), then falls back to on-disk checks.
package resolver

import (
	"bufio"
	"bytes"
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
	model.EcoNPM: true, model.EcoGo: true, model.EcoRust: true, model.EcoPython: true,
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
			ResolveProject(result.RootDir, p, logger)
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

// ResolveProject fills in resolved versions and timestamps for a single project.
// Exported so callers can pipeline scan and resolve concurrently.
func ResolveProject(rootDir string, p *model.Project, logger *log.Logger) {
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
	case model.EcoNPM:
		resolveNPM(projectDir, p)
	case model.EcoGo:
		resolveGo(projectDir, p)
	case model.EcoRust:
		resolveRust(projectDir, p)
	case model.EcoPython:
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
	dec := json.NewDecoder(bytes.NewReader(data))

	// Expect top-level {
	if t, err := dec.Token(); err != nil || t != json.Delim('{') {
		return nil
	}

	resolved := make(map[string]string, 512)

	for dec.More() {
		tok, err := dec.Token()
		if err != nil {
			break
		}
		key, ok := tok.(string)
		if !ok {
			continue
		}
		switch key {
		case "packages":
			parseLockVersionMap(dec, resolved, true)
		case "dependencies":
			if len(resolved) == 0 {
				parseLockVersionMap(dec, resolved, false)
			} else {
				skipJSONValue(dec)
			}
		default:
			skipJSONValue(dec)
		}
	}

	if len(resolved) == 0 {
		return nil
	}
	return resolved
}

// parseLockVersionMap streams a JSON object whose values are objects with a
// "version" field, extracting only the name→version pairs we need.
func parseLockVersionMap(dec *json.Decoder, resolved map[string]string, stripNodeModules bool) {
	if t, err := dec.Token(); err != nil || t != json.Delim('{') {
		return
	}
	for dec.More() {
		tok, err := dec.Token()
		if err != nil {
			return
		}
		key, ok := tok.(string)
		if !ok {
			continue
		}
		name := key
		if stripNodeModules {
			name = strings.TrimPrefix(name, "node_modules/")
		}
		ver := extractLockVersion(dec)
		if name != "" && ver != "" {
			resolved[name] = ver
		}
	}
	dec.Token() // closing }
}

// extractLockVersion reads a JSON object and returns only its "version" value,
// skipping all other fields (integrity hashes, resolved URLs, etc.) without allocating.
func extractLockVersion(dec *json.Decoder) string {
	if t, err := dec.Token(); err != nil || t != json.Delim('{') {
		return ""
	}
	var version string
	for dec.More() {
		tok, err := dec.Token()
		if err != nil {
			return version
		}
		key, ok := tok.(string)
		if !ok {
			continue
		}
		if key == "version" {
			if v, err := dec.Token(); err == nil {
				if s, ok := v.(string); ok {
					version = s
				}
			}
		} else {
			skipJSONValue(dec)
		}
	}
	dec.Token() // closing }
	return version
}

// skipJSONValue skips one complete JSON value (object, array, or scalar).
func skipJSONValue(dec *json.Decoder) {
	tok, err := dec.Token()
	if err != nil {
		return
	}
	if delim, ok := tok.(json.Delim); ok {
		switch delim {
		case '{':
			for dec.More() {
				dec.Token() // key
				skipJSONValue(dec)
			}
			dec.Token() // }
		case '[':
			for dec.More() {
				skipJSONValue(dec)
			}
			dec.Token() // ]
		}
	}
}

func resolveYarnLock(dir string, p *model.Project) bool {
	path := filepath.Join(dir, "yarn.lock")
	resolved, ok := cachedReadLock(path, parseYarnLock)
	if !ok {
		return false
	}
	return applyResolved(p, resolved)
}

// parseYarnLock uses a line-based state machine instead of regex — avoids the
// overhead of multi-line regex matching over multi-MB lock files.
// Handles both yarn v1 (`version "X"`) and berry v2+ (`version: X`) formats.
func parseYarnLock(data []byte) map[string]string {
	resolved := make(map[string]string, 256)
	sc := bufio.NewScanner(bytes.NewReader(data))
	var currentName string

	for sc.Scan() {
		line := sc.Bytes()
		if len(line) == 0 {
			currentName = ""
			continue
		}

		// Non-indented line = potential package header
		if line[0] != ' ' && line[0] != '\t' {
			currentName = yarnPackageName(line)
			continue
		}

		// Indented line starting with "version" = version declaration
		if currentName != "" {
			trimmed := bytes.TrimLeft(line, " \t")
			if len(trimmed) > 7 && trimmed[0] == 'v' &&
				string(trimmed[:7]) == "version" {
				ver := yarnVersionValue(trimmed[7:])
				if ver != "" {
					if _, exists := resolved[currentName]; !exists {
						resolved[currentName] = ver
					}
					currentName = ""
				}
			}
		}
	}
	return resolved
}

// yarnPackageName extracts the package name from a yarn.lock header line.
// Handles: "lodash@^4.17.21":  and  "@babel/core@^7.0.0", "@babel/core@^7.12.0":
func yarnPackageName(line []byte) string {
	if len(line) == 0 || line[0] == '#' {
		return ""
	}
	s := string(line)
	if s[0] == '"' {
		s = s[1:]
	}
	// For scoped packages (@scope/pkg), skip the leading @
	start := 0
	if len(s) > 0 && s[0] == '@' {
		start = 1
	}
	idx := strings.IndexByte(s[start:], '@')
	if idx < 0 {
		return ""
	}
	return s[:start+idx]
}

// yarnVersionValue extracts the version from the tail of a "version" line.
// Input is everything after "version", e.g. ` "4.17.21"` or `: 4.17.21`.
func yarnVersionValue(rest []byte) string {
	i := 0
	for i < len(rest) && (rest[i] == ' ' || rest[i] == ':' || rest[i] == '\t') {
		i++
	}
	rest = rest[i:]
	if len(rest) >= 2 && rest[0] == '"' {
		rest = rest[1:]
		if j := bytes.IndexByte(rest, '"'); j >= 0 {
			rest = rest[:j]
		}
	}
	rest = bytes.TrimRight(rest, " \t\r")
	if len(rest) == 0 {
		return ""
	}
	return string(rest)
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
	resolved := make(map[string]string, 256)
	for _, m := range pnpmDepRe.FindAllSubmatch(data, -1) {
		name := string(m[1])
		val := string(m[2])
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
	path := findLockUp(dir, "Cargo.lock")
	if path == "" {
		return
	}
	resolved, ok := cachedReadLock(path, parseCargoLock)
	if !ok {
		return
	}
	applyResolved(p, resolved)
}

func parseCargoLock(data []byte) map[string]string {
	resolved := make(map[string]string, 128)
	sc := bufio.NewScanner(bytes.NewReader(data))
	var currentName string
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if strings.HasPrefix(line, "name = ") {
			currentName = strings.Trim(strings.TrimPrefix(line, "name = "), `"`)
		} else if strings.HasPrefix(line, "version = ") && currentName != "" {
			resolved[currentName] = strings.Trim(strings.TrimPrefix(line, "version = "), `"`)
			currentName = ""
		}
	}
	return resolved
}

// findLockUp walks up from dir looking for filename, stopping at .git boundary.
func findLockUp(dir, filename string) string {
	for d := dir; ; {
		path := filepath.Join(d, filename)
		if _, err := os.Stat(path); err == nil {
			return path
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
	return ""
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
