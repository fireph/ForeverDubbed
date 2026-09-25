//go:build !windows

package update

import "testing"

func TestMacDesignatedRequirement(t *testing.T) {
	const requirement = `identifier "io.foreverdubbed.companion" and certificate leaf = H"abc123"`
	for _, prefix := range []string{"designated => ", "# designated => ", "  # designated => "} {
		got, err := designatedRequirement("Executable=/Applications/ForeverDubbed.app/Contents/MacOS/foreverdubbed\n" + prefix + requirement + "\n")
		if err != nil || got != requirement {
			t.Fatalf("%q: %s %v", prefix, got, err)
		}
	}
	if _, err := designatedRequirement("code object is not signed at all"); err == nil {
		t.Fatal("accepted unsigned app")
	}
}
