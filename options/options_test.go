package options

import (
	"testing"

	"github.com/tableauio/tableau/format"
)

func TestConfOutputOption_NeedBinpb(t *testing.T) {
	tests := []struct {
		name string
		opt  *ConfOutputOption
		want bool
	}{
		{
			name: "empty formats defaults to all formats including binpb",
			opt:  &ConfOutputOption{},
			want: true,
		},
		{
			name: "formats contains binpb",
			opt:  &ConfOutputOption{Formats: []format.Format{format.JSON, format.Bin}},
			want: true,
		},
		{
			name: "formats is json only",
			opt:  &ConfOutputOption{Formats: []format.Format{format.JSON}},
			want: false,
		},
		{
			name: "formats is txtpb only",
			opt:  &ConfOutputOption{Formats: []format.Format{format.Text}},
			want: false,
		},
		{
			name: "formats is json and txtpb without binpb",
			opt:  &ConfOutputOption{Formats: []format.Format{format.JSON, format.Text}},
			want: false,
		},
		{
			name: "formats has no binpb but messagerFormats does",
			opt: &ConfOutputOption{
				Formats:          []format.Format{format.JSON},
				MessagerFormats:  map[string][]format.Format{"ItemConf": {format.Bin}},
			},
			want: true,
		},
		{
			name: "neither formats nor messagerFormats has binpb",
			opt: &ConfOutputOption{
				Formats:          []format.Format{format.JSON},
				MessagerFormats:  map[string][]format.Format{"ItemConf": {format.Text}},
			},
			want: false,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.opt.NeedBinpb(); got != tc.want {
				t.Errorf("NeedBinpb() = %v, want %v", got, tc.want)
			}
		})
	}
}
