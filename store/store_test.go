package store

import (
	"testing"

	"github.com/tableauio/tableau/format"
	"github.com/tableauio/tableau/proto/tableaupb/unittestpb"
	"google.golang.org/protobuf/proto"
)

func TestStore(t *testing.T) {
	itemConf = &unittestpb.ItemConf{
		ItemMap: map[uint32]*unittestpb.Item{
			1: {Id: 1, Num: 10},
			2: {Id: 2, Num: 20},
			3: {Id: 3, Num: 30},
		},
	}
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
		{
			name: "export-item-conf-json",
			args: args{
				msg: itemConf,
				dir: "_out/",
				fmt: "json",
				options: []Option{
					Pretty(true),
				},
			},
			wantErr: false,
		},
		{
			name: "export-item-conf-json-alias",
			args: args{
				msg: itemConf,
				dir: "_out/subdir/",
				fmt: "json",
				options: []Option{
					Name("ItemConfAlias"),
					UseProtoNames(true),
				},
			},
			wantErr: false,
		},
		{
			name: "export-item-conf-txt",
			args: args{
				msg: itemConf,
				dir: "_out/",
				fmt: "txt",
				options: []Option{
					Pretty(true),
				},
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := Store(tt.args.msg, tt.args.dir, tt.args.fmt, tt.args.options...); (err != nil) != tt.wantErr {
				t.Errorf("Store() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
