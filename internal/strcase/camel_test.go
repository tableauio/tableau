package strcase

import "testing"

func toCamel(tb testing.TB) {
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
		result := ToCamel(in)
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
		result := ToLowerCamel(in)
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
		name         string
		acronymKey   string
		acronymValue string
		value        string
		expected     string
	}{
		{
			name:         "CCTV Custom Acronym",
			acronymKey:   "CCTV",
			acronymValue: "cctv",
			value:        "CCTVChannel",
			expected:     "CctvChannel",
		},
		{
			name:         "ABCDACME Custom Acroynm",
			acronymKey:   "ABCDACME",
			acronymValue: "AbcdAcme",
			value:        "ABCDACMEAlias",
			expected:     "AbcdAcmeAlias",
		},
		{
			name:         "PostgreSQL Custom Acronym",
			acronymKey:   "PostgreSQL",
			acronymValue: "postgreSQL",
			value:        "powerfulPostgreSQLDatabase",
			expected:     "PowerfulPostgreSQLDatabase",
		},
		{
			name:         "APIV3 Custom Acronym",
			acronymKey:   "APIV3",
			acronymValue: "apiv3",
			value:        "WebAPIV3Spec",
			expected:     "WebApiv3Spec",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ConfigureAcronym(test.acronymKey, test.acronymValue)
			if result := ToCamel(test.value); result != test.expected {
				t.Errorf("expected custom acronym result %s, got %s", test.expected, result)
			}
		})
	}
}

func TestCustomAcronymToLowerCamel(t *testing.T) {
	tests := []struct {
		name         string
		acronymKey   string
		acronymValue string
		value        string
		expected     string
	}{
		{
			name:         "CCTV Custom Acronym",
			acronymKey:   "CCTV",
			acronymValue: "cctv",
			value:        "CCTVChannel",
			expected:     "cctvChannel",
		},
		{
			name:         "ABCDACME Custom Acroynm",
			acronymKey:   "ABCDACME",
			acronymValue: "AbcdAcme",
			value:        "ABCDACMEAlias",
			expected:     "abcdAcmeAlias",
		},
		{
			name:         "PostgreSQL Custom Acronym",
			acronymKey:   "PostgreSQL",
			acronymValue: "postgreSQL",
			value:        "PowerfulPostgreSQLDatabase",
			expected:     "powerfulPostgreSQLDatabase",
		},
		{
			name:         "APIV3 Custom Acronym",
			acronymKey:   "APIV3",
			acronymValue: "apiv3",
			value:        "WebAPIV3Spec",
			expected:     "webApiv3Spec",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ConfigureAcronym(test.acronymKey, test.acronymValue)
			if result := ToLowerCamel(test.value); result != test.expected {
				t.Errorf("expected custom acronym result %s, got %s", test.expected, result)
			}
		})
	}
}

func TestAcronymRegexesToCamel(t *testing.T) {
	tests := []struct {
		name               string
		acronymPattern     string
		acronymReplacement string
		args               []struct {
			value    string
			expected string
		}
	}{
		{
			name:               "APIV3 Custom Acronym",
			acronymPattern:     "APIV3",
			acronymReplacement: "apiv3",
			args: []struct {
				value    string
				expected string
			}{
				{"WebAPIV3Spec", "WebApiv3Spec"},
			},
		},
		{
			name:               "HandleA1000Req Custom Acronym",
			acronymPattern:     `A(1\d{3})`,
			acronymReplacement: "a${1}",
			args: []struct {
				value    string
				expected string
			}{
				{"HandleA1000Req", "HandleA1000Req"},
				{"HandleA1001Reply", "HandleA1001Reply"},
				{"HandleA2000Msg", "HandleA2000Msg"},
			},
		},
		{
			name:               "Mode1V1 Custom Acronym",
			acronymPattern:     `(\d)[vV](\d)`,
			acronymReplacement: "${1}v${2}",
			args: []struct {
				value    string
				expected string
			}{
				{"Mode1V1", "Mode1v1"},
				{"Mode1v3", "Mode1v3"},
				{"Mode2v2", "Mode2v2"},
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ConfigureAcronymRegex(test.acronymPattern, test.acronymReplacement)
			for _, arg := range test.args {
				if result := ToCamel(arg.value); result != arg.expected {
					t.Errorf("expected custom acronym result %s, got %s", arg.expected, result)
				}
			}
		})
	}
}

func TestAcronymRegexesToLowerCamel(t *testing.T) {
	tests := []struct {
		name               string
		acronymPattern     string
		acronymReplacement string
		args               []struct {
			value    string
			expected string
		}
	}{
		{
			name:               "APIV3 Custom Acronym",
			acronymPattern:     "APIV3",
			acronymReplacement: "apiv3",
			args: []struct {
				value    string
				expected string
			}{
				{"WebAPIV3Spec", "webApiv3Spec"},
			},
		},
		{
			name:               "HandleA1000Req Custom Acronym",
			acronymPattern:     `A(1\d{3})`,
			acronymReplacement: "a${1}",
			args: []struct {
				value    string
				expected string
			}{
				{"HandleA1000Req", "handleA1000Req"},
				{"HandleA1001Reply", "handleA1001Reply"},
				{"HandleA2000Msg", "handleA2000Msg"},
			},
		},
		{
			name:               "Mode1V1 Custom Acronym",
			acronymPattern:     `(\d)[vV](\d)`,
			acronymReplacement: "${1}v${2}",
			args: []struct {
				value    string
				expected string
			}{
				{"Mode1V1", "mode1v1"},
				{"Mode1v3", "mode1v3"},
				{"Mode2v2", "mode2v2"},
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ConfigureAcronymRegex(test.acronymPattern, test.acronymReplacement)
			for _, arg := range test.args {
				if result := ToLowerCamel(arg.value); result != arg.expected {
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
