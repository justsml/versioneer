package matcher

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

// Rule represents one advisory/vulnerability pattern to match against.
// Format: "package-name@constraint" or just "package-name" (any version).
//
// Examples:
//
//	axios@<1.7.0
//	axios@>=1.3.0,<1.6.4
//	event-stream@=3.3.6
//	colors@>=1.4.1,<1.4.3
//	ua-parser-js@>=0.7.29,<0.7.31
//	lodash                          (any version matches)
//	@scope/pkg@>=2.0.0
type Rule struct {
	Name        string       // exact package name
	Constraints []constraint // if empty, any version matches
	Raw         string       // original line for reporting
}

type constraint struct {
	op      string // ">=", ">", "<=", "<", "=", "=="
	version semver
}

// Match checks if a given version string matches this rule.
func (r Rule) Match(version string) bool {
	if len(r.Constraints) == 0 {
		return true // name-only rule: any version matches
	}
	v := parseSemver(version)
	if !v.valid {
		return false
	}
	for _, c := range r.Constraints {
		if !c.match(v) {
			return false
		}
	}
	return true
}

func (c constraint) match(v semver) bool {
	switch c.op {
	case ">=":
		return v.gte(c.version)
	case ">":
		return v.gt(c.version)
	case "<=":
		return v.lte(c.version)
	case "<":
		return v.lt(c.version)
	case "=", "==":
		return v.eq(c.version)
	}
	return false
}

// ParseRule parses a single "name@constraints" string into a Rule.
func ParseRule(s string) (Rule, error) {
	s = strings.TrimSpace(s)
	if s == "" || s[0] == '#' {
		return Rule{}, fmt.Errorf("empty or comment line")
	}

	// Handle scoped packages: @scope/pkg@constraint
	name, constraintStr := splitNameConstraint(s)
	if name == "" {
		return Rule{}, fmt.Errorf("empty package name in %q", s)
	}

	rule := Rule{Name: name, Raw: s}

	if constraintStr == "" {
		return rule, nil
	}

	// Parse constraint(s) — comma-separated.
	parts := strings.Split(constraintStr, ",")
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		// ~= (Python compatible release) expands to two constraints:
		//   ~=1.4   → >=1.4.0, <2.0.0
		//   ~=1.4.2 → >=1.4.2, <1.5.0
		if strings.HasPrefix(part, "~=") {
			cs, err := expandCompatRelease(strings.TrimSpace(part[2:]))
			if err != nil {
				return Rule{}, fmt.Errorf("bad constraint %q in %q: %w", part, s, err)
			}
			rule.Constraints = append(rule.Constraints, cs...)
			continue
		}
		c, err := parseConstraint(part)
		if err != nil {
			return Rule{}, fmt.Errorf("bad constraint %q in %q: %w", part, s, err)
		}
		rule.Constraints = append(rule.Constraints, c)
	}

	return rule, nil
}

func splitNameConstraint(s string) (name, constraint string) {
	// Scoped: @scope/pkg@>=1.0
	if strings.HasPrefix(s, "@") {
		// Find the second @.
		rest := s[1:]
		if i := strings.Index(rest, "@"); i >= 0 {
			return s[:i+1], rest[i+1:]
		}
		return s, ""
	}
	// Unscoped: pkg@>=1.0
	if i := strings.Index(s, "@"); i >= 0 {
		return s[:i], s[i+1:]
	}
	return s, ""
}

func parseConstraint(s string) (constraint, error) {
	s = strings.TrimSpace(s)

	for _, op := range []string{">=", "<=", "==", ">", "<", "=", "~="} {
		if strings.HasPrefix(s, op) {
			ver := strings.TrimSpace(s[len(op):])
			sv := parseSemver(ver)
			if !sv.valid {
				return constraint{}, fmt.Errorf("invalid version %q", ver)
			}
			if op != "~=" {
				return constraint{op: op, version: sv}, nil
			}
			// ~= handled at parseConstraints level — should not reach here.
			return constraint{op: ">=", version: sv}, nil
		}
	}

	// No operator — treat as exact match.
	sv := parseSemver(s)
	if !sv.valid {
		return constraint{}, fmt.Errorf("invalid version %q", s)
	}
	return constraint{op: "=", version: sv}, nil
}

// expandCompatRelease implements Python's ~= operator (PEP 440).
// ~=X.Y   → >=X.Y.0, <(X+1).0.0
// ~=X.Y.Z → >=X.Y.Z, <X.(Y+1).0
func expandCompatRelease(ver string) ([]constraint, error) {
	sv := parseSemver(ver)
	if !sv.valid {
		return nil, fmt.Errorf("invalid version %q", ver)
	}

	lower := constraint{op: ">=", version: sv}

	// Determine precision from the original string to pick the upper bound.
	parts := strings.SplitN(strings.TrimPrefix(strings.TrimPrefix(ver, "v"), "="), ".", 4)
	var upper semver
	if len(parts) >= 3 {
		// ~=1.4.2 → <1.5.0
		upper = semver{major: sv.major, minor: sv.minor + 1, patch: 0, valid: true}
	} else {
		// ~=1.4 → <2.0.0
		upper = semver{major: sv.major + 1, minor: 0, patch: 0, valid: true}
	}

	return []constraint{lower, {op: "<", version: upper}}, nil
}

// LoadRulesFile reads a rules file (one rule per line, # comments, blank lines ok).
func LoadRulesFile(path string) ([]Rule, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var rules []Rule
	scanner := bufio.NewScanner(f)
	lineNum := 0
	for scanner.Scan() {
		lineNum++
		line := strings.TrimSpace(scanner.Text())
		if line == "" || line[0] == '#' {
			continue
		}
		rule, err := ParseRule(line)
		if err != nil {
			return nil, fmt.Errorf("line %d: %w", lineNum, err)
		}
		rules = append(rules, rule)
	}
	return rules, scanner.Err()
}

// ParseRulesArg parses a comma-separated list of rules from a CLI argument.
// e.g. "axios@<1.7.0,colors@>=1.4.1,event-stream"
func ParseRulesArg(s string) ([]Rule, error) {
	// Be careful with commas inside version constraints.
	// Split on comma only if what follows looks like a package name (letter or @).
	var parts []string
	current := ""
	for i := 0; i < len(s); i++ {
		if s[i] == ',' {
			// Peek ahead: if next char is a digit or operator, it's part of a constraint.
			if i+1 < len(s) && isConstraintStart(s[i+1]) {
				current += ","
				continue
			}
			parts = append(parts, current)
			current = ""
			continue
		}
		current += string(s[i])
	}
	if current != "" {
		parts = append(parts, current)
	}

	var rules []Rule
	for _, part := range parts {
		rule, err := ParseRule(part)
		if err != nil {
			return nil, err
		}
		rules = append(rules, rule)
	}
	return rules, nil
}

func isConstraintStart(c byte) bool {
	return c == '<' || c == '>' || c == '=' || c == '~' || (c >= '0' && c <= '9')
}
