package load

import (
	"fmt"
	"io"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/tableauio/tableau/format"
	"github.com/tableauio/tableau/internal/testutil"
	"github.com/tableauio/tableau/proto/tableaupb/unittestpb"
	"github.com/tableauio/tableau/xerrors"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
)

func mode(v LoadMode) *LoadMode { return &v }

func TestLoad(t *testing.T) {
	type args struct {
		msg     proto.Message
		dir     string
		fmt     format.Format
		options *MessagerOptions
	}
	tests := []struct {
		name    string
		args    args
		wantMsg proto.Message
		wantErr bool
		err     error
	}{
		{
			name: "nil",
			args: args{
				msg:     &unittestpb.ItemConf{},
				dir:     "../testdata/",
				fmt:     format.CSV,
				options: nil,
			},
			wantErr: false,
		},
		{
			name: "load-origin",
			args: args{
				msg:     &unittestpb.ItemConf{},
				dir:     "../testdata/",
				fmt:     format.CSV,
				options: &MessagerOptions{},
			},
			wantErr: false,
		},
		{
			name: "load-origin-path-failed",
			args: args{
				msg: &unittestpb.ItemConf{},
				dir: "../testdata/",
				fmt: format.CSV,
				options: &MessagerOptions{
					BaseOptions: BaseOptions{
						SubdirRewrites: map[string]string{"unittest": "unittest-invalid-dir"},
					},
				},
			},
			wantErr: true,
		},
		{
			name: "load-origin-with-sudir-rewrites",
			args: args{
				msg: &unittestpb.ItemConf{},
				dir: "../",
				fmt: format.CSV,
				options: &MessagerOptions{
					BaseOptions: BaseOptions{
						SubdirRewrites: map[string]string{"unittest/": "testdata/unittest/"},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "specified-json-format",
			args: args{
				msg: &unittestpb.ItemConf{},
				dir: "../testdata/unittest/conf/",
				fmt: format.JSON,
				options: &MessagerOptions{
					BaseOptions: BaseOptions{
						LocationName:        "Local",
						IgnoreUnknownFields: proto.Bool(true),
					},
				},
			},
			wantErr: false,
		},
		{
			name: "specified-json-format-invalid-syntax",
			args: args{
				msg: &unittestpb.ItemConf{},
				dir: "../testdata/unittest/invalidconf/",
				fmt: format.JSON,
				options: &MessagerOptions{
					BaseOptions: BaseOptions{
						LocationName:        "Local",
						IgnoreUnknownFields: proto.Bool(true),
					},
				},
			},
			wantErr: true,
			err:     xerrors.ErrE0002,
		},
		{
			name: "specified-text-format",
			args: args{
				msg: &unittestpb.ItemConf{},
				dir: "../testdata/unittest/conf/",
				fmt: format.Text,
			},
			wantErr: false,
		},
		{
			name: "specified-bin-format",
			args: args{
				msg: &unittestpb.ItemConf{},
				dir: "../testdata/unittest/conf/",
				fmt: format.Bin,
				options: &MessagerOptions{
					BaseOptions: BaseOptions{
						LocationName: "Local",
					},
				},
			},
			wantErr: false,
		},
		{
			name: "with-paths-json",
			args: args{
				msg: &unittestpb.ItemConf{},
				dir: "../testdata/",
				fmt: format.JSON,
				options: &MessagerOptions{
					Path: "../testdata/unittest/conf/ItemConf.json",
				},
			},
			wantMsg: &unittestpb.ItemConf{
				ItemMap: map[uint32]*unittestpb.Item{
					1: {
						Id:  1,
						Num: 100,
					},
					2: {
						Id:  2,
						Num: 200,
					},
					3: {
						Id:  3,
						Num: 300,
					},
				},
			},
		},
		{
			name: "with-paths-bin",
			args: args{
				msg: &unittestpb.ItemConf{},
				dir: "../testdata/",
				fmt: format.JSON,
				options: &MessagerOptions{
					Path: "../testdata/unittest/conf/ItemConf.binpb",
				},
			},
			wantErr: false,
		},
		{
			name: "invalid-paths-with-paths",
			args: args{
				msg: &unittestpb.ItemConf{},
				dir: "../testdata/",
				fmt: format.JSON,
				options: &MessagerOptions{
					Path: "../testdata/unittest/conf/ItemConf-invalid.json",
				},
			},
			wantErr: true,
		},
		{
			name: "invalid-formats-with-paths",
			args: args{
				msg: &unittestpb.ItemConf{},
				dir: "../testdata/",
				fmt: format.JSON,
				options: &MessagerOptions{
					Path: "../testdata/unittest/Unittest#ItemConf.csv",
				},
			},
			wantErr: true,
		},
		{
			name: "with-read-func",
			args: args{
				msg: &unittestpb.ItemConf{},
				dir: "../testdata/unittest/conf/",
				fmt: format.JSON,
				options: &MessagerOptions{
					BaseOptions: BaseOptions{
						ReadFunc: func(_ string) ([]byte, error) {
							return []byte(`{"itemMap":{"10":{"id":10,"num":100},"20":{"id":20,"num":200},"30":{"id":30,"num":300}}}`), nil
						},
					},
				},
			},
			wantMsg: &unittestpb.ItemConf{
				ItemMap: map[uint32]*unittestpb.Item{
					10: {
						Id:  10,
						Num: 100,
					},
					20: {
						Id:  20,
						Num: 200,
					},
					30: {
						Id:  30,
						Num: 300,
					},
				},
			},
		},
		{
			name: "with-load-func",
			args: args{
				msg: &unittestpb.ItemConf{},
				dir: "../testdata/unittest/conf/",
				fmt: format.JSON,
				options: &MessagerOptions{
					BaseOptions: BaseOptions{
						LoadFunc: func(msg proto.Message, path string, fmt format.Format, opts *MessagerOptions) error {
							bytes := []byte(`{"itemMap":{"10":{"id":10,"num":100},"20":{"id":20,"num":200},"30":{"id":30,"num":300}}}`)
							return Unmarshal(bytes, msg, path, fmt, opts)
						},
					},
				},
			},
			wantMsg: &unittestpb.ItemConf{
				ItemMap: map[uint32]*unittestpb.Item{
					10: {
						Id:  10,
						Num: 100,
					},
					20: {
						Id:  20,
						Num: 200,
					},
					30: {
						Id:  30,
						Num: 300,
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := LoadMessagerInDir(tt.args.msg, tt.args.dir, tt.args.fmt, tt.args.options)
			if (err != nil) != tt.wantErr {
				t.Errorf("Load() error = %v, wantErr %v", err, tt.wantErr)
			}
			if err != nil {
				if tt.err != nil {
					require.ErrorIs(t, err, tt.err)
				}
			} else {
				if tt.wantMsg != nil {
					testutil.AssertProtoJSONEq(t, tt.args.msg, tt.wantMsg)
				}
			}
		})
	}
}

func TestLoadJSON_E0002(t *testing.T) {
	err := LoadMessagerInDir(&unittestpb.ItemConf{}, "../testdata/", format.JSON,
		&MessagerOptions{
			Path: "../testdata/unittest/invalidconf/ItemConf.json",
		},
	)
	require.Error(t, err, "should return an error")
	require.ErrorIs(t, err, xerrors.ErrE0002)
	t.Logf("error: %s", xerrors.NewDesc(err).String())

	err = LoadMessagerInDir(&unittestpb.ItemConf{}, "../testdata/", format.Text,
		&MessagerOptions{
			Path: "../testdata/unittest/invalidconf/ItemConf.txtpb",
		},
	)
	require.Error(t, err, "should return an error")
	require.ErrorIs(t, err, xerrors.ErrE0002)
	t.Logf("error: %s", xerrors.NewDesc(err).String())
}

func TestLoadWithPatch(t *testing.T) {
	type args struct {
		msg     proto.Message
		dir     string
		fmt     format.Format
		options *MessagerOptions
	}
	tests := []struct {
		name string
		args args
	}{
		{
			name: "PatchDirs-replace",
			args: args{
				msg: &unittestpb.PatchReplaceConf{},
				dir: "../testdata/unittest/conf/",
				fmt: format.JSON,
				options: &MessagerOptions{
					BaseOptions: BaseOptions{
						PatchDirs: []string{"../testdata/unittest/patchconf/"},
					},
				},
			},
		},
		{
			name: "PatchPaths-replace",
			args: args{
				msg: &unittestpb.PatchReplaceConf{},
				dir: "../testdata/unittest/conf/",
				fmt: format.JSON,
				options: &MessagerOptions{
					PatchPaths: []string{"../testdata/unittest/patchconf/PatchReplaceConf.json"},
				},
			},
		},
		{
			name: "PatchDirs-replace-not-existed",
			args: args{
				msg: &unittestpb.PatchReplaceConf{},
				dir: "../testdata/unittest/conf/",
				fmt: format.JSON,
				options: &MessagerOptions{
					BaseOptions: BaseOptions{
						PatchDirs: []string{"../testdata/unittest/not-existed/"},
					},
				},
			},
		},
		{
			name: "PatchDirs-merge-none-map",
			args: args{
				msg: &unittestpb.PatchMergeConf{},
				dir: "../testdata/unittest/conf/",
				fmt: format.JSON,
				options: &MessagerOptions{
					BaseOptions: BaseOptions{
						PatchDirs: []string{"../testdata/unittest/patchconf/"},
					},
				},
			},
		},
		{
			name: "PatchPaths-merge-none-map",
			args: args{
				msg: &unittestpb.PatchMergeConf{},
				dir: "../testdata/unittest/conf/",
				fmt: format.JSON,
				options: &MessagerOptions{
					PatchPaths: []string{"../testdata/unittest/patchconf/PatchMergeConf.json"},
				},
			},
		},
		{
			name: "PatchPaths-with-PatchDirs-merge-none-map",
			args: args{
				msg: &unittestpb.PatchMergeConf{},
				dir: "../testdata/unittest/conf/",
				fmt: format.JSON,
				options: &MessagerOptions{
					BaseOptions: BaseOptions{
						PatchDirs: []string{"../testdata/unittest/patchconf2/"},
					},
					PatchPaths: []string{"../testdata/unittest/patchconf/PatchMergeConf.json"},
				},
			},
		},
		{
			name: "PatchDirs-merge-map",
			args: args{
				msg: &unittestpb.PatchMergeConf{},
				dir: "../testdata/unittest/conf/",
				fmt: format.JSON,
				options: &MessagerOptions{
					BaseOptions: BaseOptions{
						PatchDirs: []string{"../testdata/unittest/patchconf2/"},
					},
				},
			},
		},
		{
			name: "PatchPaths-different-format-merge-map",
			args: args{
				msg: &unittestpb.PatchMergeConf{},
				dir: "../testdata/unittest/conf/",
				fmt: format.JSON,
				options: &MessagerOptions{
					PatchPaths: []string{"../testdata/unittest/patchconf2/PatchMergeConf.txtpb"},
				},
			},
		},
		{
			name: "PatchDirs-merge-not-existed",
			args: args{
				msg: &unittestpb.PatchMergeConf{},
				dir: "../testdata/unittest/conf/",
				fmt: format.JSON,
				options: &MessagerOptions{
					BaseOptions: BaseOptions{
						PatchDirs: []string{"../testdata/unittest/not-existed/"},
					},
				},
			},
		},
		{
			name: "Recursive-patch",
			args: args{
				msg: &unittestpb.RecursivePatchConf{},
				dir: "../testdata/unittest/conf/",
				fmt: format.JSON,
				options: &MessagerOptions{
					BaseOptions: BaseOptions{
						PatchDirs: []string{"../testdata/unittest/patchconf/"},
					},
				},
			},
		},
		{
			name: "PatchDirs-merge-multiple-dirs",
			args: args{
				msg: &unittestpb.PatchMergeConf{},
				dir: "../testdata/unittest/conf/",
				fmt: format.JSON,
				options: &MessagerOptions{
					BaseOptions: BaseOptions{
						PatchDirs: []string{
							"../testdata/unittest/patchconf/",
							"../testdata/unittest/patchconf2/",
						},
					},
				},
			},
		},
		{
			name: "PatchPaths-merge-multiple-paths",
			args: args{
				msg: &unittestpb.PatchMergeConf{},
				dir: "../testdata/unittest/conf/",
				fmt: format.JSON,
				options: &MessagerOptions{
					PatchPaths: []string{
						"../testdata/unittest/patchconf/PatchMergeConf.json",
						"../testdata/unittest/patchconf2/PatchMergeConf.json",
						"some/path/that/does/not/exist",
					},
				},
			},
		},
		{
			name: "ModeOnlyMain",
			args: args{
				msg: &unittestpb.PatchMergeConf{},
				dir: "../testdata/unittest/conf/",
				fmt: format.JSON,
				options: &MessagerOptions{
					BaseOptions: BaseOptions{
						Mode: mode(ModeOnlyMain),
					},
					PatchPaths: []string{
						"../testdata/unittest/patchconf/PatchMergeConf.json",
						"../testdata/unittest/patchconf2/PatchMergeConf.json",
					},
				},
			},
		},
		{
			name: "ModeOnlyPatch",
			args: args{
				msg: &unittestpb.PatchMergeConf{},
				dir: "../testdata/unittest/conf/",
				fmt: format.JSON,
				options: &MessagerOptions{
					BaseOptions: BaseOptions{
						Mode: mode(ModeOnlyPatch),
					},
					PatchPaths: []string{
						"../testdata/unittest/patchconf/PatchMergeConf.json",
						"../testdata/unittest/patchconf2/PatchMergeConf.json",
					},
				},
			},
		},
		{
			name: "ModeOnlyPatch-but-no-valid-patch-file",
			args: args{
				msg: &unittestpb.PatchMergeConf{},
				dir: "../testdata/unittest/conf/",
				fmt: format.JSON,
				options: &MessagerOptions{
					BaseOptions: BaseOptions{
						Mode: mode(ModeOnlyPatch),
					},
					PatchPaths: []string{"some/path/that/does/not/exist"},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := LoadMessagerInDir(tt.args.msg, tt.args.dir, tt.args.fmt, tt.args.options)
			require.NoError(t, err)
			json, err := protojson.Marshal(tt.args.msg)
			require.NoError(t, err)
			t.Logf("JSON: %v", string(json))
			f, err := os.Open(fmt.Sprintf("../testdata/unittest/patchresult/%s.json", tt.name))
			require.NoError(t, err)
			wantJson, err := io.ReadAll(f)
			require.NoError(t, err)
			require.JSONEqf(t, string(wantJson), string(json), "%s: patch result not same.", tt.args.msg.ProtoReflect().Descriptor().FullName())
		})
	}
}

func TestLoadEmptyJSON_E0002(t *testing.T) {
	err := LoadMessagerInDir(&unittestpb.ItemConf{}, "../testdata/", format.JSON,
		&MessagerOptions{
			Path: "../testdata/unittest/invalidconf/Empty.json",
		},
	)
	require.ErrorIs(t, err, xerrors.ErrE0002)
	require.Contains(t, xerrors.NewDesc(err).GetValue(xerrors.KeyReason), fileContentIsEmpty)
}

func TestLoadEmptyText(t *testing.T) {
	err := LoadMessagerInDir(&unittestpb.ItemConf{}, "../testdata/", format.JSON,
		&MessagerOptions{
			Path: "../testdata/unittest/invalidconf/Empty.txtpb",
		},
	)
	require.NoError(t, err, "should return no error")
}
