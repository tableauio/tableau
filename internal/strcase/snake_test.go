package strcase

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

func toSnake(tb testing.TB) {
	var ctx Strcase
	cases := [][]string{
		{"testCase", "test_case"},
		{"TestCase", "test_case"},
		{"Test Case", "test_case"},
		{" Test Case", "test_case"},
		{"Test Case ", "test_case"},
		{" Test Case ", "test_case"},
		{"test", "test"},
		{"test_case", "test_case"},
		{"Test", "test"},
		{"", ""},
		{"ManyManyWords", "many_many_words"},
		{"manyManyWords", "many_many_words"},
		{"AnyKind of_string", "any_kind_of_string"},
		// STYLE2024: no underscore at letter <-> digit boundary.
		{"numbers2and55with000", "numbers2and55with000"},
		{"JSONData", "json_data"},
		{"userID", "user_id"},
		{"AAAbbb", "aa_abbb"},
		{"1A2", "1_a2"},
		{"A1B", "a1_b"},
		{"A1A2A3", "a1_a2_a3"},
		{"A1 A2 A3", "a1_a2_a3"},
		{"AB1AB2AB3", "ab1_ab2_ab3"},
		{"AB1 AB2 AB3", "ab1_ab2_ab3"},
		{"Tier1", "tier1"},
		{"DeviceTier", "device_tier"},
		{"some string", "some_string"},
		{" some string", "some_string"},
		// Explicit-separator + digit-led next token: must NOT produce a
		// segment that starts with a digit. The digit run is glued onto
		// the previous segment; an inner digit -> upper-letter boundary
		// inside that run is still split (yielding letter-initial
		// segments only).
		{"AB1 2CD", "ab12_cd"},
		{"foo_1bar", "foo1bar"},
		{"foo 1bar", "foo1bar"},
		{"foo-1bar", "foo1bar"},
		{"v1.2", "v12"},
		{"foo_123_bar", "foo123_bar"},
	}
	for _, i := range cases {
		in := i[0]
		out := i[1]
		result := ctx.ToSnake(in)
		if result != out {
			tb.Errorf("%q (%q != %q)", in, result, out)
		}
	}
}

func TestToSnake(t *testing.T) { toSnake(t) }

func BenchmarkToSnake(b *testing.B) {
	benchmarkSnakeTest(b, toSnake)
}

func TestCustomAcronymsToSnake(t *testing.T) {
	tests := []struct {
		name     string
		acronyms map[string]string
		args     []struct {
			value    string
			expected string
		}
	}{
		{
			name: "APIV3 Custom Acronym",
			acronyms: map[string]string{
				"APIV3": "apiv3",
			},
			args: []struct {
				value    string
				expected string
			}{
				{"WebAPIV3Spec", "web_apiv3_spec"},
			},
		},
		{
			name: "K8s Custom Acroynm",
			acronyms: map[string]string{
				"K8s": "k8s",
			},
			args: []struct {
				value    string
				expected string
			}{
				{"InK8s", "in_k8s"},
			},
		},
		{
			name: "K8s Custom Acroynm with spaces",
			acronyms: map[string]string{
				"K8s": "k8s",
			},
			args: []struct {
				value    string
				expected string
			}{
				{" InK8s  XX", "in_k8s__xx"},
			},
		},
		{
			name: "HandleA1000Req Custom Acronym",
			acronyms: map[string]string{
				`A(1\d{3})`: "a${1}",
			},
			args: []struct {
				value    string
				expected string
			}{
				{"HandleA1000Req", "handle_a1000_req"},
				{"HandleA1001AndA1002Reply", "handle_a1001_and_a1002_reply"},
				{"HandleA2000Msg", "handle_a2000_msg"},
			},
		},
		{
			name: "Mode1V1 Custom Acronym",
			acronyms: map[string]string{
				`(\d)[vV](\d)`: "${1}v${2}",
			},
			args: []struct {
				value    string
				expected string
			}{
				{"Mode1V1", "mode1v1"},
				{"Mode1v3", "mode1v3"},
				{"Mode2v2v2", "mode2v2_v2"},
			},
		},
		{
			name: "Prefix Custom Acronym",
			acronyms: map[string]string{
				`^Tom`: "tommy",
			},
			args: []struct {
				value    string
				expected string
			}{
				{"TomJerry", "tommy_jerry"},
				{"JerryTom", "jerry_tom"},
			},
		},
		{
			name: "Suffix Custom Acronym",
			acronyms: map[string]string{
				`Cat$`: "kitty",
			},
			args: []struct {
				value    string
				expected string
			}{
				{"CatMouse", "cat_mouse"},
				{"MouseCat", "mouse_kitty"},
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ctx := FromContext(NewContext(context.Background(), New(test.acronyms)))
			for _, arg := range test.args {
				if result := ctx.ToSnake(arg.value); result != arg.expected {
					t.Errorf("expected custom acronym result %s, got %s", arg.expected, result)
				}
			}
		})
	}
}

func TestCustomAcronymsToScreamingSnake(t *testing.T) {
	tests := []struct {
		name     string
		acronyms map[string]string
		args     []struct {
			value    string
			expected string
		}
	}{
		{
			name: "APIV3 Custom Acronym",
			acronyms: map[string]string{
				"APIV3": "apiv3",
			},
			args: []struct {
				value    string
				expected string
			}{
				{"WebAPIV3Spec", "WEB_APIV3_SPEC"},
			},
		},
		{
			name: "K8s Custom Acroynm",
			acronyms: map[string]string{
				"K8s": "k8s",
			},
			args: []struct {
				value    string
				expected string
			}{
				{"InK8s", "IN_K8S"},
			},
		},
		{
			name: "HandleA1000Req Custom Acronym",
			acronyms: map[string]string{
				`A(1\d{3})`: "a${1}",
			},
			args: []struct {
				value    string
				expected string
			}{
				{"HandleA1000Req", "HANDLE_A1000_REQ"},
				{"HandleA1001AndA1002Reply", "HANDLE_A1001_AND_A1002_REPLY"},
				{"HandleA2000Msg", "HANDLE_A2000_MSG"},
			},
		},
		{
			name: "Mode1V1 Custom Acronym",
			acronyms: map[string]string{
				`(\d)[vV](\d)`: "${1}v${2}",
			},
			args: []struct {
				value    string
				expected string
			}{
				{"Mode1V1", "MODE1V1"},
				{"Mode1v3", "MODE1V3"},
				{"Mode2v2v2", "MODE2V2_V2"},
			},
		},
		{
			name: "Prefix Custom Acronym",
			acronyms: map[string]string{
				`^Tom`: "tommy",
			},
			args: []struct {
				value    string
				expected string
			}{
				{"TomJerry", "TOMMY_JERRY"},
				{"JerryTom", "JERRY_TOM"},
			},
		},
		{
			name: "Suffix Custom Acronym",
			acronyms: map[string]string{
				`Cat$`: "kitty",
			},
			args: []struct {
				value    string
				expected string
			}{
				{"CatMouse", "CAT_MOUSE"},
				{"MouseCat", "MOUSE_KITTY"},
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ctx := FromContext(NewContext(context.Background(), New(test.acronyms)))
			for _, arg := range test.args {
				if result := ctx.ToScreamingSnake(arg.value); result != arg.expected {
					t.Errorf("expected custom acronym result %s, got %s", arg.expected, result)
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
			name: "APIV3 Custom Acronym",
			acronyms: map[string]string{
				"API":   "api",
				"APIV3": "apiv3",
			},
			arg: "WebAPIV3Spec",
		},
		{
			name: "HandleA1000Req Custom Acronym",
			acronyms: map[string]string{
				`A(1\d{3})`: "a${1}",
				`A(\d{4})`:  "a${1}",
			},
			arg: "HandleA1000Req",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ctx := FromContext(NewContext(context.Background(), New(test.acronyms)))
			assert.Panics(t, func() { ctx.ToScreamingSnake(test.arg) })
		})
	}
}

func toScreamingSnake(tb testing.TB) {
	var ctx Strcase
	cases := [][]string{
		{"testCase", "TEST_CASE"},
		{"Tier1", "TIER1"},
		{"DeviceTier", "DEVICE_TIER"},
		{"numbers2and55with000", "NUMBERS2AND55WITH000"},
		{"AB1AB2AB3", "AB1_AB2_AB3"},
		{"JSONData", "JSON_DATA"},
		{"userID", "USER_ID"},
		// Explicit-separator + digit-led next token must not yield a
		// segment that starts with a digit.
		{"AB1 2CD", "AB12_CD"},
		{"FOO_1BAR", "FOO1_BAR"},
		{"V1.2", "V12"},
	}
	for _, i := range cases {
		in := i[0]
		out := i[1]
		result := ctx.ToScreamingSnake(in)
		if result != out {
			tb.Errorf("%q (%q != %q)", in, result, out)
		}
	}
}

func TestToScreamingSnake(t *testing.T) { toScreamingSnake(t) }

func BenchmarkToScreamingSnake(b *testing.B) {
	benchmarkSnakeTest(b, toScreamingSnake)
}

func benchmarkSnakeTest(b *testing.B, fn func(testing.TB)) {
	for n := 0; n < b.N; n++ {
		fn(b)
	}
}
