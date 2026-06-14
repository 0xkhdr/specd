package core

import "testing"

func TestEnvInt(t *testing.T) {
	const name = "SPECD_TEST_ENVINT"

	cases := []struct {
		desc        string
		set         bool
		val         string
		def, lo, hi int
		want        int
	}{
		{desc: "unset returns default", set: false, def: 42, lo: 0, hi: 0, want: 42},
		{desc: "valid in range", set: true, val: "100", def: 5, lo: 0, hi: 0, want: 100},
		{desc: "clamps below min", set: true, val: "-3", def: 5, lo: 1, hi: 0, want: 1},
		{desc: "clamps above max", set: true, val: "9999", def: 5, lo: 1, hi: 600, want: 600},
		{desc: "no upper bound when max<=0", set: true, val: "9999", def: 5, lo: 1, hi: 0, want: 9999},
		{desc: "malformed falls back to default", set: true, val: "1000ms", def: 600, lo: 1, hi: 0, want: 600},
	}

	for _, tc := range cases {
		t.Run(tc.desc, func(t *testing.T) {
			// Empty value is treated as unset by EnvInt, matching the unset case.
			t.Setenv(name, tc.val)
			if got := EnvInt(name, tc.def, tc.lo, tc.hi); got != tc.want {
				t.Errorf("EnvInt(%q=%q) = %d, want %d", name, tc.val, got, tc.want)
			}
		})
	}
}
