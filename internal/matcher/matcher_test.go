package matcher

import "testing"

func TestSemverCompare(t *testing.T) {
	tests := []struct {
		a, b string
		want int
	}{
		{"1.0.0", "1.0.0", 0},
		{"1.0.1", "1.0.0", 1},
		{"1.0.0", "1.0.1", -1},
		{"2.0.0", "1.9.9", 1},
		{"1.0.0-alpha", "1.0.0", -1},
		{"v1.2.3", "1.2.3", 0},
	}
	for _, tt := range tests {
		a := parseSemver(tt.a)
		b := parseSemver(tt.b)
		got := a.compare(b)
		if got != tt.want {
			t.Errorf("compare(%s, %s) = %d, want %d", tt.a, tt.b, got, tt.want)
		}
	}
}

func TestRuleMatch(t *testing.T) {
	tests := []struct {
		rule    string
		version string
		want    bool
	}{
		// Exact match.
		{"axios@=1.7.0", "1.7.0", true},
		{"axios@=1.7.0", "1.7.1", false},

		// Range.
		{"axios@>=1.3.0,<1.6.4", "1.5.0", true},
		{"axios@>=1.3.0,<1.6.4", "1.6.4", false},
		{"axios@>=1.3.0,<1.6.4", "1.3.0", true},
		{"axios@>=1.3.0,<1.6.4", "1.2.9", false},

		// Greater than.
		{"lodash@>4.17.0", "4.17.1", true},
		{"lodash@>4.17.0", "4.17.0", false},

		// Less than or equal.
		{"pkg@<=2.0.0", "2.0.0", true},
		{"pkg@<=2.0.0", "2.0.1", false},

		// Name-only (any version).
		{"event-stream", "3.3.6", true},
		{"event-stream", "0.0.1", true},

		// Compatible release (~=).
		{"pkg@~=1.4.2", "1.4.2", true},
		{"pkg@~=1.4.2", "1.4.9", true},
		{"pkg@~=1.4.2", "1.5.0", false},
		{"pkg@~=1.4.2", "1.4.1", false},
		{"pkg@~=1.4", "1.4.0", true},
		{"pkg@~=1.4", "1.99.0", true},
		{"pkg@~=1.4", "2.0.0", false},

		// Scoped packages.
		{"@scope/pkg@>=2.0.0", "2.0.0", true},
		{"@scope/pkg@>=2.0.0", "1.9.9", false},
	}

	for _, tt := range tests {
		rule, err := ParseRule(tt.rule)
		if err != nil {
			t.Fatalf("ParseRule(%q): %v", tt.rule, err)
		}
		got := rule.Match(tt.version)
		if got != tt.want {
			t.Errorf("Rule(%q).Match(%q) = %v, want %v", tt.rule, tt.version, got, tt.want)
		}
	}
}

func TestParseRulesArg(t *testing.T) {
	// Mixed: name-only, exact, range with inner comma.
	rules, err := ParseRulesArg("event-stream,axios@>=1.3.0,<1.6.4,colors@=1.4.1")
	if err != nil {
		t.Fatal(err)
	}
	if len(rules) != 3 {
		t.Fatalf("expected 3 rules, got %d: %+v", len(rules), rules)
	}
	if rules[0].Name != "event-stream" || len(rules[0].Constraints) != 0 {
		t.Errorf("rule 0: %+v", rules[0])
	}
	if rules[1].Name != "axios" || len(rules[1].Constraints) != 2 {
		t.Errorf("rule 1: %+v", rules[1])
	}
	if rules[2].Name != "colors" || len(rules[2].Constraints) != 1 {
		t.Errorf("rule 2: %+v", rules[2])
	}
}
