package parser

import (
	"regexp"
	"strings"

	"github.com/justsml/versioneer/internal/model"
)

type gemfile struct{}

var gemRe = regexp.MustCompile(`gem\s+['"]([^'"]+)['"](?:\s*,\s*['"]([^'"]+)['"])?`)

func (gemfile) Parse(path string, data []byte) ([]model.Dependency, error) {
	var deps []model.Dependency
	inGroup := ""

	for _, raw := range strings.Split(string(data), "\n") {
		line := strings.TrimSpace(raw)

		if strings.HasPrefix(line, "group") && strings.Contains(line, ":development") {
			inGroup = "dev"
			continue
		}
		if strings.HasPrefix(line, "group") && strings.Contains(line, ":test") {
			inGroup = "dev"
			continue
		}
		if line == "end" {
			inGroup = ""
			continue
		}

		m := gemRe.FindStringSubmatch(line)
		if m == nil {
			continue
		}

		depType := "direct"
		if inGroup == "dev" {
			depType = "dev"
		}

		deps = append(deps, model.Dependency{
			Name:       m[1],
			Version:    m[2],
			Ecosystem:  "ruby",
			DepType:    depType,
			SourceFile: path,
		})
	}
	return deps, nil
}
