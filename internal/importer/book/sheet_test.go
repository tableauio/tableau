package book

import (
	"reflect"
	"testing"
)

func Test_parseIndexes(t *testing.T) {
	type args struct {
		str string
	}
	tests := []struct {
		name string
		args args
		want []string
	}{
		{
			name: "empty index",
			args: args{
				str: "",
			},
			want: nil,
		},
		{
			name: "one single-column index",
			args: args{
				str: "ID ",
			},
			want: []string{"ID"},
		},
		{
			name: "many single-column indexes",
			args: args{
				str: " ID, Type ",
			},
			want: []string{"ID", "Type"},
		},
		{
			name: "one multi-column index",
			args: args{
				str: "(ID, Type)",
			},
			want: []string{"(ID, Type)"},
		},
		{
			name: "one named multi-column index",
			args: args{
				str: "(ID, Type)@Item",
			},
			want: []string{"(ID, Type)@Item"},
		},
		{
			name: "one single-column index and one multi-column index with extra space separated",
			args: args{
				str: "ID, (ID, Type)",
			},
			want: []string{"ID", "(ID, Type)"},
		},
		{
			name: "one single-column index and one named multi-column index",
			args: args{
				str: "ID,(ID, Type)@Item",
			},
			want: []string{"ID", "(ID, Type)@Item"},
		},
		{
			name: "one single-column index and one named multi-column index with extra space separated",
			args: args{
				str: "ID, (ID, Type)@Item",
			},
			want: []string{"ID", "(ID, Type)@Item"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := parseIndexes(tt.args.str); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("parseIndexes() = %v, want %v", got, tt.want)
			}
		})
	}
}
