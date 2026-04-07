package parser

import (
	"strings"

	"github.com/justsml/versioneer/internal/model"
)

type pubspecYaml struct{}

func (pubspecYaml) Parse(path string, data []byte) ([]model.Dependency, error) {
	var deps []model.Dependency
	var section string

	for _, raw := range strings.Split(string(data), "\n") {
		trimmed := strings.TrimSpace(raw)

		// Top-level keys (no leading whitespace)
		if len(raw) > 0 && raw[0] != ' ' && raw[0] != '\t' {
			if strings.HasPrefix(trimmed, "dependencies:") {
				section = "direct"
			} else if strings.HasPrefix(trimmed, "dev_dependencies:") {
				section = "dev"
			} else {
				section = ""
			}
			continue
		}

		if section == "" {
			continue
		}

		// Skip sub-keys (indented more than 2 spaces under a dep)
		indent := len(raw) - len(strings.TrimLeft(raw, " \t"))
		if indent > 4 {
			continue
		}

		parts := strings.SplitN(trimmed, ":", 2)
		if len(parts) != 2 {
			continue
		}
		name := strings.TrimSpace(parts[0])
		version := strings.TrimSpace(parts[1])
		if name == "" || strings.HasPrefix(name, "#") {
			continue
		}
		// Skip complex dependency specs (git, path, sdk)
		if version == "" || strings.HasPrefix(version, "{") {
			continue
		}

		deps = append(deps, model.Dependency{
			Name:       name,
			Version:    version,
			Ecosystem:  "dart",
			DepType:    section,
			SourceFile: path,
		})
	}
	return deps, nil
}
