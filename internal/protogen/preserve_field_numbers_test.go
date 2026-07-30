package protogen

import (
	"testing"

	"github.com/tableauio/tableau/options"
)

func TestGenerator_preserveFieldNumbers(t *testing.T) {
	tests := []struct {
		name        string
		global      bool
		perMessager map[string]bool
		messager    string
		want        bool
	}{
		{"global false, no overrides", false, nil, "ItemConf", false},
		{"global true, no overrides", true, nil, "ItemConf", true},
		{"global false, messager opted in", false, map[string]bool{"ItemConf": true}, "ItemConf", true},
		{"global true, messager opted out", true, map[string]bool{"ItemConf": false}, "ItemConf", false},
		{"override only affects listed messager", true, map[string]bool{"ItemConf": false}, "HeroConf", true},
		{"unlisted messager falls back to global false", false, map[string]bool{"ItemConf": true}, "HeroConf", false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			gen := &Generator{
				OutputOpt: &options.ProtoOutputOption{
					PreserveFieldNumbers:         tc.global,
					MessagerPreserveFieldNumbers: tc.perMessager,
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
		name        string
		global      bool
		perMessager map[string]bool
		want        bool
	}{
		{"global false, empty map", false, nil, false},
		{"global true", true, nil, true},
		{"global false, map all false", false, map[string]bool{"A": false, "B": false}, false},
		{"global false, map has true", false, map[string]bool{"A": false, "B": true}, true},
		{"global true, map all false (still parses)", true, map[string]bool{"A": false}, true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			gen := &Generator{
				OutputOpt: &options.ProtoOutputOption{
					PreserveFieldNumbers:         tc.global,
					MessagerPreserveFieldNumbers: tc.perMessager,
				},
			}
			if got := gen.anyPreserveFieldNumbers(); got != tc.want {
				t.Errorf("anyPreserveFieldNumbers() = %v, want %v", got, tc.want)
			}
		})
	}
}
