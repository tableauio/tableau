package main

import (
	"fmt"
	"os"

	_ "time/tzdata"

	"github.com/davecgh/go-spew/spew"
	"github.com/fatih/color"
	"github.com/spf13/cobra"
	"github.com/tableauio/tableau/internal/atom"
	"github.com/tableauio/tableau/internal/confgen"
	"github.com/tableauio/tableau/internal/protogen"
	"github.com/tableauio/tableau/options"
	"gopkg.in/yaml.v2"
)

const version = "0.3.1"
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
		fmt.Printf("load config(options) failed: %+v\n", err)
		os.Exit(-1)
	}
	atom.InitConsoleLog(opts.LogLevel)
	switch mode {
	case ModeDefault:
		genProto(args, opts)
		genConf(args, opts)
	case ModeProto:
		genProto(args, opts)
	case ModeConf:
		genConf(args, opts)
	default:
		fmt.Printf("unknown mode: %s\n", mode)
		os.Exit(-1)
	}
}

func genProto(workbooks []string, opts *options.Options) {
	red := color.New(color.FgRed).SprintfFunc()
	// generate proto files
	g1 := protogen.NewGeneratorWithOptions(protoPackage, indir, outdir, opts)
	if len(workbooks) == 0 {
		if err := g1.Generate(); err != nil {
			atom.Log.Errorf(red("generate proto files failed: %+v", err))
			os.Exit(-1)
		}
	} else {
		for _, wbpath := range workbooks {
			if err := g1.GenOneWorkbook(wbpath); err != nil {
				atom.Log.Errorf(red("generate proto file of %s failed: %+v", wbpath, err))
				os.Exit(-1)
			}
		}
	}
}

func genConf(workbooks []string, opts *options.Options) {
	red := color.New(color.FgRed).SprintfFunc()
	// generate conf files
	g2 := confgen.NewGeneratorWithOptions(protoPackage, indir, outdir, opts)
	if len(workbooks) == 0 {
		if err := g2.Generate(opts.Workbook, opts.Worksheet); err != nil {
			atom.Log.Errorf(red("generate conf files failed: %+v", err))
			os.Exit(-1)
		}
	} else {
		for _, wbpath := range workbooks {
			if err := g2.Generate(wbpath, ""); err != nil {
				atom.Log.Errorf(red("generate conf file of %s failed: %+v", wbpath, err))
				os.Exit(-1)
			}
		}
	}
}
func loadConf(path string, out interface{}) error {
	fmt.Printf("load conf path: %s\n", path)
	d, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	err = yaml.Unmarshal(d, out)
	if err != nil {
		return err
	}
	fmt.Printf("loaded conf: %+v\n", spew.Sdump(out))
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
	ver += fmt.Sprintf(" (%s)", protogen.AppVersion())
	return ver
}
