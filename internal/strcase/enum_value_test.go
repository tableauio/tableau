package strcase

import (
	"context"
	"testing"
)

func TestEnumValue(t *testing.T) {
	var ctx Strcase
	cases := []struct {
		enum, value, want string
	}{
		// Basic prefix behavior.
		{"ItemType", "EQUIP", "ITEM_TYPE_EQUIP"},
		{"ItemType", "Fruit", "ITEM_TYPE_FRUIT"},
		// Already prefixed -> kept (and re-normalized).
		{"ItemType", "ITEM_TYPE_EQUIP", "ITEM_TYPE_EQUIP"},
		// Trailing digit on suffix is fine because suffix starts with a letter.
		{"DeviceTier", "Tier1", "DEVICE_TIER_TIER1"},
		// Pure digit suffix MUST be guarded so the post-strip remainder is a
		// valid identifier (STYLE2024 forbids "DEVICE_TIER_1").
		{"DeviceTier", "1", "DEVICE_TIER_V1"},
		{"DeviceTier", "2A", "DEVICE_TIER_V2_A"},
		// Empty value -> just the prefix.
		{"ItemType", "", "ITEM_TYPE_"},
	}
	for _, c := range cases {
		got := ctx.EnumValue(c.enum, c.value)
		if got != c.want {
			t.Errorf("EnumValue(%q, %q) = %q, want %q",
				c.enum, c.value, got, c.want)
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
