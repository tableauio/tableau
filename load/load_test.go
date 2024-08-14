package load

import (
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
				msg:     &unittestpb.ItemConf{},
				dir:     "../testdata/",
				fmt:     format.CSV,
				options: []Option{},
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
		name     string
		args     args
		wantJson string
	}{
		{
			name: "replace",
			args: args{
				msg:     &unittestpb.PatchReplaceConf{},
				dir:     "../testdata/unittest/conf/",
				fmt:     format.JSON,
				options: []Option{PatchDir("../testdata/unittest/patchconf/")},
			},
			wantJson: `{"name":"orange", "priceList":[20, 200]}`,
		},
		{
			name: "merge-none-map",
			args: args{
				msg:     &unittestpb.PatchMergeConf{},
				dir:     "../testdata/unittest/conf/",
				fmt:     format.JSON,
				options: []Option{PatchDir("../testdata/unittest/patchconf/")},
			},
			wantJson: `{"name":"orange", "priceList":[20, 200], "itemMap":{"1":{"id":1, "num":10}, "2":{"id":2, "num":20}}}`,
		},
		{
			name: "merge-map",
			args: args{
				msg:     &unittestpb.PatchMergeConf{},
				dir:     "../testdata/unittest/conf/",
				fmt:     format.JSON,
				options: []Option{PatchDir("../testdata/unittest/patchconf2/")},
			},
			wantJson: `{"name":"apple", "priceList":[10, 100], "itemMap":{"1":{"id":1, "num":99}, "2":{"id":2, "num":20}, "999":{"id":999, "num":99900}}}`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := Load(tt.args.msg, tt.args.dir, tt.args.fmt, tt.args.options...)
			require.NoError(t, err)
			json, err := protojson.Marshal(tt.args.msg)
			require.NoError(t, err)
			// t.Logf("JSON: %v", string(json))
			require.JSONEqf(t, string(json), tt.wantJson, "%s: patch result not same.", tt.args.msg.ProtoReflect().Descriptor().FullName())
		})
	}
}
