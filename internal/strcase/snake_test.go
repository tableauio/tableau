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
				{"WebAPIV3Spec", "web_apiv3_spec"},
			},
		},
		{
			name:               "K8s Custom Acroynm",
			acronymPattern:     "K8s",
			acronymReplacement: "k8s",
			args: []struct {
				value    string
				expected string
			}{
				{"InK8s", "in_k8s"},
			},
		},
		{
			name:               "K8s Custom Acroynm with spaces",
			acronymPattern:     "K8s",
			acronymReplacement: "k8s",
			args: []struct {
				value    string
				expected string
			}{
				{" InK8s  XX", "in_k8s__xx"},
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
				{"HandleA1000Req", "handle_a1000_req"},
				{"HandleA1001AndA1002Reply", "handle_a1001_and_a1002_reply"},
				{"HandleA2000Msg", "handle_a_2000_msg"},
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
				{"Mode1V1", "mode_1v1"},
				{"Mode1v3", "mode_1v3"},
				{"Mode2v2v2", "mode_2v2_v_2"},
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ConfigureAcronym(test.acronymPattern, test.acronymReplacement)
			for _, arg := range test.args {
				if result := ToSnake(arg.value); result != arg.expected {
					t.Errorf("expected custom acronym result %s, got %s", arg.expected, result)
				}
			}
		})
	}
}

func TestCustomAcronymsToScreamingSnake(t *testing.T) {
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
				{"WebAPIV3Spec", "WEB_APIV3_SPEC"},
			},
		},
		{
			name:               "K8s Custom Acroynm",
			acronymPattern:     "K8s",
			acronymReplacement: "k8s",
			args: []struct {
				value    string
				expected string
			}{
				{"InK8s", "IN_K8S"},
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
				{"HandleA1000Req", "HANDLE_A1000_REQ"},
				{"HandleA1001AndA1002Reply", "HANDLE_A1001_AND_A1002_REPLY"},
				{"HandleA2000Msg", "HANDLE_A_2000_MSG"},
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
				{"Mode1V1", "MODE_1V1"},
				{"Mode1v3", "MODE_1V3"},
				{"Mode2v2v2", "MODE_2V2_V_2"},
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ConfigureAcronym(test.acronymPattern, test.acronymReplacement)
			for _, arg := range test.args {
				if result := ToScreamingSnake(arg.value); result != arg.expected {
					t.Errorf("expected custom acronym result %s, got %s", arg.expected, result)
				}
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
