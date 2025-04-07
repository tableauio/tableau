package store

import (
	"testing"
	"time"

	"github.com/tableauio/tableau/proto/tableaupb/unittestpb"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func Test_processWhenEmitTimezones(t *testing.T) {
	type args struct {
		message       proto.Message
		locationName  string
		useProtoNames bool
	}
	tests := []struct {
		name    string
		args    args
		want    string
		wantErr bool
	}{
		{
			name: "UTC",
			args: args{
				message: &unittestpb.PatchMergeConf{
					Name: "test",
					Time: &unittestpb.PatchMergeConf_Time{
						Start: timestamppb.New(time.Date(2022, 1, 1, 0, 0, 0, 0, time.UTC)),
					},
				},
				locationName:  "UTC",
				useProtoNames: true,
			},
			want:    `{"name":"test","time":{"start":"2022-01-01T00:00:00Z"}}`,
			wantErr: false,
		},
		{
			name: "UTC+8",
			args: args{
				message: &unittestpb.PatchMergeConf{
					Name: "test",
					Time: &unittestpb.PatchMergeConf_Time{
						Start: timestamppb.New(time.Date(2022, 1, 1, 0, 0, 0, 0, time.UTC)),
					},
				},
				locationName:  "Asia/Shanghai",
				useProtoNames: true,
			},
			want:    `{"name":"test","time":{"start":"2022-01-01T08:00:00+08:00"}}`,
			wantErr: false,
		},
		{
			name: "UTC-6",
			args: args{
				message: &unittestpb.PatchMergeConf{
					Name: "test",
					Time: &unittestpb.PatchMergeConf_Time{
						Start: timestamppb.New(time.Date(2022, 1, 1, 0, 0, 0, 0, time.UTC)),
					},
				},
				locationName:  "America/Chicago",
				useProtoNames: true,
			},
			want:    `{"name":"test","time":{"start":"2021-12-31T18:00:00-06:00"}}`,
			wantErr: false,
		},
		{
			name: "UTC+0845",
			args: args{
				message: &unittestpb.PatchMergeConf{
					Name: "test",
					Time: &unittestpb.PatchMergeConf_Time{
						Start: timestamppb.New(time.Date(2022, 1, 1, 0, 0, 0, 0, time.UTC)),
					},
				},
				locationName:  "Australia/Eucla",
				useProtoNames: true,
			},
			want:    `{"name":"test","time":{"start":"2022-01-01T08:45:00+08:45"}}`,
			wantErr: false,
		},
		{
			name: "RFC3339 in string field",
			args: args{
				message: &unittestpb.PatchMergeConf{
					Name: "2022-01-01T00:00:00Z",
					Time: &unittestpb.PatchMergeConf_Time{
						Start: timestamppb.New(time.Date(2022, 1, 1, 0, 0, 0, 0, time.UTC)),
					},
				},
				locationName:  "Asia/Shanghai",
				useProtoNames: true,
			},
			want:    `{"name":"2022-01-01T00:00:00Z","time":{"start":"2022-01-01T08:00:00+08:00"}}`,
			wantErr: false,
		},
		{
			name: "invalid-location",
			args: args{
				message: &unittestpb.PatchMergeConf{
					Name: "test",
					Time: &unittestpb.PatchMergeConf_Time{
						Start: timestamppb.New(time.Date(2022, 1, 1, 0, 0, 0, 0, time.UTC)),
					},
				},
				locationName:  "invalid-location",
				useProtoNames: true,
			},
			want:    ``,
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			json, err := MarshalToJSON(tt.args.message, &MarshalOptions{
				UseProtoNames: tt.args.useProtoNames,
			})
			if err != nil {
				t.Errorf("MarshalToJSON() error = %v", err)
				return
			}
			got, err := processWhenEmitTimezones(tt.args.message, string(json), tt.args.locationName, tt.args.useProtoNames)
			if (err != nil) != tt.wantErr {
				t.Errorf("processWhenEmitTimezones() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("processWhenEmitTimezones() = %v, want %v", got, tt.want)
			}
		})
	}
}
