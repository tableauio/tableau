package confgen

import (
	"fmt"
	"os"
	"testing"

	"github.com/tableauio/tableau/format"
	"github.com/tableauio/tableau/internal/xfs"
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

func TestGenerator_GenAll(t *testing.T) {
	if err := prepareOutput(); err != nil {
		t.Fatalf("failed to create output common dir: %v", err)
	}
	outdir := "./testdata/_conf_gen_all"
	tests := []struct {
		name    string
		gen     *Generator
		wantErr bool
	}{
		{
			name: "test1",
			gen: NewGenerator("protoconf", "../../test/functest/testdata/", outdir,
				options.LocationName("Asia/Shanghai"),
				options.Conf(
					&options.ConfOption{
						Input: &options.ConfInputOption{
							ProtoPaths: []string{"../../test/functest/proto"},
							ProtoFiles: []string{"../../test/functest/proto/*.proto"},
							Formats: []format.Format{
								// format.Excel,
								format.CSV,
								format.XML,
								format.YAML,
							},
							ExcludedProtoFiles: []string{
								"../../test/functest/proto/xml__metasheet__metasheet.proto",
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
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.gen.GenAll(); (err != nil) != tt.wantErr {
				t.Errorf("Generator.GenAll() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
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
			gen: NewGenerator("protoconf", "../../test/functest/testdata/", outdir,
				options.LocationName("Asia/Shanghai"),
				options.Conf(
					&options.ConfOption{
						Input: &options.ConfInputOption{
							ProtoPaths: []string{"../../test/functest/proto"},
							ProtoFiles: []string{"../../test/functest/proto/*.proto"},
							Formats: []format.Format{
								// format.Excel,
								format.CSV,
								format.XML,
							},
							ExcludedProtoFiles: []string{
								"../../test/functest/proto/xml__metasheet__metasheet.proto",
							},
							Subdirs: []string{
								"excel/",
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

func TestGenerator_GenWorkbook_Document(t *testing.T) {
	if err := prepareOutput(); err != nil {
		t.Fatalf("failed to create output common dir: %v", err)
	}
	outdir := "./testdata/_conf_gen_workbook_document"
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
			gen: NewGenerator("protoconf", "../../test/functest/testdata/", outdir,
				options.LocationName("Asia/Shanghai"),
				options.Conf(
					&options.ConfOption{
						Input: &options.ConfInputOption{
							ProtoPaths: []string{"../../test/functest/proto"},
							ProtoFiles: []string{"../../test/functest/proto/*.proto"},
							Formats: []format.Format{
								format.YAML,
							},
							ExcludedProtoFiles: []string{
								"../../test/functest/proto/xml__metasheet__metasheet.proto",
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
