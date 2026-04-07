package parser

import (
	"strings"

	"github.com/justsml/versioneer/internal/model"
)

type cargoToml struct{}

func (cargoToml) Parse(path string, data []byte) ([]model.Dependency, error) {
	var deps []model.Dependency
	var section string

	for _, raw := range strings.Split(string(data), "\n") {
		line := strings.TrimSpace(raw)

		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			section = line[1 : len(line)-1]
			continue
		}

		depType := ""
		switch section {
		case "dependencies":
			depType = "direct"
		case "dev-dependencies":
			depType = "dev"
		case "build-dependencies":
			depType = "build"
		default:
			continue
		}

		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}

		name := strings.TrimSpace(parts[0])
		if name == "" {
			continue
		}
		version := extractCargoVersion(strings.TrimSpace(parts[1]))

		deps = append(deps, model.Dependency{
			Name:       name,
			Version:    version,
			Ecosystem:  "rust",
			DepType:    depType,
			SourceFile: path,
		})
	}
	return deps, nil
}

// extractCargoVersion handles both "1.0" and { version = "1.0", features = [...] }
func extractCargoVersion(val string) string {
	val = strings.TrimSpace(val)
	// Simple string: "1.0"
	if strings.HasPrefix(val, `"`) {
		return strings.Trim(val, `"`)
	}
	// Inline table: { version = "1.0", ... }
	if strings.HasPrefix(val, "{") {
		for _, part := range strings.Split(val, ",") {
			kv := strings.SplitN(part, "=", 2)
			if len(kv) == 2 && strings.TrimSpace(strings.Trim(kv[0], "{ ")) == "version" {
				return strings.Trim(strings.TrimSpace(kv[1]), `"} `)
			}
		}
	}
	return val
}
