package store

import "testing"

func Test_processWhenUseTimezones(t *testing.T) {
	type args struct {
		jsonStr      string
		locationName string
	}
	tests := []struct {
		name    string
		args    args
		want    string
		wantErr bool
	}{
		{
			name: "UTC+8",
			args: args{
				jsonStr:      `{"time":"2022-01-01T00:00:00Z"}`,
				locationName: "Asia/Shanghai",
			},
			want:    `{"time":"2022-01-01T08:00:00+08:00"}`,
			wantErr: false,
		},
		{
			name: "UTC-6",
			args: args{
				jsonStr:      `{"time":"2022-01-01T00:00:00Z"}`,
				locationName: "America/Chicago",
			},
			want:    `{"time":"2021-12-31T18:00:00-06:00"}`,
			wantErr: false,
		},
		{
			name: "UTC+0845",
			args: args{
				jsonStr:      `{"time":"2022-01-01T00:00:00Z"}`,
				locationName: "Australia/Eucla",
			},
			want:    `{"time":"2022-01-01T08:45:00+08:45"}`,
			wantErr: false,
		},
		{
			name: "invalid-location",
			args: args{
				jsonStr:      `{"time":"2022-01-01T00:00:00Z"}`,
				locationName: "invalid-location",
			},
			want:    ``,
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := processWhenUseTimezones(tt.args.jsonStr, tt.args.locationName)
			if (err != nil) != tt.wantErr {
				t.Errorf("processWhenUseTimezones() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("processWhenUseTimezones() = %v, want %v", got, tt.want)
			}
		})
	}
}
