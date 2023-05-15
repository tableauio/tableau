package mexporter

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/tableauio/tableau/proto/tableaupb"
	"google.golang.org/protobuf/proto"
)

var itemConf *tableaupb.TestItemConf

func init() {
	itemConf = &tableaupb.TestItemConf{
		ItemMap: map[uint32]*tableaupb.TestItem{
			1: {Id: 1, Num: 10},
			2: {Id: 2, Num: 20},
			3: {Id: 3, Num: 30},
		},
	}
}

func Test_marshalToJSON(t *testing.T) {
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
			name: "itemConf",
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
			gotOut, err := marshalToJSON(tt.args.msg, tt.args.options)
			if (err != nil) != tt.wantErr {
				t.Errorf("marshalToJSON() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			assert.EqualValues(t, tt.wantOut, gotOut)
		})
	}
}

func Test_marshalToText(t *testing.T) {
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
			name: "itemConf",
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
}
`),
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotOut, err := marshalToText(tt.args.msg, tt.args.pretty)
			if (err != nil) != tt.wantErr {
				t.Errorf("marshalToText() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			assert.EqualValues(t, tt.wantOut, gotOut)
		})
	}
}

func Test_marshalToBin(t *testing.T) {
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
			wantOut: []byte(`



`),
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotOut, err := marshalToBin(tt.args.msg)
			if (err != nil) != tt.wantErr {
				t.Errorf("marshalToBin() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			assert.EqualValues(t, tt.wantOut, gotOut)
		})
	}
}
