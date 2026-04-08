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
			if j := strings.Index(line, "]"); j > i {
				line = line[:i] + line[j+1:]
			}
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
			depType = model.DepDev
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

// pyproject.toml — lightweight extraction of [project] dependencies,
// [project.optional-dependencies], and [tool.poetry.dependencies].
type pyprojectToml struct{}

func (pyprojectToml) Parse(path string, data []byte) ([]model.Dependency, error) {
	var deps []model.Dependency
	lines := strings.Split(string(data), "\n")

	var (
		inDepArray     bool   // inside dependencies = [ ... ]
		inOptArray     bool   // inside an optional-dependencies array value
		inPoetryDeps   bool   // inside [tool.poetry.dependencies]
		currentSection string
	)

	for _, raw := range lines {
		line := strings.TrimSpace(raw)

		// Track TOML sections.
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			currentSection = line[1 : len(line)-1]
			inDepArray = false
			inOptArray = false
			inPoetryDeps = currentSection == "tool.poetry.dependencies"
			continue
		}

		// PEP 621: dependencies = [ ... ]
		if currentSection == "project" && strings.HasPrefix(line, "dependencies") && strings.Contains(line, "=") {
			// Check if array starts on this line or next
			if strings.Contains(line, "[") {
				inDepArray = true
			}
			// Handle inline: dependencies = ["foo>=1.0", "bar"]
			if strings.Contains(line, "[") && strings.Contains(line, "]") {
				inDepArray = false
				for _, dep := range extractInlineArray(line) {
					name, version := splitPythonDep(dep)
					if name != "" {
						deps = append(deps, model.Dependency{
							Name: name, Version: version,
							Ecosystem: model.EcoPython, DepType: model.DepDirect, SourceFile: path,
						})
					}
				}
			}
			continue
		}

		// PEP 621: [project.optional-dependencies] section values are arrays
		if strings.HasPrefix(currentSection, "project.optional-dependencies") {
			if strings.Contains(line, "=") && strings.Contains(line, "[") {
				inOptArray = true
				if strings.Contains(line, "]") {
					inOptArray = false
					for _, dep := range extractInlineArray(line) {
						name, version := splitPythonDep(dep)
						if name != "" {
							deps = append(deps, model.Dependency{
								Name: name, Version: version,
								Ecosystem: model.EcoPython, DepType: model.DepOptional, SourceFile: path,
							})
						}
					}
				}
				continue
			}
			if inOptArray {
				if line == "]" {
					inOptArray = false
					continue
				}
				dep := strings.Trim(line, `"', `)
				if dep == "" {
					continue
				}
				name, version := splitPythonDep(dep)
				if name != "" {
					deps = append(deps, model.Dependency{
						Name: name, Version: version,
						Ecosystem: model.EcoPython, DepType: model.DepOptional, SourceFile: path,
					})
				}
				continue
			}
		}

		// Inside PEP 621 dependencies array
		if inDepArray {
			if line == "]" {
				inDepArray = false
				continue
			}
			dep := strings.Trim(line, `"', `)
			if dep == "" {
				continue
			}
			name, version := splitPythonDep(dep)
			if name != "" {
				deps = append(deps, model.Dependency{
					Name: name, Version: version,
					Ecosystem: model.EcoPython, DepType: model.DepDirect, SourceFile: path,
				})
			}
			continue
		}

		// Poetry: [tool.poetry.dependencies]
		if inPoetryDeps {
			if line == "" || strings.HasPrefix(line, "#") {
				continue
			}
			parts := strings.SplitN(line, "=", 2)
			if len(parts) != 2 {
				continue
			}
			name := strings.TrimSpace(parts[0])
			if name == "python" {
				continue // skip python version constraint
			}
			val := strings.TrimSpace(parts[1])
			version := ""
			if strings.HasPrefix(val, `"`) || strings.HasPrefix(val, `'`) {
				version = strings.Trim(val, `"'`)
			} else if strings.HasPrefix(val, "{") {
				// Inline table: {version = "^2.28", optional = true}
				if m := extractInlineTableVersion(val); m != "" {
					version = m
				}
			}
			deps = append(deps, model.Dependency{
				Name: name, Version: version,
				Ecosystem: model.EcoPython, DepType: model.DepDirect, SourceFile: path,
			})
		}
	}
	return deps, nil
}

// extractInlineArray extracts items from a TOML inline array like: key = ["foo>=1.0", "bar"]
func extractInlineArray(line string) []string {
	start := strings.Index(line, "[")
	end := strings.LastIndex(line, "]")
	if start < 0 || end <= start {
		return nil
	}
	inner := line[start+1 : end]
	var items []string
	for _, item := range strings.Split(inner, ",") {
		item = strings.Trim(strings.TrimSpace(item), `"'`)
		if item != "" {
			items = append(items, item)
		}
	}
	return items
}

// extractInlineTableVersion extracts version from {version = "^2.28", ...}
func extractInlineTableVersion(s string) string {
	s = strings.Trim(s, "{}")
	for _, part := range strings.Split(s, ",") {
		part = strings.TrimSpace(part)
		if strings.HasPrefix(part, "version") {
			kv := strings.SplitN(part, "=", 2)
			if len(kv) == 2 {
				return strings.Trim(strings.TrimSpace(kv[1]), `"'`)
			}
		}
	}
	return ""
}
