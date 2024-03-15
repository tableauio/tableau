package mexporter

import (
	"testing"

	"github.com/tableauio/tableau/options"
	"github.com/tableauio/tableau/proto/tableaupb/unittestpb"
)

var testMessageExporter *Exporter

func init() {
	itemConf = &unittestpb.ItemConf{
		ItemMap: map[uint32]*unittestpb.Item{
			1: {Id: 1, Num: 10},
			2: {Id: 2, Num: 20},
			3: {Id: 3, Num: 30},
		},
	}
	outputOpt := &options.ConfOutputOption{
		Subdir: "conf",
		Pretty: true,
	}
	testMessageExporter = New("ItemConf", itemConf, "_out/", outputOpt)
}

func Test_messageExporter_Export(t *testing.T) {
	tests := []struct {
		name    string
		x       *Exporter
		wantErr bool
	}{
		{
			name:    "export-item-conf",
			x:       testMessageExporter,
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.x.Export(); (err != nil) != tt.wantErr {
				t.Errorf("messageExporter.Export() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
