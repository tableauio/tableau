package main

import (
	"fmt"
	"os"

	_ "time/tzdata"

	"github.com/davecgh/go-spew/spew"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"github.com/tableauio/tableau/internal/confgen"
	"github.com/tableauio/tableau/internal/protogen"
	"github.com/tableauio/tableau/log"
	"github.com/tableauio/tableau/options"
	"github.com/tableauio/tableau/xerrors"
	"gopkg.in/yaml.v2"
)

const version = "0.5.3"
const (
	ModeDefault = "default" // generate both proto and conf files
	ModeProto   = "proto"
	ModeConf    = "conf"
)

var (
	protoPackage string
	indir        string
	outdir       string
	mode         string
	// protoFiles         []string
	configPath         string
	needOutputConfTmpl bool
)

func main() {
	var rootCmd = &cobra.Command{
		Use:     "tableauc [FILE]...",
		Version: genVersion(),
		Short:   "Tableauc is a modern configuration converter",
		Long:    `Complete documentation is available at https://tableauio.github.io`,
		Run:     runCmd,
	}

	rootCmd.Flags().StringVarP(&protoPackage, "proto-package", "p", "protoconf", "Proto package name")
	rootCmd.Flags().StringVarP(&indir, "indir", "i", ".", "Input directory, default is current directory")
	rootCmd.Flags().StringVarP(&outdir, "outdir", "o", ".", "Output directory, default is current directory")
	// rootCmd.Flags().StringSliceVarP(&protoFiles, "proto-files", "", nil, "Specify proto files to generate configurations. Glob pattern is supported")
	rootCmd.Flags().StringVarP(&mode, "mode", "m", "default", `Available mode: default, proto, and conf. 
- default: generate both proto and conf files.
- proto: generate proto files only.
- conf: generate conf files only.
`)
	rootCmd.Flags().StringVarP(&configPath, "config", "c", "./config.yaml", "Config file path")
	rootCmd.Flags().BoolVarP(&needOutputConfTmpl, "output-config-template", "t", false, "Output config template")

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(-1)
	}
}

func runCmd(cmd *cobra.Command, args []string) {
	if needOutputConfTmpl {
		outputConfTmpl()
		return
	}

	opts := &options.Options{}
	err := loadConf(configPath, opts)
	if err != nil {
		log.Errorf("load config(options) failed: %+v", err)
		os.Exit(-1)
	}
	if err := log.Init(opts.Log); err != nil {
		log.Errorf("init log failed: %+v", err)
		os.Exit(-1)
	}
	log.Debugf("loaded tableau config: %+v", spew.Sdump(opts))
	switch mode {
	case ModeDefault:
		genProto(args, opts)
		genConf(args, opts)
	case ModeProto:
		genProto(args, opts)
	case ModeConf:
		genConf(args, opts)
	default:
		log.Errorf("unknown mode: %s", mode)
		os.Exit(-1)
	}
}

func genProto(workbooks []string, opts *options.Options) {
	// generate proto files
	gen := protogen.NewGeneratorWithOptions(protoPackage, indir, outdir, opts)
	if err := gen.Generate(workbooks...); err != nil {
		logError(ModeProto, err)
		os.Exit(-1)
	}
}

func genConf(workbooks []string, opts *options.Options) {
	// generate conf files
	gen := confgen.NewGeneratorWithOptions(protoPackage, indir, outdir, opts)
	if err := gen.Generate(workbooks...); err != nil {
		logError(ModeConf, err)
		os.Exit(-1)
	}
}

func logError(mode string, err error) {
	if log.Mode() == log.ModeFull {
		log.Errorf("generate %s file failed: %+v", mode, err)
	}
	if log.Lang() == log.LangEn {
		log.Errorf("%s", xerrors.NewDesc(err).String())
	} else {
		log.Errorf("%s", xerrors.NewDesc(err).StringZh())
	}
}

func loadConf(path string, out interface{}) error {
	d, err := os.ReadFile(path)
	if err != nil {
		return errors.WithStack(err)
	}
	err = yaml.Unmarshal(d, out)
	if err != nil {
		return errors.WithStack(err)
	}
	return nil
}

func outputConfTmpl() {
	defaultConf := options.NewDefault()
	d, err := yaml.Marshal(defaultConf)
	if err != nil {
		fmt.Printf("marshal failed: %+v\n", err)
		os.Exit(-1)
	}
	fmt.Println(string(d))
}

func genVersion() string {
	ver := version
	ver += fmt.Sprintf(" (%s, %s)", protogen.AppVersion(), confgen.AppVersion())
	return ver
}
