package store

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/tableauio/tableau/proto/tableaupb/unittestpb"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
)

var itemConf *unittestpb.ItemConf

func init() {
	itemConf = &unittestpb.ItemConf{
		ItemMap: map[uint32]*unittestpb.Item{
			1: {Id: 1, Num: 10},
			2: {Id: 2, Num: 20},
			3: {Id: 3, Num: 30},
		},
	}
}

func TestMarshalToJSONTimezones(t *testing.T) {
	tests := []struct {
		name    string
		msg     proto.Message
		options *MarshalOptions
		want    string
		wantErr bool
	}{
		{
			name: "timestamp",
			msg: &unittestpb.PatchMergeConf{
				Time: &unittestpb.PatchMergeConf_Time{
					Start: timestamppb.New(time.Date(2022, 1, 1, 0, 0, 0, 0, time.UTC)),
				},
			},
			options: &MarshalOptions{EmitTimezones: true, LocationName: "Asia/Shanghai", UseProtoNames: true},
			want:    `{"time":{"start":"2022-01-01T08:00:00+08:00"}}`,
		},
		{
			name: "ordinary timestamp string",
			msg: &unittestpb.PatchMergeConf{
				Name: "2022-01-01T00:00:00Z",
			},
			options: &MarshalOptions{EmitTimezones: true, LocationName: "Asia/Shanghai", UseProtoNames: true},
			want:    `{"name":"2022-01-01T00:00:00Z"}`,
		},
		{
			name:    "invalid location without timestamp",
			msg:     itemConf,
			options: &MarshalOptions{EmitTimezones: true, LocationName: "invalid-location"},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := MarshalToJSON(tt.msg, tt.options)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
			assert.Equal(t, tt.want, string(got))
		})
	}
}

func Test_MarshalToJSON(t *testing.T) {
	type args struct {
		msg     proto.Message
		options *MarshalOptions
	}
	tests := []struct {
		name    string
		args    args
		wantOut []byte
		wantErr bool
	}{
		{
			name: "item-conf-compact-output",
			args: args{
				msg: itemConf,
				options: &MarshalOptions{
					Pretty:          false,
					EmitUnpopulated: false,
					UseProtoNames:   false,
					UseEnumNumbers:  false,
				},
			},
			wantOut: []byte(`{"itemMap":{"1":{"id":1,"num":10},"2":{"id":2,"num":20},"3":{"id":3,"num":30}}}`),
			wantErr: false,
		},
		{
			name: "item-conf-pretty-output",
			args: args{
				msg: itemConf,
				options: &MarshalOptions{
					Pretty:          true,
					EmitUnpopulated: false,
					UseProtoNames:   false,
					UseEnumNumbers:  false,
				},
			},
			wantOut: []byte(`{
    "itemMap": {
        "1": {
            "id": 1,
            "num": 10
        },
        "2": {
            "id": 2,
            "num": 20
        },
        "3": {
            "id": 3,
            "num": 30
        }
    }
}`),
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotOut, err := MarshalToJSON(tt.args.msg, tt.args.options)
			if (err != nil) != tt.wantErr {
				t.Errorf("MarshalToJSON() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			assert.EqualValues(t, tt.wantOut, gotOut)
		})
	}
}

func Test_MarshalToText(t *testing.T) {
	type args struct {
		msg    proto.Message
		pretty bool
	}
	tests := []struct {
		name    string
		args    args
		wantOut []byte
		wantErr bool
	}{
		{
			name: "item-conf-compact-output",
			args: args{
				msg:    itemConf,
				pretty: false,
			},
			wantOut: []byte(`item_map:{key:1 value:{id:1 num:10}} item_map:{key:2 value:{id:2 num:20}} item_map:{key:3 value:{id:3 num:30}}`),
			wantErr: false,
		},
		{
			name: "item-conf-pretty-output",
			args: args{
				msg:    itemConf,
				pretty: true,
			},
			wantOut: []byte(`item_map: {
  key: 1
  value: {
    id: 1
    num: 10
  }
}
item_map: {
  key: 2
  value: {
    id: 2
    num: 20
  }
}
item_map: {
  key: 3
  value: {
    id: 3
    num: 30
  }
}`),
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotOut, err := MarshalToText(tt.args.msg, tt.args.pretty)
			if (err != nil) != tt.wantErr {
				t.Errorf("MarshalToText() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			assert.EqualValues(t, tt.wantOut, gotOut)
		})
	}
}

func Test_MarshalToBin(t *testing.T) {
	type args struct {
		msg proto.Message
	}
	tests := []struct {
		name    string
		args    args
		wantOut []byte
		wantErr bool
	}{
		{
			name: "itemConf",
			args: args{
				msg: itemConf,
			},
			// nolint:staticcheck
			wantOut: []byte(`



`),
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotOut, err := MarshalToBin(tt.args.msg)
			if (err != nil) != tt.wantErr {
				t.Errorf("MarshalToBin() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			assert.EqualValues(t, tt.wantOut, gotOut)
		})
	}
}
