package main

import (
	"testing"

	"github.com/justsml/versioneer/internal/matcher"
	"github.com/justsml/versioneer/internal/model"
)

// ---------------------------------------------------------------------------
// extractConcreteVersion
// ---------------------------------------------------------------------------

func TestExtractConcreteVersion(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{input: "1.7.9", want: "1.7.9"},
		{input: "v2.0.0", want: "v2.0.0"},
		{input: "", want: ""},
		{input: "*", want: ""},
		{input: "^1.3.0", want: ""},
		{input: "~1.2.3", want: ""},
		{input: ">=1.0.0", want: ""},
		// Starts with a digit so the current implementation treats it as concrete.
		{input: "1.0.0 || 2.0.0", want: ""},
		{input: "latest", want: ""},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := extractConcreteVersion(tt.input)
			if got != tt.want {
				t.Errorf("extractConcreteVersion(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// filter
// ---------------------------------------------------------------------------

func TestFilter(t *testing.T) {
	base := &model.ScanResult{
		RootDir: "/code",
		Projects: []model.Project{
			{
				Path:      "/code/app",
				Ecosystem: "npm",
				Dependencies: []model.Dependency{
					{Name: "react", Ecosystem: "npm", DepType: "direct"},
					{Name: "react-dom", Ecosystem: "npm", DepType: "direct"},
					{Name: "eslint", Ecosystem: "npm", DepType: "dev"},
				},
			},
			{
				Path:      "/code/api",
				Ecosystem: "go",
				Dependencies: []model.Dependency{
					{Name: "gin", Ecosystem: "go", DepType: "direct"},
					{Name: "testify", Ecosystem: "go", DepType: "dev"},
				},
			},
		},
		TotalDeps: 5,
	}

	t.Run("filter by dependency name", func(t *testing.T) {
		result := filter(clone(base), "react", "", "")
		if len(result.Projects) != 1 {
			t.Fatalf("expected 1 project, got %d", len(result.Projects))
		}
		if len(result.Projects[0].Dependencies) != 2 {
			t.Errorf("expected 2 deps (react, react-dom), got %d", len(result.Projects[0].Dependencies))
		}
		if result.TotalDeps != 2 {
			t.Errorf("expected TotalDeps=2, got %d", result.TotalDeps)
		}
	})

	t.Run("filter by ecosystem", func(t *testing.T) {
		result := filter(clone(base), "", "go", "")
		if len(result.Projects) != 1 {
			t.Fatalf("expected 1 project, got %d", len(result.Projects))
		}
		if result.Projects[0].Path != "/code/api" {
			t.Errorf("expected /code/api project, got %s", result.Projects[0].Path)
		}
		if result.TotalDeps != 2 {
			t.Errorf("expected TotalDeps=2, got %d", result.TotalDeps)
		}
	})

	t.Run("filter by dep type", func(t *testing.T) {
		result := filter(clone(base), "", "", "dev")
		if len(result.Projects) != 2 {
			t.Fatalf("expected 2 projects, got %d", len(result.Projects))
		}
		if result.TotalDeps != 2 {
			t.Errorf("expected TotalDeps=2, got %d", result.TotalDeps)
		}
	})

	t.Run("combined filters", func(t *testing.T) {
		result := filter(clone(base), "eslint", "npm", "dev")
		if len(result.Projects) != 1 {
			t.Fatalf("expected 1 project, got %d", len(result.Projects))
		}
		if len(result.Projects[0].Dependencies) != 1 {
			t.Errorf("expected 1 dep, got %d", len(result.Projects[0].Dependencies))
		}
		if result.Projects[0].Dependencies[0].Name != "eslint" {
			t.Errorf("expected eslint, got %s", result.Projects[0].Dependencies[0].Name)
		}
	})

	t.Run("filter produces empty result", func(t *testing.T) {
		result := filter(clone(base), "nonexistent", "", "")
		if len(result.Projects) != 0 {
			t.Errorf("expected 0 projects, got %d", len(result.Projects))
		}
		if result.TotalDeps != 0 {
			t.Errorf("expected TotalDeps=0, got %d", result.TotalDeps)
		}
	})

	t.Run("projects with no matching deps are excluded", func(t *testing.T) {
		result := filter(clone(base), "gin", "", "")
		if len(result.Projects) != 1 {
			t.Fatalf("expected 1 project, got %d", len(result.Projects))
		}
		if result.Projects[0].Path != "/code/api" {
			t.Errorf("expected /code/api, got %s", result.Projects[0].Path)
		}
	})
}

// clone does a shallow copy of the ScanResult so filter tests don't interfere
// with each other (filter returns a new result, but just in case).
func clone(r *model.ScanResult) *model.ScanResult {
	projects := make([]model.Project, len(r.Projects))
	for i, p := range r.Projects {
		deps := make([]model.Dependency, len(p.Dependencies))
		copy(deps, p.Dependencies)
		p.Dependencies = deps
		projects[i] = p
	}
	return &model.ScanResult{
		RootDir:      r.RootDir,
		Projects:     projects,
		TotalDeps:    r.TotalDeps,
		ScanDuration: r.ScanDuration,
	}
}

// ---------------------------------------------------------------------------
// checkVulns
// ---------------------------------------------------------------------------

func mustRule(t *testing.T, s string) matcher.Rule {
	t.Helper()
	r, err := matcher.ParseRule(s)
	if err != nil {
		t.Fatalf("ParseRule(%q): %v", s, err)
	}
	return r
}

func TestCheckVulns(t *testing.T) {
	t.Run("resolved version matching rule", func(t *testing.T) {
		result := &model.ScanResult{
			Projects: []model.Project{
				{
					Path: "/app",
					Dependencies: []model.Dependency{
						{Name: "axios", Version: "^1.5.0", Resolved: "1.5.1"},
					},
				},
			},
		}
		rules := []matcher.Rule{mustRule(t, "axios@>=1.3.0,<1.6.0")}

		result, hits := checkVulns(result, rules)
		if hits != 1 {
			t.Errorf("expected 1 hit, got %d", hits)
		}
		if len(result.Projects) != 1 {
			t.Fatalf("expected 1 project, got %d", len(result.Projects))
		}
		if result.Projects[0].Dependencies[0].Name != "axios" {
			t.Errorf("expected axios, got %s", result.Projects[0].Dependencies[0].Name)
		}
	})

	t.Run("no resolved version but concrete spec version", func(t *testing.T) {
		result := &model.ScanResult{
			Projects: []model.Project{
				{
					Path: "/app",
					Dependencies: []model.Dependency{
						{Name: "axios", Version: "1.5.1", Resolved: ""},
					},
				},
			},
		}
		rules := []matcher.Rule{mustRule(t, "axios@>=1.3.0,<1.6.0")}

		result, hits := checkVulns(result, rules)
		if hits != 1 {
			t.Errorf("expected 1 hit, got %d", hits)
		}
	})

	t.Run("range spec marked UNRESOLVED", func(t *testing.T) {
		result := &model.ScanResult{
			Projects: []model.Project{
				{
					Path: "/app",
					Dependencies: []model.Dependency{
						{Name: "axios", Version: "^1.5.0", Resolved: ""},
					},
				},
			},
		}
		rules := []matcher.Rule{mustRule(t, "axios@>=1.3.0,<1.6.0")}

		result, hits := checkVulns(result, rules)
		if hits != 1 {
			t.Errorf("expected 1 hit (unresolved), got %d", hits)
		}
		if result.Projects[0].Dependencies[0].Resolved != "UNRESOLVED" {
			t.Errorf("expected Resolved=UNRESOLVED, got %q", result.Projects[0].Dependencies[0].Resolved)
		}
	})

	t.Run("no matches returns 0", func(t *testing.T) {
		result := &model.ScanResult{
			Projects: []model.Project{
				{
					Path: "/app",
					Dependencies: []model.Dependency{
						{Name: "lodash", Version: "4.17.21", Resolved: "4.17.21"},
					},
				},
			},
		}
		rules := []matcher.Rule{mustRule(t, "axios@>=1.3.0,<1.6.0")}

		result, hits := checkVulns(result, rules)
		if hits != 0 {
			t.Errorf("expected 0 hits, got %d", hits)
		}
	})

	t.Run("name-only rule matches any version", func(t *testing.T) {
		result := &model.ScanResult{
			Projects: []model.Project{
				{
					Path: "/app",
					Dependencies: []model.Dependency{
						{Name: "colors", Version: "1.4.2", Resolved: "1.4.2"},
					},
				},
			},
		}
		rules := []matcher.Rule{mustRule(t, "colors")}

		result, hits := checkVulns(result, rules)
		if hits != 1 {
			t.Errorf("expected 1 hit, got %d", hits)
		}
	})
}
