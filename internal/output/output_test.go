package output

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/justsml/versioneer/internal/model"
)

func testResult() *model.ScanResult {
	now := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)
	return &model.ScanResult{
		RootDir:      "/home/user/projects",
		TotalDeps:    3,
		ScanDuration: 42 * time.Millisecond,
		Projects: []model.Project{
			{
				Path:             "/home/user/projects/app",
				Ecosystem:        "npm",
				ManifestFile:     "app/package.json",
				ManifestModified: &now,
				Dependencies: []model.Dependency{
					{Name: "express", Version: "^4.18.0", Resolved: "4.18.2", ResolvedBy: "lockfile", Ecosystem: "npm", DepType: "direct", SourceFile: "app/package.json"},
					{Name: "jest", Version: "^29.0.0", Ecosystem: "npm", DepType: "dev", SourceFile: "app/package.json"},
				},
			},
			{
				Path:         "/home/user/projects/lib",
				Ecosystem:    "go",
				ManifestFile: "lib/go.mod",
				Dependencies: []model.Dependency{
					{Name: "golang.org/x/text", Version: "v0.14.0", Ecosystem: "go", DepType: "direct", SourceFile: "lib/go.mod"},
				},
			},
		},
	}
}

func testResultMinimal() *model.ScanResult {
	return &model.ScanResult{
		RootDir:      "/tmp/test",
		TotalDeps:    1,
		ScanDuration: 5 * time.Millisecond,
		Projects: []model.Project{
			{
				Path:         "/tmp/test",
				Ecosystem:    "go",
				ManifestFile: "go.mod",
				Dependencies: []model.Dependency{
					{Name: "example.com/foo", Version: "v1.0.0", Ecosystem: "go", DepType: "direct", SourceFile: "go.mod"},
				},
			},
		},
	}
}

func testResultEmpty() *model.ScanResult {
	return &model.ScanResult{
		RootDir:      "/tmp/empty",
		ScanDuration: 1 * time.Millisecond,
	}
}

func TestGet(t *testing.T) {
	for _, name := range []string{"json", "jsonl", "csv", "markdown", "md", "table", "summary"} {
		f, err := Get(name)
		if err != nil {
			t.Errorf("Get(%q) returned error: %v", name, err)
		}
		if f == nil {
			t.Errorf("Get(%q) returned nil formatter", name)
		}
	}

	_, err := Get("nope")
	if err == nil {
		t.Error("Get(\"nope\") should return error")
	}
}

func TestNames(t *testing.T) {
	names := Names()
	if len(names) < 6 {
		t.Errorf("expected at least 6 format names, got %d", len(names))
	}
}

func TestJSONFormat(t *testing.T) {
	var buf bytes.Buffer
	result := testResult()
	if err := (jsonFmt{}).Format(&buf, result); err != nil {
		t.Fatal(err)
	}

	var decoded model.ScanResult
	if err := json.Unmarshal(buf.Bytes(), &decoded); err != nil {
		t.Fatalf("output is not valid JSON: %v", err)
	}
	if decoded.TotalDeps != 3 {
		t.Errorf("total_deps = %d, want 3", decoded.TotalDeps)
	}
	if len(decoded.Projects) != 2 {
		t.Errorf("projects = %d, want 2", len(decoded.Projects))
	}
}

func TestJSONLFormat(t *testing.T) {
	var buf bytes.Buffer
	result := testResult()
	if err := (jsonlFmt{}).Format(&buf, result); err != nil {
		t.Fatal(err)
	}

	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	if len(lines) != 3 {
		t.Errorf("expected 3 JSONL lines, got %d", len(lines))
	}

	for i, line := range lines {
		var dep model.Dependency
		if err := json.Unmarshal([]byte(line), &dep); err != nil {
			t.Errorf("line %d is not valid JSON: %v", i, err)
		}
		if dep.Name == "" {
			t.Errorf("line %d has empty name", i)
		}
	}
}

func TestCSVFormat(t *testing.T) {
	var buf bytes.Buffer
	result := testResult()
	if err := (csvFmt{}).Format(&buf, result); err != nil {
		t.Fatal(err)
	}

	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	// header + 3 dep rows
	if len(lines) != 4 {
		t.Errorf("expected 4 CSV lines (header + 3 rows), got %d", len(lines))
	}

	header := lines[0]
	if !strings.Contains(header, "name") || !strings.Contains(header, "version") {
		t.Errorf("CSV header missing expected columns: %s", header)
	}
}

func TestCSVFormatWithResolved(t *testing.T) {
	var buf bytes.Buffer
	result := testResult()
	if err := (csvFmt{}).Format(&buf, result); err != nil {
		t.Fatal(err)
	}

	header := strings.Split(strings.TrimSpace(buf.String()), "\n")[0]
	if !strings.Contains(header, "resolved") {
		t.Errorf("CSV header should contain 'resolved' column when resolved data present: %s", header)
	}
}

func TestCSVFormatMinimal(t *testing.T) {
	var buf bytes.Buffer
	result := testResultMinimal()
	if err := (csvFmt{}).Format(&buf, result); err != nil {
		t.Fatal(err)
	}

	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	if len(lines) != 2 {
		t.Errorf("expected 2 CSV lines (header + 1 row), got %d", len(lines))
	}

	header := lines[0]
	if strings.Contains(header, "resolved") {
		t.Errorf("CSV header should not contain 'resolved' when no resolved data: %s", header)
	}
}

func TestMarkdownFormat(t *testing.T) {
	var buf bytes.Buffer
	result := testResult()
	if err := (markdownFmt{}).Format(&buf, result); err != nil {
		t.Fatal(err)
	}

	out := buf.String()
	if !strings.Contains(out, "# Dependency Audit Report") {
		t.Error("markdown output missing report header")
	}
	if !strings.Contains(out, "app/package.json") {
		t.Error("markdown output missing project heading")
	}
	if !strings.Contains(out, "express") {
		t.Error("markdown output missing dependency name")
	}
	if !strings.Contains(out, "|") {
		t.Error("markdown output missing table formatting")
	}
}

func TestMarkdownFormatResolved(t *testing.T) {
	var buf bytes.Buffer
	result := testResult()
	if err := (markdownFmt{}).Format(&buf, result); err != nil {
		t.Fatal(err)
	}

	out := buf.String()
	if !strings.Contains(out, "Installed") {
		t.Error("markdown should show Installed column when resolved data present")
	}
	if !strings.Contains(out, "4.18.2") {
		t.Error("markdown should contain resolved version")
	}
}

func TestTableFormat(t *testing.T) {
	var buf bytes.Buffer
	result := testResult()
	if err := (tableFmt{}).Format(&buf, result); err != nil {
		t.Fatal(err)
	}

	out := buf.String()
	if !strings.Contains(out, "express") {
		t.Error("table output missing dependency")
	}
	if !strings.Contains(out, "3 dependencies across 2 projects") {
		t.Error("table output missing summary line")
	}
}

func TestTableFormatMinimal(t *testing.T) {
	var buf bytes.Buffer
	result := testResultMinimal()
	if err := (tableFmt{}).Format(&buf, result); err != nil {
		t.Fatal(err)
	}

	out := buf.String()
	if !strings.Contains(out, "NAME") {
		t.Error("table should show column headers")
	}
	if !strings.Contains(out, "example.com/foo") {
		t.Error("table should contain the dependency")
	}
}

func TestSummaryFormat(t *testing.T) {
	var buf bytes.Buffer
	result := testResult()
	if err := (summaryFmt{}).Format(&buf, result); err != nil {
		t.Fatal(err)
	}

	out := buf.String()
	if !strings.Contains(out, "Projects:     2") {
		t.Error("summary missing project count")
	}
	if !strings.Contains(out, "Dependencies: 3") {
		t.Error("summary missing dependency count")
	}
	if !strings.Contains(out, "By ecosystem:") {
		t.Error("summary missing ecosystem breakdown")
	}
	if !strings.Contains(out, "npm") {
		t.Error("summary missing npm ecosystem")
	}
}

func TestSummaryFormatResolved(t *testing.T) {
	var buf bytes.Buffer
	result := testResult()
	if err := (summaryFmt{}).Format(&buf, result); err != nil {
		t.Fatal(err)
	}

	out := buf.String()
	if !strings.Contains(out, "Resolved:") {
		t.Error("summary should show resolved stats when data present")
	}
	if !strings.Contains(out, "lockfile") {
		t.Error("summary should show resolve sources")
	}
}

func TestEmptyResult(t *testing.T) {
	result := testResultEmpty()
	formatters := []struct {
		name string
		fmt  Formatter
	}{
		{"json", jsonFmt{}},
		{"jsonl", jsonlFmt{}},
		{"csv", csvFmt{}},
		{"markdown", markdownFmt{}},
		{"table", tableFmt{}},
		{"summary", summaryFmt{}},
	}

	for _, tc := range formatters {
		t.Run(tc.name, func(t *testing.T) {
			var buf bytes.Buffer
			if err := tc.fmt.Format(&buf, result); err != nil {
				t.Errorf("%s formatter failed on empty result: %v", tc.name, err)
			}
		})
	}
}

func TestHelpers(t *testing.T) {
	t.Run("anyResolved", func(t *testing.T) {
		if !anyResolved(testResult()) {
			t.Error("expected true for result with resolved deps")
		}
		if anyResolved(testResultMinimal()) {
			t.Error("expected false for result with no resolved deps")
		}
		if anyResolved(testResultEmpty()) {
			t.Error("expected false for empty result")
		}
	})

	t.Run("anyTimestamps", func(t *testing.T) {
		if !anyTimestamps(testResult()) {
			t.Error("expected true for result with timestamps")
		}
		if anyTimestamps(testResultMinimal()) {
			t.Error("expected false for result with no timestamps")
		}
	})
}
