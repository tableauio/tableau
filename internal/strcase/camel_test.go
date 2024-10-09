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

func BenchmarkToLowerCamel(b *testing.B) {
	benchmarkCamelTest(b, toLowerCamel)
}

func benchmarkCamelTest(b *testing.B, fn func(testing.TB)) {
	for n := 0; n < b.N; n++ {
		fn(b)
	}
}
