package resolver_test

import (
	"context"
	"io"
	"log"
	"os"
	"path/filepath"
	"testing"

	"github.com/justsml/versioneer/internal/model"
	"github.com/justsml/versioneer/internal/resolver"
	"github.com/justsml/versioneer/internal/scanner"
)

var discardLogger = log.New(io.Discard, "", 0)

func TestResolveNPMFromNodeModules(t *testing.T) {
	dir := t.TempDir()

	// Create package.json with a dependency on react.
	pkgJSON := []byte(`{"dependencies":{"react":"^18.0.0"}}`)
	if err := os.WriteFile(filepath.Join(dir, "package.json"), pkgJSON, 0o644); err != nil {
		t.Fatal(err)
	}

	// Create node_modules/react/package.json with the installed version.
	nmDir := filepath.Join(dir, "node_modules", "react")
	if err := os.MkdirAll(nmDir, 0o755); err != nil {
		t.Fatal(err)
	}
	reactPkg := []byte(`{"name":"react","version":"18.2.0"}`)
	if err := os.WriteFile(filepath.Join(nmDir, "package.json"), reactPkg, 0o644); err != nil {
		t.Fatal(err)
	}

	// Scan to discover the project.
	result, err := scanner.Scan(context.Background(), dir, discardLogger)
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	if len(result.Projects) != 1 {
		t.Fatalf("expected 1 project, got %d", len(result.Projects))
	}
	if result.Projects[0].Ecosystem != "npm" {
		t.Fatalf("expected npm ecosystem, got %q", result.Projects[0].Ecosystem)
	}

	// Resolve versions.
	resolver.Resolve(context.Background(), result, discardLogger)

	var found bool
	for _, dep := range result.Projects[0].Dependencies {
		if dep.Name == "react" {
			if dep.Resolved != "18.2.0" {
				t.Errorf("react resolved = %q, want 18.2.0", dep.Resolved)
			}
			found = true
		}
	}
	if !found {
		t.Error("react dependency not found in scan results")
	}
}

func TestResolveGoFromGoSum(t *testing.T) {
	dir := t.TempDir()

	// Create go.mod with a dependency.
	goMod := []byte("module example.com/test\n\ngo 1.22\n\nrequire golang.org/x/text v0.14.0\n")
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), goMod, 0o644); err != nil {
		t.Fatal(err)
	}

	// Create go.sum with version checksums.
	goSum := []byte("golang.org/x/text v0.14.0 h1:ScX5w1eTa3QqT8oi6+ziP7dTV1S2+ALU0bI+0zXKWiQ=\ngolang.org/x/text v0.14.0/go.mod h1:18ZOQIKpY8NJVqYksKHtTdi31H5itFRjB5/qKTNYzSU=\n")
	if err := os.WriteFile(filepath.Join(dir, "go.sum"), goSum, 0o644); err != nil {
		t.Fatal(err)
	}

	// Scan to discover the project.
	result, err := scanner.Scan(context.Background(), dir, discardLogger)
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	if len(result.Projects) != 1 {
		t.Fatalf("expected 1 project, got %d", len(result.Projects))
	}
	if result.Projects[0].Ecosystem != "go" {
		t.Fatalf("expected go ecosystem, got %q", result.Projects[0].Ecosystem)
	}

	// Resolve versions.
	resolver.Resolve(context.Background(), result, discardLogger)

	var found bool
	for _, dep := range result.Projects[0].Dependencies {
		if dep.Name == "golang.org/x/text" {
			if dep.Resolved != "v0.14.0" {
				t.Errorf("golang.org/x/text resolved = %q, want v0.14.0", dep.Resolved)
			}
			found = true
		}
	}
	if !found {
		t.Error("golang.org/x/text dependency not found in scan results")
	}
}

func TestResolveSkipsUnknownEcosystem(t *testing.T) {
	// Verify that Resolve doesn't panic on an unsupported ecosystem.
	result := &model.ScanResult{
		RootDir: t.TempDir(),
		Projects: []model.Project{
			{
				Path:         ".",
				Ecosystem:    "unknown",
				ManifestFile: "unknown.lock",
				Dependencies: []model.Dependency{
					{Name: "foo", Version: "1.0.0", Ecosystem: "unknown"},
				},
			},
		},
	}

	resolver.Resolve(context.Background(), result, discardLogger)

	if result.Projects[0].Dependencies[0].Resolved != "" {
		t.Error("expected empty resolved for unknown ecosystem")
	}
}
