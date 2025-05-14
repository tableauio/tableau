package store

import (
	"fmt"
	"testing"
	"time"

	"github.com/tableauio/tableau/proto/tableaupb/unittestpb"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func Test_processWhenEmitTimezones(t *testing.T) {
	type args struct {
		message         proto.Message
		locationName    string
		emitUnpopulated bool
		useProtoNames   bool
	}
	tests := []struct {
		name    string
		args    args
		want    string
		wantErr bool
	}{
		{
			name: "UTC-empty",
			args: args{
				message: &unittestpb.PatchMergeConf{
					Name: "test",
					Time: &unittestpb.PatchMergeConf_Time{
						Start: &timestamppb.Timestamp{},
					},
				},
				locationName:  "UTC",
				useProtoNames: true,
			},
			want:    `{"name":"test","time":{"start":"1970-01-01T00:00:00Z"}}`,
			wantErr: false,
		},
		{
			name: "UTC-nil",
			args: args{
				message: &unittestpb.PatchMergeConf{
					Name: "test",
					Time: &unittestpb.PatchMergeConf_Time{
						Start: nil,
					},
				},
				locationName:  "UTC",
				useProtoNames: true,
			},
			want:    `{"name":"test","time":{}}`,
			wantErr: false,
		},
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
			name: "complicated message",
			args: args{
				message: &unittestpb.JsonUtilTestData{
					NormalField: &unittestpb.PatchMergeConf{
						Name: "normal",
						Time: &unittestpb.PatchMergeConf_Time{
							Start: timestamppb.New(time.Date(2022, 1, 1, 0, 0, 0, 0, time.UTC)),
						},
					},
					ListField: []*unittestpb.PatchMergeConf{
						{
							Name: "list elem 0",
							Time: &unittestpb.PatchMergeConf_Time{
								Start: timestamppb.New(time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC)),
							},
						},
						{
							Name: "list elem 1",
							Time: &unittestpb.PatchMergeConf_Time{
								Start: timestamppb.New(time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)),
							},
						},
					},
					MapField: map[int32]*unittestpb.PatchMergeConf{
						2025: {
							Name: "map key 2025",
							Time: &unittestpb.PatchMergeConf_Time{
								Start: timestamppb.New(time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)),
							},
						},
						2026: {
							Name: "map key 2026",
							Time: &unittestpb.PatchMergeConf_Time{
								Start: timestamppb.New(time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)),
							},
						},
					},
				},
				locationName:  "Asia/Shanghai",
				useProtoNames: true,
			},
			want:    `{"normal_field":{"name":"normal","time":{"start":"2022-01-01T08:00:00+08:00"}},"list_field":[{"name":"list elem 0","time":{"start":"2023-01-01T08:00:00+08:00"}},{"name":"list elem 1","time":{"start":"2024-01-01T08:00:00+08:00"}}],"map_field":{"2025":{"name":"map key 2025","time":{"start":"2025-01-01T08:00:00+08:00"}},"2026":{"name":"map key 2026","time":{"start":"2026-01-01T08:00:00+08:00"}}}}`,
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
		{
			name: "UTC-empty-emit-unpopulated",
			args: args{
				message: &unittestpb.PatchMergeConf_Time{
					Start: &timestamppb.Timestamp{},
				},
				locationName:    "UTC",
				emitUnpopulated: true,
			},
			want:    `{"start":"1970-01-01T00:00:00Z","expiry":null}`,
			wantErr: false,
		},
		{
			name: "UTC-nil-emit-unpopulated",
			args: args{
				message: &unittestpb.PatchMergeConf_Time{
					Start: nil,
				},
				locationName:    "UTC",
				emitUnpopulated: true,
			},
			want:    `{"start":null,"expiry":null}`,
			wantErr: false,
		},
		{
			name: "UTC+8-emit-unpopulated",
			args: args{
				message: &unittestpb.PatchMergeConf_Time{
					Start: timestamppb.New(time.Date(2022, 1, 1, 0, 0, 0, 0, time.UTC)),
				},
				locationName:    "Asia/Shanghai",
				emitUnpopulated: true,
			},
			want:    `{"start":"2022-01-01T08:00:00+08:00","expiry":null}`,
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			json, err := MarshalToJSON(tt.args.message, &MarshalOptions{
				EmitUnpopulated: tt.args.emitUnpopulated,
				UseProtoNames:   tt.args.useProtoNames,
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

func prepare(size int) (proto.Message, string) {
	message := &unittestpb.JsonUtilTestData{}
	for i := 0; i < size; i++ {
		message.ListField = append(message.ListField, &unittestpb.PatchMergeConf{
			Name: fmt.Sprintf("list elem %d", i),
			Time: &unittestpb.PatchMergeConf_Time{
				Start: timestamppb.New(time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC)),
			},
		})
	}
	json, _ := MarshalToJSON(message, &MarshalOptions{
		UseProtoNames: true,
	})
	return message, string(json)
}

func Benchmark_regexp1(b *testing.B) {
	_, json := prepare(1)
	for i := 0; i < b.N; i++ {
		_, _ = processWhenEmitTimezonesByRegexp(json, "Asia/Shanghai")
	}
}
func Benchmark_regexp10(b *testing.B) {
	_, json := prepare(10)
	for i := 0; i < b.N; i++ {
		_, _ = processWhenEmitTimezonesByRegexp(json, "Asia/Shanghai")
	}
}
func Benchmark_regexp100(b *testing.B) {
	_, json := prepare(100)
	for i := 0; i < b.N; i++ {
		_, _ = processWhenEmitTimezonesByRegexp(json, "Asia/Shanghai")
	}
}
func Benchmark_regexp1000(b *testing.B) {
	_, json := prepare(1000)
	for i := 0; i < b.N; i++ {
		_, _ = processWhenEmitTimezonesByRegexp(json, "Asia/Shanghai")
	}
}

func Benchmark_regexp10000(b *testing.B) {
	_, json := prepare(10000)
	for i := 0; i < b.N; i++ {
		_, _ = processWhenEmitTimezonesByRegexp(json, "Asia/Shanghai")
	}
}

func Benchmark_regexp100000(b *testing.B) {
	_, json := prepare(100000)
	for i := 0; i < b.N; i++ {
		_, _ = processWhenEmitTimezonesByRegexp(json, "Asia/Shanghai")
	}
}

func Benchmark_sonic1(b *testing.B) {
	message, json := prepare(1)
	for i := 0; i < b.N; i++ {
		_, _ = processWhenEmitTimezones(message, json, "Asia/Shanghai", true)
	}
}
func Benchmark_sonic10(b *testing.B) {
	message, json := prepare(10)
	for i := 0; i < b.N; i++ {
		_, _ = processWhenEmitTimezones(message, json, "Asia/Shanghai", true)
	}
}

func Benchmark_sonic100(b *testing.B) {
	message, json := prepare(100)
	for i := 0; i < b.N; i++ {
		_, _ = processWhenEmitTimezones(message, json, "Asia/Shanghai", true)
	}
}

func Benchmark_sonic1000(b *testing.B) {
	message, json := prepare(1000)
	for i := 0; i < b.N; i++ {
		_, _ = processWhenEmitTimezones(message, json, "Asia/Shanghai", true)
	}
}

func Benchmark_sonic10000(b *testing.B) {
	message, json := prepare(10000)
	for i := 0; i < b.N; i++ {
		_, _ = processWhenEmitTimezones(message, json, "Asia/Shanghai", true)
	}
}

func Benchmark_sonic100000(b *testing.B) {
	message, json := prepare(100000)
	for i := 0; i < b.N; i++ {
		_, _ = processWhenEmitTimezones(message, json, "Asia/Shanghai", true)
	}
}
