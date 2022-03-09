package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/tableauio/tableau/internal/atom"
	"github.com/tableauio/tableau/internal/protogen"
	"github.com/tableauio/tableau/options"
	"gopkg.in/yaml.v2"
)

var (
	protoPackage   string
	goPackage      string
	inputDir       string
	outputDir      string
	confPath       string
	outputConfTmpl bool
)

func main() {
	var rootCmd = &cobra.Command{
		Use:     "tableauc [FILE]...",
		Version: protogen.Version,
		Short:   "Tableauc is a protoconf generator",
		Long:    `Complete documentation is available at https://tableauio.github.io`,
		Run: func(cmd *cobra.Command, args []string) {

			if outputConfTmpl {
				OutputConfTmpl()
				return
			}

			opts := &options.Options{}
			err := LoadConf(confPath, opts)
			if err != nil {
				fmt.Printf("load config(options) failed: %+v\n", err)
				os.Exit(-1)
			}
			g := protogen.NewGeneratorWithOptions(protoPackage, goPackage, inputDir, outputDir, opts)
			if len(args) == 0 {
				if err := g.Generate(); err != nil {
					atom.Log.Errorf("generate failed: %+v", err)
					os.Exit(-1)
				}
			} else {
				for _, filename := range args {
					if err := g.GenOneWorkbook(filename); err != nil {
						atom.Log.Errorf("generate failed: %+v", err)
						os.Exit(-1)
					}
				}
			}
		},
	}

	rootCmd.Flags().StringVarP(&protoPackage, "proto-package", "p", "protoconf", "proto package name")
	rootCmd.Flags().StringVarP(&goPackage, "go-package", "g", "protoconf", "go package name")
	rootCmd.Flags().StringVarP(&inputDir, "indir", "i", "./", "input directory")
	rootCmd.Flags().StringVarP(&outputDir, "outdir", "o", "./", "output directory")
	rootCmd.Flags().StringVarP(&confPath, "conf", "c", "./conf.yaml", "config file path")
	rootCmd.Flags().BoolVarP(&outputConfTmpl, "output-conf-tmpl", "t", false, "config template to file template.yaml")

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func LoadConf(path string, out interface{}) error {
	fmt.Printf("load conf path: %s\n", path)
	d, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	err = yaml.Unmarshal(d, out)
	if err != nil {
		return err
	}
	fmt.Printf("loaded conf: %+v\n", out)
	return nil
}

func OutputConfTmpl() {
	defaultConf := options.NewDefault()
	d, err := yaml.Marshal(defaultConf)
	if err != nil {
		fmt.Printf("marshal failed: %+v\n", err)
		os.Exit(-1)
	}
	fmt.Println(string(d))
}
