package book

import (
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/tableauio/tableau/proto/tableaupb"
	"github.com/tableauio/tableau/proto/tableaupb/internalpb"
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

func TestSheet_GetDataName(t *testing.T) {
	tests := []struct {
		name string
		s    *Sheet
		want string
	}{
		{
			name: "TableSheet",
			s:    NewTableSheet("TableSheet", nil),
			want: "TableSheet",
		},
		{
			name: "DocumentSheet",
			s: NewDocumentSheet("DocumentSheet", &Node{
				Name: "DocumentSheet",
			}),
			want: "DocumentSheet",
		},
		{
			name: "@DocumentSheet",
			s: NewDocumentSheet("@DocumentSheet", &Node{
				Name: "@DocumentSheet",
			}),
			want: "DocumentSheet",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.s.GetDataName(); got != tt.want {
				t.Errorf("Sheet.GetDataName() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSheet_GetDebugName(t *testing.T) {
	tests := []struct {
		name string
		s    *Sheet
		want string
	}{
		{
			name: "basic",
			s: &Sheet{
				Name: "Sheet",
				Meta: &internalpb.Metasheet{},
			},
			want: "Sheet",
		},
		{
			name: "with-alias",
			s: &Sheet{
				Name: "Sheet",
				Meta: &internalpb.Metasheet{Alias: "AliasSheet"},
			},
			want: "Sheet (alias: AliasSheet)",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.s.GetDebugName(); got != tt.want {
				t.Errorf("Sheet.GetDebugName() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSheet_GetProtoName(t *testing.T) {
	tests := []struct {
		name string
		s    *Sheet
		want string
	}{
		{
			name: "basic",
			s: &Sheet{
				Name: "Sheet",
				Meta: &internalpb.Metasheet{},
			},
			want: "Sheet",
		},
		{
			name: "with-alias",
			s: &Sheet{
				Name: "Sheet",
				Meta: &internalpb.Metasheet{Alias: "AliasSheet"},
			},
			want: "AliasSheet",
		},
		{
			name: "@DocumentSheet",
			s: &Sheet{
				Name:     "Sheet",
				Document: &Node{Name: "@DocumentSheet"},
				Meta:     &internalpb.Metasheet{},
			},
			want: "DocumentSheet",
		},
		{
			name: "@DocumentSheet with alias",
			s: &Sheet{
				Name:     "Sheet",
				Document: &Node{Name: "@DocumentSheet"},
				Meta:     &internalpb.Metasheet{Alias: "AliasSheet"},
			},
			want: "AliasSheet",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.s.GetProtoName(); got != tt.want {
				t.Errorf("Sheet.GetProtoName() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSheet_ToWorkseet(t *testing.T) {
	tests := []struct {
		name string
		s    *Sheet
		want *internalpb.Worksheet
	}{
		{
			name: "Sheet",
			s: &Sheet{
				Name:     "Sheet",
				Document: &Node{Name: "Sheet"},
				Meta: &internalpb.Metasheet{
					Sheet: "Sheet",
					Patch: tableaupb.Patch_PATCH_MERGE,
					// Loader options:
					OrderedMap: true,
					Index:      "ID@Item",
				},
			},
			want: &internalpb.Worksheet{
				Name: "Sheet",
				Options: &tableaupb.WorksheetOptions{
					Name:  "Sheet",
					Patch: tableaupb.Patch_PATCH_MERGE,
					// Loader options:
					OrderedMap: true,
					Index:      parseIndexes("ID@Item"),
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.s.ToWorkseet(); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Sheet.ToWorkseet() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSetMetasheetName(t *testing.T) {
	assert.Panicsf(t, assert.PanicTestFunc(func() {
		SetMetasheetName("No@StartMetasheetName")
	}), "SetMetasheetName() should panic when metasheet name not starts with '@'")
}
