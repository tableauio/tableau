package protogen

import (
	"testing"

	"github.com/tableauio/tableau/options"
)

func rules(rs ...options.PreserveFieldNumbersRule) []options.PreserveFieldNumbersRule {
	return rs
}

func TestGenerator_preserveFieldNumbers(t *testing.T) {
	tests := []struct {
		name     string
		global   bool
		rules    []options.PreserveFieldNumbersRule
		messager string
		want     bool
	}{
		{"global false, no rules", false, nil, "ItemConf", false},
		{"global true, no rules", true, nil, "ItemConf", true},
		{"global false, exact opt-in", false, rules(options.PreserveFieldNumbersRule{Pattern: "ItemConf", Preserve: true}), "ItemConf", true},
		{"global true, exact opt-out", true, rules(options.PreserveFieldNumbersRule{Pattern: "ItemConf", Preserve: false}), "ItemConf", false},
		{"regex prefix opt-out", true, rules(options.PreserveFieldNumbersRule{Pattern: "^Temp", Preserve: false}), "TempItemConf", false},
		{"regex prefix does not overmatch", true, rules(options.PreserveFieldNumbersRule{Pattern: "^Temp", Preserve: false}), "ItemConf", true},
		{"alternation", false, rules(options.PreserveFieldNumbersRule{Pattern: "Item|Hero", Preserve: true}), "HeroConf", true},
		{"alternation no match falls back to global false", false, rules(options.PreserveFieldNumbersRule{Pattern: "Item|Hero", Preserve: true}), "SkillConf", false},
		{"first match wins", true, rules(
			options.PreserveFieldNumbersRule{Pattern: ".*", Preserve: false},
			options.PreserveFieldNumbersRule{Pattern: "Item", Preserve: true},
		), "ItemConf", false},
		{"later rule applies when earlier does not match", false, rules(
			options.PreserveFieldNumbersRule{Pattern: "^None", Preserve: true},
			options.PreserveFieldNumbersRule{Pattern: "Conf$", Preserve: true},
		), "ItemConf", true},
		{"substring match (unanchored)", true, rules(options.PreserveFieldNumbersRule{Pattern: "Item", Preserve: false}), "MyItemConf", false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			gen := &Generator{
				OutputOpt: &options.ProtoOutputOption{
					PreserveFieldNumbers:         tc.global,
					MessagerPreserveFieldNumbers: tc.rules,
				},
			}
			if got := gen.preserveFieldNumbers(tc.messager); got != tc.want {
				t.Errorf("preserveFieldNumbers(%q) = %v, want %v", tc.messager, got, tc.want)
			}
		})
	}
}

func TestGenerator_anyPreserveFieldNumbers(t *testing.T) {
	tests := []struct {
		name   string
		global bool
		rules  []options.PreserveFieldNumbersRule
		want   bool
	}{
		{"global false, empty rules", false, nil, false},
		{"global true", true, nil, true},
		{"global false, rules all preserve=false", false, rules(
			options.PreserveFieldNumbersRule{Pattern: ".*", Preserve: false}), false},
		{"global false, rules has preserve=true", false, rules(
			options.PreserveFieldNumbersRule{Pattern: "A", Preserve: false},
			options.PreserveFieldNumbersRule{Pattern: "B", Preserve: true}), true},
		{"global true, rules all preserve=false (still parses)", true, rules(
			options.PreserveFieldNumbersRule{Pattern: "A", Preserve: false}), true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			gen := &Generator{
				OutputOpt: &options.ProtoOutputOption{
					PreserveFieldNumbers:         tc.global,
					MessagerPreserveFieldNumbers: tc.rules,
				},
			}
			if got := gen.anyPreserveFieldNumbers(); got != tc.want {
				t.Errorf("anyPreserveFieldNumbers() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestGenerator_compiledPreserveRules_invalidPatternPanics(t *testing.T) {
	gen := &Generator{
		OutputOpt: &options.ProtoOutputOption{
			PreserveFieldNumbers: true,
			MessagerPreserveFieldNumbers: rules(
				options.PreserveFieldNumbersRule{Pattern: "[invalid", Preserve: false}),
		},
	}
	defer func() {
		if r := recover(); r == nil {
			t.Errorf("expected panic for invalid regex pattern")
		}
	}()
	gen.compiledPreserveRules()
}
