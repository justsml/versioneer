package parser

import (
	"regexp"
	"strings"

	"github.com/justsml/versioneer/internal/model"
)

// requirements.txt
type requirementsTxt struct{}

var reqVersionRe = regexp.MustCompile(`^([a-zA-Z0-9_.-]+)\s*([><=!~]+\s*[\w.*]+(?:\s*,\s*[><=!~]+\s*[\w.*]+)*)?\s*`)

func (requirementsTxt) Parse(path string, data []byte) ([]model.Dependency, error) {
	var deps []model.Dependency
	for _, raw := range strings.Split(string(data), "\n") {
		line := strings.TrimSpace(raw)
		if line == "" || line[0] == '#' || line[0] == '-' {
			continue
		}
		// Strip inline comments and extras like [security]
		if i := strings.Index(line, "#"); i >= 0 {
			line = line[:i]
		}
		if i := strings.Index(line, "["); i >= 0 {
			line = line[:i] + line[strings.Index(line, "]")+1:]
		}
		line = strings.TrimSpace(line)

		name, version := splitPythonDep(line)
		if name == "" {
			continue
		}
		deps = append(deps, model.Dependency{
			Name:       name,
			Version:    version,
			Ecosystem:  "python",
			DepType:    "direct",
			SourceFile: path,
		})
	}
	return deps, nil
}

func splitPythonDep(s string) (name, version string) {
	m := reqVersionRe.FindStringSubmatch(s)
	if m == nil {
		return "", ""
	}
	return m[1], strings.TrimSpace(m[2])
}

// Pipfile (simple TOML-like parsing for [packages] and [dev-packages])
type pipfile struct{}

func (pipfile) Parse(path string, data []byte) ([]model.Dependency, error) {
	var deps []model.Dependency
	var section string

	for _, raw := range strings.Split(string(data), "\n") {
		line := strings.TrimSpace(raw)
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			section = line[1 : len(line)-1]
			continue
		}
		if section != "packages" && section != "dev-packages" {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}
		name := strings.TrimSpace(parts[0])
		version := strings.Trim(strings.TrimSpace(parts[1]), `"'`)
		depType := "direct"
		if section == "dev-packages" {
			depType = "dev"
		}
		deps = append(deps, model.Dependency{
			Name:       name,
			Version:    version,
			Ecosystem:  "python",
			DepType:    depType,
			SourceFile: path,
		})
	}
	return deps, nil
}

// pyproject.toml — lightweight extraction of [project] dependencies
type pyprojectToml struct{}

func (pyprojectToml) Parse(path string, data []byte) ([]model.Dependency, error) {
	var deps []model.Dependency
	var inDeps bool

	for _, raw := range strings.Split(string(data), "\n") {
		line := strings.TrimSpace(raw)

		if strings.HasPrefix(line, "dependencies") && strings.Contains(line, "[") {
			inDeps = true
			continue
		}
		if inDeps {
			if line == "]" {
				inDeps = false
				continue
			}
			// Strip quotes and whitespace: "requests>=2.28"
			dep := strings.Trim(line, `"', `)
			if dep == "" {
				continue
			}
			name, version := splitPythonDep(dep)
			if name != "" {
				deps = append(deps, model.Dependency{
					Name:       name,
					Version:    version,
					Ecosystem:  "python",
					DepType:    "direct",
					SourceFile: path,
				})
			}
		}
	}
	return deps, nil
}
