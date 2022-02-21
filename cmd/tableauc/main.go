package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/tableauio/tableau/format"
	"github.com/tableauio/tableau/internal/atom"
	"github.com/tableauio/tableau/internal/protogen"
	"github.com/tableauio/tableau/options"
)

var (
	protoPackage             string
	goPackage                string
	locationName             string
	inputDir                 string
	outputDir                string
	filenameWithSubdirPrefix bool
	filenameSuffix           string
	inputFormat              string
	logLevel                 string

	// xlsx header
	namerow  int32
	typerow  int32
	noterow  int32
	datarow  int32
	nameline int32
	typeline int32

	imports []string
)

func main() {
	var rootCmd = &cobra.Command{
		Use:     "tableauc [FILE]...",
		Version: protogen.Version,
		Short:   "Tableauc is a protoconf generator",
		Long:    `Complete documentation is available at https://tableauio.github.io`,
		Run: func(cmd *cobra.Command, args []string) {
			// Do Stuff Here
			formatType := format.Excel
			switch inputFormat {
			case "excel":
				formatType = format.Excel
			case "csv":
				formatType = format.CSV
			case "xml":
				formatType = format.XML
			}
			g := protogen.NewGenerator(protoPackage, goPackage, inputDir, outputDir,
				options.Header(&options.HeaderOption{
					Namerow: namerow,
					Typerow: typerow,
					Noterow: noterow,
					Datarow: datarow,

					Nameline: nameline,
					Typeline: typeline,
				},
				),
				options.Imports(imports),
				options.LocationName(locationName),
				options.Output(
					&options.OutputOption{
						FilenameSuffix:           filenameSuffix,
						FilenameWithSubdirPrefix: filenameWithSubdirPrefix,
					},
				),
				options.Input(&options.InputOption{
					Format: formatType,
				}),
			)
			atom.InitZap(logLevel)
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

	rootCmd.Flags().StringVarP(&protoPackage, "proto-package", "", "protoconf", "proto package name")
	rootCmd.Flags().StringVarP(&goPackage, "go-package", "", "protoconfpb", "go package name")
	rootCmd.Flags().StringVarP(&locationName, "location-name", "", "", "location name for locale time parser")
	rootCmd.Flags().StringVarP(&inputDir, "input-dir", "i", "./", "input directory")
	rootCmd.Flags().StringVarP(&outputDir, "output-dir", "o", "./", "output directory")
	rootCmd.Flags().BoolVarP(&filenameWithSubdirPrefix, "with-subdir-prefix", "", false, "output filename with subdir prefix")
	rootCmd.Flags().StringVarP(&filenameSuffix, "suffix", "s", "", "output filename suffix")
	rootCmd.Flags().StringVarP(&logLevel, "log-level", "", "info", "log level: debug, info, warn, error")
	rootCmd.Flags().StringVarP(&inputFormat, "format", "", "excel", "input file format: excel, xml")

	rootCmd.Flags().Int32VarP(&namerow, "namerow", "", 1, "name row in xlsx")
	rootCmd.Flags().Int32VarP(&typerow, "typerow", "", 2, "type row in xlsx")
	rootCmd.Flags().Int32VarP(&noterow, "noterow", "", 3, "note row in xlsx")
	rootCmd.Flags().Int32VarP(&datarow, "datarow", "", 4, "data row in xlsx")
	rootCmd.Flags().Int32VarP(&nameline, "nameline", "", 0, "name line in xlsx cell")
	rootCmd.Flags().Int32VarP(&typeline, "typeline", "", 0, "type line in xlsx cell")

	rootCmd.Flags().StringSliceVarP(&imports, "imports", "", nil, "import common protobuf files")

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
