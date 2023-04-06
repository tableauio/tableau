package importer

import "testing"

func TestNeedSheet(t *testing.T) {
	type args struct {
		sheetName      string
		wantSheetNames []string
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{
			name: "empty-wantSheetNames",
			args: args{
				sheetName: "Sheet1",
			},
			want: true,
		},
		{
			name: "in-wantSheetNames",
			args: args{
				sheetName: "Sheet1",
				wantSheetNames: []string{"Sheet1", "Sheet2"},
			},
			want: true,
		},
		{
			name: "not-in-wantSheetNames",
			args: args{
				sheetName: "ItemConf",
				wantSheetNames: []string{"Sheet1", "Sheet2"},
			},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := NeedSheet(tt.args.sheetName, tt.args.wantSheetNames); got != tt.want {
				t.Errorf("NeedSheet() = %v, want %v", got, tt.want)
			}
		})
	}
}
