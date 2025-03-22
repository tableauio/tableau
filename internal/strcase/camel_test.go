package strcase

import "testing"

func toCamel(tb testing.TB) {
	var acronyms Acronyms
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
		{"numbers2And55with000", "Numbers2And55With000"},
		{"ID", "Id"},
		{"CONSTANT_CASE", "ConstantCase"},
	}
	for _, i := range cases {
		in := i[0]
		out := i[1]
		result := acronyms.ToCamel(in)
		if result != out {
			tb.Errorf("%q (%q != %q)", in, result, out)
		}
	}
}

func TestToCamel(t *testing.T) {
	toCamel(t)
}

func BenchmarkToCamel(b *testing.B) {
	benchmarkCamelTest(b, toCamel)
}

func toLowerCamel(tb testing.TB) {
	var acronyms Acronyms
	cases := [][]string{
		{"foo-bar", "fooBar"},
		{"TestCase", "testCase"},
		{"", ""},
		{"AnyKind of_string", "anyKindOfString"},
		{"AnyKind.of-string", "anyKindOfString"},
		{"ID", "id"},
		{"some string", "someString"},
		{" some string", "someString"},
		{"CONSTANT_CASE", "constantCase"},
	}
	for _, i := range cases {
		in := i[0]
		out := i[1]
		result := acronyms.ToLowerCamel(in)
		if result != out {
			tb.Errorf("%q (%q != %q)", in, result, out)
		}
	}
}

func TestToLowerCamel(t *testing.T) {
	toLowerCamel(t)
}

func TestCustomAcronymToCamel(t *testing.T) {
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
			acronyms := ParseAcronyms(test.acronyms)
			for _, arg := range test.args {
				if result := acronyms.ToCamel(arg.value); result != arg.expected {
					t.Errorf("expected custom acronym result %s, got %s", arg.expected, result)
				}
			}
		})
	}
}

func TestCustomAcronymToLowerCamel(t *testing.T) {
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
				{"WebAPIV3Spec", "webApiv3Spec"},
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
				{"InK8s", "inK8s"},
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
				{"HandleA1000Req", "handleA1000Req"},
				{"HandleA1001AndA1002Reply", "handleA1001AndA1002Reply"},
				{"HandleA2000Msg", "handleA2000Msg"},
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
				{"Mode2v2v2", "mode2v2V2"},
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
				{"TomJerry", "tommyJerry"},
				{"JerryTom", "jerryTom"},
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
				{"CatMouse", "catMouse"},
				{"MouseCat", "mouseKitty"},
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			acronyms := ParseAcronyms(test.acronyms)
			for _, arg := range test.args {
				if result := acronyms.ToLowerCamel(arg.value); result != arg.expected {
					t.Errorf("expected custom acronym result %s, got %s", arg.expected, result)
				}
			}
		})
	}
}

func BenchmarkToLowerCamel(b *testing.B) {
	benchmarkCamelTest(b, toLowerCamel)
}

func benchmarkCamelTest(b *testing.B, fn func(testing.TB)) {
	for n := 0; n < b.N; n++ {
		fn(b)
	}
}
