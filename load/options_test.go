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
		{
			name: "max-errors-per-sheet inherits global",
			args: args{
				o: []Option{
					MaxErrorsPerSheet(5),
				},
				name: "ItemConf",
			},
			want: &MessagerOptions{
				BaseOptions: BaseOptions{
					MaxErrorsPerSheet: 5,
				},
			},
		},
		{
			name: "max-errors-per-sheet messager overrides global",
			args: args{
				o: []Option{
					MaxErrorsPerSheet(5),
					WithMessagerOptions(map[string]*MessagerOptions{
						"ItemConf": {
							BaseOptions: BaseOptions{
								MaxErrorsPerSheet: 10,
							},
						},
					}),
				},
				name: "ItemConf",
			},
			want: &MessagerOptions{
				BaseOptions: BaseOptions{
					MaxErrorsPerSheet: 10,
				},
			},
		},
		{
			name: "max-errors-per-sheet messager unset falls back to global",
			args: args{
				o: []Option{
					MaxErrorsPerSheet(5),
					WithMessagerOptions(map[string]*MessagerOptions{
						"ItemConf": {
							BaseOptions: BaseOptions{
								IgnoreUnknownFields: proto.Bool(true),
							},
						},
					}),
				},
				name: "ItemConf",
			},
			want: &MessagerOptions{
				BaseOptions: BaseOptions{
					IgnoreUnknownFields: proto.Bool(true),
					MaxErrorsPerSheet:   5,
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts := ParseOptions(tt.args.o...)
			got := opts.ParseMessagerOptionsByName(tt.args.name)
			want := *tt.want
			want.referredCache = opts.referredCache
			if !reflect.DeepEqual(got, &want) {
				t.Errorf("Options.ParseMessagerOptionsByName() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestOptions_ReferredCacheScope(t *testing.T) {
	opts := ParseOptions()
	if opts.referredCache == nil {
		t.Fatal("ParseOptions() referredCache = nil")
	}

	itemOpts := opts.ParseMessagerOptionsByName("ItemConf")
	mallOpts := opts.ParseMessagerOptionsByName("MallConf")
	if itemOpts.referredCache != opts.referredCache || mallOpts.referredCache != opts.referredCache {
		t.Fatal("messager options do not share their parent Options cache")
	}

	otherOpts := ParseOptions().ParseMessagerOptionsByName("ItemConf")
	if itemOpts.referredCache == otherOpts.referredCache {
		t.Fatal("separate Options instances share a referred cache")
	}
}

func TestMaxErrorsPerSheetOption(t *testing.T) {
	tests := []struct {
		name string
		n    int
		want int
	}{
		{name: "negative", n: -3, want: -3},
		{name: "zero", n: 0, want: 0},
		{name: "one", n: 1, want: 1},
		{name: "many", n: 42, want: 42},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts := ParseOptions(MaxErrorsPerSheet(tt.n))
			if opts.MaxErrorsPerSheet != tt.want {
				t.Errorf("MaxErrorsPerSheet(%d) => opts.MaxErrorsPerSheet = %d, want %d", tt.n, opts.MaxErrorsPerSheet, tt.want)
			}
		})
	}
}

func TestMessagerOptions_GetMaxErrorsPerSheet(t *testing.T) {
	tests := []struct {
		name string
		o    *MessagerOptions
		want int
	}{
		{
			name: "nil receiver => fail-fast (1)",
			o:    nil,
			want: 1,
		},
		{
			name: "zero (default) => fail-fast (1)",
			o:    &MessagerOptions{},
			want: 1,
		},
		{
			name: "negative => fail-fast (1)",
			o: &MessagerOptions{
				BaseOptions: BaseOptions{MaxErrorsPerSheet: -10},
			},
			want: 1,
		},
		{
			name: "explicit 1 => fail-fast (1)",
			o: &MessagerOptions{
				BaseOptions: BaseOptions{MaxErrorsPerSheet: 1},
			},
			want: 1,
		},
		{
			name: "aggregate N",
			o: &MessagerOptions{
				BaseOptions: BaseOptions{MaxErrorsPerSheet: 100},
			},
			want: 100,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.o.GetMaxErrorsPerSheet(); got != tt.want {
				t.Errorf("GetMaxErrorsPerSheet() = %d, want %d", got, tt.want)
			}
		})
	}
}
