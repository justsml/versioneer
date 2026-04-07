package output

import (
	"encoding/json"
	"io"

	"github.com/justsml/versioneer/internal/model"
)

type jsonFmt struct{}

func (jsonFmt) Format(w io.Writer, result *model.ScanResult) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(result)
}
