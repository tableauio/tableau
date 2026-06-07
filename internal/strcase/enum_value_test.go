package strcase

import (
	"context"
	"testing"
)

// enumValueExpect describes the EnumValue output under both naming
// styles. wantLegacy == "" means "legacy result is identical to want".
type enumValueExpect struct {
	enum       string
	value      string
	want       string // STYLE2024 result
	wantLegacy string // legacy result; empty means same as want
}

func (c enumValueExpect) expectedLegacy() string {
	if c.wantLegacy == "" {
		return c.want
	}
	return c.wantLegacy
}

func enumValueCases() []enumValueExpect {
	return []enumValueExpect{
		// --- agreement set ---
		{enum: "ItemType", value: "EQUIP", want: "ITEM_TYPE_EQUIP"},
		{enum: "ItemType", value: "Fruit", want: "ITEM_TYPE_FRUIT"},
		// Already prefixed -> kept (and re-normalized) under both styles.
		{enum: "ItemType", value: "ITEM_TYPE_EQUIP", want: "ITEM_TYPE_EQUIP"},
		// Empty value -> just the prefix.
		{enum: "ItemType", value: "", want: "ITEM_TYPE_"},

		// --- divergence set ---
		// Letter <-> digit boundary: STYLE2024 keeps the digit attached
		// to the trailing letter; legacy splits it.
		{enum: "DeviceTier", value: "Tier1", want: "DEVICE_TIER_TIER1", wantLegacy: "DEVICE_TIER_TIER_1"},
		// Pure digit suffix: STYLE2024 injects a leading "V" so the
		// post-prefix remainder is a valid identifier; legacy emits the
		// raw digit (DEVICE_TIER_1 was historically accepted).
		{enum: "DeviceTier", value: "1", want: "DEVICE_TIER_V1", wantLegacy: "DEVICE_TIER_1"},
		{enum: "DeviceTier", value: "2A", want: "DEVICE_TIER_V2_A", wantLegacy: "DEVICE_TIER_2_A"},
	}
}

func TestEnumValue(t *testing.T) {
	std := New(nil)
	legacy := NewLegacy(nil)
	for _, c := range enumValueCases() {
		if got := std.EnumValue(c.enum, c.value); got != c.want {
			t.Errorf("STYLE2024 EnumValue(%q, %q) = %q, want %q",
				c.enum, c.value, got, c.want)
		}
		if got := legacy.EnumValue(c.enum, c.value); got != c.expectedLegacy() {
			t.Errorf("legacy   EnumValue(%q, %q) = %q, want %q",
				c.enum, c.value, got, c.expectedLegacy())
		}
	}
}

func TestStyle2024_AcronymsAndContext(t *testing.T) {
	ctx := FromContext(NewContext(context.Background(), New(map[string]string{
		`(\d)[vV](\d)`: "${1}v${2}",
	})))
	cases := []struct {
		fn       func(string) string
		in, want string
	}{
		{ctx.ToSnake, "Mode1V1", "mode1v1"},
		{ctx.ToScreamingSnake, "Mode1V1", "MODE1V1"},
	}
	for _, c := range cases {
		got := c.fn(c.in)
		if got != c.want {
			t.Errorf("style2024 acronym(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}
