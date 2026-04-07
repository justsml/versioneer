package output

import "github.com/justsml/versioneer/internal/model"

// anyResolved returns true if any dependency has a resolved version.
func anyResolved(r *model.ScanResult) bool {
	for i := range r.Projects {
		for j := range r.Projects[i].Dependencies {
			if r.Projects[i].Dependencies[j].Resolved != "" {
				return true
			}
		}
	}
	return false
}

// anyTimestamps returns true if any project has timestamp data.
func anyTimestamps(r *model.ScanResult) bool {
	for i := range r.Projects {
		if r.Projects[i].ManifestModified != nil {
			return true
		}
	}
	return false
}
