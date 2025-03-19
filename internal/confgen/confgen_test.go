package confgen

import (
	"fmt"
	"os"
	"testing"

	"github.com/tableauio/tableau/format"
	"github.com/tableauio/tableau/internal/x/xfs"
	"github.com/tableauio/tableau/log"
	"github.com/tableauio/tableau/options"
)

func prepareOutput() error {
	// prepare output common dir
	outdir := "./testdata/_conf"
	err := os.MkdirAll(outdir, xfs.DefaultDirPerm)
	if err != nil {
		return fmt.Errorf("failed to create output dir: %v", err)
	}
	return nil
}

func TestGenerator_GenWorkbook(t *testing.T) {
	if err := prepareOutput(); err != nil {
		t.Fatalf("failed to create output common dir: %v", err)
	}
	outdir := "./testdata/_conf_gen_workbook"
	type args struct {
		bookSpecifiers []string
	}
	tests := []struct {
		name    string
		gen     *Generator
		args    args
		wantErr bool
	}{
		{
			name: "test1",
			gen: NewGenerator("protoconf", "../../test/functest/testdata/default/", outdir,
				options.LocationName("Asia/Shanghai"),
				options.Conf(
					&options.ConfOption{
						Input: &options.ConfInputOption{
							ProtoPaths: []string{"../../test/functest/proto/default"},
							ProtoFiles: []string{"../../test/functest/proto/default/*.proto"},
							Formats: []format.Format{
								// format.Excel,
								format.CSV,
								format.YAML,
							},
							ExcludedProtoFiles: []string{
								"../../test/functest/proto/default/xml__metasheet__metasheet.proto",
							},
							Subdirs: []string{
								"excel/",
								"yaml/",
							},
						},
						Output: &options.ConfOutputOption{
							Pretty:          true,
							Formats:         []format.Format{format.JSON},
							EmitUnpopulated: true,
						},
					},
				),
				options.Log(
					&log.Options{
						Level: "DEBUG",
						Mode:  "FULL",
					},
				),
				options.Lang("zh"),
			),
			args: args{
				bookSpecifiers: []string{
					"excel/struct/Struct#*.csv",
					"excel/map/Map#*.csv",
					"yaml/Map.yaml",
				},
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.gen.GenWorkbook(tt.args.bookSpecifiers...); (err != nil) != tt.wantErr {
				t.Errorf("Generator.GenWorkbook() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
