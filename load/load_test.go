package load

import (
	"fmt"
	"io"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/tableauio/tableau/format"
	"github.com/tableauio/tableau/proto/tableaupb/unittestpb"
	"github.com/tableauio/tableau/xerrors"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
)

func TestLoad(t *testing.T) {
	type args struct {
		msg     proto.Message
		dir     string
		fmt     format.Format
		options []Option
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
		errcode string
	}{
		{
			name: "load-origin",
			args: args{
				msg: &unittestpb.ItemConf{},
				dir: "../testdata/",
				fmt: format.CSV,
				options: []Option{
					Filter(nil), // just for test coverage
				},
			},
			wantErr: false,
		},
		{
			name: "load-origin-path-failed",
			args: args{
				msg: &unittestpb.ItemConf{},
				dir: "../testdata/",
				fmt: format.CSV,
				options: []Option{
					SubdirRewrites(map[string]string{"unittest": "unittest-invalid-dir"}),
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
				options: []Option{
					SubdirRewrites(map[string]string{
						"unittest/": "testdata/unittest/",
					}),
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
				options: []Option{
					LocationName("Local"),
					IgnoreUnknownFields(),
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
				options: []Option{
					LocationName("Local"),
					IgnoreUnknownFields(),
				},
			},
			wantErr: true,
			errcode: "E0002",
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
				options: []Option{
					LocationName("Local"),
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
				options: []Option{
					Paths(map[string]string{
						"ItemConf": "../testdata/unittest/conf/ItemConf.json",
					}),
				},
			},
			wantErr: false,
		},
		{
			name: "with-paths-bin",
			args: args{
				msg: &unittestpb.ItemConf{},
				dir: "../testdata/",
				fmt: format.JSON,
				options: []Option{
					Paths(map[string]string{
						"ItemConf": "../testdata/unittest/conf/ItemConf.bin",
					}),
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
				options: []Option{
					Paths(map[string]string{
						"ItemConf": "../testdata/unittest/conf/ItemConf-invalid.json",
					}),
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
				options: []Option{
					Paths(map[string]string{
						"ItemConf": "../testdata/unittest/Unittest#ItemConf.csv",
					}),
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
				options: []Option{
					LocationName("Local"),
					WithReadFunc(func(_ string) ([]byte, error) {
						return []byte(`{"itemMap":{"1":{"id":1,"num":100},"2":{"id":2,"num":200},"3":{"id":3,"num":300}}}`), nil
					}),
				},
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := Load(tt.args.msg, tt.args.dir, tt.args.fmt, tt.args.options...)
			if (err != nil) != tt.wantErr {
				t.Errorf("Load() error = %v, wantErr %v", err, tt.wantErr)
			}
			if err != nil {
				if tt.errcode != "" {
					desc := xerrors.NewDesc(err)
					require.Equal(t, tt.errcode, desc.ErrCode())
				}
			}
			// opts := prototext.MarshalOptions{
			// 	Multiline: true,
			// 	Indent:    "    ",
			// }
			// txt, _ := opts.Marshal(tt.args.msg)
			// t.Logf("text: %v", string(txt))
			// json, _ := protojson.Marshal(tt.args.msg)
			// t.Logf("JSON: %v", string(json))
		})
	}
}

func TestLoadJSON_E0002(t *testing.T) {
	err := Load(&unittestpb.ItemConf{}, "../testdata/", format.JSON,
		Paths(map[string]string{
			"ItemConf": "../testdata/unittest/invalidconf/ItemConf.json",
		}),
	)
	require.Error(t, err, "should return an error")
	desc := xerrors.NewDesc(err)
	require.Equal(t, "E0002", desc.ErrCode())
	t.Logf("error: %s", desc.String())

	err = Load(&unittestpb.ItemConf{}, "../testdata/", format.Text,
		Paths(map[string]string{
			"ItemConf": "../testdata/unittest/invalidconf/ItemConf.txt",
		}),
	)
	require.Error(t, err, "should return an error")
	desc = xerrors.NewDesc(err)
	require.Equal(t, "E0002", desc.ErrCode())
	t.Logf("error: %s", desc.String())
}

func TestLoadWithPatch(t *testing.T) {
	type args struct {
		msg     proto.Message
		dir     string
		fmt     format.Format
		options []Option
	}
	tests := []struct {
		name string
		args args
	}{
		{
			name: "PatchDirs-replace",
			args: args{
				msg:     &unittestpb.PatchReplaceConf{},
				dir:     "../testdata/unittest/conf/",
				fmt:     format.JSON,
				options: []Option{PatchDirs("../testdata/unittest/patchconf/")},
			},
		},
		{
			name: "PatchPaths-replace",
			args: args{
				msg:     &unittestpb.PatchReplaceConf{},
				dir:     "../testdata/unittest/conf/",
				fmt:     format.JSON,
				options: []Option{PatchPaths(map[string][]string{"PatchReplaceConf": {"../testdata/unittest/patchconf/PatchReplaceConf.json"}})},
			},
		},
		{
			name: "PatchDirs-replace-not-existed",
			args: args{
				msg:     &unittestpb.PatchReplaceConf{},
				dir:     "../testdata/unittest/conf/",
				fmt:     format.JSON,
				options: []Option{PatchDirs("../testdata/unittest/not-existed/")},
			},
		},
		{
			name: "PatchDirs-merge-none-map",
			args: args{
				msg:     &unittestpb.PatchMergeConf{},
				dir:     "../testdata/unittest/conf/",
				fmt:     format.JSON,
				options: []Option{PatchDirs("../testdata/unittest/patchconf/")},
			},
		},
		{
			name: "PatchPaths-merge-none-map",
			args: args{
				msg:     &unittestpb.PatchMergeConf{},
				dir:     "../testdata/unittest/conf/",
				fmt:     format.JSON,
				options: []Option{PatchPaths(map[string][]string{"PatchMergeConf": {"../testdata/unittest/patchconf/PatchMergeConf.json"}})},
			},
		},
		{
			name: "PatchPaths-with-PatchDirs-merge-none-map",
			args: args{
				msg:     &unittestpb.PatchMergeConf{},
				dir:     "../testdata/unittest/conf/",
				fmt:     format.JSON,
				options: []Option{PatchPaths(map[string][]string{"PatchMergeConf": {"../testdata/unittest/patchconf/PatchMergeConf.json"}}), PatchDirs("../testdata/unittest/patchconf2/")},
			},
		},
		{
			name: "PatchDirs-merge-map",
			args: args{
				msg:     &unittestpb.PatchMergeConf{},
				dir:     "../testdata/unittest/conf/",
				fmt:     format.JSON,
				options: []Option{PatchDirs("../testdata/unittest/patchconf2/")},
			},
		},
		{
			name: "PatchPaths-different-format-merge-map",
			args: args{
				msg:     &unittestpb.PatchMergeConf{},
				dir:     "../testdata/unittest/conf/",
				fmt:     format.JSON,
				options: []Option{PatchPaths(map[string][]string{"PatchMergeConf": {"../testdata/unittest/patchconf2/PatchMergeConf.txt"}})},
			},
		},
		{
			name: "PatchDirs-merge-not-existed",
			args: args{
				msg:     &unittestpb.PatchMergeConf{},
				dir:     "../testdata/unittest/conf/",
				fmt:     format.JSON,
				options: []Option{PatchDirs("../testdata/unittest/not-existed/")},
			},
		},
		{
			name: "Recursive-patch",
			args: args{
				msg:     &unittestpb.RecursivePatchConf{},
				dir:     "../testdata/unittest/conf/",
				fmt:     format.JSON,
				options: []Option{PatchDirs("../testdata/unittest/patchconf/")},
			},
		},
		{
			name: "PatchDirs-merge-multiple-dirs",
			args: args{
				msg:     &unittestpb.PatchMergeConf{},
				dir:     "../testdata/unittest/conf/",
				fmt:     format.JSON,
				options: []Option{PatchDirs("../testdata/unittest/patchconf/", "../testdata/unittest/patchconf2/")},
			},
		},
		{
			name: "PatchPaths-merge-multiple-paths",
			args: args{
				msg:     &unittestpb.PatchMergeConf{},
				dir:     "../testdata/unittest/conf/",
				fmt:     format.JSON,
				options: []Option{PatchPaths(map[string][]string{"PatchMergeConf": {"../testdata/unittest/patchconf/PatchMergeConf.json", "../testdata/unittest/patchconf2/PatchMergeConf.json", "some/path/that/does/not/exist"}})},
			},
		},
		{
			name: "ModeOnlyMain",
			args: args{
				msg:     &unittestpb.PatchMergeConf{},
				dir:     "../testdata/unittest/conf/",
				fmt:     format.JSON,
				options: []Option{PatchPaths(map[string][]string{"PatchMergeConf": {"../testdata/unittest/patchconf/PatchMergeConf.json", "../testdata/unittest/patchconf2/PatchMergeConf.json"}}), Mode(ModeOnlyMain)},
			},
		},
		{
			name: "ModeOnlyPatch",
			args: args{
				msg:     &unittestpb.PatchMergeConf{},
				dir:     "../testdata/unittest/conf/",
				fmt:     format.JSON,
				options: []Option{PatchPaths(map[string][]string{"PatchMergeConf": {"../testdata/unittest/patchconf/PatchMergeConf.json", "../testdata/unittest/patchconf2/PatchMergeConf.json"}}), Mode(ModeOnlyPatch)},
			},
		},
		{
			name: "ModeOnlyPatch-but-no-valid-patch-file",
			args: args{
				msg:     &unittestpb.PatchMergeConf{},
				dir:     "../testdata/unittest/conf/",
				fmt:     format.JSON,
				options: []Option{PatchPaths(map[string][]string{"PatchMergeConf": {"some/path/that/does/not/exist"}}), Mode(ModeOnlyPatch)},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := Load(tt.args.msg, tt.args.dir, tt.args.fmt, tt.args.options...)
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
	err := Load(&unittestpb.ItemConf{}, "../testdata/", format.JSON,
		Paths(map[string]string{
			"ItemConf": "../testdata/unittest/invalidconf/Empty.json",
		}),
	)
	require.Error(t, err, "should return an error")
	desc := xerrors.NewDesc(err)
	require.Equal(t, "E0002", desc.ErrCode())
	require.Contains(t, desc.GetValue(xerrors.KeyReason), fileContentIsEmpty)
	t.Logf("error: %s", desc.String())
}

func TestLoadEmptyText(t *testing.T) {
	err := Load(&unittestpb.ItemConf{}, "../testdata/", format.JSON,
		Paths(map[string]string{
			"ItemConf": "../testdata/unittest/invalidconf/Empty.txt",
		}),
	)
	require.NoError(t, err, "should return no error")
}
