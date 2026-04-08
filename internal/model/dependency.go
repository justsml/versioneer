package model

import "time"

// Ecosystem identifies a package manager ecosystem.
type Ecosystem = string

const (
	EcoNPM    Ecosystem = "npm"
	EcoGo     Ecosystem = "go"
	EcoPython Ecosystem = "python"
	EcoRust   Ecosystem = "rust"
	EcoRuby   Ecosystem = "ruby"
	EcoJava   Ecosystem = "java"
	EcoPHP    Ecosystem = "php"
	EcoDart   Ecosystem = "dart"
)

// DepType classifies how a dependency is used.
type DepType = string

const (
	DepDirect   DepType = "direct"
	DepDev      DepType = "dev"
	DepIndirect DepType = "indirect"
	DepPeer     DepType = "peer"
	DepOptional DepType = "optional"
	DepBuild    DepType = "build"
	DepRuntime  DepType = "runtime"
	DepTest     DepType = "test"
)

// Dependency represents a single resolved dependency from a project file.
type Dependency struct {
	Name       string    `json:"name"`
	Version    string    `json:"version"`              // version expression from manifest
	Resolved   string    `json:"resolved,omitempty"`    // actual installed/locked version
	Ecosystem  Ecosystem `json:"ecosystem"`
	DepType    DepType   `json:"dep_type"`
	SourceFile string    `json:"source_file"`
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
