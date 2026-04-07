package protogen

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/tableauio/tableau/internal/importer/book"
	"github.com/tableauio/tableau/internal/importer/book/tableparser"
	"github.com/tableauio/tableau/xerrors"
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

// Test_prepareOutdir_delExisted verifies that prepareOutdir correctly retains
// imported proto files and removes non-imported ones when delExisted is true.
// It covers two scenarios:
//   - Glob pattern in importFiles (e.g. "testdata/output/proto/*.proto")
//   - Relative path with dot-segment in importFiles (e.g. "./testdata/output/proto/common.proto")
func Test_prepareOutdir_delExisted(t *testing.T) {
	const outdir = "testdata/output/proto"

	setup := func(t *testing.T, extraFiles []string) {
		t.Helper()
		for _, f := range extraFiles {
			if err := os.WriteFile(f, []byte("// temp proto\n"), 0644); err != nil {
				t.Fatalf("setup: failed to create temp file %s: %v", f, err)
			}
		}
		t.Cleanup(func() {
			// Restore: remove any leftover temp files so other tests are unaffected.
			for _, f := range extraFiles {
				_ = os.Remove(f)
			}
		})
	}

	t.Run("glob-pattern-keeps-matched-removes-others", func(t *testing.T) {
		// Create two extra proto files; only common.proto is the real import.
		// Using a glob pattern "testdata/output/proto/*.proto" should match ALL
		// proto files in the directory, so none should be deleted.
		extra := []string{
			outdir + "/temp1.proto",
			outdir + "/temp2.proto",
		}
		setup(t, extra)

		err := prepareOutdir(outdir, []string{outdir + "/*.proto"}, true)
		require.NoError(t, err)

		// All proto files (including temp ones) must still exist because they
		// were all matched by the glob pattern.
		for _, f := range append(extra, outdir+"/common.proto") {
			_, statErr := os.Stat(f)
			require.NoError(t, statErr, "file should be retained by glob import: %s", f)
		}
	})

	t.Run("glob-pattern-removes-non-imported", func(t *testing.T) {
		// Import only common.proto via glob; temp files should be deleted.
		extra := []string{
			outdir + "/temp3.proto",
			outdir + "/temp4.proto",
		}
		setup(t, extra)

		err := prepareOutdir(outdir, []string{outdir + "/common.proto"}, true)
		require.NoError(t, err)

		// common.proto must be retained.
		_, statErr := os.Stat(outdir + "/common.proto")
		require.NoError(t, statErr, "imported file should be retained")

		// temp files must have been deleted.
		for _, f := range extra {
			_, statErr := os.Stat(f)
			require.True(t, os.IsNotExist(statErr), "non-imported file should be removed: %s", f)
		}
	})

	t.Run("relative-path-with-dot-segment-keeps-imported", func(t *testing.T) {
		// Use a relative path with a leading "./" — xfs.IsSamePath must resolve
		// it to the same absolute path as the file inside outdir.
		extra := []string{
			outdir + "/temp5.proto",
		}
		setup(t, extra)

		err := prepareOutdir(outdir, []string{"./" + outdir + "/common.proto"}, true)
		require.NoError(t, err)

		// common.proto must be retained despite the dot-segment in importFiles.
		_, statErr := os.Stat(outdir + "/common.proto")
		require.NoError(t, statErr, "imported file with relative path should be retained")

		// temp5.proto must have been deleted (not in importFiles).
		_, statErr = os.Stat(outdir + "/temp5.proto")
		require.True(t, os.IsNotExist(statErr), "non-imported file should be removed")
	})

	t.Run("absolute-path-keeps-imported", func(t *testing.T) {
		// Use an absolute path in importFiles — xfs.IsSamePath must match it
		// against the relative path constructed inside prepareOutdir.
		extra := []string{
			outdir + "/temp6.proto",
		}
		setup(t, extra)

		absCommon, err := filepath.Abs(outdir + "/common.proto")
		require.NoError(t, err)

		err = prepareOutdir(outdir, []string{absCommon}, true)
		require.NoError(t, err)

		// common.proto must be retained when specified as an absolute path.
		_, statErr := os.Stat(outdir + "/common.proto")
		require.NoError(t, statErr, "imported file with absolute path should be retained")

		// temp6.proto must have been deleted (not in importFiles).
		_, statErr = os.Stat(outdir + "/temp6.proto")
		require.True(t, os.IsNotExist(statErr), "non-imported file should be removed")
	})
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
				dir:      `./testdata/output/proto/`,
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

func Test_wrapDebugErr(t *testing.T) {
	testTransposeSheetHeader := &tableHeader{
		Header: &tableparser.Header{
			NameRow: 1,
			TypeRow: 2,
			NoteRow: 3,
		},
		Positioner:  &book.TransposedTable{},
		nameRowData: []string{"ID", "Value", "", "Kind"},
		typeRowData: []string{"map<int32, Item>", "int32", "", "int32"},
		noteRowData: []string{"Item's ID", "Item's value", "", "Item's kind"},
		validNames:  map[string]int{},
	}

	type args struct {
		err       error
		bookName  string
		sheetName string
		sh        *tableHeader
		cursor    int
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
		err     error
	}{
		{
			name: "E0001",
			args: args{
				err:       xerrors.E0001("TestSheet", "TestBook"),
				bookName:  "TestBook",
				sheetName: "TestSheet",
				sh:        testSheetHeader,
				cursor:    0,
			},
			err:     xerrors.ErrE0001,
			wantErr: true,
		},
		{
			name: "E0001 transpose",
			args: args{
				err:       xerrors.E0001("TestSheet", "TestBook"),
				bookName:  "TestBook",
				sheetName: "TestSheet",
				sh:        testTransposeSheetHeader,
				cursor:    0,
			},
			err:     xerrors.ErrE0001,
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := wrapDebugErr(tt.args.err, tt.args.bookName, tt.args.sheetName, tt.args.sh, tt.args.cursor)
			if (err != nil) != tt.wantErr {
				t.Errorf("wrapDebugErr() error = %v, wantErr %v", err, tt.wantErr)
			}
			if err != nil {
				require.ErrorIs(t, err, tt.err)
				desc := xerrors.NewDesc(err)
				require.Equal(t, desc.GetValue(xerrors.KeyBookName), tt.args.bookName)
				require.Equal(t, desc.GetValue(xerrors.KeySheetName), tt.args.sheetName)
			}
		})
	}
}
