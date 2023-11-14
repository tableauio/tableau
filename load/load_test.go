package load

import (
	"testing"

	"github.com/tableauio/tableau/format"
	"github.com/tableauio/tableau/proto/tableaupb/unittestpb"
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
	}{
		// TODO: Add test cases.
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
			name: "load-origin-with-sudir-rewrites",
			args: args{
				msg:     &unittestpb.ItemConf{},
				dir:     "../",
				fmt:     format.CSV,
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
			if err := Load(tt.args.msg, tt.args.dir, tt.args.fmt, tt.args.options...); (err != nil) != tt.wantErr {
				t.Errorf("Load() error = %v, wantErr %v", err, tt.wantErr)
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
