package updaterui

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"foreverdubbed/internal/update"
)

type fakeDialog struct {
	done    chan struct{}
	closed  bool
	textErr error
	values  []int
}

func (d *fakeDialog) Text(string) error     { return d.textErr }
func (d *fakeDialog) Value(v int) error     { d.values = append(d.values, v); return nil }
func (d *fakeDialog) Close() error          { d.closed = true; return nil }
func (d *fakeDialog) Done() <-chan struct{} { return d.done }

func TestUpdateLifecycle(t *testing.T) {
	boom := errors.New("test failure")
	for _, failure := range []string{"", "ready", "wait", "apply", "restart"} {
		t.Run("failure_"+failure, func(t *testing.T) {
			dialog := &fakeDialog{done: make(chan struct{})}
			var calls []string
			step := func(name string) error {
				if dialog.closed {
					t.Fatal("progress closed before update finished")
				}
				calls = append(calls, name)
				if failure == name {
					return boom
				}
				return nil
			}
			ops := operations{
				ready: func(string) error { return step("ready") },
				wait: func(ctx context.Context, _ update.Plan) error {
					if _, ok := ctx.Deadline(); !ok {
						t.Fatal("unbounded wait for parent")
					}
					return step("wait")
				},
				apply: func(_ string, report update.Reporter) error {
					report(update.Progress{Stage: "Installing", Done: 1, Total: 2})
					report(update.Progress{Stage: "Installing", Done: 2, Total: 2})
					return step("apply")
				},
				restart: func(string) error { return step("restart") },
			}
			err := run("plan.json", update.Plan{}, dialog, ops)
			if (failure == "" && err != nil) || (failure != "" && !errors.Is(err, boom)) {
				t.Fatalf("error=%v", err)
			}
			want := []string{"ready", "wait", "apply", "restart"}
			if failure != "" {
				for i, name := range want {
					if name == failure {
						want = want[:i+1]
						break
					}
				}
			}
			if !reflect.DeepEqual(calls, want) {
				t.Fatalf("calls=%v want=%v", calls, want)
			}
			if !dialog.closed {
				t.Fatal("progress left open")
			}
			for _, value := range dialog.values {
				if value >= 100 {
					t.Fatal("enabled native OK button before completion")
				}
			}
		})
	}
}

func TestDialogFailureDoesNotInterruptReplacement(t *testing.T) {
	for _, closeWindow := range []bool{false, true} {
		dialog := &fakeDialog{done: make(chan struct{})}
		applied, restarted := false, false
		ops := operations{
			ready: func(string) error { return nil },
			wait:  func(context.Context, update.Plan) error { return nil },
			apply: func(_ string, report update.Reporter) error {
				if closeWindow {
					close(dialog.done)
				} else {
					dialog.textErr = errors.New("window unavailable")
				}
				report(update.Progress{Stage: "Installing", Done: 1, Total: 2})
				applied = true
				return nil
			},
			restart: func(string) error { restarted = true; return nil },
		}
		if err := run("plan.json", update.Plan{}, dialog, ops); err != nil {
			t.Fatal(err)
		}
		if !applied || !restarted || !dialog.closed {
			t.Fatal("dialog failure interrupted update lifecycle")
		}
	}
}

func TestDialogMustStartBeforeReadiness(t *testing.T) {
	for _, closed := range []bool{false, true} {
		dialog := &fakeDialog{done: make(chan struct{})}
		if closed {
			close(dialog.done)
		} else {
			dialog.textErr = errors.New("no display")
		}
		ready := false
		ops := operations{ready: func(string) error { ready = true; return nil }}
		if err := run("plan.json", update.Plan{}, dialog, ops); err == nil {
			t.Fatal("accepted failed dialog")
		}
		if ready || !dialog.closed {
			t.Fatal("parent must keep running when progress fails to start")
		}
	}
}

func TestInvalidPlan(t *testing.T) {
	if err := Run("missing-plan.json"); err == nil {
		t.Fatal("accepted missing plan")
	}
}
