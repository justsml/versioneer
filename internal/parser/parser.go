package parser

import (
	"path/filepath"

	"github.com/versioneer/versioneer/internal/model"
)

// Parser extracts dependencies from a manifest file.
type Parser interface {
	// Parse reads the file at path and returns dependencies.
	Parse(path string, data []byte) ([]model.Dependency, error)
}

// manifestParsers maps filename -> parser.
var manifestParsers = map[string]Parser{
	"go.mod":            goMod{},
	"package.json":      packageJSON{},
	"requirements.txt":  requirementsTxt{},
	"Pipfile":           pipfile{},
	"pyproject.toml":    pyprojectToml{},
	"Cargo.toml":        cargoToml{},
	"Gemfile":           gemfile{},
	"pom.xml":           pomXML{},
	"build.gradle":      gradle{},
	"build.gradle.kts":  gradle{},
	"composer.json":     composerJSON{},
	"pubspec.yaml":      pubspecYaml{},
}

// ManifestFiles returns all recognized manifest filenames.
func ManifestFiles() map[string]struct{} {
	out := make(map[string]struct{}, len(manifestParsers))
	for k := range manifestParsers {
		out[k] = struct{}{}
	}
	return out
}

// ForFile returns the parser for a given file path, or nil.
func ForFile(path string) Parser {
	return manifestParsers[filepath.Base(path)]
}
