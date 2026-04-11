package confgen

import (
	"reflect"
	"testing"

	"buf.build/go/protovalidate"
	"github.com/tableauio/tableau/format"
	"github.com/tableauio/tableau/options"
	"github.com/tableauio/tableau/proto/tableaupb/unittestpb"
	"github.com/tableauio/tableau/xerrors"
	"google.golang.org/protobuf/proto"
)

func Test_parseBookSpecifier(t *testing.T) {
	type args struct {
		bookSpecifier string
	}
	tests := []struct {
		name          string
		args          args
		wantBookName  string
		wantSheetName string
		wantErr       bool
	}{
		{
			name: "xlsx-only-workbook",
			args: args{
				bookSpecifier: "testdata/excel/Item.xlsx",
			},
			wantBookName:  "testdata/excel/Item.xlsx",
			wantSheetName: "",
			wantErr:       false,
		},
		{
			name: "xlsx-with-sheet",
			args: args{
				bookSpecifier: "testdata/excel/Item.xlsx#Item",
			},
			wantBookName:  "testdata/excel/Item.xlsx",
			wantSheetName: "Item",
			wantErr:       false,
		},
		{
			name: "dir-path-with-special-char-#",
			args: args{
				bookSpecifier: "testdata/excel#dir/Item.xlsx#Item",
			},
			wantBookName:  "testdata/excel#dir/Item.xlsx",
			wantSheetName: "Item",
			wantErr:       false,
		},
		{
			name: "csv-only-workbook",
			args: args{
				bookSpecifier: "testdata/csv/Item#Item.csv",
			},
			wantBookName:  "testdata/csv/Item#*.csv",
			wantSheetName: "",
			wantErr:       false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotBookName, gotSheetName, err := parseBookSpecifier(tt.args.bookSpecifier)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseBookSpecifier() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if gotBookName != tt.wantBookName {
				t.Errorf("parseBookSpecifier() gotBookName = %v, want %v", gotBookName, tt.wantBookName)
			}
			if gotSheetName != tt.wantSheetName {
				t.Errorf("parseBookSpecifier() gotSheetName = %v, want %v", gotSheetName, tt.wantSheetName)
			}
		})
	}
}

func Test_storeMessage(t *testing.T) {
	itemConf := &unittestpb.ItemConf{
		ItemMap: map[uint32]*unittestpb.Item{
			1: {Id: 1, Num: 10},
			2: {Id: 2, Num: 20},
			3: {Id: 3, Num: 30},
		},
	}
	type args struct {
		msg       proto.Message
		name      string
		outputDir string
		opt       *options.ConfOutputOption
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name: "export-item-conf",
			args: args{
				msg:       itemConf,
				name:      "ItemConfAlias",
				outputDir: "_out/",
				opt: &options.ConfOutputOption{
					Subdir:          "",
					Formats:         []format.Format{"json"},
					Pretty:          true,
					EmitUnpopulated: true,
				},
			},
			wantErr: false,
		},
		{
			name: "export-item-conf-subdir",
			args: args{
				msg:       itemConf,
				name:      "",
				outputDir: "_out/",
				opt: &options.ConfOutputOption{
					Subdir:          "subdir",
					Formats:         nil,
					Pretty:          true,
					EmitUnpopulated: true,
				},
			},
			wantErr: false,
		},
		{
			name: "protovalidate-field-pass",
			args: args{
				msg: &unittestpb.ValidateConf{
					Id:   0,
					Name: "short",
				},
				name:      "ValidateConf",
				outputDir: "_out/",
				opt: &options.ConfOutputOption{
					Formats: []format.Format{"json"},
					Pretty:  true,
				},
			},
			wantErr: false,
		},
		{
			name: "protovalidate-field-fail",
			args: args{
				msg: &unittestpb.ValidateConf{
					Id:   0,
					Name: "this exceeds max_len of 10",
				},
				name:      "ValidateConf",
				outputDir: "_out/",
				opt: &options.ConfOutputOption{
					Formats: []format.Format{"json"},
					Pretty:  true,
				},
			},
			wantErr: true,
		},
		{
			name: "protovalidate-message-pass",
			args: args{
				msg: &unittestpb.ValidateConf{
					Id:   0,
					Name: "",
				},
				name:      "ValidateConf",
				outputDir: "_out/",
				opt: &options.ConfOutputOption{
					Formats: []format.Format{"json"},
					Pretty:  true,
				},
			},
			wantErr: false,
		},
		{
			name: "protovalidate-message-fail",
			args: args{
				msg: &unittestpb.ValidateConf{
					Id:   1,
					Name: "",
				},
				name:      "ValidateConf",
				outputDir: "_out/",
				opt: &options.ConfOutputOption{
					Formats: []format.Format{"json"},
					Pretty:  true,
				},
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			validator, err := protovalidate.New()
			if err != nil {
				t.Fatalf("failed to create validator: %v", err)
			}
			if err := storeMessage(tt.args.msg, tt.args.name, "UTC", tt.args.outputDir, tt.args.opt, validator); (err != nil) != tt.wantErr {
				t.Errorf("storeMessage() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func Test_parseOutputFormats(t *testing.T) {
	type args struct {
		msg proto.Message
		opt *options.ConfOutputOption
	}
	tests := []struct {
		name string
		args args
		want []format.Format
	}{
		{
			name: "default",
			args: args{
				msg: &unittestpb.ItemConf{},
				opt: &options.ConfOutputOption{},
			},
			want: []format.Format{format.JSON, format.Bin, format.Text},
		},
		{
			name: "global",
			args: args{
				msg: &unittestpb.ItemConf{},
				opt: &options.ConfOutputOption{
					Formats: []format.Format{format.Bin},
					MessagerFormats: map[string][]format.Format{
						"TaskConf": {format.JSON},
					},
				},
			},
			want: []format.Format{format.Bin},
		},
		{
			name: "messager-level",
			args: args{
				msg: &unittestpb.ItemConf{},
				opt: &options.ConfOutputOption{
					Formats: []format.Format{format.Bin},
					MessagerFormats: map[string][]format.Format{
						"TaskConf": {format.JSON},
						"ItemConf": {format.Text},
					},
				},
			},
			want: []format.Format{format.Text},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := parseOutputFormats(tt.args.msg, tt.args.opt); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("parseOutputFormats() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_validate(t *testing.T) {
	type args struct {
		msg proto.Message
	}
	tests := []struct {
		name       string
		args       args
		wantErr    bool
		wantReason string // exact KeyReason string extracted from the first error
	}{
		{
			// No constraint violated: id==0 satisfies message-level CEL, name is short.
			name: "pass",
			args: args{
				msg: &unittestpb.ValidateConf{Id: 0, Name: "ok"},
			},
			wantErr: false,
		},
		{
			// Field-level violation: name exceeds max_len:10.
			// FieldValue is valid and quoted in the Reason string.
			name: "field-level-violation",
			args: args{
				msg: &unittestpb.ValidateConf{Id: 0, Name: "this exceeds max_len of 10"},
			},
			wantErr:    true,
			wantReason: `"this exceeds max_len of 10" violates rule: name: value length must be at most 10 characters`,
		},
		{
			// Message-level violation: id>0 but name is empty, violating the CEL expression.
			// FieldValue is NOT set; falls back to field path (empty string "").
			name: "message-level-violation",
			args: args{
				msg: &unittestpb.ValidateConf{Id: 1, Name: ""},
			},
			wantErr:    true,
			wantReason: `"" violates rule: name must be non-empty when id is positive`,
		},
		{
			// Both field-level and message-level violations at once.
			// wantReason checks the first joined error (field-level).
			name: "multiple-violations",
			args: args{
				msg: &unittestpb.ValidateConf{Id: 1, Name: "this exceeds max_len of 10"},
			},
			wantErr:    true,
			wantReason: `"this exceeds max_len of 10" violates rule: name: value length must be at most 10 characters`,
		},
		{
			// List field passes: tag_list has <=3 items, satisfying max_items:3.
			name: "list-pass",
			args: args{
				msg: &unittestpb.ValidateConf{Id: 0, Name: "ok", TagList: []string{"a", "b", "c"}},
			},
			wantErr: false,
		},
		{
			// List field violation: tag_list exceeds max_items:3.
			// FieldValue is a list (pointer), so falls back to field path "tag_list".
			name: "list-level-violation",
			args: args{
				msg: &unittestpb.ValidateConf{Id: 0, Name: "ok", TagList: []string{"a", "b", "c", "d"}},
			},
			wantErr:    true,
			wantReason: `"tag_list" violates rule: tag_list: value must contain no more than 3 item(s)`,
		},
		{
			// Map field passes: prop_map has <=2 pairs, satisfying max_pairs:2.
			name: "map-pass",
			args: args{
				msg: &unittestpb.ValidateConf{Id: 0, Name: "ok", PropMap: map[string]int32{"x": 1, "y": 2}},
			},
			wantErr: false,
		},
		{
			// Map field violation: prop_map exceeds max_pairs:2.
			// FieldValue is a map (pointer), so falls back to field path "prop_map".
			name: "map-level-violation",
			args: args{
				msg: &unittestpb.ValidateConf{Id: 0, Name: "ok", PropMap: map[string]int32{"x": 1, "y": 2, "z": 3}},
			},
			wantErr:    true,
			wantReason: `"prop_map" violates rule: prop_map: map must be at most 2 entries`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			validator, err := protovalidate.New()
			if err != nil {
				t.Fatalf("failed to create validator: %v", err)
			}
			err = validate(tt.args.msg, validator)
			if (err != nil) != tt.wantErr {
				t.Errorf("validate() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr && tt.wantReason != "" {
				// Extract the first joined error and compare KeyReason exactly.
				firstErr := err
				if joined, ok := err.(interface{ Unwrap() []error }); ok {
					if errs := joined.Unwrap(); len(errs) > 0 {
						firstErr = errs[0]
					}
				}
				gotReason, _ := xerrors.NewDesc(firstErr).GetValue(xerrors.KeyReason).(string)
				if gotReason != tt.wantReason {
					t.Errorf("validate() KeyReason =\n\t%q\nwant:\n\t%q", gotReason, tt.wantReason)
				}
			}
		})
	}
}
