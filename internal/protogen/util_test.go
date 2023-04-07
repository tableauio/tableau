package protogen

import (
	"testing"

	"github.com/tableauio/tableau/options"
	"github.com/tableauio/tableau/proto/tableaupb"
	"google.golang.org/protobuf/proto"
)

func Test_prepareOutdir(t *testing.T) {
	type args struct {
		outdir      string
		importFiles []string
		delExisted  bool
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name: "new-outdir",
			args: args{
				outdir:      "testdata/_output/path/to/dir",
				importFiles: []string{},
				delExisted:  true,
			},
			wantErr: false,
		},
		{
			name: "existed-outdir",
			args: args{
				outdir:      "testdata/output/proto",
				importFiles: []string{"testdata/output/proto/common.proto"},
				delExisted:  false,
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := prepareOutdir(tt.args.outdir, tt.args.importFiles, tt.args.delExisted); (err != nil) != tt.wantErr {
				t.Errorf("prepareOutdir() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func Test_getRelCleanSlashPath(t *testing.T) {
	type args struct {
		rootdir  string
		dir      string
		filename string
	}
	tests := []struct {
		name    string
		args    args
		want    string
		wantErr bool
	}{
		{
			name: "relative-clean-slash-path",
			args: args{
				rootdir:  "testdata",
				dir:      `./testdata/output\proto/`,
				filename: "common.proto",
			},
			want:    "output/proto/common.proto",
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := getRelCleanSlashPath(tt.args.rootdir, tt.args.dir, tt.args.filename)
			if (err != nil) != tt.wantErr {
				t.Errorf("getRelCleanSlashPath() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("getRelCleanSlashPath() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_mergeHeaderOptions(t *testing.T) {
	type args struct {
		sheetMeta *tableaupb.Metasheet
		headerOpt *options.HeaderOption
	}
	tests := []struct {
		name string
		args args
	}{
		{
			name: "merge-header-options",
			args: args{
				sheetMeta: &tableaupb.Metasheet{
					Namerow: 2,
				},
				headerOpt: &options.HeaderOption{
					Namerow:  1,
					Typerow:  2,
					Noterow:  3,
					Datarow:  4,
					Nameline: 1,
					Typeline: 2,
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mergeHeaderOptions(tt.args.sheetMeta, tt.args.headerOpt)
			wantSheetMeta := &tableaupb.Metasheet{
				Namerow:  2,
				Typerow:  2,
				Noterow:  3,
				Datarow:  4,
				Nameline: 1,
				Typeline: 2,
			}
			if !proto.Equal(wantSheetMeta, tt.args.sheetMeta) {
				t.Errorf("mergeHeaderOptions() output %v, want %v", tt.args.sheetMeta, wantSheetMeta)
			}
		})
	}
}

func Test_genProtoFilePath(t *testing.T) {
	type args struct {
		bookName string
		suffix   string
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{
			name: "merge-header-options",
			args: args{
				bookName: "item",
				suffix:   "_conf",
			},
			want: "item_conf.proto",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := genProtoFilePath(tt.args.bookName, tt.args.suffix); got != tt.want {
				t.Errorf("genProtoFilePath() = %v, want %v", got, tt.want)
			}
		})
	}
}
