package output

import (
	"fmt"
	"io"
	"strings"

	"github.com/justsml/versioneer/internal/model"
	"github.com/justsml/versioneer/internal/resolver"
)

type tableFmt struct{}

func (tableFmt) Format(w io.Writer, result *model.ScanResult) error {
	hasRes := anyResolved(result)
	hasTimes := anyTimestamps(result)

	var header []string
	var rows [][]string

	for _, p := range result.Projects {
		mAge := resolver.RelativeAge(p.ManifestModified)
		dAge := resolver.RelativeAge(p.DepsDirModified)

		for _, d := range p.Dependencies {
			resolved := d.Resolved
			if resolved == "" {
				resolved = "—"
			}

			switch {
			case hasRes && hasTimes:
				if header == nil {
					header = []string{"NAME", "SPEC", "INSTALLED", "ECO", "TYPE", "MANIFEST", "DEPS DIR", "SOURCE"}
				}
				rows = append(rows, []string{d.Name, d.Version, resolved, d.Ecosystem, d.DepType, mAge, dAge, d.SourceFile})
			case hasRes:
				if header == nil {
					header = []string{"NAME", "SPEC", "INSTALLED", "ECOSYSTEM", "TYPE", "SOURCE"}
				}
				rows = append(rows, []string{d.Name, d.Version, resolved, d.Ecosystem, d.DepType, d.SourceFile})
			default:
				if header == nil {
					header = []string{"NAME", "VERSION", "ECOSYSTEM", "TYPE", "SOURCE"}
				}
				rows = append(rows, []string{d.Name, d.Version, d.Ecosystem, d.DepType, d.SourceFile})
			}
		}
	}

	if header == nil {
		// No data — pick a default header so we still print the summary.
		header = []string{"NAME", "VERSION", "ECOSYSTEM", "TYPE", "SOURCE"}
	}

	writeTable(w, header, rows)
	fmt.Fprintf(w, "\n%d dependencies across %d projects (scanned in %s)\n",
		result.TotalDeps, len(result.Projects), result.ScanDuration)
	return nil
}

// writeTable renders header + rows with auto-sized columns.
func writeTable(w io.Writer, header []string, rows [][]string) {
	ncols := len(header)
	widths := make([]int, ncols)
	for i, h := range header {
		widths[i] = len(h)
	}
	for _, row := range rows {
		for i := range min(len(row), ncols) {
			widths[i] = max(widths[i], len(row[i]))
		}
	}

	// Build format string: all columns left-aligned with 2-space gap,
	// last column has no padding.
	var fmtParts []string
	for i, w := range widths {
		if i < ncols-1 {
			fmtParts = append(fmtParts, fmt.Sprintf("%%-%ds", w))
		} else {
			fmtParts = append(fmtParts, "%s")
		}
	}
	fmtStr := strings.Join(fmtParts, "  ") + "\n"

	// Print header.
	vals := make([]any, ncols)
	for i, h := range header {
		vals[i] = h
	}
	fmt.Fprintf(w, fmtStr, vals...)

	// Print separator.
	sepParts := make([]string, ncols)
	for i, w := range widths {
		sepParts[i] = strings.Repeat("─", w)
	}
	fmt.Fprint(w, strings.Join(sepParts, "  ")+"\n")

	// Print rows.
	for _, row := range rows {
		for i := range ncols {
			if i < len(row) {
				vals[i] = row[i]
			} else {
				vals[i] = ""
			}
		}
		fmt.Fprintf(w, fmtStr, vals...)
	}
}
