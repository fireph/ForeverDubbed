package releaseversion

import "testing"

func TestVersions(t *testing.T) {
	for _, bad := range []string{"", "1.2", "1.2.3.4", "1.2.-3", "1.2.3-beta", "1.02.3", "+1.2.3", "1.2.18446744073709551616"} {
		if _, err := Parse(bad); err == nil {
			t.Errorf("accepted %q", bad)
		}
	}
	if !Newer("v0.10.0", "0.9.9") || Newer("1.0.0", "1.0.0") || Newer("0.9.9", "0.10.0") {
		t.Fatal("incorrect version comparison")
	}
}

func TestComparisonValidatesBothVersionsCompletely(t *testing.T) {
	for _, bad := range []string{"2.bad.bad", "2.0.bad", "2.0.01", "2.-1.0", "2.0.0-beta", "2.0.18446744073709551616"} {
		if Newer(bad, "1.0.0") || Newer("3.0.0", bad) {
			t.Errorf("compared malformed version %q", bad)
		}
	}
	for _, text := range []string{"1.2.3", "v1.2.3"} {
		got, err := Parse(text)
		if err != nil || got != [3]uint64{1, 2, 3} {
			t.Fatalf("Parse(%q) = %v, %v", text, got, err)
		}
	}
}
