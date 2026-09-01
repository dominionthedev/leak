package event

import "testing"

func TestModifierValuesAreDistinctSingleBits(t *testing.T) {
	// Regression test: the const block used `1 << iota` with ModNone as
	// the first (explicit-value) line, which shifts iota by one and
	// wastes bit 0 — ModShift came out as 2, not 1. That happened to be
	// harmless internally (every use goes through the named constant),
	// but it's not what the block looks like it produces, and it stops
	// being harmless the moment anything assumes ModShift == 1.
	want := map[string]Modifier{
		"ModNone":  0,
		"ModShift": 1,
		"ModAlt":   2,
		"ModCtrl":  4,
		"ModMeta":  8,
	}
	got := map[string]Modifier{
		"ModNone":  ModNone,
		"ModShift": ModShift,
		"ModAlt":   ModAlt,
		"ModCtrl":  ModCtrl,
		"ModMeta":  ModMeta,
	}
	for name, w := range want {
		if got[name] != w {
			t.Errorf("%s = %d, want %d", name, got[name], w)
		}
	}
}

func TestModifierCombinable(t *testing.T) {
	combo := ModShift | ModCtrl
	if combo&ModShift == 0 {
		t.Error("expected ModShift bit set in combo")
	}
	if combo&ModCtrl == 0 {
		t.Error("expected ModCtrl bit set in combo")
	}
	if combo&ModAlt != 0 {
		t.Error("expected ModAlt bit not set in combo")
	}
}
