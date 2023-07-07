package confgen

import (
	"fmt"
	"os"
	"testing"

	"github.com/tableauio/tableau/format"
	"github.com/tableauio/tableau/options"
)

func prepareOutput() error {
	// prepare output common dir
	outdir := "./testdata/_conf"
	err := os.MkdirAll(outdir, 0700)
	if err != nil {
		return fmt.Errorf("failed to create output dir: %v", err)
	}
	return nil
}

func TestGenerator_GenAll(t *testing.T) {
	if err := prepareOutput(); err != nil {
		t.Fatalf("failed to create output common dir: %v", err)
	}
	outdir := "./testdata/_conf"
	tests := []struct {
		name    string
		gen     *Generator
		wantErr bool
	}{
		{
			name: "test1",
			gen: NewGenerator("unittest", "../../testdata/", outdir,
				options.LocationName("Asia/Shanghai"),
				options.Conf(
					&options.ConfOption{
						Input: &options.ConfInputOption{
							ProtoPaths: []string{"../../proto/"},
							ProtoFiles: []string{"../../proto/tableau/protobuf/unittest/unittest.proto"},
							Formats: []format.Format{
								format.CSV,
							},
						},
						Output: &options.ConfOutputOption{
							Pretty:          true,
							Formats:         []format.Format{format.JSON},
							EmitUnpopulated: true,
						},
					},
				),
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
