package load

import (
	"testing"

	"github.com/tableauio/tableau/format"
	"github.com/tableauio/tableau/proto/tableaupb/unittestpb"
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
	}{
		// TODO: Add test cases.
		{
			name: "test1",
			args: args{
				msg:     &unittestpb.ItemConf{},
				dir:     "../testdata/",
				fmt:     format.CSV,
				options: []Option{},
			},
			wantErr: false,
		},
		{
			name: "with-paths",
			args: args{
				msg: &unittestpb.ItemConf{},
				dir: "../testdata/",
				fmt: format.CSV,
				options: []Option{
					Paths(map[string]string{
						"ItemConf": "../testdata/unittest/conf/ItemConf.json",
					}),
				},
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := Load(tt.args.msg, tt.args.dir, tt.args.fmt, tt.args.options...); (err != nil) != tt.wantErr {
				t.Errorf("Load() error = %v, wantErr %v", err, tt.wantErr)
			}
			v, _ := protojson.Marshal(tt.args.msg)
			t.Logf("msg: %v", string(v))
		})
	}
}