package output

import (
	"fmt"
	"io"
	"sort"
	"time"

	"github.com/justsml/versioneer/internal/model"
	"github.com/justsml/versioneer/internal/resolver"
)

type summaryFmt struct{}

func (summaryFmt) Format(w io.Writer, result *model.ScanResult) error {
	ecosystems := map[string]int{}
	depTypes := map[string]int{}
	resolveSources := map[string]int{}
	resolved := 0
	unresolved := 0

	for _, p := range result.Projects {
		for _, d := range p.Dependencies {
			ecosystems[d.Ecosystem]++
			depTypes[d.DepType]++
			if d.Resolved != "" {
				resolved++
				if d.ResolvedBy != "" {
					resolveSources[d.ResolvedBy]++
				}
			} else {
				unresolved++
			}
		}
	}

	fmt.Fprintf(w, "Scan: %s\n", result.RootDir)
	fmt.Fprintf(w, "  Projects:     %d\n", len(result.Projects))
	fmt.Fprintf(w, "  Dependencies: %d\n", result.TotalDeps)
	if resolved > 0 || unresolved > 0 {
		fmt.Fprintf(w, "  Resolved:     %d / %d (%.0f%%)\n",
			resolved, result.TotalDeps, float64(resolved)/float64(max(result.TotalDeps, 1))*100)
		if len(resolveSources) > 0 {
			for _, kv := range sortedMap(resolveSources) {
				fmt.Fprintf(w, "    via %-10s %d\n", kv.key, kv.val)
			}
		}
	}
	fmt.Fprintf(w, "  Duration:     %s\n\n", result.ScanDuration)

	fmt.Fprintln(w, "By ecosystem:")
	for _, kv := range sortedMap(ecosystems) {
		fmt.Fprintf(w, "  %-12s %d\n", kv.key, kv.val)
	}

	fmt.Fprintln(w, "\nBy type:")
	for _, kv := range sortedMap(depTypes) {
		fmt.Fprintf(w, "  %-12s %d\n", kv.key, kv.val)
	}

	// Timestamp staleness summary.
	if anyTimestamps(result) {
		fmt.Fprintln(w, "\nStaleness:")
		staleManifest := 0
		staleDeps := 0
		noDeps := 0
		threshold := 90 * 24 * time.Hour // 90 days

		for _, p := range result.Projects {
			if p.ManifestModified != nil && time.Since(*p.ManifestModified) > threshold {
				staleManifest++
			}
			if p.DepsDirModified != nil && time.Since(*p.DepsDirModified) > threshold {
				staleDeps++
			}
			if p.DepsDir == "" {
				noDeps++
			}
		}

		fmt.Fprintf(w, "  Manifests >90d old:   %d / %d\n", staleManifest, len(result.Projects))
		fmt.Fprintf(w, "  Deps dirs >90d old:   %d / %d\n", staleDeps, len(result.Projects))
		fmt.Fprintf(w, "  No deps dir found:    %d / %d\n", noDeps, len(result.Projects))

		// Oldest and newest deps dirs.
		var oldest, newest *time.Time
		var oldestProject, newestProject string
		for _, p := range result.Projects {
			if p.DepsDirModified == nil {
				continue
			}
			if oldest == nil || p.DepsDirModified.Before(*oldest) {
				oldest = p.DepsDirModified
				oldestProject = p.ManifestFile
			}
			if newest == nil || p.DepsDirModified.After(*newest) {
				newest = p.DepsDirModified
				newestProject = p.ManifestFile
			}
		}
		if oldest != nil {
			fmt.Fprintf(w, "  Oldest deps install:  %s (%s) — %s\n",
				resolver.FormatTime(oldest), resolver.RelativeAge(oldest), oldestProject)
		}
		if newest != nil {
			fmt.Fprintf(w, "  Newest deps install:  %s (%s) — %s\n",
				resolver.FormatTime(newest), resolver.RelativeAge(newest), newestProject)
		}
	}

	return nil
}

type kv struct {
	key string
	val int
}

func sortedMap(m map[string]int) []kv {
	out := make([]kv, 0, len(m))
	for k, v := range m {
		out = append(out, kv{k, v})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].val > out[j].val })
	return out
}
