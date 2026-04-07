package output

import (
	"fmt"
	"io"

	"github.com/justsml/versioneer/internal/model"
)

// Formatter writes scan results to an output stream.
type Formatter interface {
	Format(w io.Writer, result *model.ScanResult) error
}

var registry = map[string]Formatter{
	"json":     jsonFmt{},
	"jsonl":    jsonlFmt{},
	"csv":      csvFmt{},
	"markdown": markdownFmt{},
	"md":       markdownFmt{},
	"table":    tableFmt{},
	"summary":  summaryFmt{},
}

// Get returns a formatter by name, or an error if unknown.
func Get(name string) (Formatter, error) {
	f, ok := registry[name]
	if !ok {
		return nil, fmt.Errorf("unknown format %q (available: json, jsonl, csv, markdown, table, summary)", name)
	}
	return f, nil
}

// Names returns all registered format names.
func Names() []string {
	out := make([]string, 0, len(registry))
	for k := range registry {
		out = append(out, k)
	}
	return out
}
