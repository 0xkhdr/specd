package security

import "testing"

// goModFixture loads a *.mod fixture but presents it under a go.mod path so the
// scanner's manifest filter (basename go.mod) accepts it.
func goModFixture(t *testing.T, rel string) TrackedFile {
	t.Helper()
	f := readFixture(t, rel)
	return TrackedFile{Path: "go.mod", Content: f.Content}
}

func TestSlopsquat(t *testing.T) {
	t.Run("flags_typosquat_of_popular_module", func(t *testing.T) {
		findings := slopsquatScanner{}.Scan([]TrackedFile{goModFixture(t, "slopsquat/typo.mod")})
		if len(findings) != 1 {
			t.Fatalf("expected exactly one typosquat finding, got %+v", findings)
		}
		if findings[0].Rule != "typosquat" {
			t.Fatalf("rule = %s", findings[0].Rule)
		}
	})

	t.Run("exact_match_is_not_a_finding", func(t *testing.T) {
		if f := (slopsquatScanner{}).Scan([]TrackedFile{goModFixture(t, "slopsquat/exact.mod")}); len(f) != 0 {
			t.Fatalf("exact popular deps should not be flagged: %+v", f)
		}
	})

	t.Run("non_gomod_files_skipped", func(t *testing.T) {
		if f := (slopsquatScanner{}).Scan([]TrackedFile{{Path: "notes.txt", Content: []byte("golang.org/x/tolls\n")}}); len(f) != 0 {
			t.Fatalf("only manifests are parsed: %+v", f)
		}
	})

	t.Run("damerau_levenshtein_thresholds", func(t *testing.T) {
		cases := []struct {
			candidate, pop string
			want           bool
		}{
			{"golang.org/x/tolls", "golang.org/x/tools", true}, // 1 edit, long
			{"github.com/pkg/erors", "github.com/pkg/errors", true},
			{"github.com/pkg/errors", "github.com/pkg/errors", false}, // exact
			{"completely/different", "golang.org/x/tools", false},
		}
		for _, c := range cases {
			if got := withinTypoDistance(c.candidate, c.pop); got != c.want {
				t.Errorf("withinTypoDistance(%q,%q) = %v, want %v", c.candidate, c.pop, got, c.want)
			}
		}
	})
}
