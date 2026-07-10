package tableaupb

import "slices"

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
//
// Deprecated: Use [Fraction.Match] instead.
func Compare(left *Fraction, cmp *Comparator) bool {
	return left.Match(cmp)
}

// Match reports whether the fraction satisfies the comparator's
// relational condition (==, !=, <, <=, >, >=). Comparison is done by
// cross-multiplication to stay in exact integer arithmetic for performance
// and accuracy.
func (f *Fraction) Match(cmp *Comparator) bool {
	other := cmp.GetValue()
	lval := int64(f.GetNum()) * int64(other.GetDen())
	rval := int64(other.GetNum()) * int64(f.GetDen())
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
		// Unknown sign is treated as no-match; proto enums are open-ended (forward-compat), not a bug.
		return false
	}
}

// MatchAny reports whether the fraction matches any of the given comparators (OR logic).
// It returns false if no comparators are provided.
func (f *Fraction) MatchAny(cmps ...*Comparator) bool {
	return slices.ContainsFunc(cmps, f.Match)
}

// MatchAll reports whether the fraction matches all of the given comparators (AND logic).
// It returns true if no comparators are provided (vacuous truth).
func (f *Fraction) MatchAll(cmps ...*Comparator) bool {
	for _, cmp := range cmps {
		if !f.Match(cmp) {
			return false
		}
	}
	return true
}
