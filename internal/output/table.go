package output

import (
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/justsml/versioneer/internal/model"
	"github.com/justsml/versioneer/internal/resolver"
)

type tableFmt struct{}

func (tableFmt) Format(w io.Writer, result *model.ScanResult) error {
	hasRes := anyResolved(result)
	hasTimes := anyTimestamps(result)

	// Sort projects by manifest file for grouped output.
	projects := make([]model.Project, len(result.Projects))
	copy(projects, result.Projects)
	sort.Slice(projects, func(i, j int) bool {
		return projects[i].ManifestFile < projects[j].ManifestFile
	})

	// Sort dependencies alphabetically within each project.
	for i := range projects {
		sort.Slice(projects[i].Dependencies, func(a, b int) bool {
			return projects[i].Dependencies[a].Name < projects[i].Dependencies[b].Name
		})
	}

	// Build header + grouped rows. A nil row marks a group boundary.
	var header []string
	type row struct {
		cols  []string // nil = group separator
		label string   // only set for separators
	}
	var rows []row

	for _, p := range projects {
		if len(p.Dependencies) == 0 {
			continue
		}

		mAge := resolver.RelativeAge(p.ManifestModified)
		dAge := resolver.RelativeAge(p.DepsDirModified)

		// Group header.
		rows = append(rows, row{label: fmt.Sprintf("%s (%d deps)", p.ManifestFile, len(p.Dependencies))})

		for _, d := range p.Dependencies {
			resolved := d.Resolved
			if resolved == "" {
				resolved = "—"
			}
			via := d.ResolvedBy

			switch {
			case hasRes && hasTimes:
				if header == nil {
					header = []string{"NAME", "SPEC", "INSTALLED", "VIA", "ECO", "TYPE", "MANIFEST", "DEPS DIR"}
				}
				rows = append(rows, row{cols: []string{d.Name, d.Version, resolved, via, d.Ecosystem, d.DepType, mAge, dAge}})
			case hasRes:
				if header == nil {
					header = []string{"NAME", "SPEC", "INSTALLED", "VIA", "ECOSYSTEM", "TYPE"}
				}
				rows = append(rows, row{cols: []string{d.Name, d.Version, resolved, via, d.Ecosystem, d.DepType}})
			default:
				if header == nil {
					header = []string{"NAME", "VERSION", "ECOSYSTEM", "TYPE"}
				}
				rows = append(rows, row{cols: []string{d.Name, d.Version, d.Ecosystem, d.DepType}})
			}
		}
	}

	if header == nil {
		header = []string{"NAME", "VERSION", "ECOSYSTEM", "TYPE"}
	}

	// Compute column widths across all data rows.
	ncols := len(header)
	widths := make([]int, ncols)
	for i, h := range header {
		widths[i] = len(h)
	}
	for _, r := range rows {
		if r.cols == nil {
			continue
		}
		for i := range min(len(r.cols), ncols) {
			widths[i] = max(widths[i], len(r.cols[i]))
		}
	}

	// Build format string.
	var fmtParts []string
	for i, w := range widths {
		if i < ncols-1 {
			fmtParts = append(fmtParts, fmt.Sprintf("%%-%ds", w))
		} else {
			fmtParts = append(fmtParts, "%s")
		}
	}
	fmtStr := strings.Join(fmtParts, "  ") + "\n"

	// Total table width for separator lines.
	tableWidth := 0
	for i, w := range widths {
		tableWidth += w
		if i < ncols-1 {
			tableWidth += 2
		}
	}

	// Render.
	vals := make([]any, ncols)
	headerPrinted := false

	for _, r := range rows {
		if r.cols == nil {
			// Group separator.
			if headerPrinted {
				fmt.Fprintln(w) // blank line between groups
			}
			label := "── " + r.label + " "
			pad := tableWidth - len(label)
			if pad < 0 {
				pad = 0
			}
			fmt.Fprint(w, label+strings.Repeat("─", pad)+"\n")

			// Print header + separator under each group.
			for i, h := range header {
				vals[i] = h
			}
			fmt.Fprintf(w, fmtStr, vals...)
			sepParts := make([]string, ncols)
			for i, w := range widths {
				sepParts[i] = strings.Repeat("─", w)
			}
			fmt.Fprint(w, strings.Join(sepParts, "  ")+"\n")
			headerPrinted = true
			continue
		}

		for i := range ncols {
			if i < len(r.cols) {
				vals[i] = r.cols[i]
			} else {
				vals[i] = ""
			}
		}
		fmt.Fprintf(w, fmtStr, vals...)
	}

	fmt.Fprintf(w, "\n%d dependencies across %d projects (scanned in %s)\n",
		result.TotalDeps, len(result.Projects), result.ScanDuration)
	return nil
}
