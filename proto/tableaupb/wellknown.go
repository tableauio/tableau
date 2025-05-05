package tableaupb

func (x *Fraction) GetValue() float64 {
	if x != nil {
		return float64(x.Num) / float64(x.Den)
	}
	return 0
}

func (x *Comparator) MatchFloat(v float64) bool {
	if x != nil {
		switch x.Sign {
		case Comparator_SIGN_EQUAL:
			return v == x.Value.GetValue()
		case Comparator_SIGN_NOT_EQUAL:
			return v != x.Value.GetValue()
		case Comparator_SIGN_LESS:
			return v < x.Value.GetValue()
		case Comparator_SIGN_LESS_OR_EQUAL:
			return v <= x.Value.GetValue()
		case Comparator_SIGN_GREATER:
			return v > x.Value.GetValue()
		case Comparator_SIGN_GREATER_OR_EQUAL:
			return v >= x.Value.GetValue()
		}
	}
	return false
}

func (x *Comparator) MatchInt(v int64) bool {
	return x.MatchFloat(float64(v))
}
