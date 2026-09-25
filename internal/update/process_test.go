package update

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// A copied test executable stands in for the independent GUI helper. This
// exercises real child-process startup/waiting on Linux and Windows alike.
func TestMain(m *testing.M) {
	if mode := os.Getenv("FDB_TEST_UPDATER_CHILD"); mode != "" && len(os.Args) == 2 && filepath.Base(os.Args[1]) == "plan.json" {
		if mode == "delay" {
			time.Sleep(2 * time.Second)
		}
		if err := Ready(os.Args[1]); err != nil {
			os.Exit(2)
		}
		time.Sleep(400 * time.Millisecond)
		os.Exit(0)
	}
	os.Exit(m.Run())
}
func helperFixture(t *testing.T) string {
	t.Helper()
	_, plan := installFixture(t)
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	if err = copyFile(exe, filepath.Join(filepath.Dir(plan), "foreverdubbed-updater.exe"), 0755); err != nil {
		t.Fatal(err)
	}
	return plan
}
func TestHelperHandoffAndProcessWait(t *testing.T) {
	t.Setenv("FDB_TEST_UPDATER_CHILD", "ready")
	plan := helperFixture(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := Launch(ctx, plan); err != nil {
		t.Fatal(err)
	}
	p, err := LoadPlan(plan)
	if err != nil || p.HelperPID <= 0 {
		t.Fatal(p, err)
	}
	short, stop := context.WithTimeout(context.Background(), 20*time.Millisecond)
	if err = WaitForApp(short, Plan{ParentPID: p.HelperPID}); err == nil {
		t.Fatal("returned while child was still running")
	}
	stop()
	if err = WaitForApp(ctx, Plan{ParentPID: p.HelperPID}); err != nil {
		t.Fatal(err)
	}
}
func TestCancelHelperBeforeReady(t *testing.T) {
	t.Setenv("FDB_TEST_UPDATER_CHILD", "delay")
	plan := helperFixture(t)
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	if err := Launch(ctx, plan); err == nil {
		t.Fatal("canceled handoff succeeded")
	}
	if _, err := os.Stat(filepath.Join(filepath.Dir(plan), "ready")); !os.IsNotExist(err) {
		t.Fatal("canceled helper reached installation")
	}
}
