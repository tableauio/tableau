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
	"github.com/tableauio/tableau/options"
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

func TestVersionPatternConfig(t *testing.T) {
	dir := t.TempDir()
	files := map[string]string{
		"config.yaml": fmt.Sprintf(`lang: en
locationName: Asia/Shanghai
log:
  level: ERROR
  mode: SIMPLE
  sink: CONSOLE
proto:
  input:
    versionPattern: "9.9.9"
  output: {}
conf:
  input:
    protoPaths: [%q]
    protoFiles: [%q]
  output:
    formats: [json]
`, filepath.ToSlash(dir), filepath.ToSlash(filepath.Join(dir, "*.proto"))),
		"Global#@TABLEAU.csv":      "Sheet\nGlobalVersion\n",
		"Global#GlobalVersion.csv": "Version\nversion\nVersion\n1.2.3\n",
		"Override#@TABLEAU.csv":    "Sheet,VersionPattern\n#,9.99.9\nBookVersion,\nSheetVersion,9.99.99\n",
		"Override#BookVersion.csv": "Version\nversion\nVersion\n1.2.3\n",
		"Override#SheetVersion.csv": `Version,CustomVersion
version,"version|{pattern:""9.9.99""}"
Version,Custom version
1.2.3,1.2.3
`,
	}
	for name, data := range files {
		require.NoError(t, os.WriteFile(filepath.Join(dir, name), []byte(data), 0o600))
	}

	cmd := newCmd(t, "-c", filepath.Join(dir, "config.yaml"), "-i", dir, "-o", dir)
	require.NoError(t, runE(cmd, nil))

	for name, patterns := range map[string][]string{
		"global.proto":   {`version_pattern:"9.9.9"`},
		"override.proto": {`version_pattern:"9.99.9"`, `version_pattern:"9.99.99"`, `pattern:"9.9.99"`},
	} {
		data, err := os.ReadFile(filepath.Join(dir, name))
		require.NoError(t, err)
		for _, pattern := range patterns {
			assert.Contains(t, string(data), pattern, name)
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
}

func TestLegacyVersionPatternFailsBeforeGeneration(t *testing.T) {
	const config = `lang: en
locationName: Asia/Shanghai
log:
  level: ERROR
  mode: SIMPLE
  sink: CONSOLE
conf:
  input:
    protoPaths: ["../../test/functest/proto/default"]
    protoFiles: ["../../test/functest/proto/default/excel__wellknown__well_known.proto"]
    versionPattern: "9.99.99"
  output:
    formats: [json]
`
	dir := t.TempDir()
	configFile := filepath.Join(dir, "config.yaml")
	outputDir := filepath.Join(dir, "output")
	require.NoError(t, os.WriteFile(configFile, []byte(config), 0o600))

	cmd := newCmd(t, "-c", configFile, "-m", ModeConf, "-i", "../../test/functest/testdata/default", "-o", outputDir)
	err := runE(cmd, nil)
	require.ErrorContains(t, err, "conf.input.versionPattern has moved to proto.input.versionPattern")
	require.ErrorContains(t, err, "regenerate the proto files")
	require.NoDirExists(t, outputDir)
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
