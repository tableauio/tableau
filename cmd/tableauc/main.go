package main

import (
	"fmt"
	"os"

	_ "time/tzdata"

	"github.com/spf13/cobra"
	"github.com/tableauio/tableau"
	"github.com/tableauio/tableau/format"
	"github.com/tableauio/tableau/log"
	"github.com/tableauio/tableau/options"
	"go.uber.org/zap/zapcore"
	"gopkg.in/yaml.v3"
)

const (
	ModeDefault = "default" // generate both proto and conf files
	ModeProto   = "proto"   // generate proto files only
	ModeConf    = "conf"    // generate conf files only.
)

var (
	protoPackage string
	indir        string
	outdir       string

	preserveFieldNumbers bool

	confInputIgnoreUnknownWorkbook bool
	confOutputSubdir               string
	confOutputFormats              []string

	mode             string
	configPath       string
	showConfigSample bool
	dryRun           options.DryRun
)

func main() {
	rootCmd := newRootCmd()
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(-1)
	}
}

// newRootCmd builds the root cobra command with all flags registered.
// Extracted from main for testability.
func newRootCmd() *cobra.Command {
	rootCmd := &cobra.Command{
		Use:     "tableauc [FILE]...",
		Version: genVersion(),
		Short:   "tableauc is a modern configuration converter.",
		Long:    `Complete documentation is available on https://tableauio.github.io.`,
		Run:     run,
	}

	rootCmd.Flags().StringVarP(&protoPackage, "proto-package", "p", "protoconf", "Protobuf package name.")
	rootCmd.Flags().StringVarP(&indir, "indir", "i", ".", "Input directory, default is current directory.")
	rootCmd.Flags().StringVarP(&outdir, "outdir", "o", ".", "Output directory, default is current directory.")
	rootCmd.Flags().BoolVarP(&preserveFieldNumbers, "preserve-field-numbers", "", false, `Preserve protobuf field numbers for backward/forward compatibility (assign new fields the max field number + 1), set it to override proto.output.preserveFieldNumbers.`)
	rootCmd.Flags().StringVarP(&confOutputSubdir, "conf-output-subdir", "", "", "Conf output sub-directory, set it to override conf.output.subdir.")
	rootCmd.Flags().StringSliceVarP(&confOutputFormats, "conf-output-formats", "", nil, "Available format: json, binpb, and txtpb, set it to override conf.output.formats.")
	rootCmd.Flags().BoolVarP(&confInputIgnoreUnknownWorkbook, "conf-input-ignore-unknown-workbook", "", false, `Whether converter will not report an error and abort if a workbook
is not recognized in proto files.`)

	rootCmd.Flags().StringVarP(&mode, "mode", "m", "default", `Available mode: default, proto, and conf.
  - default: generate both proto and conf files.
  - proto: generate proto files only.
  - conf: generate conf files only.
`)
	rootCmd.Flags().StringVarP(&configPath, "config", "c", "", "Tableauc config file path, e.g.: ./config.yaml.")
	rootCmd.Flags().BoolVarP(&showConfigSample, "show-config-sample", "s", false, "Show config sample.")
	rootCmd.Flags().StringVarP(&dryRun, "dry-run", "", "", "Preview the final result, available: patch.")

	return rootCmd
}

func run(cmd *cobra.Command, args []string) {
	// hook all errors and exit -1
	if err := runE(cmd, args); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(-1)
	}
}

func runE(cmd *cobra.Command, args []string) error {
	if showConfigSample {
		return ShowConfigSample()
	}

	config, err := loadConfig(configPath)
	if err != nil {
		return fmt.Errorf("load config failed: %s", err)
	}
	if err := tableau.SetLang(config.Lang); err != nil {
		return fmt.Errorf("set lang failed: %s", err)
	}
	if err := log.Init(config.Log); err != nil {
		return fmt.Errorf("init log failed: %s", err)
	}
	applyFlags(cmd, config)
	yamlOut, _ := yaml.Marshal(config)
	log.Debugf("loaded config:\n%s", string(yamlOut))

	switch mode {
	case ModeDefault:
		if err := genProto(args, config); err != nil {
			return err
		}
		if err := genConf(args, config); err != nil {
			return err
		}
	case ModeProto:
		return genProto(args, config)
	case ModeConf:
		return genConf(args, config)
	default:
		return fmt.Errorf("unknown mode: %s", mode)
	}

	return nil
}

// applyFlags applies command-line flag overrides to the loaded config. Each
// override takes effect only when its flag is explicitly provided on the
// command line, so config-file values are preserved when a flag is omitted.
//
// --preserve-field-numbers is bidirectional: both --preserve-field-numbers
// and --preserve-field-numbers=false override the config value (the latter
// disables it even when set to true in the config file). The other flags
// preserve their pre-existing, one-directional override semantics.
//
// NOTE: --conf-output-subdir is applied later in genConf to gain dynamic
// output subdir ability, so it is intentionally not handled here.
func applyFlags(cmd *cobra.Command, config *options.Options) {
	if cmd.Flags().Changed("preserve-field-numbers") {
		// override proto.output.preserveFieldNumbers in config file if the
		// flag is explicitly set (either true or false).
		v, _ := cmd.Flags().GetBool("preserve-field-numbers")
		config.Proto.Output.PreserveFieldNumbers = v
	}
	if cmd.Flags().Changed("conf-output-formats") {
		formats, _ := cmd.Flags().GetStringSlice("conf-output-formats")
		if len(formats) != 0 {
			var fs []format.Format
			for _, f := range formats {
				fs = append(fs, format.Format(f))
			}
			config.Conf.Output.Formats = fs
		}
	}
	if cmd.Flags().Changed("conf-input-ignore-unknown-workbook") {
		// use command argument if provided
		if v, _ := cmd.Flags().GetBool("conf-input-ignore-unknown-workbook"); v {
			config.Conf.Input.IgnoreUnknownWorkbook = true
		}
	}
	if cmd.Flags().Changed("dry-run") {
		// use command argument if provided
		if v, _ := cmd.Flags().GetString("dry-run"); v != "" {
			config.Conf.Output.DryRun = options.DryRun(v)
		}
	}
}

// genProto runs the proto generator to convert the specified workbooks into .proto files.
func genProto(workbooks []string, config *options.Options) error {
	gen := tableau.NewProtoGeneratorWithOptions(protoPackage, indir, outdir, config)
	if err := gen.Generate(workbooks...); err != nil {
		return formatError(ModeProto, err)
	}
	return nil
}

// genConf runs the conf generator to convert the specified workbooks into configuration files.
func genConf(workbooks []string, config *options.Options) error {
	if confOutputSubdir != "" {
		// override conf.output.subdir field in config file, in order
		// to gain dynamic output subdir ability.
		config.Conf.Output.Subdir = confOutputSubdir
	}
	gen := tableau.NewConfGeneratorWithOptions(protoPackage, indir, outdir, config)
	if err := gen.Generate(workbooks...); err != nil {
		return formatError(ModeConf, err)
	}
	return nil
}

// formatError formats the generation error message. At debug level, it includes the full stack
// trace (%+v) for detailed diagnostics; at higher levels, it uses a concise format (%v).
func formatError(mode string, err error) error {
	if log.LevelEnabled(zapcore.DebugLevel) {
		return fmt.Errorf("generate %s failed: \n%+v", mode, err)
	} else {
		return fmt.Errorf("generate %s failed: \n%v", mode, err)
	}
}

func loadConfig(path string) (*options.Options, error) {
	if path == "" {
		return options.NewDefault(), nil
	}
	config := &options.Options{}
	d, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	if err := yaml.Unmarshal(d, config); err != nil {
		return nil, err
	}
	return config, nil
}

func ShowConfigSample() error {
	defaultConf := options.NewDefault()
	d, err := yaml.Marshal(defaultConf)
	if err != nil {
		return err
	}
	fmt.Println(string(d))
	return nil
}

func genVersion() string {
	info := tableau.GetVersionInfo()
	ver := info.Version + "\n"
	ver += "Details:\n"
	ver += fmt.Sprintf(" %-16s %s\n", "Git commit:", info.Revision)
	ver += fmt.Sprintf(" %-16s %s\n", "Commit time:", info.Time)
	ver += fmt.Sprintf(" %-16s %s\n", "Protogen:", info.ProtogenVersion)
	ver += fmt.Sprintf(" %-16s %s\n", "Confgen:", info.ConfgenVersion)
	ver += fmt.Sprintf(" %-16s %s\n", "Experimental:", info.Experimental)
	return ver
}
