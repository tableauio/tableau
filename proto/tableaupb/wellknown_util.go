package tableaupb

import (
	"time"

	"google.golang.org/protobuf/types/known/timestamppb"
)

// NewFraction creates a new fraction.
func NewFraction(num, den int32) *Fraction {
	return &Fraction{Num: num, Den: den}
}

// NewIntegerFraction creates a new special fraction: integer. When a fraction has a denominator
// of 1 (e.g.: a/1), it is referred to as a whole number or an integer.
func NewIntegerFraction(num int32) *Fraction {
	return NewFraction(num, 1)
}

// NewComparator creates a new fraction comparator.
func NewComparator(sign Comparator_Sign, num, den int32) *Comparator {
	return &Comparator{
		Sign:  sign,
		Value: NewFraction(num, den),
	}
}

// NewIntegerComparator creates a new comparator compared to a special fraction: integer.
// When a fraction has a denominator of 1 (e.g.: a/1), it is referred to as a whole number or an integer.
func NewIntegerComparator(sign Comparator_Sign, num int32) *Comparator {
	return &Comparator{
		Sign:  sign,
		Value: NewIntegerFraction(num),
	}
}

// Compare returns true if the given fraction matches the given comparator.
func Compare(left *Fraction, cmp *Comparator) bool {
	right := cmp.GetValue()
	// cross-multiply to compare
	lval := int64(left.GetNum()) * int64(right.GetDen())
	rval := int64(right.GetNum()) * int64(left.GetDen())
	switch cmp.GetSign() {
	case Comparator_SIGN_EQUAL:
		return lval == rval
	case Comparator_SIGN_NOT_EQUAL:
		return lval != rval
	case Comparator_SIGN_LESS:
		return lval < rval
	case Comparator_SIGN_LESS_OR_EQUAL:
		return lval <= rval
	case Comparator_SIGN_GREATER:
		return lval > rval
	case Comparator_SIGN_GREATER_OR_EQUAL:
		return lval >= rval
	default:
		panic("invalid compare operator")
	}
}

// LocalTime converts a timestamp to a local time.
//
// NOTE: The [Timestamp.AsTime] method returns a UTC time, so we provide
// [LocalTime] to convert it to a local time for easy use.
//
// [Timestamp.AsTime]: https://pkg.go.dev/google.golang.org/protobuf/types/known/timestamppb#Timestamp.AsTime
func LocalTime(ts *timestamppb.Timestamp) time.Time {
	return ts.AsTime().Local()
}
