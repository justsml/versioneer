package parser

import (
	"strings"

	"github.com/justsml/versioneer/internal/model"
)

type goMod struct{}

func (goMod) Parse(path string, data []byte) ([]model.Dependency, error) {
	var deps []model.Dependency
	var inRequire bool

	for _, raw := range strings.Split(string(data), "\n") {
		line := strings.TrimSpace(raw)

		if line == ")" {
			inRequire = false
			continue
		}
		if strings.HasPrefix(line, "require (") || strings.HasPrefix(line, "require(") {
			inRequire = true
			continue
		}

		// Single-line require: require github.com/foo/bar v1.2.3
		if strings.HasPrefix(line, "require ") && !strings.Contains(line, "(") {
			parts := strings.Fields(line)
			if len(parts) >= 3 {
				deps = append(deps, model.Dependency{
					Name:       parts[1],
					Version:    parts[2],
					Ecosystem:  model.EcoGo,
					DepType:    depTypeGo(line),
					SourceFile: path,
				})
			}
			continue
		}

		if inRequire {
			line = stripComment(line)
			parts := strings.Fields(line)
			if len(parts) >= 2 {
				deps = append(deps, model.Dependency{
					Name:       parts[0],
					Version:    parts[1],
					Ecosystem:  model.EcoGo,
					DepType:    depTypeGo(raw),
					SourceFile: path,
				})
			}
		}
	}
	return deps, nil
}

func depTypeGo(line string) string {
	if strings.Contains(line, "// indirect") {
		return model.DepIndirect
	}
	return model.DepDirect
}

func stripComment(s string) string {
	if i := strings.Index(s, "//"); i >= 0 {
		return s[:i]
	}
	return s
}
