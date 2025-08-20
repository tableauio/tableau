package load

import (
	"reflect"
	"testing"

	"google.golang.org/protobuf/proto"
)

func TestOptions_ParseMessagerOptionsByName(t *testing.T) {
	type args struct {
		o    *Options
		name string
	}
	tests := []struct {
		name string
		args args
		want *MessagerOptions
	}{
		{
			name: "nil",
			args: args{
				o:    nil,
				name: "ItemConf",
			},
			want: &MessagerOptions{},
		},
		{
			name: "base",
			args: args{
				o: &Options{
					BaseOptions: BaseOptions{
						IgnoreUnknownFields: proto.Bool(true),
					},
				},
				name: "ItemConf",
			},
			want: &MessagerOptions{
				BaseOptions: BaseOptions{
					IgnoreUnknownFields: proto.Bool(true),
				},
			},
		},
		{
			name: "override",
			args: args{
				o: &Options{
					BaseOptions: BaseOptions{
						IgnoreUnknownFields: proto.Bool(true),
					},
					MessagerOptions: map[string]*MessagerOptions{
						"ItemConf": {
							BaseOptions: BaseOptions{
								IgnoreUnknownFields: proto.Bool(false),
							},
						},
					},
				},
				name: "ItemConf",
			},
			want: &MessagerOptions{
				BaseOptions: BaseOptions{
					IgnoreUnknownFields: proto.Bool(false),
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.args.o.ParseMessagerOptionsByName(tt.args.name); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Options.ParseMessagerOptionsByName() = %v, want %v", got, tt.want)
			}
		})
	}
}
