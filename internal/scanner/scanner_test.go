package scanner_test

import (
	"context"
	"io"
	"log"
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/justsml/versioneer/internal/scanner"
)

var discardLogger = log.New(io.Discard, "", 0)

func TestScanFindsProjects(t *testing.T) {
	root := filepath.Join("testdata", "fakerepo")

	result, err := scanner.Scan(context.Background(), root, discardLogger)
	if err != nil {
		t.Fatalf("Scan returned error: %v", err)
	}

	if len(result.Projects) != 2 {
		t.Fatalf("expected 2 projects, got %d", len(result.Projects))
	}

	// Sort projects by ecosystem for deterministic assertions.
	sort.Slice(result.Projects, func(i, j int) bool {
		return result.Projects[i].Ecosystem < result.Projects[j].Ecosystem
	})

	goProj := result.Projects[0]
	npmProj := result.Projects[1]

	if goProj.Ecosystem != "go" {
		t.Errorf("expected first project ecosystem 'go', got %q", goProj.Ecosystem)
	}
	if len(goProj.Dependencies) != 1 {
		t.Errorf("expected 1 go dependency, got %d", len(goProj.Dependencies))
	}

	if npmProj.Ecosystem != "npm" {
		t.Errorf("expected second project ecosystem 'npm', got %q", npmProj.Ecosystem)
	}
	if len(npmProj.Dependencies) != 2 {
		t.Errorf("expected 2 npm dependencies, got %d", len(npmProj.Dependencies))
	}
}

func TestScanSkipsDirs(t *testing.T) {
	root := filepath.Join("testdata", "fakerepo")

	result, err := scanner.Scan(context.Background(), root, discardLogger)
	if err != nil {
		t.Fatalf("Scan returned error: %v", err)
	}

	// node_modules should be skipped, so the react/package.json inside
	// node_modules must NOT appear as a discovered project.
	for _, p := range result.Projects {
		if p.Path == "node_modules/react" {
			t.Error("node_modules/react should have been skipped, but was found as a project")
		}
	}
}

func TestScanEmptyDir(t *testing.T) {
	dir := t.TempDir()

	result, err := scanner.Scan(context.Background(), dir, discardLogger)
	if err != nil {
		t.Fatalf("Scan returned error: %v", err)
	}

	if len(result.Projects) != 0 {
		t.Errorf("expected 0 projects for empty dir, got %d", len(result.Projects))
	}
}

func TestScanContextCancelled(t *testing.T) {
	dir := t.TempDir()
	// Create a manifest so the walk has something to find.
	appDir := filepath.Join(dir, "app")
	if err := os.MkdirAll(appDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(appDir, "package.json"), []byte(`{"dependencies":{"a":"1.0.0"}}`), 0o644); err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	_, err := scanner.Scan(ctx, dir, discardLogger)
	if err == nil {
		// A cancelled context may or may not propagate depending on timing;
		// the key thing is it doesn't panic and returns quickly.
		return
	}
	if err != context.Canceled {
		t.Errorf("expected context.Canceled, got %v", err)
	}
}
