package parser

import (
	"encoding/json"

	"github.com/justsml/versioneer/internal/model"
)

type packageJSON struct{}

func (packageJSON) Parse(path string, data []byte) ([]model.Dependency, error) {
	var pkg struct {
		Dependencies         map[string]string `json:"dependencies"`
		DevDependencies      map[string]string `json:"devDependencies"`
		PeerDependencies     map[string]string `json:"peerDependencies"`
		OptionalDependencies map[string]string `json:"optionalDependencies"`
	}
	if err := json.Unmarshal(data, &pkg); err != nil {
		return nil, err
	}

	total := len(pkg.Dependencies) + len(pkg.DevDependencies) +
		len(pkg.PeerDependencies) + len(pkg.OptionalDependencies)
	deps := make([]model.Dependency, 0, total)

	add := func(m map[string]string, depType string) {
		for name, version := range m {
			deps = append(deps, model.Dependency{
				Name:       name,
				Version:    version,
				Ecosystem:  "npm",
				DepType:    depType,
				SourceFile: path,
			})
		}
	}

	add(pkg.Dependencies, "direct")
	add(pkg.DevDependencies, "dev")
	add(pkg.PeerDependencies, "peer")
	add(pkg.OptionalDependencies, "optional")

	return deps, nil
}
