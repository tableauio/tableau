package tableaupb

import (
	"math"
	"testing"
)

func TestMatch(t *testing.T) {
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
			if got := tt.args.left.Match(tt.args.cmp); got != tt.want {
				t.Errorf("Match() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestMatchAny(t *testing.T) {
	left := NewFraction(1, 2)
	tests := []struct {
		name string
		cmps []*Comparator
		want bool
	}{
		{
			name: "empty",
			cmps: nil,
			want: false,
		},
		{
			name: "any match (first)",
			cmps: []*Comparator{
				NewComparator(Comparator_SIGN_EQUAL, 1, 2),   // 1/2 == 1/2 → true
				NewComparator(Comparator_SIGN_LESS, 1, 4),   // 1/2 < 1/4 → false
			},
			want: true,
		},
		{
			name: "any match (last)",
			cmps: []*Comparator{
				NewComparator(Comparator_SIGN_LESS, 1, 4),            // 1/2 < 1/4 → false
				NewComparator(Comparator_SIGN_GREATER, 1, 4),         // 1/2 > 1/4 → true
			},
			want: true,
		},
		{
			name: "no match",
			cmps: []*Comparator{
				NewComparator(Comparator_SIGN_LESS, 1, 4),                 // 1/2 < 1/4 → false
				NewComparator(Comparator_SIGN_GREATER_OR_EQUAL, 3, 4),    // 1/2 >= 3/4 → false
			},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := left.MatchAny(tt.cmps...); got != tt.want {
				t.Errorf("MatchAny() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestMatchAll(t *testing.T) {
	left := NewFraction(1, 2)
	tests := []struct {
		name string
		cmps []*Comparator
		want bool
	}{
		{
			name: "empty (vacuous truth)",
			cmps: nil,
			want: true,
		},
		{
			name: "all match",
			cmps: []*Comparator{
				NewComparator(Comparator_SIGN_EQUAL, 2, 4),            // 1/2 == 2/4 → true
				NewComparator(Comparator_SIGN_GREATER, 1, 4),         // 1/2 > 1/4 → true
				NewComparator(Comparator_SIGN_LESS_OR_EQUAL, 3, 4),   // 1/2 <= 3/4 → true
			},
			want: true,
		},
		{
			name: "one fails",
			cmps: []*Comparator{
				NewComparator(Comparator_SIGN_EQUAL, 2, 4),   // 1/2 == 2/4 → true
				NewComparator(Comparator_SIGN_LESS, 1, 4),    // 1/2 < 1/4 → false
			},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := left.MatchAll(tt.cmps...); got != tt.want {
				t.Errorf("MatchAll() = %v, want %v", got, tt.want)
			}
		})
	}
}
