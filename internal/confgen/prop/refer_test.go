package prop

import (
	"reflect"
	"testing"
)

func Test_parseRefer(t *testing.T) {
	type args struct {
		text string
	}
	tests := []struct {
		name    string
		args    args
		want    *ReferInfo
		wantErr bool
	}{
		// TODO: Add test cases.
		{
			name: "without alias",
			args: args{
				text: "Item.ID",
			},
			want: &ReferInfo{"Item", "", "ID"},
		},
		{
			name: "with alias",
			args: args{
				text: "Item(ItemConf).ID",
			},
			want: &ReferInfo{"Item", "(ItemConf)", "ID"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseRefer(tt.args.text)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseRefer() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("parseRefer() = %v, want %v", got, tt.want)
			}
		})
	}
}
