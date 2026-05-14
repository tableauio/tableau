package strcase

import (
	"context"
	"testing"
)

func toPascal(tb testing.TB) {
	var ctx Strcase
	cases := [][]string{
		{"test_case", "TestCase"},
		{"test.case", "TestCase"},
		{"test", "Test"},
		{"TestCase", "TestCase"},
		{" test  case ", "TestCase"},
		{"", ""},
		{"many_many_words", "ManyManyWords"},
		{"AnyKind of_string", "AnyKindOfString"},
		{"odd-fix", "OddFix"},
		// Single chunk whose first character is lower-case: only the
		// first character is upper-cased; the rest of the chunk
		// (including the inner "with") is preserved as-is per the
		// chunk-based ToPascal rules.
		{"numbers2And55with000", "Numbers2And55with000"},
		{"ID", "Id"},
		{"CONSTANT_CASE", "ConstantCase"},
		// Each underscore-separated chunk is classified independently.
		{"PVP", "Pvp"},
		{"PVE_DATA", "PveData"},
		{"HeroNTagMFcX_SCORE", "HeroNTagMFcXScore"},
	}
	for _, i := range cases {
		in := i[0]
		out := i[1]
		result := ctx.ToPascal(in)
		if result != out {
			tb.Errorf("%q (%q != %q)", in, result, out)
		}
	}
}

func TestToPascal(t *testing.T) {
	toPascal(t)
}

func BenchmarkToPascal(b *testing.B) {
	benchmarkPascalTest(b, toPascal)
}

func TestToCamel(t *testing.T) {
	var ctx Strcase
	cases := [][]string{
		{"", ""},
		{"test_case", "testCase"},
		{"test", "test"},
		{"TestCase", "testCase"},
		{" test  case ", "testCase"},
		{"many_many_words", "manyManyWords"},
		{"AnyKind of_string", "anyKindOfString"},
		{"odd-fix", "oddFix"},
		{"ID", "id"},
		{"CONSTANT_CASE", "constantCase"},
		{"PVP", "pvp"},
		{"PVE_DATA", "pveData"},
		{"HeroNTagMFcX_SCORE", "heroNTagMFcXScore"},
	}
	for _, c := range cases {
		in, out := c[0], c[1]
		if result := ctx.ToCamel(in); result != out {
			t.Errorf("%q (%q != %q)", in, result, out)
		}
	}
}

func TestCustomAcronymToPascal(t *testing.T) {
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
				{"WebAPIV3Spec", "WebApiv3Spec"},
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
				{"InK8s", "InK8s"},
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
				{"HandleA1000Req", "HandleA1000Req"},
				{"HandleA1001AndA1002Reply", "HandleA1001AndA1002Reply"},
				{"HandleA2000Msg", "HandleA2000Msg"},
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
				{"Mode1V1", "Mode1v1"},
				{"Mode1v3", "Mode1v3"},
				{"Mode2v2v2", "Mode2v2V2"},
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
				{"TomJerry", "TommyJerry"},
				{"JerryTom", "JerryTom"},
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
				{"CatMouse", "CatMouse"},
				{"MouseCat", "MouseKitty"},
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ctx := FromContext(NewContext(context.Background(), New(test.acronyms)))
			for _, arg := range test.args {
				if result := ctx.ToPascal(arg.value); result != arg.expected {
					t.Errorf("expected custom acronym result %s, got %s", arg.expected, result)
				}
			}
		})
	}
}

func benchmarkPascalTest(b *testing.B, fn func(testing.TB)) {
	for n := 0; n < b.N; n++ {
		fn(b)
	}
}
