package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tableauio/tableau/format"
	"github.com/tableauio/tableau/internal/x/xproto/protoc"
	"github.com/tableauio/tableau/load"
	"github.com/tableauio/tableau/options"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/dynamicpb"
)

// newCmd parses the given CLI args against a freshly built root command and
// returns it, ready to be passed to applyFlags. It mirrors how tableauc is
// actually invoked (real flag registration), so a mismatch between a flag's
// registered name and the name looked up inside applyFlags would surface here.
func newCmd(t *testing.T, args ...string) *cobra.Command {
	t.Helper()
	cmd := newRootCmd()
	require.NoError(t, cmd.ParseFlags(args))
	return cmd
}

func TestSharedProtoConfig(t *testing.T) {
	const baseConfig = `lang: en
locationName: Asia/Shanghai
log:
  level: ERROR
  mode: SIMPLE
  sink: CONSOLE
`
	dir := t.TempDir()
	const protoConfig = baseConfig + `
proto:
  input:
    header:
      datarow: 5
      sep: "|"
      subsep: "="
      versionPattern: %q
  output: {}
`
	files := map[string]string{
		"protogen.yaml":         fmt.Sprintf(protoConfig, "9.9.9"),
		"protogen-updated.yaml": fmt.Sprintf(protoConfig, "9.999.999"),
		"protogen-without-header.yaml": baseConfig + `
proto:
  input: {}
  output: {}
`,
		"confgen.yaml": baseConfig + fmt.Sprintf(`
conf:
  input:
    protoPaths: [%q]
    protoFiles: [%q, %q]
  output:
    formats: [json]
`, filepath.ToSlash(dir), filepath.ToSlash(filepath.Join(dir, "global.proto")), filepath.ToSlash(filepath.Join(dir, "override.proto"))),
		"Global#@TABLEAU.csv":      "Sheet\nGlobalVersion\n",
		"Global#GlobalVersion.csv": "Version,Values\nversion,\"map<string,int32>\"\nVersion,Values\nSkipped row,\n1.2.3,alpha=1|beta=2\n",
		"Override#@TABLEAU.csv":    "Sheet,VersionPattern,Sep,Subsep,Datarow\n#,9.99.9,;,/,4\nBookVersion,,,,\nSheetVersion,9.99.99,,,\n",
		"Override#BookVersion.csv": "Version,Values\nversion,\"map<string,int32>\"\nVersion,Values\n1.2.3,alpha/1;beta/2\n",
		"Override#SheetVersion.csv": `Version,CustomVersion
version,"version|{pattern:""9.9.99""}"
Version,Custom version
1.2.3,1.2.3
`,
	}
	for name, data := range files {
		require.NoError(t, os.WriteFile(filepath.Join(dir, name), []byte(data), 0o600))
	}

	cmd := newCmd(t, "-c", filepath.Join(dir, "protogen.yaml"), "-m", ModeProto, "-i", dir, "-o", dir)
	require.NoError(t, runE(cmd, nil))

	cmd = newCmd(t, "-c", filepath.Join(dir, "confgen.yaml"), "-m", ModeConf, "-i", dir, "-o", dir)
	require.NoError(t, runE(cmd, nil))

	for name, declarations := range map[string][]string{
		"protoconf_tableau_defaults.proto": {`option (tableau.confgen) = {datarow:5 sep:"|" subsep:"=" version_pattern:"9.9.9"};`},
		"global.proto":                     {`import public "protoconf_tableau_defaults.proto";`},
		"override.proto":                   {`import public "protoconf_tableau_defaults.proto";`, `sep:";" subsep:"/" version_pattern:"9.99.9"`, `version_pattern:"9.99.99"`, `pattern:"9.9.99"`},
	} {
		data, err := os.ReadFile(filepath.Join(dir, name))
		require.NoError(t, err)
		for _, declaration := range declarations {
			assert.Contains(t, string(data), declaration, name)
		}
		if name == "global.proto" {
			assert.NotContains(t, string(data), "version_pattern:", name)
			assert.NotContains(t, string(data), "sep:", name)
			assert.NotContains(t, string(data), "datarow:", name)
		}
	}

	for name, want := range map[string]map[string]string{
		"GlobalVersion.json": {"version": "123"},
		"BookVersion.json":   {"version": "1023"},
		"SheetVersion.json":  {"version": "10203", "customVersion": "1203"},
	} {
		data, err := os.ReadFile(filepath.Join(dir, name))
		require.NoError(t, err)
		var conf map[string]struct{ Val string }
		require.NoError(t, json.Unmarshal(data, &conf))
		for field, val := range want {
			assert.Equal(t, val, conf[field].Val, "%s %s", name, field)
		}
	}

	for _, name := range []string{"GlobalVersion", "BookVersion"} {
		data, err := os.ReadFile(filepath.Join(dir, name+".json"))
		require.NoError(t, err)
		var conf struct{ ValuesMap map[string]int32 }
		require.NoError(t, json.Unmarshal(data, &conf))
		assert.Equal(t, map[string]int32{"alpha": 1, "beta": 2}, conf.ValuesMap)
	}

	// The source checker reads the shared defaults from the workbook descriptor.
	globalPath := filepath.Join(dir, "global.proto")
	prFiles, err := protoc.NewFiles([]string{dir}, []string{globalPath})
	require.NoError(t, err)
	desc, err := prFiles.FindDescriptorByName("protoconf.GlobalVersion")
	require.NoError(t, err)
	msg := dynamicpb.NewMessage(desc.(protoreflect.MessageDescriptor))
	require.NoError(t, load.LoadMessagerInDir(msg, dir, format.CSV, nil))
	loaded, err := protojson.Marshal(msg)
	require.NoError(t, err)
	globalJSON := filepath.Join(dir, "GlobalVersion.json")
	generated, err := os.ReadFile(globalJSON)
	require.NoError(t, err)
	assert.JSONEq(t, string(generated), string(loaded))

	// A failed selected run keeps the old defaults. A successful run replaces them.
	defaultsPath := filepath.Join(dir, "protoconf_tableau_defaults.proto")
	defaults, err := os.ReadFile(defaultsPath)
	require.NoError(t, err)
	unselected, err := os.ReadFile(globalPath)
	require.NoError(t, err)
	const selectedSheet = "Override#BookVersion.csv"
	selectedPath := filepath.Join(dir, selectedSheet)
	require.NoError(t, os.WriteFile(selectedPath, []byte("Version\n.MissingType\nVersion\n1.2.3\n"), 0o600))
	cmd = newCmd(t, "-c", filepath.Join(dir, "protogen-updated.yaml"), "-m", ModeProto, "-i", dir, "-o", dir)
	require.ErrorContains(t, runE(cmd, []string{selectedSheet}), "predefined type not found")
	after, err := os.ReadFile(defaultsPath)
	require.NoError(t, err)
	require.Equal(t, defaults, after)
	require.NoError(t, os.WriteFile(selectedPath, []byte(files[selectedSheet]), 0o600))
	require.NoError(t, runE(cmd, []string{selectedSheet}))
	defaults, err = os.ReadFile(defaultsPath)
	require.NoError(t, err)
	assert.Contains(t, string(defaults), `version_pattern:"9.999.999"`)
	unchanged, err := os.ReadFile(globalPath)
	require.NoError(t, err)
	assert.Equal(t, unselected, unchanged)

	// Generating a selection without header overrides leaves existing imports valid.
	cmd = newCmd(t, "-c", filepath.Join(dir, "protogen-without-header.yaml"), "-m", ModeProto, "-i", dir, "-o", dir)
	require.NoError(t, runE(cmd, []string{selectedSheet}))
	after, err = os.ReadFile(defaultsPath)
	require.NoError(t, err)
	assert.Equal(t, defaults, after)
	selected, err := os.ReadFile(filepath.Join(dir, "override.proto"))
	require.NoError(t, err)
	assert.NotContains(t, string(selected), "protoconf_tableau_defaults.proto")
	cmd = newCmd(t, "-c", filepath.Join(dir, "confgen.yaml"), "-m", ModeConf, "-i", dir, "-o", dir)
	require.NoError(t, runE(cmd, nil))
	updated, err := os.ReadFile(globalJSON)
	require.NoError(t, err)
	var conf struct{ Version struct{ Val string } }
	require.NoError(t, json.Unmarshal(updated, &conf))
	assert.Equal(t, "1002003", conf.Version.Val)

	// A full run regenerates every workbook, so the old defaults file can be removed.
	cmd = newCmd(t, "-c", filepath.Join(dir, "protogen-without-header.yaml"), "-m", ModeProto, "-i", dir, "-o", dir)
	require.NoError(t, runE(cmd, nil))
	require.NoFileExists(t, defaultsPath)
	regenerated, err := os.ReadFile(filepath.Join(dir, "global.proto"))
	require.NoError(t, err)
	assert.NotContains(t, string(regenerated), "protoconf_tableau_defaults.proto")
}

func TestProtoDefaultsFileFollowsHeaderPresence(t *testing.T) {
	const configTemplate = `lang: en
log: {level: ERROR, mode: SIMPLE, sink: CONSOLE}
proto:
  input:%s
  output: {}
`
	for _, tc := range []struct {
		name         string
		input        string
		defaultsDecl string
	}{
		{name: "no config file"},
		{name: "header omitted", input: " {}"},
		{name: "explicit empty header", input: "\n    header: {}", defaultsDecl: `option (tableau.confgen) = {};`},
		{
			name: "explicit built-in header values",
			input: `
    header: {namerow: 1, typerow: 2, noterow: 3, datarow: 4, sep: ",", subsep: ":", versionPattern: "255.255.255"}`,
			defaultsDecl: `option (tableau.confgen) = {namerow:1 typerow:2 noterow:3 datarow:4 sep:"," subsep:":" version_pattern:"255.255.255"};`,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			args := []string{"-m", ModeProto, "-i", dir, "-o", dir}
			if tc.input != "" {
				configPath := filepath.Join(dir, "config.yaml")
				require.NoError(t, os.WriteFile(configPath, []byte(fmt.Sprintf(configTemplate, tc.input)), 0o600))
				args = append(args, "-c", configPath)
			}
			require.NoError(t, os.WriteFile(filepath.Join(dir, "Book#@TABLEAU.csv"), []byte("Sheet\nVersionConf\n"), 0o600))
			require.NoError(t, os.WriteFile(filepath.Join(dir, "Book#VersionConf.csv"), []byte("Version\nversion\nVersion\n1.2.3\n"), 0o600))

			cmd := newCmd(t, args...)
			require.NoError(t, runE(cmd, nil))
			schemaPath := filepath.Join(dir, "book.proto")
			schema, err := os.ReadFile(schemaPath)
			require.NoError(t, err)
			assert.Contains(t, string(schema), `option (tableau.workbook) = {name:"Book#*.csv"};`)

			defaultsPath := filepath.Join(dir, "protoconf_tableau_defaults.proto")
			if tc.defaultsDecl == "" {
				require.NoFileExists(t, defaultsPath)
				assert.NotContains(t, string(schema), "protoconf_tableau_defaults.proto")
				return
			}
			defaults, err := os.ReadFile(defaultsPath)
			require.NoError(t, err)
			assert.Contains(t, string(defaults), tc.defaultsDecl)
			assert.Contains(t, string(schema), `import public "protoconf_tableau_defaults.proto";`)
			_, err = protoc.NewFiles([]string{dir}, []string{schemaPath})
			require.NoError(t, err)
		})
	}
}

// TestApplyFlags_PreserveFieldNumbers covers the --preserve-field-numbers
// flag's bidirectional override of proto.output.preserveFieldNumbers.
//
// The key case is "config true + flag false -> false": a plain bool flag
// cannot distinguish "not passed" (default false) from "explicitly passed
// false", so this only works because applyFlags gates on
// cmd.Flags().Changed(...) rather than the flag's truthiness.
func TestApplyFlags_PreserveFieldNumbers(t *testing.T) {
	cases := []struct {
		name string
		seed bool // config value before applying flags
		args []string
		want bool
	}{
		{"flag omitted: default false preserved", false, nil, false},
		{"flag omitted: config true preserved", true, nil, true},
		{"bare flag enables from false", false, []string{"--preserve-field-numbers"}, true},
		{"bare flag keeps true", true, []string{"--preserve-field-numbers"}, true},
		{"explicit true enables from false", false, []string{"--preserve-field-numbers=true"}, true},
		{"explicit true keeps true", true, []string{"--preserve-field-numbers=true"}, true},
		{"explicit false disables even when config true", true, []string{"--preserve-field-numbers=false"}, false},
		{"explicit false keeps false", false, []string{"--preserve-field-numbers=false"}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cmd := newCmd(t, tc.args...)
			config := options.NewDefault()
			config.Proto.Output.PreserveFieldNumbers = tc.seed
			applyFlags(cmd, config)
			assert.Equal(t, tc.want, config.Proto.Output.PreserveFieldNumbers)
		})
	}
}

func TestApplyFlags_Profiling(t *testing.T) {
	cases := []struct {
		name string
		seed bool
		args []string
		want bool
	}{
		{"flag omitted: default false preserved", false, nil, false},
		{"flag omitted: config true preserved", true, nil, true},
		{"bare flag enables from false", false, []string{"--profiling"}, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cmd := newCmd(t, tc.args...)
			config := options.NewDefault()
			config.Profiling = tc.seed
			applyFlags(cmd, config)
			assert.Equal(t, tc.want, config.Profiling)
		})
	}
}

// TestApplyFlags_ConfOutputFormats verifies the consolidation of the
// --conf-output-formats override into applyFlags did not change behavior:
// the flag overrides only when a non-empty list is provided.
func TestApplyFlags_ConfOutputFormats(t *testing.T) {
	t.Run("flag overrides formats", func(t *testing.T) {
		cmd := newCmd(t, "--conf-output-formats=binpb,txtpb")
		config := options.NewDefault()
		applyFlags(cmd, config)
		assert.Equal(t, []format.Format{format.Bin, format.Text}, config.Conf.Output.Formats)
	})
	t.Run("flag omitted preserves config formats", func(t *testing.T) {
		cmd := newCmd(t)
		config := options.NewDefault()
		want := []format.Format{format.JSON}
		applyFlags(cmd, config)
		assert.Equal(t, want, config.Conf.Output.Formats)
	})
}

// TestApplyFlags_ConfInputIgnoreUnknownWorkbook verifies the
// --conf-input-ignore-unknown-workbook override still only enables (never
// disables): its pre-existing one-directional semantics are preserved.
func TestApplyFlags_ConfInputIgnoreUnknownWorkbook(t *testing.T) {
	t.Run("flag enables", func(t *testing.T) {
		cmd := newCmd(t, "--conf-input-ignore-unknown-workbook")
		config := options.NewDefault()
		applyFlags(cmd, config)
		assert.True(t, config.Conf.Input.IgnoreUnknownWorkbook)
	})
	t.Run("flag omitted preserves config", func(t *testing.T) {
		cmd := newCmd(t)
		config := options.NewDefault()
		config.Conf.Input.IgnoreUnknownWorkbook = true
		applyFlags(cmd, config)
		assert.True(t, config.Conf.Input.IgnoreUnknownWorkbook)
	})
	t.Run("flag=false does not disable (existing one-directional behavior)", func(t *testing.T) {
		cmd := newCmd(t, "--conf-input-ignore-unknown-workbook=false")
		config := options.NewDefault()
		config.Conf.Input.IgnoreUnknownWorkbook = true
		applyFlags(cmd, config)
		assert.True(t, config.Conf.Input.IgnoreUnknownWorkbook)
	})
}

// TestApplyFlags_DryRun verifies the --dry-run override still only applies
// when a non-empty value is provided.
func TestApplyFlags_DryRun(t *testing.T) {
	t.Run("flag overrides dry-run", func(t *testing.T) {
		cmd := newCmd(t, "--dry-run=patch")
		config := options.NewDefault()
		applyFlags(cmd, config)
		assert.Equal(t, options.DryRunPatch, config.Conf.Output.DryRun)
	})
	t.Run("flag omitted preserves config", func(t *testing.T) {
		cmd := newCmd(t)
		config := options.NewDefault()
		applyFlags(cmd, config)
		assert.Equal(t, options.DryRun(""), config.Conf.Output.DryRun)
	})
}
