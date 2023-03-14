package main

import (
	"fmt"
	"os"

	_ "time/tzdata"

	"github.com/davecgh/go-spew/spew"
	"github.com/spf13/cobra"
	"github.com/tableauio/tableau"
	"github.com/tableauio/tableau/log"
	"github.com/tableauio/tableau/options"
	"github.com/tableauio/tableau/xerrors"
	"gopkg.in/yaml.v2"
)

const version = "0.5.6"
const (
	ModeDefault = "default" // generate both proto and conf files
	ModeProto   = "proto"   // generate proto files only
	ModeConf    = "conf"    // generate conf files only.
)

var (
	protoPackage     string
	indir            string
	outdir           string
	confOutputSubdir string
	mode             string
	configPath       string
	showConfigSample bool
)

func main() {
	var rootCmd = &cobra.Command{
		Use:     "tableauc [FILE]...",
		Version: genVersion(),
		Short:   "tableauc is a modern configuration converter.",
		Long:    `Complete documentation is available on https://tableauio.github.io.`,
		Run:     run,
	}

	rootCmd.Flags().StringVarP(&protoPackage, "proto-package", "p", "protoconf", "protobuf package name")
	rootCmd.Flags().StringVarP(&indir, "indir", "i", ".", "input directory, default is current directory")
	rootCmd.Flags().StringVarP(&outdir, "outdir", "o", ".", "output directory, default is current directory")
	rootCmd.Flags().StringVarP(&confOutputSubdir, "conf-output-subdir", "", "", "conf output sub-directory, set it to override output.conf.subdir")
	rootCmd.Flags().StringVarP(&mode, "mode", "m", "default", `available mode: default, proto, and conf. 
  - default: generate both proto and conf files.
  - proto: generate proto files only.
  - conf: generate conf files only.
`)
	rootCmd.Flags().StringVarP(&configPath, "config", "c", "", "tableauc config file path, e.g.: ./config.yaml")
	rootCmd.Flags().BoolVarP(&showConfigSample, "show-config-sample", "s", false, "show config sample")

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(-1)
	}
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
	log.Debugf("load config success: %+v", spew.Sdump(config))

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

func genProto(workbooks []string, config *options.Options) error {
	// generate proto files
	gen := tableau.NewProtoGeneratorWithOptions(protoPackage, indir, outdir, config)
	if err := gen.Generate(workbooks...); err != nil {
		logError(ModeProto, err)
		return fmt.Errorf("generate proto failed")
	}
	return nil
}

func genConf(workbooks []string, config *options.Options) error {
	// generate conf files
	if confOutputSubdir != "" {
		// override conf.output.subdir field in config file, in order
		// to gain dynamic output subdir ability.
		config.Conf.Output.Subdir = confOutputSubdir
	}
	gen := tableau.NewConfGeneratorWithOptions(protoPackage, indir, outdir, config)
	if err := gen.Generate(workbooks...); err != nil {
		logError(ModeConf, err)
		return fmt.Errorf("generate conf failed")
	}
	return nil
}

func logError(mode string, err error) {
	if log.Mode() == log.ModeFull {
		log.Errorf("generate %s file failed: %+v", mode, err)
	}
	log.Errorf("%s", xerrors.NewDesc(err))
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
	verInfo := tableau.GetVersionInfo()
	ver := version
	ver += fmt.Sprintf(" (%s, %s)", verInfo.ProtoGenVer, verInfo.ConfGenVer)
	return ver
}
