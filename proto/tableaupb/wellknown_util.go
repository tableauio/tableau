package tableaupb

import "math/big"

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
	return left.Cmp(cmp)
}

func (f *Fraction) AsRat() *big.Rat {
	return big.NewRat(int64(f.GetNum()), int64(f.GetDen()))
}

func (f *Fraction) Cmp(cmp *Comparator) bool {
	other := cmp.GetValue()
	// cross-multiply to compare
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
		panic("invalid compare operator")
	}
}
