package output

import (
	"fmt"
	"io"
	"strings"

	"github.com/versioneer/versioneer/internal/model"
	"github.com/versioneer/versioneer/internal/resolver"
)

type tableFmt struct{}

func (tableFmt) Format(w io.Writer, result *model.ScanResult) error {
	hasRes := anyResolved(result)
	hasTimes := anyTimestamps(result)

	if hasRes && hasTimes {
		return formatAuditTable(w, result)
	}
	if hasRes {
		return formatResolvedTable(w, result)
	}
	return formatBasicTable(w, result)
}

// formatAuditTable: full security audit view with resolved versions and timestamps.
func formatAuditTable(w io.Writer, result *model.ScanResult) error {
	type row struct {
		name, version, resolved, eco, depType string
		manifestAge, depsAge, source           string
	}

	header := row{"NAME", "SPEC", "INSTALLED", "ECO", "TYPE", "MANIFEST", "DEPS DIR", "SOURCE"}

	var rows []row
	for _, p := range result.Projects {
		mAge := resolver.RelativeAge(p.ManifestModified)
		dAge := resolver.RelativeAge(p.DepsDirModified)

		for _, d := range p.Dependencies {
			resolved := d.Resolved
			if resolved == "" {
				resolved = "—"
			}
			rows = append(rows, row{
				name:        d.Name,
				version:     d.Version,
				resolved:    resolved,
				eco:         d.Ecosystem,
				depType:     d.DepType,
				manifestAge: mAge,
				depsAge:     dAge,
				source:      d.SourceFile,
			})
		}
	}

	w8 := [8]int{
		len(header.name), len(header.version), len(header.resolved), len(header.eco),
		len(header.depType), len(header.manifestAge), len(header.depsAge), len(header.source),
	}
	for _, r := range rows {
		w8[0] = max(w8[0], len(r.name))
		w8[1] = max(w8[1], len(r.version))
		w8[2] = max(w8[2], len(r.resolved))
		w8[3] = max(w8[3], len(r.eco))
		w8[4] = max(w8[4], len(r.depType))
		w8[5] = max(w8[5], len(r.manifestAge))
		w8[6] = max(w8[6], len(r.depsAge))
		w8[7] = max(w8[7], len(r.source))
	}

	fmtStr := fmt.Sprintf("%%-%ds  %%-%ds  %%-%ds  %%-%ds  %%-%ds  %%-%ds  %%-%ds  %%s\n",
		w8[0], w8[1], w8[2], w8[3], w8[4], w8[5], w8[6])
	sep := makeSep(w8[:])

	fmt.Fprintf(w, fmtStr, header.name, header.version, header.resolved, header.eco,
		header.depType, header.manifestAge, header.depsAge, header.source)
	fmt.Fprint(w, sep)
	for _, r := range rows {
		fmt.Fprintf(w, fmtStr, r.name, r.version, r.resolved, r.eco,
			r.depType, r.manifestAge, r.depsAge, r.source)
	}

	fmt.Fprintf(w, "\n%d dependencies across %d projects (scanned in %s)\n",
		result.TotalDeps, len(result.Projects), result.ScanDuration)
	return nil
}

// formatResolvedTable: resolved versions without timestamps.
func formatResolvedTable(w io.Writer, result *model.ScanResult) error {
	type row struct{ name, version, resolved, eco, depType, source string }
	header := row{"NAME", "SPEC", "INSTALLED", "ECOSYSTEM", "TYPE", "SOURCE"}

	var rows []row
	for i := range result.Projects {
		for _, d := range result.Projects[i].Dependencies {
			resolved := d.Resolved
			if resolved == "" {
				resolved = "—"
			}
			rows = append(rows, row{d.Name, d.Version, resolved, d.Ecosystem, d.DepType, d.SourceFile})
		}
	}

	w6 := [6]int{len(header.name), len(header.version), len(header.resolved), len(header.eco), len(header.depType), len(header.source)}
	for _, r := range rows {
		w6[0] = max(w6[0], len(r.name))
		w6[1] = max(w6[1], len(r.version))
		w6[2] = max(w6[2], len(r.resolved))
		w6[3] = max(w6[3], len(r.eco))
		w6[4] = max(w6[4], len(r.depType))
		w6[5] = max(w6[5], len(r.source))
	}

	fmtStr := fmt.Sprintf("%%-%ds  %%-%ds  %%-%ds  %%-%ds  %%-%ds  %%s\n",
		w6[0], w6[1], w6[2], w6[3], w6[4])
	sep := makeSep(w6[:])

	fmt.Fprintf(w, fmtStr, header.name, header.version, header.resolved, header.eco, header.depType, header.source)
	fmt.Fprint(w, sep)
	for _, r := range rows {
		fmt.Fprintf(w, fmtStr, r.name, r.version, r.resolved, r.eco, r.depType, r.source)
	}
	fmt.Fprintf(w, "\n%d dependencies across %d projects (scanned in %s)\n",
		result.TotalDeps, len(result.Projects), result.ScanDuration)
	return nil
}

// formatBasicTable: no resolved versions, no timestamps.
func formatBasicTable(w io.Writer, result *model.ScanResult) error {
	type row struct{ name, version, eco, depType, source string }
	header := row{"NAME", "VERSION", "ECOSYSTEM", "TYPE", "SOURCE"}

	var rows []row
	for i := range result.Projects {
		for _, d := range result.Projects[i].Dependencies {
			rows = append(rows, row{d.Name, d.Version, d.Ecosystem, d.DepType, d.SourceFile})
		}
	}

	w5 := [5]int{len(header.name), len(header.version), len(header.eco), len(header.depType), len(header.source)}
	for _, r := range rows {
		w5[0] = max(w5[0], len(r.name))
		w5[1] = max(w5[1], len(r.version))
		w5[2] = max(w5[2], len(r.eco))
		w5[3] = max(w5[3], len(r.depType))
		w5[4] = max(w5[4], len(r.source))
	}

	fmtStr := fmt.Sprintf("%%-%ds  %%-%ds  %%-%ds  %%-%ds  %%s\n",
		w5[0], w5[1], w5[2], w5[3])
	sep := makeSep(w5[:])

	fmt.Fprintf(w, fmtStr, header.name, header.version, header.eco, header.depType, header.source)
	fmt.Fprint(w, sep)
	for _, r := range rows {
		fmt.Fprintf(w, fmtStr, r.name, r.version, r.eco, r.depType, r.source)
	}
	fmt.Fprintf(w, "\n%d dependencies across %d projects (scanned in %s)\n",
		result.TotalDeps, len(result.Projects), result.ScanDuration)
	return nil
}

func makeSep(widths []int) string {
	parts := make([]string, len(widths))
	for i, w := range widths {
		parts[i] = strings.Repeat("─", w)
	}
	return strings.Join(parts, "  ") + "\n"
}
