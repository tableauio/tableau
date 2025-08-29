package load

import (
	"reflect"
	"testing"

	"google.golang.org/protobuf/proto"
)

func TestOptions_ParseMessagerOptionsByName(t *testing.T) {
	type args struct {
		o    []Option
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
				o: []Option{
					IgnoreUnknownFields(),
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
				o: []Option{
					IgnoreUnknownFields(),
					WithMessagerOptions(map[string]*MessagerOptions{
						"ItemConf": {
							BaseOptions: BaseOptions{
								IgnoreUnknownFields: proto.Bool(false),
							},
						},
					}),
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
			opts := ParseOptions(tt.args.o...)
			if got := opts.ParseMessagerOptionsByName(tt.args.name); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Options.ParseMessagerOptionsByName() = %v, want %v", got, tt.want)
			}
		})
	}
}
