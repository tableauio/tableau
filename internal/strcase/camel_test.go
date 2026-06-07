package strcase

import (
	"context"
	"testing"
)

// caseExpect describes the expected output of a conversion under both
// naming styles. If wantLegacy is the empty string, the legacy result is
// expected to match want. To express "the legacy result is intentionally
// the empty string", set wantLegacy to "" AND use the dedicated test
// helper that already shares want=="" semantics — in our test corpus the
// only legitimately empty-output input is also "" itself, so this
// shortcut is unambiguous in practice.
type caseExpect struct {
	in         string
	want       string // STYLE2024 result
	wantLegacy string // legacy result; empty means same as want
}

// expectedLegacy returns the legacy expectation, falling back to want
// when wantLegacy was left as the zero value to mark "no difference".
func (c caseExpect) expectedLegacy() string {
	if c.wantLegacy == "" {
		return c.want
	}
	return c.wantLegacy
}

// runCamelCases drives the same set of cases against both the STYLE2024
// and the legacy Strcase. Differences between the two styles are
// expressed by populating wantLegacy.
func runCamelCases(tb testing.TB, fn func(*Strcase, string) string, cases []caseExpect) {
	tb.Helper()
	std := New(nil)
	legacy := NewLegacy(nil)
	for _, c := range cases {
		if got := fn(std, c.in); got != c.want {
			tb.Errorf("STYLE2024 %q -> %q, want %q", c.in, got, c.want)
		}
		if got := fn(legacy, c.in); got != c.expectedLegacy() {
			tb.Errorf("legacy   %q -> %q, want %q", c.in, got, c.expectedLegacy())
		}
	}
}

func camelCases() []caseExpect {
	return []caseExpect{
		// Cases where STYLE2024 and legacy agree.
		{in: "test_case", want: "TestCase"},
		{in: "test.case", want: "TestCase"},
		{in: "test", want: "Test"},
		{in: "TestCase", want: "TestCase"},
		{in: " test  case ", want: "TestCase"},
		{in: "", want: ""},
		{in: "many_many_words", want: "ManyManyWords"},
		{in: "AnyKind of_string", want: "AnyKindOfString"},
		{in: "odd-fix", want: "OddFix"},
		{in: "ID", want: "Id"},
		{in: "CONSTANT_CASE", want: "ConstantCase"},

		// Cases where STYLE2024 and legacy intentionally diverge.
		// STYLE2024 leaves a single chunk's tail untouched after the first
		// upper-cased character; legacy uppercases EVERY post-digit letter.
		{in: "numbers2And55with000", want: "Numbers2And55with000", wantLegacy: "Numbers2And55With000"},
		// Each underscore-separated chunk is classified independently
		// under STYLE2024.
		{in: "PVP", want: "Pvp"},
		{in: "PVE_DATA", want: "PveData"},
		// Legacy lowers any uppercase letter that follows another uppercase
		// letter, so "NT", "MF" and the trailing "X" all collapse.
		{in: "HeroNTagMFcX_SCORE", want: "HeroNTagMFcXScore", wantLegacy: "HeroNtagMfcXScore"},
	}
}

func TestToCamel(t *testing.T) {
	runCamelCases(t, (*Strcase).ToCamel, camelCases())
}

func BenchmarkToCamel(b *testing.B) {
	cases := camelCases()
	for n := 0; n < b.N; n++ {
		runCamelCases(b, (*Strcase).ToCamel, cases)
	}
}

func lowerCamelCases() []caseExpect {
	return []caseExpect{
		{in: "", want: ""},
		{in: "test_case", want: "testCase"},
		{in: "test", want: "test"},
		{in: "TestCase", want: "testCase"},
		{in: " test  case ", want: "testCase"},
		{in: "many_many_words", want: "manyManyWords"},
		{in: "AnyKind of_string", want: "anyKindOfString"},
		{in: "odd-fix", want: "oddFix"},
		{in: "ID", want: "id"},
		{in: "CONSTANT_CASE", want: "constantCase"},
		{in: "PVP", want: "pvp"},
		{in: "PVE_DATA", want: "pveData"},
		{in: "HeroNTagMFcX_SCORE", want: "heroNTagMFcXScore", wantLegacy: "heroNtagMfcXScore"},
		{in: "foo-bar", want: "fooBar"},
		{in: "AnyKind.of-string", want: "anyKindOfString"},
		{in: "some string", want: "someString"},
		{in: " some string", want: "someString"},
	}
}

func TestToLowerCamel(t *testing.T) {
	runCamelCases(t, (*Strcase).ToLowerCamel, lowerCamelCases())
}

// customAcronymCamelCase is a single fixture for the custom-acronym
// camel/lowerCamel tests below. The legacy and STYLE2024 algorithms
// already produce identical output for these acronym scenarios.
type customAcronymCamelCase struct {
	name     string
	acronyms map[string]string
	args     []struct {
		value     string
		wantCamel string
		wantLower string
	}
}

func customAcronymCamelFixtures() []customAcronymCamelCase {
	return []customAcronymCamelCase{
		{
			name:     "APIV3 Custom Acronym",
			acronyms: map[string]string{"APIV3": "apiv3"},
			args: []struct {
				value     string
				wantCamel string
				wantLower string
			}{
				{"WebAPIV3Spec", "WebApiv3Spec", "webApiv3Spec"},
			},
		},
		{
			name:     "K8s Custom Acroynm",
			acronyms: map[string]string{"K8s": "k8s"},
			args: []struct {
				value     string
				wantCamel string
				wantLower string
			}{
				{"InK8s", "InK8s", "inK8s"},
			},
		},
		{
			name:     "HandleA1000Req Custom Acronym",
			acronyms: map[string]string{`A(1\d{3})`: "a${1}"},
			args: []struct {
				value     string
				wantCamel string
				wantLower string
			}{
				{"HandleA1000Req", "HandleA1000Req", "handleA1000Req"},
				{"HandleA1001AndA1002Reply", "HandleA1001AndA1002Reply", "handleA1001AndA1002Reply"},
				{"HandleA2000Msg", "HandleA2000Msg", "handleA2000Msg"},
			},
		},
		{
			name:     "Mode1V1 Custom Acronym",
			acronyms: map[string]string{`(\d)[vV](\d)`: "${1}v${2}"},
			args: []struct {
				value     string
				wantCamel string
				wantLower string
			}{
				{"Mode1V1", "Mode1v1", "mode1v1"},
				{"Mode1v3", "Mode1v3", "mode1v3"},
				{"Mode2v2v2", "Mode2v2V2", "mode2v2V2"},
			},
		},
		{
			name:     "Prefix Custom Acronym",
			acronyms: map[string]string{`^Tom`: "tommy"},
			args: []struct {
				value     string
				wantCamel string
				wantLower string
			}{
				{"TomJerry", "TommyJerry", "tommyJerry"},
				{"JerryTom", "JerryTom", "jerryTom"},
			},
		},
		{
			name:     "Suffix Custom Acronym",
			acronyms: map[string]string{`Cat$`: "kitty"},
			args: []struct {
				value     string
				wantCamel string
				wantLower string
			}{
				{"CatMouse", "CatMouse", "catMouse"},
				{"MouseCat", "MouseKitty", "mouseKitty"},
			},
		},
	}
}

func TestCustomAcronymToCamel(t *testing.T) {
	for _, test := range customAcronymCamelFixtures() {
		t.Run(test.name, func(t *testing.T) {
			std := FromContext(NewContext(context.Background(), New(test.acronyms)))
			legacy := FromContext(NewContext(context.Background(), NewLegacy(test.acronyms)))
			for _, arg := range test.args {
				if got := std.ToCamel(arg.value); got != arg.wantCamel {
					t.Errorf("STYLE2024 ToCamel(%q) = %q, want %q", arg.value, got, arg.wantCamel)
				}
				if got := legacy.ToCamel(arg.value); got != arg.wantCamel {
					t.Errorf("legacy   ToCamel(%q) = %q, want %q", arg.value, got, arg.wantCamel)
				}
			}
		})
	}
}

func TestCustomAcronymToLowerCamel(t *testing.T) {
	for _, test := range customAcronymCamelFixtures() {
		t.Run(test.name, func(t *testing.T) {
			std := FromContext(NewContext(context.Background(), New(test.acronyms)))
			legacy := FromContext(NewContext(context.Background(), NewLegacy(test.acronyms)))
			for _, arg := range test.args {
				if got := std.ToLowerCamel(arg.value); got != arg.wantLower {
					t.Errorf("STYLE2024 ToLowerCamel(%q) = %q, want %q", arg.value, got, arg.wantLower)
				}
				if got := legacy.ToLowerCamel(arg.value); got != arg.wantLower {
					t.Errorf("legacy   ToLowerCamel(%q) = %q, want %q", arg.value, got, arg.wantLower)
				}
			}
		})
	}
}
