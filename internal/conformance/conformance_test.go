// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

package conformance

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"testing"
	"time"

	"github.com/csmarshall/schrodeck/internal/ports"
	"github.com/csmarshall/schrodeck/internal/ports/fake"
)

// recorder is a TB that records failures instead of failing the test, so a test can assert that a suite DOES fail on a known-bad implementation.
type recorder struct{ failures []string }

func (r *recorder) Helper() {}
func (r *recorder) Errorf(format string, args ...any) {
	r.failures = append(r.failures, fmt.Sprintf(format, args...))
}

// requireFailed is the RED half of every known-bad test: the suite must have recorded at least one failure, and the failures are logged so the run shows WHY it failed.
func requireFailed(t *testing.T, rec *recorder, what string) {
	t.Helper()
	if len(rec.failures) == 0 {
		t.Fatalf("suite passed %s", what)
	}
	for _, f := range rec.failures {
		t.Logf("suite correctly failed: %s", f)
	}
}

// harness timings are generous where waiting costs nothing on the good path (SettleMax, QuitTimeout) and tight only where the suite must observe a window (SettleQuiet).
func harness(t *testing.T, app ports.AppControl, dir string) AppHarness {
	t.Helper()
	return AppHarness{App: app, ProfilesDir: dir, QuitTimeout: 5 * time.Second, SettleQuiet: 200 * time.Millisecond, SettleMax: 15 * time.Second}
}

// --- AppControl ------------------------------------------------------------

func TestAppControlQuitOnFake(t *testing.T) {
	dir := t.TempDir()
	AppControlQuit(t, harness(t, fake.NewApp(dir), dir))
}

// writesAfterQuit violates contract A: Quit returns nil but the process keeps writing ProfilesDir.
type writesAfterQuit struct{ *fake.App }

func (w writesAfterQuit) Quit(timeout time.Duration) error {
	if err := w.App.Quit(timeout); err != nil {
		return err
	}
	go func() {
		time.Sleep(20 * time.Millisecond) // well inside the suite's post-quit window
		_ = os.WriteFile(filepath.Join(w.App.ProfilesDir, "late.json"), []byte("{}"), 0o644)
	}()
	return nil
}

func TestAppControlQuitCatchesWriteAfterQuit(t *testing.T) {
	dir := t.TempDir()
	rec := &recorder{}
	AppControlQuit(rec, harness(t, writesAfterQuit{fake.NewApp(dir)}, dir))
	requireFailed(t, rec, "an app that writes ProfilesDir after Quit returned nil")
}

// stillRunningAfterQuit violates contract A differently: Quit returns nil but Running still reports true.
type stillRunningAfterQuit struct{ *fake.App }

func (s stillRunningAfterQuit) Running() (bool, error) { return true, nil }

func TestAppControlQuitCatchesStillRunning(t *testing.T) {
	dir := t.TempDir()
	rec := &recorder{}
	AppControlQuit(rec, harness(t, stillRunningAfterQuit{fake.NewApp(dir)}, dir))
	requireFailed(t, rec, "an app that is still running after Quit returned nil")
}

func TestAppControlQuitTimeoutOnFake(t *testing.T) {
	app := fake.NewApp(t.TempDir())
	app.IgnoreQuit = true // set before Launch: the fake reads it under a mutex but this field is written without one
	AppControlQuitTimeout(t, app, 50*time.Millisecond)
}

// liesAboutQuit violates contract A: it reports success while still running.
type liesAboutQuit struct{ *fake.App }

func (l liesAboutQuit) Quit(timeout time.Duration) error {
	_ = l.App.Quit(timeout)
	return nil
}

func TestAppControlQuitTimeoutCatchesFalseSuccess(t *testing.T) {
	app := fake.NewApp(t.TempDir())
	app.IgnoreQuit = true
	rec := &recorder{}
	AppControlQuitTimeout(rec, liesAboutQuit{app}, 50*time.Millisecond)
	requireFailed(t, rec, "an app whose Quit returns nil while it is still running")
}

// givesUpEarly violates the timeout half of Quit: it errors well before the timeout it was given.
type givesUpEarly struct{ *fake.App }

func (g givesUpEarly) Quit(timeout time.Duration) error { return fake.ErrQuitTimeout }

func TestAppControlQuitTimeoutCatchesEarlyGiveUp(t *testing.T) {
	app := fake.NewApp(t.TempDir())
	app.IgnoreQuit = true
	rec := &recorder{}
	AppControlQuitTimeout(rec, givesUpEarly{app}, 300*time.Millisecond)
	requireFailed(t, rec, "an app whose Quit gives up before its timeout")
}

func TestAppControlLaunchSettleOnFake(t *testing.T) {
	dir := t.TempDir()
	AppControlLaunchSettle(t, harness(t, fake.NewApp(dir), dir))
}

// settlesImmediately violates contract A: WaitSettled returns as soon as the process is up, without waiting for ProfilesDir to go quiet. The good fake's WaitSettled cannot be seen to fail on its own (its Launch writes synchronously), so this known-bad is the evidence that the suite can fail at all.
type settlesImmediately struct{ *fake.App }

func (s settlesImmediately) WaitSettled(quietFor, max time.Duration) error { return nil }

func TestAppControlLaunchSettleCatchesNoWait(t *testing.T) {
	dir := t.TempDir()
	rec := &recorder{}
	AppControlLaunchSettle(rec, harness(t, settlesImmediately{fake.NewApp(dir)}, dir))
	requireFailed(t, rec, "an app whose WaitSettled ignores ongoing writes")
}

// --- Watcher ---------------------------------------------------------------

const debounce = 200 * time.Millisecond

func TestWatcherConformanceOnFake(t *testing.T) {
	w := fake.Watcher{Interval: 10 * time.Millisecond}
	WatcherRecursive(t, w, t.TempDir(), debounce)
	WatcherBurstIsOneEvent(t, w, t.TempDir(), debounce)
}

func TestWatcherConformanceRepeated(t *testing.T) {
	w := fake.Watcher{Interval: 10 * time.Millisecond}
	for i := 0; i < 5; i++ {
		WatcherBurstIsOneEvent(t, w, t.TempDir(), debounce)
	}
}

// flatSignature describes the direct entries of paths (size and mtime for files), the way a non-recursive implementation would see them.
func flatSignature(paths []string) map[string]string {
	m := map[string]string{}
	for _, p := range paths {
		entries, _ := os.ReadDir(p)
		for _, e := range entries {
			if info, err := e.Info(); err == nil && !e.IsDir() {
				m[filepath.Join(p, e.Name())] = fmt.Sprintf("%d:%d", info.Size(), info.ModTime().UnixNano())
			} else {
				m[filepath.Join(p, e.Name())] = "dir"
			}
		}
	}
	return m
}

// topLevelOnly violates contract A: it does not watch recursively.
type topLevelOnly struct{}

func (topLevelOnly) Watch(ctx context.Context, paths []string, debounce time.Duration) (<-chan ports.Event, error) {
	prev := flatSignature(paths)
	ch := make(chan ports.Event, 4)
	go func() {
		defer close(ch)
		for {
			select {
			case <-ctx.Done():
				return
			case <-time.After(10 * time.Millisecond):
				cur := flatSignature(paths)
				if fmt.Sprint(cur) != fmt.Sprint(prev) {
					prev = cur
					select {
					case ch <- ports.Event{Paths: paths}:
					case <-ctx.Done():
						return
					}
				}
			}
		}
	}()
	return ch, nil
}

func TestWatcherRecursiveCatchesTopLevelOnly(t *testing.T) {
	rec := &recorder{}
	WatcherRecursive(rec, topLevelOnly{}, t.TempDir(), debounce)
	requireFailed(t, rec, "a watcher that only sees the top level")
}

// perWrite violates contract A: no debounce, one event per changed file.
type perWrite struct{}

func (perWrite) Watch(ctx context.Context, paths []string, debounce time.Duration) (<-chan ports.Event, error) {
	prev := flatSignature(paths)
	ch := make(chan ports.Event, 64)
	go func() {
		defer close(ch)
		for {
			select {
			case <-ctx.Done():
				return
			case <-time.After(5 * time.Millisecond):
				cur := flatSignature(paths)
				var names []string
				for name, sig := range cur {
					if prev[name] != sig {
						names = append(names, name)
					}
				}
				sort.Strings(names)
				prev = cur
				for _, name := range names {
					select {
					case ch <- ports.Event{Paths: []string{name}}:
					case <-ctx.Done():
						return
					}
				}
			}
		}
	}()
	return ch, nil
}

func TestWatcherBurstCatchesPerWriteEvents(t *testing.T) {
	rec := &recorder{}
	WatcherBurstIsOneEvent(rec, perWrite{}, t.TempDir(), debounce)
	requireFailed(t, rec, "a watcher that sends one event per write")
}

// --- HostIdentity ----------------------------------------------------------

func TestHostIdentityOnFake(t *testing.T) {
	HostIdentityStable(t, fake.HostIdentity{Hardware: "HW-TEST-0001", User: "alice", Friendly: "test mac"})
}

// flappingIdentity violates contract A: the hardware id differs on every call.
type flappingIdentity struct {
	mu *sync.Mutex
	n  *int
}

func (f flappingIdentity) HardwareID() (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	*f.n++
	return fmt.Sprintf("HW-%d", *f.n), nil
}
func (f flappingIdentity) UserName() string     { return "alice" }
func (f flappingIdentity) FriendlyName() string { return "test mac" }

func TestHostIdentityCatchesUnstableID(t *testing.T) {
	rec := &recorder{}
	n := 0
	HostIdentityStable(rec, flappingIdentity{&sync.Mutex{}, &n})
	requireFailed(t, rec, "a hardware id that changes between calls")
}

// emptyIdentity violates contract A: no hardware id and no user.
type emptyIdentity struct{}

func (emptyIdentity) HardwareID() (string, error) { return "", nil }
func (emptyIdentity) UserName() string            { return "" }
func (emptyIdentity) FriendlyName() string        { return "" }

func TestHostIdentityCatchesEmptyID(t *testing.T) {
	rec := &recorder{}
	HostIdentityStable(rec, emptyIdentity{})
	requireFailed(t, rec, "an identity with an empty hardware id and user name")
}
