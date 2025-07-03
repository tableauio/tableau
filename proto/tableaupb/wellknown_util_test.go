package tableaupb

import (
	"math"
	"reflect"
	"testing"
	"time"

	"google.golang.org/protobuf/types/known/timestamppb"
)

func TestCompare(t *testing.T) {
	type args struct {
		left *Fraction
		cmp  *Comparator
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{
			name: "==",
			args: args{
				left: NewFraction(1, 2),
				cmp:  NewComparator(Comparator_SIGN_EQUAL, 2, 4),
			},
			want: true,
		},
		{
			name: "!=",
			args: args{
				left: NewFraction(1, 2),
				cmp:  NewComparator(Comparator_SIGN_NOT_EQUAL, 1, 4),
			},
			want: true,
		},
		{
			name: "<",
			args: args{
				left: NewFraction(1, 2),
				cmp:  NewComparator(Comparator_SIGN_LESS, 3, 4),
			},
			want: true,
		},
		{
			name: "<=",
			args: args{
				left: NewFraction(1, 2),
				cmp:  NewComparator(Comparator_SIGN_LESS_OR_EQUAL, 3, 4),
			},
			want: true,
		},
		{
			name: ">",
			args: args{
				left: NewFraction(1, 2),
				cmp:  NewComparator(Comparator_SIGN_GREATER, 1, 4),
			},
			want: true,
		},
		{
			name: ">=",
			args: args{
				left: NewFraction(1, 2),
				cmp:  NewComparator(Comparator_SIGN_GREATER_OR_EQUAL, 2, 4),
			},
			want: true,
		},
		{
			name: "max int32",
			args: args{
				left: NewFraction(math.MaxInt32-1, math.MaxInt32),              // 2147483646 / 2147483647
				cmp:  NewComparator(Comparator_SIGN_GREATER, 1, math.MaxInt32), // 1 / 2147483647
			},
			want: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Compare(tt.args.left, tt.args.cmp); got != tt.want {
				t.Errorf("Compare() = %v, want %v", got, tt.want)
			}
		})
	}
}

func localTime(value string) time.Time {
	loc, _ := time.LoadLocation("Local")
	t, _ := time.ParseInLocation(time.DateTime, value, loc)
	return t
}

func TestLocalTime(t *testing.T) {
	type args struct {
		ts *timestamppb.Timestamp
	}
	tests := []struct {
		name string
		args args
		want time.Time
	}{
		{
			name: "local time",
			args: args{
				ts: timestamppb.New(localTime("2021-08-30 00:00:00")),
			},
			want: localTime("2021-08-30 00:00:00"),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := LocalTime(tt.args.ts)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("LocalTime() = %v, want %v", got, tt.want)
			}
			if got.Location() == time.UTC {
				t.Errorf("Location = %v, want Local", got.Location().String())
			}
		})
	}
}
