package output

import (
	"encoding/csv"
	"io"

	"github.com/versioneer/versioneer/internal/model"
	"github.com/versioneer/versioneer/internal/resolver"
)

type csvFmt struct{}

func (csvFmt) Format(w io.Writer, result *model.ScanResult) error {
	cw := csv.NewWriter(w)
	defer cw.Flush()

	hasRes := anyResolved(result)
	hasTimes := anyTimestamps(result)

	// Build header dynamically.
	header := []string{"name", "version"}
	if hasRes {
		header = append(header, "resolved")
	}
	header = append(header, "ecosystem", "dep_type")
	if hasTimes {
		header = append(header, "manifest_modified", "deps_dir_modified", "deps_dir")
	}
	header = append(header, "source_file")

	if err := cw.Write(header); err != nil {
		return err
	}

	for _, p := range result.Projects {
		for _, d := range p.Dependencies {
			row := []string{d.Name, d.Version}
			if hasRes {
				resolved := d.Resolved
				if resolved == "" {
					resolved = ""
				}
				row = append(row, resolved)
			}
			row = append(row, d.Ecosystem, d.DepType)
			if hasTimes {
				row = append(row,
					resolver.FormatTime(p.ManifestModified),
					resolver.FormatTime(p.DepsDirModified),
					p.DepsDir,
				)
			}
			row = append(row, d.SourceFile)
			if err := cw.Write(row); err != nil {
				return err
			}
		}
	}
	return nil
}
