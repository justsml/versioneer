package resolver

import (
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/justsml/versioneer/internal/model"
)

// depsDirCandidates maps ecosystem -> possible dependency directories to check.
var depsDirCandidates = map[string][]string{
	"npm":    {"node_modules"},
	"go":     {"vendor"},
	"python": {".venv", "venv", "env", "__pypackages__"},
	"rust":   {"target"},
	"ruby":   {"vendor/bundle", "vendor"},
	"java":   {".gradle/caches", ".m2"},
	"php":    {"vendor"},
	"dart":   {".dart_tool", ".pub-cache"},
}

// collectTimestamps fills in the Modified timestamps on a project.
func collectTimestamps(rootDir string, p *model.Project) {
	absManifest := p.ManifestFile
	if !filepath.IsAbs(absManifest) {
		absManifest = filepath.Join(rootDir, absManifest)
	}
	projectDir := filepath.Dir(absManifest)

	// 1. Manifest file modified time.
	if info, err := os.Stat(absManifest); err == nil {
		t := info.ModTime()
		p.ManifestModified = &t
	}

	// 2. Project directory modified time (the dir itself, not recursive).
	if info, err := os.Stat(projectDir); err == nil {
		t := info.ModTime()
		p.ProjectModified = &t
	}

	// 3. Deps directory — walk up for monorepos (same strategy as lock files).
	candidates := depsDirCandidates[p.Ecosystem]
	if len(candidates) == 0 {
		return
	}

	for d := projectDir; ; {
		for _, candidate := range candidates {
			depsPath := filepath.Join(d, candidate)
			if info, err := os.Stat(depsPath); err == nil && info.IsDir() {
				t := info.ModTime()
				p.DepsDirModified = &t
				rel, _ := filepath.Rel(rootDir, depsPath)
				p.DepsDir = rel
				return
			}
		}
		parent := filepath.Dir(d)
		if parent == d {
			break
		}
		// Stop at repo root.
		if _, err := os.Stat(filepath.Join(d, ".git")); err == nil && d != projectDir {
			break
		}
		d = parent
	}
}

// formatTime returns a human-readable timestamp, or "—" if nil.
func FormatTime(t *time.Time) string {
	if t == nil {
		return "—"
	}
	return t.Format("2006-01-02 15:04")
}

// RelativeAge returns a human-friendly age string like "3d ago" or "2mo ago".
func RelativeAge(t *time.Time) string {
	if t == nil {
		return "—"
	}
	d := time.Since(*t)
	switch {
	case d < time.Minute:
		return "just now"
	case d < time.Hour:
		return pluralize(int(d.Minutes()), "min") + " ago"
	case d < 24*time.Hour:
		return pluralize(int(d.Hours()), "hr") + " ago"
	case d < 30*24*time.Hour:
		return pluralize(int(d.Hours()/24), "day") + " ago"
	case d < 365*24*time.Hour:
		return pluralize(int(d.Hours()/(24*30)), "mo") + " ago"
	default:
		return pluralize(int(d.Hours()/(24*365)), "yr") + " ago"
	}
}

func pluralize(n int, unit string) string {
	if n == 1 {
		return "1" + unit
	}
	return strconv.Itoa(n) + unit
}
