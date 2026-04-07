// Package matcher provides ecosystem-aware version range matching for security sweeps.
//
// Supported range syntax per ecosystem:
//
//	npm:    ^1.2.3, ~1.2.3, >=1.0.0 <2.0.0, 1.2.x, *
//	python: >=1.0,<2.0  ~=1.4  ==1.4.2
//	go:     >=v1.0.0,<v1.5.0  (comma-separated constraints)
//	rust:   ^1.2, ~1.2, >=1.0.0,<2.0.0
//	ruby:   ~> 1.2, >= 1.0, < 2.0
//	general: exact match, prefix match, wildcard *
package matcher

import (
	"strconv"
	"strings"
)

// semver is a parsed semantic version.
type semver struct {
	major, minor, patch int
	pre                 string // pre-release suffix
	valid               bool
}

func parseSemver(s string) semver {
	s = strings.TrimPrefix(s, "v")
	s = strings.TrimPrefix(s, "=")
	s = strings.TrimSpace(s)
	if s == "" {
		return semver{}
	}

	// Split off pre-release.
	pre := ""
	if i := strings.IndexAny(s, "-+"); i >= 0 {
		pre = s[i:]
		s = s[:i]
	}

	parts := strings.SplitN(s, ".", 4)
	v := semver{pre: pre, valid: true}

	if len(parts) >= 1 {
		n, err := strconv.Atoi(parts[0])
		if err != nil {
			return semver{}
		}
		v.major = n
	}
	if len(parts) >= 2 {
		// Handle "x" and "*" wildcards as 0.
		p := strings.TrimRight(parts[1], "x*X")
		if p == "" {
			p = "0"
		}
		n, err := strconv.Atoi(p)
		if err != nil {
			return semver{}
		}
		v.minor = n
	}
	if len(parts) >= 3 {
		p := strings.TrimRight(parts[2], "x*X")
		if p == "" {
			p = "0"
		}
		n, err := strconv.Atoi(p)
		if err != nil {
			return semver{}
		}
		v.patch = n
	}

	return v
}

// compare returns -1, 0, or 1.
func (a semver) compare(b semver) int {
	if a.major != b.major {
		return cmpInt(a.major, b.major)
	}
	if a.minor != b.minor {
		return cmpInt(a.minor, b.minor)
	}
	if a.patch != b.patch {
		return cmpInt(a.patch, b.patch)
	}
	// Pre-release sorts before release.
	if a.pre == "" && b.pre != "" {
		return 1
	}
	if a.pre != "" && b.pre == "" {
		return -1
	}
	return strings.Compare(a.pre, b.pre)
}

func (a semver) gte(b semver) bool { return a.compare(b) >= 0 }
func (a semver) gt(b semver) bool  { return a.compare(b) > 0 }
func (a semver) lte(b semver) bool { return a.compare(b) <= 0 }
func (a semver) lt(b semver) bool  { return a.compare(b) < 0 }
func (a semver) eq(b semver) bool  { return a.compare(b) == 0 }

func cmpInt(a, b int) int {
	if a < b {
		return -1
	}
	return 1
}
