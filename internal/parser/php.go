package parser

import (
	"encoding/json"
	"strings"

	"github.com/justsml/versioneer/internal/model"
)

type composerJSON struct{}

func (composerJSON) Parse(path string, data []byte) ([]model.Dependency, error) {
	var comp struct {
		Require    map[string]string `json:"require"`
		RequireDev map[string]string `json:"require-dev"`
	}
	if err := json.Unmarshal(data, &comp); err != nil {
		return nil, err
	}

	deps := make([]model.Dependency, 0, len(comp.Require)+len(comp.RequireDev))
	for name, version := range comp.Require {
		if name == "php" || strings.HasPrefix(name, "ext-") {
			continue
		}
		deps = append(deps, model.Dependency{
			Name:       name,
			Version:    version,
			Ecosystem:  model.EcoPHP,
			DepType:    model.DepDirect,
			SourceFile: path,
		})
	}
	for name, version := range comp.RequireDev {
		deps = append(deps, model.Dependency{
			Name:       name,
			Version:    version,
			Ecosystem:  model.EcoPHP,
			DepType:    model.DepDev,
			SourceFile: path,
		})
	}
	return deps, nil
}
