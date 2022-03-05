package importer

import "testing"

func TestCSVImporter_ExportExcel(t *testing.T) {
	tests := []struct {
		name    string
		x       *CSVImporter
		wantErr bool
	}{
		// TODO: Add test cases.
		{
			name: "test",
			x:    NewCSVImporter("testdata/Test#Test.csv", nil, nil),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.x.ExportExcel(); (err != nil) != tt.wantErr {
				t.Errorf("CSVImporter.Export2Excel() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
