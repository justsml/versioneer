package output

import (
	"encoding/json"
	"io"

	"github.com/justsml/versioneer/internal/model"
)

type jsonlFmt struct{}

func (jsonlFmt) Format(w io.Writer, result *model.ScanResult) error {
	enc := json.NewEncoder(w)
	for i := range result.Projects {
		for j := range result.Projects[i].Dependencies {
			if err := enc.Encode(result.Projects[i].Dependencies[j]); err != nil {
				return err
			}
		}
	}
	return nil
}
