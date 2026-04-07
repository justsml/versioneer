package model

import "time"

// Dependency represents a single resolved dependency from a project file.
type Dependency struct {
	Name       string `json:"name"`
	Version    string `json:"version"`              // version expression from manifest
	Resolved   string `json:"resolved,omitempty"`    // actual installed/locked version
	Ecosystem  string `json:"ecosystem"`             // "go", "npm", "python", "rust", "ruby", "java"
	DepType    string `json:"dep_type"`              // "direct", "dev", "indirect", "optional"
	SourceFile string `json:"source_file"`
}

// Project represents a discovered project root with its dependency manifest.
type Project struct {
	Path         string       `json:"path"`
	Ecosystem    string       `json:"ecosystem"`
	ManifestFile string       `json:"manifest_file"`
	Dependencies []Dependency `json:"dependencies"`
	ScannedAt    time.Time    `json:"scanned_at"`

	// Timestamps for audit context.
	ProjectModified  *time.Time `json:"project_modified,omitempty"`  // newest file in project dir
	ManifestModified *time.Time `json:"manifest_modified,omitempty"` // the manifest file itself
	DepsDirModified  *time.Time `json:"deps_dir_modified,omitempty"` // node_modules, vendor, .venv, etc.
	DepsDir          string     `json:"deps_dir,omitempty"`          // which deps dir was found
}

// ScanResult is the top-level output of a scan.
type ScanResult struct {
	RootDir      string    `json:"root_dir"`
	Projects     []Project `json:"projects"`
	TotalDeps    int       `json:"total_deps"`
	ScanDuration time.Duration `json:"scan_duration"`
}
