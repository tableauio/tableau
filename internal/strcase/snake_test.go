package strcase

import "testing"

func toSnake(tb testing.TB) {
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
		{"numbers2and55with000", "numbers_2_and_55_with_000"},
		{"JSONData", "json_data"},
		{"userID", "user_id"},
		{"AAAbbb", "aa_abbb"},
		{"1A2", "1_a_2"},
		{"A1B", "a_1_b"},
		{"A1A2A3", "a_1_a_2_a_3"},
		{"A1 A2 A3", "a_1_a_2_a_3"},
		{"AB1AB2AB3", "ab_1_ab_2_ab_3"},
		{"AB1 AB2 AB3", "ab_1_ab_2_ab_3"},
		{"some string", "some_string"},
		{" some string", "some_string"},
	}
	for _, i := range cases {
		in := i[0]
		out := i[1]
		result := ToSnake(in)
		if result != out {
			tb.Errorf("%q (%q != %q)", in, result, out)
		}
	}
}

func TestToSnake(t *testing.T) { toSnake(t) }

func BenchmarkToSnake(b *testing.B) {
	benchmarkSnakeTest(b, toSnake)
}

func toSnakeWithIgnore(tb testing.TB) {
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
		{"numbers2and55with000", "numbers_2_and_55_with_000"},
		{"JSONData", "json_data"},
		{"AwesomeActivity.UserID", "awesome_activity.user_id", "."},
		{"AwesomeActivity.User.Id", "awesome_activity.user.id", "."},
		{"AwesomeUsername@Awesome.Com", "awesome_username@awesome.com", ".@"},
		{"lets-ignore all.of dots-and-dashes", "lets-ignore_all.of_dots-and-dashes", ".-"},
	}
	for _, i := range cases {
		in := i[0]
		out := i[1]
		var ignore string
		ignore = ""
		if len(i) == 3 {
			ignore = i[2]
		}
		result := ToSnakeWithIgnore(in, ignore)
		if result != out {
			istr := ""
			if len(i) == 3 {
				istr = " ignoring '" + i[2] + "'"
			}
			tb.Errorf("%q (%q != %q%s)", in, result, out, istr)
		}
	}
}

func TestToSnakeWithIgnore(t *testing.T) { toSnakeWithIgnore(t) }

func BenchmarkToSnakeWithIgnore(b *testing.B) {
	benchmarkSnakeTest(b, toSnakeWithIgnore)
}

func TestCustomAcronymsToSnake(t *testing.T) {
	tests := []struct {
		name         string
		acronymKey   string
		acronymValue string
		value        string
		expected     string
	}{
		// TODO: API and APIV3, find the longest acronym
		// {
		// 	name:         "API Custom Acronym",
		// 	acronymKey:   "API",
		// 	acronymValue: "api",
		// 	expected:     "Api",
		// },
		{
			name:         "APIV3 Custom Acronym",
			acronymKey:   "APIV3",
			acronymValue: "apiv3",
			value:        "WebAPIV3Spec",
			expected:     "web_apiv3_spec",
		},
		{
			name:         "APIV3 Custom Acronym with spaces",
			acronymKey:   "APIV3",
			acronymValue: "apiv3",
			value:        "APIV3 Spec ",
			expected:     "apiv3_spec",
		},
		{
			name:         "K8s Custom Acroynm",
			acronymKey:   "K8s",
			acronymValue: "k8s",
			value:        "InK8s",
			expected:     "in_k8s",
		},
		{
			name:         "K8s Custom Acroynm with spaces",
			acronymKey:   "K8s",
			acronymValue: "k8s",
			value:        " InK8s  XX",
			expected:     "in_k8s__xx",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ConfigureAcronym(test.acronymKey, test.acronymValue)
			if result := ToSnake(test.value); result != test.expected {
				t.Errorf("expected custom acronym result %s, got %s", test.expected, result)
			}
		})
	}
}

func toDelimited(tb testing.TB) {
	cases := [][]string{
		{"testCase", "test@case"},
		{"TestCase", "test@case"},
		{"Test Case", "test@case"},
		{" Test Case", "test@case"},
		{"Test Case ", "test@case"},
		{" Test Case ", "test@case"},
		{"test", "test"},
		{"test_case", "test@case"},
		{"Test", "test"},
		{"", ""},
		{"ManyManyWords", "many@many@words"},
		{"manyManyWords", "many@many@words"},
		{"AnyKind of_string", "any@kind@of@string"},
		{"numbers2and55with000", "numbers@2@and@55@with@000"},
		{"JSONData", "json@data"},
		{"userID", "user@id"},
		{"AAAbbb", "aa@abbb"},
		{"test-case", "test@case"},
	}
	for _, i := range cases {
		in := i[0]
		out := i[1]
		result := ToDelimited(in, '@')
		if result != out {
			tb.Errorf("%q (%q != %q)", in, result, out)
		}
	}
}

func TestToDelimited(t *testing.T) { toDelimited(t) }

func BenchmarkToDelimited(b *testing.B) {
	benchmarkSnakeTest(b, toDelimited)
}

func toScreamingSnake(tb testing.TB) {
	cases := [][]string{
		{"testCase", "TEST_CASE"},
	}
	for _, i := range cases {
		in := i[0]
		out := i[1]
		result := ToScreamingSnake(in)
		if result != out {
			tb.Errorf("%q (%q != %q)", in, result, out)
		}
	}
}

func TestToScreamingSnake(t *testing.T) { toScreamingSnake(t) }

func BenchmarkToScreamingSnake(b *testing.B) {
	benchmarkSnakeTest(b, toScreamingSnake)
}

func toKebab(tb testing.TB) {
	cases := [][]string{
		{"testCase", "test-case"},
	}
	for _, i := range cases {
		in := i[0]
		out := i[1]
		result := ToKebab(in)
		if result != out {
			tb.Errorf("%q (%q != %q)", in, result, out)
		}
	}
}

func TestToKebab(t *testing.T) { toKebab(t) }

func BenchmarkToKebab(b *testing.B) {
	benchmarkSnakeTest(b, toKebab)
}

func toScreamingKebab(tb testing.TB) {
	cases := [][]string{
		{"testCase", "TEST-CASE"},
	}
	for _, i := range cases {
		in := i[0]
		out := i[1]
		result := ToScreamingKebab(in)
		if result != out {
			tb.Errorf("%q (%q != %q)", in, result, out)
		}
	}
}

func TestToScreamingKebab(t *testing.T) { toScreamingKebab(t) }

func BenchmarkToScreamingKebab(b *testing.B) {
	benchmarkSnakeTest(b, toScreamingKebab)
}

func toScreamingDelimited(tb testing.TB) {
	cases := [][]string{
		{"testCase", "TEST.CASE"},
	}
	for _, i := range cases {
		in := i[0]
		out := i[1]
		result := ToScreamingDelimited(in, '.', "", true)
		if result != out {
			tb.Errorf("%q (%q != %q)", in, result, out)
		}
	}
}

func TestToScreamingDelimited(t *testing.T) { toScreamingDelimited(t) }

func BenchmarkToScreamingDelimited(b *testing.B) {
	benchmarkSnakeTest(b, toScreamingDelimited)
}

func toScreamingDelimitedWithIgnore(tb testing.TB) {
	cases := [][]string{
		{"AnyKind of_string", "ANY.KIND OF.STRING", ".", " "},
	}
	for _, i := range cases {
		in := i[0]
		out := i[1]
		delimiter := i[2][0]
		ignore := i[3][0]
		result := ToScreamingDelimited(in, delimiter, string(ignore), true)
		if result != out {
			istr := ""
			if len(i) == 4 {
				istr = " ignoring '" + i[3] + "'"
			}
			tb.Errorf("%q (%q != %q%s)", in, result, out, istr)
		}
	}
}

func TestToScreamingDelimitedWithIgnore(t *testing.T) { toScreamingDelimitedWithIgnore(t) }

func BenchmarkToScreamingDelimitedWithIgnore(b *testing.B) {
	benchmarkSnakeTest(b, toScreamingDelimitedWithIgnore)
}

func benchmarkSnakeTest(b *testing.B, fn func(testing.TB)) {
	for n := 0; n < b.N; n++ {
		fn(b)
	}
}
