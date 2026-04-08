package resolver

import (
	"encoding/json"
	"testing"
)

func TestParsePackageLock(t *testing.T) {
	// npm v2/v3 format with "packages" key.
	data := []byte(`{
  "lockfileVersion": 3,
  "packages": {
    "": { "name": "myapp" },
    "node_modules/react": { "version": "18.2.0" },
    "node_modules/react-dom": { "version": "18.2.0" },
    "node_modules/@scope/pkg": { "version": "1.0.5" }
  }
}`)
	resolved := parsePackageLock(data)
	if resolved == nil {
		t.Fatal("expected non-nil result")
	}
	tests := map[string]string{
		"react":      "18.2.0",
		"react-dom":  "18.2.0",
		"@scope/pkg": "1.0.5",
	}
	for name, want := range tests {
		if got := resolved[name]; got != want {
			t.Errorf("parsePackageLock[%s] = %q, want %q", name, got, want)
		}
	}
}

func TestParsePackageLockV1Fallback(t *testing.T) {
	// npm v1 format with "dependencies" key and no "packages".
	data := []byte(`{
  "lockfileVersion": 1,
  "dependencies": {
    "lodash": { "version": "4.17.21" },
    "express": { "version": "4.18.2" }
  }
}`)
	resolved := parsePackageLock(data)
	if resolved == nil {
		t.Fatal("expected non-nil result")
	}
	if got := resolved["lodash"]; got != "4.17.21" {
		t.Errorf("lodash = %q, want 4.17.21", got)
	}
	if got := resolved["express"]; got != "4.18.2" {
		t.Errorf("express = %q, want 4.18.2", got)
	}
}

func TestParseYarnLock(t *testing.T) {
	data := []byte(`# yarn lockfile v1

"react@^18.0.0":
  version "18.2.0"
  resolved "https://registry.yarnpkg.com/react/-/react-18.2.0.tgz"

"lodash@^4.17.0":
  version "4.17.21"
  resolved "https://registry.yarnpkg.com/lodash/-/lodash-4.17.21.tgz"

"@scope/pkg@^1.0.0":
  version "1.0.5"
  resolved "https://registry.yarnpkg.com/@scope/pkg/-/pkg-1.0.5.tgz"
`)
	resolved := parseYarnLock(data)
	if resolved == nil {
		t.Fatal("expected non-nil result")
	}
	tests := map[string]string{
		"react":      "18.2.0",
		"lodash":     "4.17.21",
		"@scope/pkg": "1.0.5",
	}
	for name, want := range tests {
		if got := resolved[name]; got != want {
			t.Errorf("parseYarnLock[%s] = %q, want %q", name, got, want)
		}
	}
}

func TestParseYarnLockBerry(t *testing.T) {
	data := []byte(`__metadata:
  version: 6

"react@npm:^18.0.0":
  version: 18.2.0
  resolution: "react@npm:18.2.0"

"@scope/pkg@npm:^1.0.0":
  version: 1.0.5
  resolution: "@scope/pkg@npm:1.0.5"
`)
	resolved := parseYarnLock(data)
	if resolved == nil {
		t.Fatal("expected non-nil result")
	}
	if got := resolved["react"]; got != "18.2.0" {
		t.Errorf("react = %q, want 18.2.0", got)
	}
	if got := resolved["@scope/pkg"]; got != "1.0.5" {
		t.Errorf("@scope/pkg = %q, want 1.0.5", got)
	}
}

func TestParsePnpmLock(t *testing.T) {
	// Real pnpm lock files have blank lines between sections.
	data := []byte("lockfileVersion: '6.0'\n\ndependencies:\n  react: 18.2.0\n  lodash: 4.17.21\n  '@scope/pkg': 1.0.5(peer@2.0.0)\n\ndevDependencies:\n  typescript: 5.3.3\n")
	resolved := parsePnpmLock(data)
	if resolved == nil {
		t.Fatal("expected non-nil result")
	}
	tests := map[string]string{
		"react":      "18.2.0",
		"lodash":     "4.17.21",
		"@scope/pkg": "1.0.5",
		"typescript":  "5.3.3",
	}
	for name, want := range tests {
		if got := resolved[name]; got != want {
			t.Errorf("parsePnpmLock[%s] = %q, want %q", name, got, want)
		}
	}
}

func TestNormalizePythonName(t *testing.T) {
	tests := []struct{ input, want string }{
		{"Flask", "flask"},
		{"my_package", "my-package"},
		{"My.Package", "my-package"},
		{"My-Package", "my-package"},
		{"UPPER_CASE.Mixed", "upper-case-mixed"},
	}
	for _, tt := range tests {
		if got := normalizePythonName(tt.input); got != tt.want {
			t.Errorf("normalizePythonName(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestParseBunLock(t *testing.T) {
	data := []byte(`{
  "lockfileVersion": 1,
  // this is a JSONC comment
  "workspaces": {
    "": {
      "name": "myapp",
      "dependencies": {
        "express": "^4.21.0",
        "@babel/core": "^7.26.0",
      },
    },
  },
  "packages": {
    "express@4.21.2": ["express@4.21.2", "", { "dependencies": { "accepts": "~1.3.8" } }, "sha512-abc123"],
    "@babel/core@7.26.0": ["@babel/core@7.26.0", "", {}, "sha512-def456"],
    "accepts@1.3.8": ["accepts@1.3.8", "", {}, "sha512-ghi789"],
  },
}`)
	resolved := parseBunLock(data)
	if resolved == nil {
		t.Fatal("expected non-nil result")
	}
	tests := map[string]string{
		"express":     "4.21.2",
		"@babel/core": "7.26.0",
		"accepts":     "1.3.8",
	}
	for name, want := range tests {
		if got := resolved[name]; got != want {
			t.Errorf("parseBunLock[%s] = %q, want %q", name, got, want)
		}
	}
}

func TestParseBunLockWorkspace(t *testing.T) {
	// Workspace entries have only 1 element — they should be skipped (no version to extract).
	data := []byte(`{
  "lockfileVersion": 1,
  "packages": {
    "my-lib@workspace:packages/lib": ["my-lib@workspace:packages/lib"],
    "express@4.21.2": ["express@4.21.2", "", {}, "sha512-abc"],
  },
}`)
	resolved := parseBunLock(data)
	if resolved == nil {
		t.Fatal("expected non-nil result")
	}
	if _, exists := resolved["my-lib"]; exists {
		t.Error("workspace entry should not produce a resolved version")
	}
	if got := resolved["express"]; got != "4.21.2" {
		t.Errorf("express = %q, want 4.21.2", got)
	}
}

func TestParseBunPmLs(t *testing.T) {
	output := []byte(`/home/user/project node_modules (5)
├── express@4.21.2
├── @types/node@22.13.4
├── typescript@5.7.3
├── @babel/core@7.26.0
└── lodash@4.17.21
`)
	resolved := parseBunPmLs(output)
	tests := map[string]string{
		"express":      "4.21.2",
		"@types/node":  "22.13.4",
		"typescript":   "5.7.3",
		"@babel/core":  "7.26.0",
		"lodash":       "4.17.21",
	}
	for name, want := range tests {
		if got := resolved[name]; got != want {
			t.Errorf("parseBunPmLs[%s] = %q, want %q", name, got, want)
		}
	}
}

func TestSplitLastAt(t *testing.T) {
	tests := []struct {
		input, name, ver string
	}{
		{"express@4.21.2", "express", "4.21.2"},
		{"@babel/core@7.26.0", "@babel/core", "7.26.0"},
		{"@types/node@22.13.4", "@types/node", "22.13.4"},
		{"lodash", "lodash", ""},
		{"@scope/pkg", "@scope/pkg", ""},
	}
	for _, tt := range tests {
		name, ver := splitLastAt(tt.input)
		if name != tt.name || ver != tt.ver {
			t.Errorf("splitLastAt(%q) = (%q, %q), want (%q, %q)", tt.input, name, ver, tt.name, tt.ver)
		}
	}
}

func TestStripJSONC(t *testing.T) {
	input := []byte(`{
  // comment
  "key": "value", // inline comment
  "arr": [1, 2, 3,],
  "obj": {"a": 1,},
  "str": "has // not a comment",
}`)
	got := stripJSONC(input)
	// Verify it's valid JSON now.
	var v any
	if err := json.Unmarshal(got, &v); err != nil {
		t.Fatalf("stripJSONC produced invalid JSON: %v\noutput: %s", err, got)
	}
}

func TestParseBunLockEmpty(t *testing.T) {
	if got := parseBunLock([]byte("not json")); got != nil {
		t.Errorf("expected nil for invalid input, got %v", got)
	}
	if got := parseBunLock([]byte(`{"packages":{}}`)); len(got) != 0 {
		t.Errorf("expected empty map for empty packages, got %v", got)
	}
}

func TestParsePackageLockEmpty(t *testing.T) {
	// Invalid JSON should return nil.
	if got := parsePackageLock([]byte("not json")); got != nil {
		t.Errorf("expected nil for invalid JSON, got %v", got)
	}
	// Empty packages should return empty map.
	if got := parsePackageLock([]byte(`{"packages":{}}`)); len(got) != 0 {
		t.Errorf("expected empty map for empty packages, got %v", got)
	}
}
