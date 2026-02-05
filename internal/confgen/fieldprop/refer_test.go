package fieldprop

import (
	"context"
	"reflect"
	"testing"

	"github.com/tableauio/tableau/proto/tableaupb"
	_ "github.com/tableauio/tableau/proto/tableaupb/unittestpb"
	"google.golang.org/protobuf/reflect/protoregistry"
)

func Test_parseRefer(t *testing.T) {
	type args struct {
		text string
	}
	tests := []struct {
		name    string
		args    args
		want    *ReferDesc
		wantErr bool
	}{
		{
			name: "without alias",
			args: args{
				text: "Item.ID",
			},
			want: &ReferDesc{"Item", "", "ID"},
		},
		{
			name: "with alias",
			args: args{
				text: "Item(ItemConf).ID",
			},
			want: &ReferDesc{"Item", "ItemConf", "ID"},
		},
		{
			name: "special-sheet-name-and-with-alias",
			args: args{
				text: "Item-(Award)(ItemConf).ID",
			},
			want: &ReferDesc{"Item-(Award)", "ItemConf", "ID"},
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

func TestInReferredSpace(t *testing.T) {
	type args struct {
		prop     *tableaupb.FieldProp
		cellData string
		input    *Input
	}
	tests := []struct {
		name    string
		args    args
		want    bool
		wantErr bool
	}{
		{
			name: "in referred value space",
			args: args{
				prop: &tableaupb.FieldProp{
					Refer: "ItemConf.ID",
				},
				cellData: "1",
				input: &Input{
					ProtoPackage:   "unittest",
					InputDir:       "../../../testdata",
					SubdirRewrites: nil,
					PRFiles:        protoregistry.GlobalFiles,
					Present:        true,
				},
			},
			want:    true,
			wantErr: false,
		},
		{
			name: "not in referred value space",
			args: args{
				prop: &tableaupb.FieldProp{
					Refer: "ItemConf(ItemConf).ID",
				},
				cellData: "999",
				input: &Input{
					ProtoPackage:   "unittest",
					InputDir:       "../../../testdata",
					SubdirRewrites: nil,
					PRFiles:        protoregistry.GlobalFiles,
					Present:        true,
				},
			},
			want:    false,
			wantErr: false,
		},
		{
			name: "in ignored referred value space",
			args: args{
				prop: &tableaupb.FieldProp{
					Refer: "ItemConf(ItemConf).ID",
				},
				cellData: "4",
				input: &Input{
					ProtoPackage:   "unittest",
					InputDir:       "../../../testdata",
					SubdirRewrites: nil,
					PRFiles:        protoregistry.GlobalFiles,
					Present:        true,
				},
			},
			want:    false,
			wantErr: false,
		},
		{
			name: "in referred value space with subdir rewrites",
			args: args{
				prop: &tableaupb.FieldProp{
					Refer: "ItemConf(ItemConf).ID",
				},
				cellData: "1",
				input: &Input{
					ProtoPackage: "unittest",
					InputDir:     "../../../testdata/unittest",
					SubdirRewrites: map[string]string{
						"unittest/": "",
					},
					PRFiles: protoregistry.GlobalFiles,
					Present: true,
				},
			},
			want:    true,
			wantErr: false,
		},
		{
			name: "not in referred value space with subdir rewrites",
			args: args{
				prop: &tableaupb.FieldProp{
					Refer: "ItemConf(ItemConf).ID",
				},
				cellData: "999",
				input: &Input{
					ProtoPackage: "unittest",
					InputDir:     "../../../",
					SubdirRewrites: map[string]string{
						"unittest/": "testdata/unittest/",
					},
					PRFiles: protoregistry.GlobalFiles,
					Present: true,
				},
			},
			want:    false,
			wantErr: false,
		},
		{
			name: "in referred value space(transposed)",
			args: args{
				prop: &tableaupb.FieldProp{
					Refer: "Transpose.Name",
				},
				cellData: "Robin",
				input: &Input{
					ProtoPackage:   "unittest",
					InputDir:       "../../../testdata",
					SubdirRewrites: nil,
					PRFiles:        protoregistry.GlobalFiles,
					Present:        true,
				},
			},
			want:    true,
			wantErr: false,
		},
		{
			name: "not referred value space(transposed)",
			args: args{
				prop: &tableaupb.FieldProp{
					Refer: "Transpose.Name",
				},
				cellData: "Thomas",
				input: &Input{
					ProtoPackage:   "unittest",
					InputDir:       "../../../testdata",
					SubdirRewrites: nil,
					PRFiles:        protoregistry.GlobalFiles,
					Present:        true,
				},
			},
			want:    false,
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := InReferredSpace(context.Background(), tt.args.prop, tt.args.cellData, tt.args.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("InReferredSpace() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("InReferredSpace() = %v, want %v", got, tt.want)
			}
		})
	}
}
