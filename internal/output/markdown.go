package output

import (
	"fmt"
	"io"
	"sort"

	"github.com/justsml/versioneer/internal/model"
	"github.com/justsml/versioneer/internal/resolver"
)

type markdownFmt struct{}

func (markdownFmt) Format(w io.Writer, result *model.ScanResult) error {
	hasRes := anyResolved(result)
	hasTimes := anyTimestamps(result)

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

	fmt.Fprintf(w, "# Dependency Audit Report\n\n")
	fmt.Fprintf(w, "**Root:** `%s`  \n", result.RootDir)
	fmt.Fprintf(w, "**Projects:** %d | **Dependencies:** %d | **Scan time:** %s\n\n",
		len(projects), result.TotalDeps, result.ScanDuration)

	for _, p := range projects {
		fmt.Fprintf(w, "## %s\n\n", p.ManifestFile)

		if hasTimes {
			fmt.Fprintf(w, "- **Manifest modified:** %s\n", resolver.FormatTime(p.ManifestModified))
			if p.DepsDir != "" {
				fmt.Fprintf(w, "- **Deps dir:** `%s` (modified %s)\n", p.DepsDir, resolver.FormatTime(p.DepsDirModified))
			} else {
				fmt.Fprintf(w, "- **Deps dir:** _not found_\n")
			}
			fmt.Fprintln(w)
		}

		if len(p.Dependencies) == 0 {
			fmt.Fprintf(w, "_No dependencies found._\n\n")
			continue
		}

		if hasRes {
			fmt.Fprintf(w, "| Name | Spec | Installed | Via | Type |\n")
			fmt.Fprintf(w, "|------|------|-----------|-----|------|\n")
			for _, d := range p.Dependencies {
				resolved := d.Resolved
				if resolved == "" {
					resolved = "—"
				}
				fmt.Fprintf(w, "| %s | `%s` | **%s** | %s | %s |\n", d.Name, d.Version, resolved, d.ResolvedBy, d.DepType)
			}
		} else {
			fmt.Fprintf(w, "| Name | Version | Type |\n")
			fmt.Fprintf(w, "|------|---------|------|\n")
			for _, d := range p.Dependencies {
				fmt.Fprintf(w, "| %s | %s | %s |\n", d.Name, d.Version, d.DepType)
			}
		}
		fmt.Fprintln(w)
	}
	return nil
}
