package strcase

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

// runSnakeCases drives the same set of cases against both the STYLE2024
// and the legacy Strcase. Differences between the two styles are
// expressed by populating wantLegacy on the caseExpect.
func runSnakeCases(tb testing.TB, fn func(*Strcase, string) string, cases []caseExpect) {
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

func snakeCases() []caseExpect {
	return []caseExpect{
		// --- agreement set ---
		{in: "testCase", want: "test_case"},
		{in: "TestCase", want: "test_case"},
		{in: "Test Case", want: "test_case"},
		{in: " Test Case", want: "test_case"},
		{in: "Test Case ", want: "test_case"},
		{in: " Test Case ", want: "test_case"},
		{in: "test", want: "test"},
		{in: "test_case", want: "test_case"},
		{in: "Test", want: "test"},
		{in: "", want: ""},
		{in: "ManyManyWords", want: "many_many_words"},
		{in: "manyManyWords", want: "many_many_words"},
		{in: "AnyKind of_string", want: "any_kind_of_string"},
		{in: "JSONData", want: "json_data"},
		{in: "userID", want: "user_id"},
		{in: "AAAbbb", want: "aa_abbb"},
		{in: "DeviceTier", want: "device_tier"},
		{in: "some string", want: "some_string"},
		{in: " some string", want: "some_string"},

		// --- divergence set: letter <-> digit boundaries ---
		// STYLE2024 keeps the digit run glued to the adjacent letter run;
		// legacy splits it.
		{in: "numbers2and55with000", want: "numbers2and55with000", wantLegacy: "numbers_2_and_55_with_000"},
		{in: "1A2", want: "1_a2", wantLegacy: "1_a_2"},
		{in: "A1B", want: "a1_b", wantLegacy: "a_1_b"},
		{in: "A1A2A3", want: "a1_a2_a3", wantLegacy: "a_1_a_2_a_3"},
		{in: "A1 A2 A3", want: "a1_a2_a3", wantLegacy: "a_1_a_2_a_3"},
		{in: "AB1AB2AB3", want: "ab1_ab2_ab3", wantLegacy: "ab_1_ab_2_ab_3"},
		{in: "AB1 AB2 AB3", want: "ab1_ab2_ab3", wantLegacy: "ab_1_ab_2_ab_3"},
		{in: "Tier1", want: "tier1", wantLegacy: "tier_1"},

		// --- divergence set: explicit-separator + digit-led next token ---
		// STYLE2024 glues the digit run onto the previous segment to
		// avoid a digit-led segment; legacy keeps each separator as a
		// single delimiter and additionally splits at the letter<->digit
		// boundary.
		{in: "AB1 2CD", want: "ab12_cd", wantLegacy: "ab_1_2_cd"},
		{in: "foo_1bar", want: "foo1bar", wantLegacy: "foo_1_bar"},
		{in: "foo 1bar", want: "foo1bar", wantLegacy: "foo_1_bar"},
		{in: "foo-1bar", want: "foo1bar", wantLegacy: "foo_1_bar"},
		{in: "v1.2", want: "v12", wantLegacy: "v_1_2"},
		{in: "foo_123_bar", want: "foo123_bar", wantLegacy: "foo_123_bar"},
	}
}

func TestToSnake(t *testing.T) { runSnakeCases(t, (*Strcase).ToSnake, snakeCases()) }

func BenchmarkToSnake(b *testing.B) {
	cases := snakeCases()
	for n := 0; n < b.N; n++ {
		runSnakeCases(b, (*Strcase).ToSnake, cases)
	}
}

func screamingSnakeCases() []caseExpect {
	return []caseExpect{
		{in: "testCase", want: "TEST_CASE"},
		{in: "JSONData", want: "JSON_DATA"},
		{in: "userID", want: "USER_ID"},
		{in: "DeviceTier", want: "DEVICE_TIER"},

		// --- divergence set: letter <-> digit boundaries ---
		{in: "Tier1", want: "TIER1", wantLegacy: "TIER_1"},
		{in: "numbers2and55with000", want: "NUMBERS2AND55WITH000", wantLegacy: "NUMBERS_2_AND_55_WITH_000"},
		{in: "AB1AB2AB3", want: "AB1_AB2_AB3", wantLegacy: "AB_1_AB_2_AB_3"},
		{in: "V1.2", want: "V12", wantLegacy: "V_1_2"},

		// --- divergence set: explicit-separator + digit-led next token ---
		{in: "AB1 2CD", want: "AB12_CD", wantLegacy: "AB_1_2_CD"},
		{in: "FOO_1BAR", want: "FOO1_BAR", wantLegacy: "FOO_1_BAR"},
	}
}

func TestToScreamingSnake(t *testing.T) {
	runSnakeCases(t, (*Strcase).ToScreamingSnake, screamingSnakeCases())
}

func BenchmarkToScreamingSnake(b *testing.B) {
	cases := screamingSnakeCases()
	for n := 0; n < b.N; n++ {
		runSnakeCases(b, (*Strcase).ToScreamingSnake, cases)
	}
}

// customAcronymSnakeCase shares one fixture across ToSnake /
// ToScreamingSnake for both naming styles. wantSnakeLegacy /
// wantScreamingLegacy hold the legacy expectation when it differs from
// STYLE2024; an empty string means "same as the STYLE2024 want".
type customAcronymSnakeCase struct {
	name     string
	acronyms map[string]string
	args     []struct {
		value               string
		wantSnake           string
		wantScreaming       string
		wantSnakeLegacy     string
		wantScreamingLegacy string
	}
}

func customAcronymSnakeFixtures() []customAcronymSnakeCase {
	return []customAcronymSnakeCase{
		{
			name:     "APIV3 Custom Acronym",
			acronyms: map[string]string{"APIV3": "apiv3"},
			args: []struct {
				value               string
				wantSnake           string
				wantScreaming       string
				wantSnakeLegacy     string
				wantScreamingLegacy string
			}{
				{value: "WebAPIV3Spec", wantSnake: "web_apiv3_spec", wantScreaming: "WEB_APIV3_SPEC"},
			},
		},
		{
			name:     "K8s Custom Acroynm",
			acronyms: map[string]string{"K8s": "k8s"},
			args: []struct {
				value               string
				wantSnake           string
				wantScreaming       string
				wantSnakeLegacy     string
				wantScreamingLegacy string
			}{
				{value: "InK8s", wantSnake: "in_k8s", wantScreaming: "IN_K8S"},
			},
		},
		{
			name:     "HandleA1000Req Custom Acronym",
			acronyms: map[string]string{`A(1\d{3})`: "a${1}"},
			args: []struct {
				value               string
				wantSnake           string
				wantScreaming       string
				wantSnakeLegacy     string
				wantScreamingLegacy string
			}{
				{value: "HandleA1000Req", wantSnake: "handle_a1000_req", wantScreaming: "HANDLE_A1000_REQ"},
				{value: "HandleA1001AndA1002Reply", wantSnake: "handle_a1001_and_a1002_reply", wantScreaming: "HANDLE_A1001_AND_A1002_REPLY"},
				// "A2000" doesn't match the acronym pattern, so legacy
				// splits at the letter <-> digit boundary while
				// STYLE2024 keeps the digit attached.
				{
					value: "HandleA2000Msg", wantSnake: "handle_a2000_msg", wantScreaming: "HANDLE_A2000_MSG",
					wantSnakeLegacy: "handle_a_2000_msg", wantScreamingLegacy: "HANDLE_A_2000_MSG",
				},
			},
		},
		{
			name:     "Mode1V1 Custom Acronym",
			acronyms: map[string]string{`(\d)[vV](\d)`: "${1}v${2}"},
			args: []struct {
				value               string
				wantSnake           string
				wantScreaming       string
				wantSnakeLegacy     string
				wantScreamingLegacy string
			}{
				// Acronym match emits "1v1"; STYLE2024 keeps "Mode"
				// glued to it, legacy splits before the digit.
				{value: "Mode1V1", wantSnake: "mode1v1", wantScreaming: "MODE1V1", wantSnakeLegacy: "mode_1v1", wantScreamingLegacy: "MODE_1V1"},
				{value: "Mode1v3", wantSnake: "mode1v3", wantScreaming: "MODE1V3", wantSnakeLegacy: "mode_1v3", wantScreamingLegacy: "MODE_1V3"},
				// Two overlapping matches consume "2v2" then leave
				// "v2"; legacy further splits the trailing "v_2".
				{value: "Mode2v2v2", wantSnake: "mode2v2_v2", wantScreaming: "MODE2V2_V2", wantSnakeLegacy: "mode_2v2_v_2", wantScreamingLegacy: "MODE_2V2_V_2"},
			},
		},
		{
			name:     "Prefix Custom Acronym",
			acronyms: map[string]string{`^Tom`: "tommy"},
			args: []struct {
				value               string
				wantSnake           string
				wantScreaming       string
				wantSnakeLegacy     string
				wantScreamingLegacy string
			}{
				{value: "TomJerry", wantSnake: "tommy_jerry", wantScreaming: "TOMMY_JERRY"},
				{value: "JerryTom", wantSnake: "jerry_tom", wantScreaming: "JERRY_TOM"},
			},
		},
		{
			name:     "Suffix Custom Acronym",
			acronyms: map[string]string{`Cat$`: "kitty"},
			args: []struct {
				value               string
				wantSnake           string
				wantScreaming       string
				wantSnakeLegacy     string
				wantScreamingLegacy string
			}{
				{value: "CatMouse", wantSnake: "cat_mouse", wantScreaming: "CAT_MOUSE"},
				{value: "MouseCat", wantSnake: "mouse_kitty", wantScreaming: "MOUSE_KITTY"},
			},
		},
	}
}

// snakeLegacyExpect / screamingLegacyExpect default to want when
// wantSnakeLegacy / wantScreamingLegacy is the empty string.
func snakeLegacyExpect(arg struct {
	value               string
	wantSnake           string
	wantScreaming       string
	wantSnakeLegacy     string
	wantScreamingLegacy string
},
) string {
	if arg.wantSnakeLegacy == "" {
		return arg.wantSnake
	}
	return arg.wantSnakeLegacy
}

func screamingLegacyExpect(arg struct {
	value               string
	wantSnake           string
	wantScreaming       string
	wantSnakeLegacy     string
	wantScreamingLegacy string
},
) string {
	if arg.wantScreamingLegacy == "" {
		return arg.wantScreaming
	}
	return arg.wantScreamingLegacy
}

func TestCustomAcronymsToSnake(t *testing.T) {
	for _, test := range customAcronymSnakeFixtures() {
		t.Run(test.name, func(t *testing.T) {
			std := FromContext(NewContext(context.Background(), New(test.acronyms)))
			legacy := FromContext(NewContext(context.Background(), NewLegacy(test.acronyms)))
			for _, arg := range test.args {
				if got := std.ToSnake(arg.value); got != arg.wantSnake {
					t.Errorf("STYLE2024 ToSnake(%q) = %q, want %q", arg.value, got, arg.wantSnake)
				}
				wantLegacy := snakeLegacyExpect(arg)
				if got := legacy.ToSnake(arg.value); got != wantLegacy {
					t.Errorf("legacy   ToSnake(%q) = %q, want %q", arg.value, got, wantLegacy)
				}
			}
		})
	}
}

func TestCustomAcronymsToScreamingSnake(t *testing.T) {
	for _, test := range customAcronymSnakeFixtures() {
		t.Run(test.name, func(t *testing.T) {
			std := FromContext(NewContext(context.Background(), New(test.acronyms)))
			legacy := FromContext(NewContext(context.Background(), NewLegacy(test.acronyms)))
			for _, arg := range test.args {
				if got := std.ToScreamingSnake(arg.value); got != arg.wantScreaming {
					t.Errorf("STYLE2024 ToScreamingSnake(%q) = %q, want %q", arg.value, got, arg.wantScreaming)
				}
				wantLegacy := screamingLegacyExpect(arg)
				if got := legacy.ToScreamingSnake(arg.value); got != wantLegacy {
					t.Errorf("legacy   ToScreamingSnake(%q) = %q, want %q", arg.value, got, wantLegacy)
				}
			}
		})
	}
}

func TestPanicOnMultipleAcronymMatches(t *testing.T) {
	tests := []struct {
		name     string
		acronyms map[string]string
		arg      string
	}{
		{
			name:     "APIV3 Custom Acronym",
			acronyms: map[string]string{"API": "api", "APIV3": "apiv3"},
			arg:      "WebAPIV3Spec",
		},
		{
			name:     "HandleA1000Req Custom Acronym",
			acronyms: map[string]string{`A(1\d{3})`: "a${1}", `A(\d{4})`: "a${1}"},
			arg:      "HandleA1000Req",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			std := FromContext(NewContext(context.Background(), New(test.acronyms)))
			legacy := FromContext(NewContext(context.Background(), NewLegacy(test.acronyms)))
			assert.Panics(t, func() { std.ToScreamingSnake(test.arg) })
			assert.Panics(t, func() { legacy.ToScreamingSnake(test.arg) })
		})
	}
}
