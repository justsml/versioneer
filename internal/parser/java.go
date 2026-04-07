package parser

import (
	"encoding/xml"
	"regexp"
	"strings"

	"github.com/versioneer/versioneer/internal/model"
)

// pom.xml
type pomXML struct{}

func (pomXML) Parse(path string, data []byte) ([]model.Dependency, error) {
	var pom struct {
		Dependencies struct {
			Dep []struct {
				GroupID    string `xml:"groupId"`
				ArtifactID string `xml:"artifactId"`
				Version    string `xml:"version"`
				Scope      string `xml:"scope"`
			} `xml:"dependency"`
		} `xml:"dependencies"`
	}

	if err := xml.Unmarshal(data, &pom); err != nil {
		return nil, err
	}

	deps := make([]model.Dependency, 0, len(pom.Dependencies.Dep))
	for _, d := range pom.Dependencies.Dep {
		depType := "direct"
		switch d.Scope {
		case "test":
			depType = "dev"
		case "provided", "runtime", "system":
			depType = d.Scope
		}
		deps = append(deps, model.Dependency{
			Name:       d.GroupID + ":" + d.ArtifactID,
			Version:    d.Version,
			Ecosystem:  "java",
			DepType:    depType,
			SourceFile: path,
		})
	}
	return deps, nil
}

// build.gradle / build.gradle.kts
type gradle struct{}

var gradleDepRe = regexp.MustCompile(
	`(?:implementation|api|compileOnly|runtimeOnly|testImplementation|testRuntimeOnly|annotationProcessor)\s*[\(]?\s*['"]([^'"]+)['"]`,
)

func (gradle) Parse(path string, data []byte) ([]model.Dependency, error) {
	var deps []model.Dependency

	for _, m := range gradleDepRe.FindAllStringSubmatch(string(data), -1) {
		coord := m[1]
		parts := strings.SplitN(coord, ":", 3)
		if len(parts) < 2 {
			continue
		}

		name := parts[0] + ":" + parts[1]
		version := ""
		if len(parts) == 3 {
			version = parts[2]
		}

		depType := "direct"
		if strings.Contains(m[0], "test") || strings.Contains(m[0], "Test") {
			depType = "dev"
		}

		deps = append(deps, model.Dependency{
			Name:       name,
			Version:    version,
			Ecosystem:  "java",
			DepType:    depType,
			SourceFile: path,
		})
	}
	return deps, nil
}
