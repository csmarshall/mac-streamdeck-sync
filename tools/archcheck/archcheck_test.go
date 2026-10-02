// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

package archcheck

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

const mod = "example.com/root"

func pkg(path string, imports ...string) Package {
	return Package{ImportPath: path, Module: &Module{Path: mod}, Imports: imports}
}

func TestCoreViolations(t *testing.T) {
	pkgs := []Package{
		pkg(mod+"/internal/core", "fmt", "os", "path/filepath"),                                         // fine: os is allowed
		pkg(mod+"/internal/bad", "os/exec"),                                                             // OS-specific
		pkg(mod+"/internal/bad2", "golang.org/x/sys/unix"),                                              // OS-specific (prefix)
		pkg(mod+"/internal/bad3", mod+"/internal/connector/macos"),                                      // core importing a connector
		pkg(mod+"/internal/connector/macos", "os/exec"),                                                 // connectors are exempt
		pkg(mod+"/cmd/tool", "os/signal"),                                                               // commands are exempt
		pkg(mod+"/tools/x", "os/exec"),                                                                  // tools are exempt
		{ImportPath: mod + "/internal/t", Module: &Module{Path: mod}, TestImports: []string{"syscall"}}, // tests count too
		{ImportPath: "other.com/lib", Module: &Module{Path: "other.com"}, Imports: []string{"os/exec"}}, // other modules ignored
	}
	violators := map[string]bool{}
	for _, v := range CoreViolations(pkgs, mod) {
		violators[strings.SplitN(v, " imports ", 2)[0]] = true
	}
	want := []string{mod + "/internal/bad", mod + "/internal/bad2", mod + "/internal/bad3", mod + "/internal/t"}
	if len(violators) != len(want) {
		t.Errorf("violating packages %v, want exactly %v", violators, want)
	}
	for _, w := range want {
		if !violators[w] {
			t.Errorf("%s not reported", w)
		}
	}
}

func TestBoundaryViolations(t *testing.T) {
	pkgs := []Package{
		{ImportPath: mod + "/sub/x", Module: &Module{Path: mod + "/sub"}},
		{ImportPath: "fmt"},
		{ImportPath: mod + "/internal/core", Module: &Module{Path: mod}},
	}
	got := BoundaryViolations(pkgs, mod)
	if len(got) != 1 || !strings.Contains(got[0], mod+"/internal/core") {
		t.Fatalf("BoundaryViolations = %v", got)
	}
	if v := BoundaryViolations(pkgs[:2], mod); len(v) != 0 {
		t.Fatalf("clean dependency set reported %v", v)
	}
}

func write(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// End to end against real `go list`: a nested module that imports its
// parent must be reported (the ADR 0031 known-bad).
func TestBoundaryEndToEnd(t *testing.T) {
	root := t.TempDir()
	write(t, filepath.Join(root, "go.mod"), "module example.com/root\n\ngo 1.27\n")
	write(t, filepath.Join(root, "p", "p.go"), "package p\n\nconst X = 1\n")
	write(t, filepath.Join(root, "sub", "go.mod"), "module example.com/root/sub\n\ngo 1.27\n\nrequire example.com/root v0.0.0\n\nreplace example.com/root => ../\n")
	write(t, filepath.Join(root, "sub", "s.go"), "package sub\n\nimport \"example.com/root/p\"\n\nconst Y = p.X\n")

	forbidden, err := ModulePath(root)
	if err != nil {
		t.Fatal(err)
	}
	pkgs, err := List(filepath.Join(root, "sub"), []string{"GOFLAGS=-mod=mod"}, "-deps", "./...")
	if err != nil {
		t.Fatal(err)
	}
	if v := BoundaryViolations(pkgs, forbidden); len(v) == 0 {
		t.Fatal("a nested module importing its parent was not reported")
	}
}

// End to end: a core package that imports os/exec only on darwin must be
// reported when listed for darwin, which is why main.go lists every GOOS.
func TestCoreEndToEndPerGOOS(t *testing.T) {
	root := t.TempDir()
	write(t, filepath.Join(root, "go.mod"), "module example.com/root\n\ngo 1.27\n")
	write(t, filepath.Join(root, "internal", "core", "core.go"), "package core\n\nconst X = 1\n")
	write(t, filepath.Join(root, "internal", "core", "core_darwin.go"), "package core\n\nimport _ \"os/exec\"\n")
	for goos, wantViolation := range map[string]bool{"linux": false, "darwin": true} {
		pkgs, err := List(root, []string{"GOOS=" + goos}, "./...")
		if err != nil {
			t.Fatal(err)
		}
		got := CoreViolations(pkgs, "example.com/root")
		if (len(got) > 0) != wantViolation {
			t.Errorf("GOOS=%s: violations %v, want violation=%v", goos, got, wantViolation)
		}
	}
}

// runArchcheck builds the command and runs it (`go run` collapses every nonzero child exit to 1, which would hide the exit-code contract). The test's working directory is tools/archcheck.
func runArchcheck(t *testing.T, args ...string) (stdout string, exitCode int) {
	t.Helper()
	bin := filepath.Join(t.TempDir(), "archcheck")
	if out, err := exec.Command("go", "build", "-o", bin, "./cmd/archcheck").CombinedOutput(); err != nil {
		t.Fatalf("build: %v\n%s", err, out)
	}
	cmd := exec.Command(bin, args...)
	var out bytes.Buffer
	cmd.Stdout = &out
	err := cmd.Run()
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return out.String(), exitErr.ExitCode()
	}
	if err != nil {
		t.Fatal(err)
	}
	return out.String(), 0
}

// -print-goos is the single home of the GOOS list; ci.yml's cross-build loop reads it from here.
func TestPrintGOOS(t *testing.T) {
	out, code := runArchcheck(t, "-print-goos")
	if code != 0 {
		t.Fatalf("exit %d, want 0", code)
	}
	if got, want := strings.Fields(out), []string{"darwin", "linux", "windows"}; !slices.Equal(got, want) {
		t.Fatalf("-print-goos printed %q, want one per line %v", got, want)
	}
	if strings.Count(out, "\n") != 3 {
		t.Fatalf("-print-goos output %q is not exactly one GOOS per line", out)
	}
}

func TestUsageErrorExitsTwo(t *testing.T) {
	if _, code := runArchcheck(t, "-mode", "nonsense"); code != 2 {
		t.Fatalf("exit %d, want 2", code)
	}
}
