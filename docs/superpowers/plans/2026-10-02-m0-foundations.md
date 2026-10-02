# M0 Foundations Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Stand up the schrodeck repository's Go scaffold and its CI gate: a `schrodeck` CLI skeleton whose `--json` output is contract E, the OS port interfaces from contract A with fakes and a conformance-suite skeleton, an empty `deckformat` module, and CI checks that each have been seen to fail on a known-bad input.

**Architecture:** One Go module (`github.com/csmarshall/schrodeck`) holds the platform-neutral core, the CLI and the port interfaces; a second, nested module (`github.com/csmarshall/schrodeck/deckformat`) is created empty so its import boundary (ADR 0031) is enforced from the first commit. Nothing in M0 reads or writes the Stream Deck app's files. CI runs every check on Linux, and the tests again on macOS.

**Tech Stack:** Go 1.27.1 (stdlib only: `log/slog`, `flag`, `encoding/json`, `go/parser`), staticcheck v0.8.1, GitHub Actions (`actions/checkout` v7.0.1, `actions/setup-go` v7.0.0, pinned by commit SHA), bash.

**Spec:** [docs/specs/2026-10-01-sync-design.md](../../specs/2026-10-01-sync-design.md). Delivery order: [docs/adr/README.md § Delivery order](../../adr/README.md#delivery-order), row M0. Tracking issue: #3.

## Global Constraints

- **PUBLIC REPO.** No hostnames, usernames, real home paths, device serials, `Device.UUID` values, tokens or real profile manifests in any file, test, commit message or PR body. Placeholders: `<user>`, `<host>`, `<deck>`. Tests that need a home path or a serial-bearing device id build the string by concatenation (`"/Users/" + "alice"`, `"@(1)[4057/143/" + "AB12CD34EF" + "]"`) so the repository's own leak scan stays clean.
- **Never write a backslash-u escape literally** in Go source, test data or docs produced from this plan; build it as `"\x5c" + "u00e9"`. (Some tooling in this project's authoring path rewrites such sequences into the character, which silently changes a test's meaning.)
- Go: `go 1.27` with `toolchain go1.27.1` in every `go.mod`. CI reads the version from `go.mod` (`go-version-file`), so the version has one home.
- Module paths: `github.com/csmarshall/schrodeck` (root) and `github.com/csmarshall/schrodeck/deckformat` (nested, own `go.mod`, ADR 0031). No `go.work` file: `deckformat` must build on its own, and the root module will reach it through a `replace` directive when it first imports it (M1).
- Every source file (`.go`, `.sh`, `.swift`) starts with the MPL-2.0 Exhibit A header (after a shebang line, if any). The Go form is exactly:
  ```go
  // This Source Code Form is subject to the terms of the Mozilla Public
  // License, v. 2.0. If a copy of the MPL was not distributed with this
  // file, You can obtain one at https://mozilla.org/MPL/2.0/.
  ```
  Shell scripts use the same three lines with `# `.
- **Core packages** (everything in the root module except `cmd/...`, `internal/connector/...` and `tools/...`) import no OS-specific package (`os/exec`, `os/signal`, `os/user`, `syscall`, `golang.org/x/sys/...`, cgo) and no connector, command or tool package (ADR 0018). `os`, `io/fs` and `path/filepath` are allowed.
- **`deckformat` imports no package from the root module** (ADR 0031).
- **The port list lives only in [docs/contracts/os-connector.md](../../contracts/os-connector.md)** (contract A). Code mirrors it; prose elsewhere links to it and never restates the list or its count.
- **Contract E:** every `--json` document is one JSON object on one line on stdout, with `schema_version` (currently `1`) first. Logs go to stderr only. Adding a field is non-breaking; removing, renaming or retyping one bumps `schema_version`.
- Exit codes: `0` success, `1` the command ran and reports failure, `2` usage error.
- Logging: `log/slog` text to stderr; every record has a UTC RFC 3339 timestamp, a level and context attributes (`component=…`); the level comes from `SCHRODECK_LOG_LEVEL` (`debug|info|warn|error`, default `info`). Never log a raw hardware id, serial or token.
- Shell scripts: `#!/usr/bin/env bash`, `unset TMOUT`, `set -euo pipefail`, a usage line in the header comment, bash 3.2 compatible (macOS `/bin/bash`: no `mapfile`, no associative arrays). Never pipe a producer into an early-exiting consumer (`| grep -q`, `| head -1`) under `pipefail`; use `grep -c … -gt 0` or `sed -n 1p`.
- Every detector (CI check) has a known-bad input it is seen to fail on and a known-good input it is seen to pass on, both run in CI.
- Terminology: "revision" is a store record; "commit" only ever means git. Not relevant to M0 code, but no identifier may use "commit" for a store concept.
- Prose (docs, PR bodies, commit bodies) is not hard-wrapped.
- Workflow: issue #3 → worktree `~/work/claude/schrodeck-worktrees/3-scaffold` on branch `3-scaffold` → one commit per task (`feat(#3): …`, `ci(#3): …`, `docs(#3): …`) → one PR with `Closes #3` → green CI → code-review subagent on the diff → owner review → squash merge. M0 is a single PR because CI does not exist until it lands.

## Review Focus

1. **Stdout purity under `--json`:** with `SCHRODECK_LOG_LEVEL=debug`, a UI parsing stdout must still get exactly one JSON line, including when the command reports failure. Pinned by `TestJSONStdoutIsOneLineEvenWithDebugLogs` in Task 3.
2. **Flag placement and unknown input:** `schrodeck --json status`, `schrodeck status --bogus`, `schrodeck nope` and bare `schrodeck` must each exit `2` with a message on stderr, never panic, never print a partial JSON document. Pinned by `TestUsageErrors` in Task 3.
3. **Fork pull requests have no secrets:** the leak scan must then still run its generic patterns and say plainly that the personal patterns were not checked, rather than reporting an unqualified "clean". Pinned by the `leak-scan reports generic-only mode` case in Task 8's self-test.
4. **The whole module builds for every OS we claim to keep possible,** not only the one the developer runs: a darwin-only file sneaking an OS import into a core package is invisible to `go list` on Linux. Pinned by Task 7 running the core check under `GOOS=darwin`, `linux` and `windows`, and Task 9's cross-build step.
5. **Timing-sensitive conformance tests on a loaded CI runner:** the watcher's "a burst within the debounce window is one event" assertion must not flake when the runner is slow. Pinned by `TestWatcherConformanceRepeated` in Task 6 (the suite run five times against the fake).

---

## File structure

```
schrodeck/
├── go.mod                                  root module; go 1.27, toolchain go1.27.1
├── cmd/schrodeck/main.go                   process entry: logger from env, signals, cli.Run
├── internal/
│   ├── version/version.go                  build version (set by -ldflags in M6)
│   ├── logging/logging.go (+_test)         slog setup, SCHRODECK_LOG_LEVEL
│   ├── cli/                                command router and contract E envelope
│   │   ├── cli.go  json.go  commands.go
│   │   ├── cli_test.go
│   │   └── testdata/golden/*.json
│   ├── ports/ports.go                      contract A interfaces (mirror of os-connector.md)
│   ├── ports/fake/                         in-memory / temp-dir fakes for every port
│   │   ├── fake.go  app.go  watcher.go  fake_test.go
│   └── conformance/                        suites written once against the ports
│       ├── conformance.go  appcontrol.go  watcher.go  identity.go
│       └── conformance_test.go             runs suites on fakes; known-bad fakes must fail
├── deckformat/
│   ├── go.mod                              nested module (ADR 0031)
│   └── doc.go
├── tools/
│   ├── archcheck/  main.go archcheck.go archcheck_test.go
│   ├── portcheck/  main.go portcheck.go portcheck_test.go
│   └── ci/  check-headers.sh  check-gofmt.sh  leak-scan.sh  selftest.sh
├── .github/workflows/ci.yml
└── docs/contracts/cli-json.md              contract E, new in this milestone
```

---

### Task 0: Worktree

- [ ] **Step 1: Create the worktree for issue #3**

```bash
cd ~/work/personal/schrodeck
git fetch origin
git worktree add -b 3-scaffold ~/work/claude/schrodeck-worktrees/3-scaffold origin/main
cd ~/work/claude/schrodeck-worktrees/3-scaffold
```

Every later step runs from this directory.

---

### Task 1: Module skeleton and the MPL header check

**Files:**
- Create: `go.mod`, `internal/version/version.go`, `cmd/schrodeck/main.go` (temporary body, replaced in Task 3), `deckformat/go.mod`, `deckformat/doc.go`, `tools/ci/check-headers.sh`, `tools/ci/selftest.sh`
- Modify: `.gitignore` (add the built binary)

**Interfaces:**
- Produces: `version.Version string` (package `github.com/csmarshall/schrodeck/internal/version`); `tools/ci/check-headers.sh [file...]` exits 0 when every file has the header, 1 otherwise; `tools/ci/selftest.sh` runs every detector self-test and exits non-zero on the first wrong verdict count.

- [ ] **Step 1: Write the header checker's self-test first**

Create `tools/ci/selftest.sh`:

```bash
#!/usr/bin/env bash
# This Source Code Form is subject to the terms of the Mozilla Public
# License, v. 2.0. If a copy of the MPL was not distributed with this
# file, You can obtain one at https://mozilla.org/MPL/2.0/.
#
# Self-tests for the CI detectors in tools/ci/. Each detector is run against a
# known-good input (must pass) and at least one known-bad input (must fail).
# A detector that has never been seen to fail has not been tested.
# Usage: tools/ci/selftest.sh   (from the repository root)
unset TMOUT
set -euo pipefail

here=$(cd "$(dirname "$0")" && pwd)
failures=0
scratch=$(mktemp -d)
trap 'rm -rf "$scratch"' EXIT

# expect <wanted-exit-code> <description> <command...>
expect() {
  local want=$1 desc=$2
  shift 2
  local got=0
  "$@" >"$scratch/last.out" 2>&1 || got=$?
  if [[ $got -eq $want ]]; then
    echo "ok    $desc"
  else
    echo "FAIL  $desc (exit $got, want $want)"
    sed 's/^/      | /' "$scratch/last.out"
    failures=$((failures + 1))
  fi
}

header_go() {
  printf '%s\n' \
    '// This Source Code Form is subject to the terms of the Mozilla Public' \
    '// License, v. 2.0. If a copy of the MPL was not distributed with this' \
    '// file, You can obtain one at https://mozilla.org/MPL/2.0/.'
}

# --- check-headers.sh -------------------------------------------------------
{ header_go; printf 'package good\n'; } >"$scratch/good.go"
printf 'package bad\n' >"$scratch/bad.go"
{
  printf '#!/usr/bin/env bash\n'
  printf '# This Source Code Form is subject to the terms of the Mozilla Public\n'
  printf '# License, v. 2.0. If a copy of the MPL was not distributed with this\n'
  printf '# file, You can obtain one at https://mozilla.org/MPL/2.0/.\n'
} >"$scratch/good.sh"
expect 0 "check-headers passes a Go file with the header" "$here/check-headers.sh" "$scratch/good.go"
expect 0 "check-headers passes a script with shebang + header" "$here/check-headers.sh" "$scratch/good.sh"
expect 1 "check-headers fails a Go file without the header" "$here/check-headers.sh" "$scratch/bad.go"
expect 1 "check-headers fails when one of several files lacks it" "$here/check-headers.sh" "$scratch/good.go" "$scratch/bad.go"

if [[ $failures -gt 0 ]]; then
  echo "selftest: $failures detector verdict(s) wrong" >&2
  exit 1
fi
echo "selftest: all detector verdicts as expected"
```

```bash
chmod +x tools/ci/selftest.sh
```

- [ ] **Step 2: Run it to verify it fails**

Run: `tools/ci/selftest.sh`
Expected: FAIL lines for every case (exit 127, "No such file or directory"), final exit 1.

- [ ] **Step 3: Write the checker**

Create `tools/ci/check-headers.sh`:

```bash
#!/usr/bin/env bash
# This Source Code Form is subject to the terms of the Mozilla Public
# License, v. 2.0. If a copy of the MPL was not distributed with this
# file, You can obtain one at https://mozilla.org/MPL/2.0/.
#
# Fails if a source file lacks the MPL-2.0 Exhibit A header in its first 5 lines
# (CLAUDE.md: "MPL-2.0 Exhibit A header on every source file").
# Usage: tools/ci/check-headers.sh [file...]
#        With no arguments, checks every tracked .go, .sh and .swift file.
unset TMOUT
set -euo pipefail

readonly MARKER='This Source Code Form is subject to the terms of the Mozilla Public'
missing=0
checked=0

check() {
  local f=$1 hits
  checked=$((checked + 1))
  hits=$(head -n 5 "$f" | grep -c -F "$MARKER" || true)
  if [[ $hits -eq 0 ]]; then
    echo "missing MPL-2.0 header: $f" >&2
    missing=$((missing + 1))
  fi
}

if [[ $# -gt 0 ]]; then
  for f in "$@"; do check "$f"; done
else
  while IFS= read -r f; do check "$f"; done < <(git ls-files '*.go' '*.sh' '*.swift')
fi

if [[ $missing -gt 0 ]]; then
  echo "check-headers: $missing of $checked file(s) lack the header" >&2
  exit 1
fi
echo "check-headers: $checked file(s) ok"
```

```bash
chmod +x tools/ci/check-headers.sh
```

- [ ] **Step 4: Run the self-test to verify it passes**

Run: `tools/ci/selftest.sh`
Expected: four `ok` lines, `selftest: all detector verdicts as expected`, exit 0.

- [ ] **Step 5: Create the modules and the entry point**

`go.mod`:

```
module github.com/csmarshall/schrodeck

go 1.27

toolchain go1.27.1
```

`internal/version/version.go`:

```go
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

// Package version holds the build's version string. Release builds set it with
// -ldflags "-X github.com/csmarshall/schrodeck/internal/version.Version=<tag>"
// (ADR 0028); every other build reports "dev".
package version

// Version is the schrodeck version this binary was built from.
var Version = "dev"
```

`cmd/schrodeck/main.go` (temporary; Task 3 replaces the body):

```go
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

// Command schrodeck keeps Stream Deck setups identical across computers.
package main

import (
	"fmt"

	"github.com/csmarshall/schrodeck/internal/version"
)

func main() {
	fmt.Println("schrodeck", version.Version)
}
```

`deckformat/go.mod`:

```
module github.com/csmarshall/schrodeck/deckformat

go 1.27

toolchain go1.27.1
```

`deckformat/doc.go`:

```go
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

// Package deckformat is a toolkit for reading, comparing and checking the
// Stream Deck app's on-disk profile format (ADR 0031). It is a separate Go
// module so that it can be extracted into its own repository; it must never
// import a package of the schrodeck module (a CI check enforces this).
//
// Interoperability only: it reads and writes the user's own configuration
// files and never decrypts Elgato's plugin bundles or redistributes Elgato
// code or assets.
package deckformat
```

Append to `.gitignore`:

```
# Local build output
/schrodeck
```

- [ ] **Step 6: Verify both modules build and every file has its header**

Run: `git add -A && go build ./... && (cd deckformat && go build ./...) && go run ./cmd/schrodeck && tools/ci/check-headers.sh`
Expected: `schrodeck dev`, then `check-headers: 5 file(s) ok` (`cmd/schrodeck/main.go`, `internal/version/version.go`, `deckformat/doc.go`, `tools/ci/check-headers.sh`, `tools/ci/selftest.sh`). The checker reads tracked files, hence the `git add -A` first.

- [ ] **Step 7: Commit**

```bash
git add -A
git commit -m "feat(#3): Go module skeleton, nested deckformat module, MPL header check with self-test"
```

---

### Task 2: Logging

**Files:**
- Create: `internal/logging/logging.go`, `internal/logging/logging_test.go`

**Interfaces:**
- Produces: `logging.EnvLevel = "SCHRODECK_LOG_LEVEL"`; `logging.ParseLevel(s string) (slog.Level, error)`; `logging.New(w io.Writer, levelText string) (*slog.Logger, error)`.

- [ ] **Step 1: Write the failing tests**

`internal/logging/logging_test.go`:

```go
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

package logging

import (
	"bytes"
	"log/slog"
	"regexp"
	"strings"
	"testing"
)

func TestParseLevel(t *testing.T) {
	cases := []struct {
		in      string
		want    slog.Level
		wantErr bool
	}{
		{"", slog.LevelInfo, false},
		{"debug", slog.LevelDebug, false},
		{"INFO", slog.LevelInfo, false},
		{" warn ", slog.LevelWarn, false},
		{"warning", slog.LevelWarn, false},
		{"error", slog.LevelError, false},
		{"verbose", 0, true},
	}
	for _, c := range cases {
		got, err := ParseLevel(c.in)
		if (err != nil) != c.wantErr {
			t.Errorf("ParseLevel(%q) error = %v, wantErr %v", c.in, err, c.wantErr)
			continue
		}
		if err == nil && got != c.want {
			t.Errorf("ParseLevel(%q) = %v, want %v", c.in, got, c.want)
		}
	}
}

func TestParseLevelErrorNamesTheVariable(t *testing.T) {
	_, err := ParseLevel("loud")
	if err == nil || !strings.Contains(err.Error(), EnvLevel) {
		t.Fatalf("error %v should name %s so the user knows what to fix", err, EnvLevel)
	}
}

func TestNewFiltersBelowLevelAndKeepsContext(t *testing.T) {
	var buf bytes.Buffer
	log, err := New(&buf, "warn")
	if err != nil {
		t.Fatal(err)
	}
	log.Info("hidden")
	log.Warn("shown", "component", "test")
	out := buf.String()
	if strings.Contains(out, "hidden") {
		t.Errorf("info record logged at warn level: %q", out)
	}
	for _, want := range []string{"level=WARN", "msg=shown", "component=test", "time="} {
		if !strings.Contains(out, want) {
			t.Errorf("output %q lacks %q", out, want)
		}
	}
}

func TestTimestampIsUTCRFC3339(t *testing.T) {
	var buf bytes.Buffer
	log, err := New(&buf, "info")
	if err != nil {
		t.Fatal(err)
	}
	log.Info("x")
	if !regexp.MustCompile(`^time=\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(\.\d+)?Z `).MatchString(buf.String()) {
		t.Fatalf("timestamp is not UTC RFC 3339: %q", buf.String())
	}
}
```

- [ ] **Step 2: Run to verify failure**

Run: `go test ./internal/logging/`
Expected: FAIL, `undefined: ParseLevel` (and `New`, `EnvLevel`).

- [ ] **Step 3: Implement**

`internal/logging/logging.go`:

```go
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

// Package logging configures schrodeck's structured logs: every record has a
// UTC timestamp, a level and context attributes, and the level is set by the
// SCHRODECK_LOG_LEVEL environment variable.
package logging

import (
	"fmt"
	"io"
	"log/slog"
	"strings"
	"time"
)

// EnvLevel is the environment variable that sets the minimum log level.
const EnvLevel = "SCHRODECK_LOG_LEVEL"

// DefaultLevel applies when EnvLevel is unset or empty.
const DefaultLevel = slog.LevelInfo

// ParseLevel turns the text of EnvLevel into a level.
func ParseLevel(s string) (slog.Level, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "":
		return DefaultLevel, nil
	case "debug":
		return slog.LevelDebug, nil
	case "info":
		return slog.LevelInfo, nil
	case "warn", "warning":
		return slog.LevelWarn, nil
	case "error":
		return slog.LevelError, nil
	}
	return 0, fmt.Errorf("%s=%q: want debug, info, warn or error", EnvLevel, s)
}

// New returns a logger writing logfmt-style text records to w.
func New(w io.Writer, levelText string) (*slog.Logger, error) {
	level, err := ParseLevel(levelText)
	if err != nil {
		return nil, err
	}
	h := slog.NewTextHandler(w, &slog.HandlerOptions{Level: level, ReplaceAttr: utcTime})
	return slog.New(h), nil
}

// utcTime renders the record time in UTC so logs from different Macs line up.
func utcTime(groups []string, a slog.Attr) slog.Attr {
	if a.Key == slog.TimeKey && len(groups) == 0 {
		a.Value = slog.StringValue(a.Value.Time().UTC().Format(time.RFC3339Nano))
	}
	return a
}
```

- [ ] **Step 4: Run to verify it passes**

Run: `go test ./internal/logging/`
Expected: `ok  github.com/csmarshall/schrodeck/internal/logging`

- [ ] **Step 5: Commit**

```bash
git add internal/logging
git commit -m "feat(#3): structured logging with UTC timestamps and SCHRODECK_LOG_LEVEL"
```

---

### Task 3: CLI skeleton and contract E

**Files:**
- Create: `internal/cli/cli.go`, `internal/cli/json.go`, `internal/cli/commands.go`, `internal/cli/cli_test.go`, `internal/cli/testdata/golden/{version,status,doctor-empty,doctor-fail}.json`, `docs/contracts/cli-json.md`
- Modify: `cmd/schrodeck/main.go` (replace body), `docs/contracts/README.md` (row E "Where" column), `docs/adr/0024-documented-contracts.md` (the consequence line that says E still lives in ADR 0018)

**Interfaces:**
- Consumes: `logging.New`, `logging.EnvLevel`, `version.Version`.
- Produces:
  - `cli.SchemaVersion = 1`
  - `cli.Env{Stdout, Stderr io.Writer; Version string; Logger *slog.Logger; Checks []cli.Check}`
  - `cli.Check{ID string; Run func(ctx context.Context) cli.CheckResult}`, `cli.CheckResult{ID, Status, Detail string}` with `Status` one of `cli.StatusPass|StatusFail|StatusSkip` (M1 replaces `Checks` with probes built from the connector).
  - `cli.Run(ctx context.Context, args []string, env cli.Env) int`
  - Exit codes `cli.ExitOK=0`, `cli.ExitFail=1`, `cli.ExitUsage=2`.

- [ ] **Step 1: Write the golden files (the expected contract E documents)**

`internal/cli/testdata/golden/version.json`:

```json
{"schema_version":1,"command":"version","ok":true,"data":{"schrodeck_version":"test"}}
```

`internal/cli/testdata/golden/status.json`:

```json
{"schema_version":1,"command":"status","ok":true,"data":{"schrodeck_version":"test"}}
```

`internal/cli/testdata/golden/doctor-empty.json`:

```json
{"schema_version":1,"command":"doctor","ok":true,"data":{"checks":[]}}
```

`internal/cli/testdata/golden/doctor-fail.json`:

```json
{"schema_version":1,"command":"doctor","ok":false,"data":{"checks":[{"id":"A1","status":"pass"},{"id":"A2","status":"fail","detail":"broken on purpose"}]}}
```

Each file ends with exactly one newline (the encoder's).

- [ ] **Step 2: Write the failing tests**

`internal/cli/cli_test.go`:

```go
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

var update = flag.Bool("update", false, "rewrite golden files from current output")

func testEnv(logTo io.Writer, checks ...Check) (Env, *bytes.Buffer, *bytes.Buffer) {
	var out, errb bytes.Buffer
	if logTo == nil {
		logTo = io.Discard
	}
	logger := slog.New(slog.NewTextHandler(logTo, &slog.HandlerOptions{Level: slog.LevelDebug}))
	return Env{Stdout: &out, Stderr: &errb, Version: "test", Logger: logger, Checks: checks}, &out, &errb
}

func fixedCheck(id, status, detail string) Check {
	return Check{ID: id, Run: func(context.Context) CheckResult {
		return CheckResult{ID: id, Status: status, Detail: detail}
	}}
}

func TestGoldenJSON(t *testing.T) {
	cases := []struct {
		golden   string
		args     []string
		checks   []Check
		wantCode int
	}{
		{"version", []string{"version", "--json"}, nil, ExitOK},
		{"status", []string{"status", "--json"}, nil, ExitOK},
		{"doctor-empty", []string{"doctor", "--json"}, nil, ExitOK},
		{"doctor-fail", []string{"doctor", "--json"},
			[]Check{fixedCheck("A1", StatusPass, ""), fixedCheck("A2", StatusFail, "broken on purpose")}, ExitFail},
	}
	for _, c := range cases {
		t.Run(c.golden, func(t *testing.T) {
			env, out, errb := testEnv(nil, c.checks...)
			code := Run(context.Background(), c.args, env)
			if code != c.wantCode {
				t.Fatalf("exit %d, want %d; stderr %q", code, c.wantCode, errb.String())
			}
			path := filepath.Join("testdata", "golden", c.golden+".json")
			if *update {
				if err := os.WriteFile(path, out.Bytes(), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			want, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(out.Bytes(), want) {
				t.Fatalf("stdout\n%s\nwant (golden %s)\n%s", out.Bytes(), path, want)
			}
		})
	}
}

func TestJSONStdoutIsOneLineEvenWithDebugLogs(t *testing.T) {
	for _, checks := range [][]Check{nil, {fixedCheck("A2", StatusFail, "x")}} {
		var logs bytes.Buffer
		env, out, _ := testEnv(&logs, checks...)
		Run(context.Background(), []string{"doctor", "--json"}, env)
		lines := strings.Split(strings.TrimSuffix(out.String(), "\n"), "\n")
		if len(lines) != 1 {
			t.Fatalf("stdout has %d lines, want 1: %q", len(lines), out.String())
		}
		var doc map[string]any
		if err := json.Unmarshal([]byte(lines[0]), &doc); err != nil {
			t.Fatalf("stdout is not one JSON document: %v", err)
		}
		if logs.Len() == 0 {
			t.Fatalf("debug logs were expected on the log writer; the test would not detect logs leaking to stdout otherwise")
		}
	}
}

func TestUsageErrors(t *testing.T) {
	cases := [][]string{
		nil,
		{"nope"},
		{"--json", "status"},
		{"status", "--bogus"},
		{"status", "extra-arg"},
	}
	for _, args := range cases {
		env, out, errb := testEnv(nil)
		code := Run(context.Background(), args, env)
		if code != ExitUsage {
			t.Errorf("%q: exit %d, want %d", args, code, ExitUsage)
		}
		if out.Len() != 0 {
			t.Errorf("%q: usage errors must not print to stdout, got %q", args, out.String())
		}
		if errb.Len() == 0 {
			t.Errorf("%q: usage error printed no message", args)
		}
	}
}

func TestHelpExitsZero(t *testing.T) {
	env, _, errb := testEnv(nil)
	if code := Run(context.Background(), []string{"help"}, env); code != ExitOK {
		t.Fatalf("help: exit %d", code)
	}
	for _, name := range []string{"version", "status", "doctor"} {
		if !strings.Contains(errb.String(), name) {
			t.Errorf("help text lacks command %q", name)
		}
	}
}

func TestTextOutput(t *testing.T) {
	env, out, _ := testEnv(nil)
	if code := Run(context.Background(), []string{"status"}, env); code != ExitOK {
		t.Fatalf("exit %d", code)
	}
	if !strings.HasPrefix(out.String(), "schrodeck test") {
		t.Fatalf("status text = %q", out.String())
	}
}

// Contract E's documented schema_version must equal the code's. The doc is
// prose, so it can't derive the value; this test is the next best thing.
func TestSchemaVersionDocumented(t *testing.T) {
	doc, err := os.ReadFile(filepath.Join("..", "..", "docs", "contracts", "cli-json.md"))
	if err != nil {
		t.Fatal(err)
	}
	want := fmt.Sprintf("Current `schema_version`: **%d**", SchemaVersion)
	if !strings.Contains(string(doc), want) {
		t.Fatalf("docs/contracts/cli-json.md does not contain %q", want)
	}
}
```

- [ ] **Step 3: Run to verify failure**

Run: `go test ./internal/cli/`
Expected: FAIL, `undefined: Env` (and the other identifiers).

- [ ] **Step 4: Implement the envelope**

`internal/cli/json.go`:

```go
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

package cli

import (
	"encoding/json"
	"io"
)

// SchemaVersion is contract E's version (docs/contracts/cli-json.md). Bump it
// when a field is removed, renamed or changes type; adding a field is not a
// breaking change.
const SchemaVersion = 1

// envelope is the one JSON object every command prints under --json.
type envelope struct {
	SchemaVersion int        `json:"schema_version"`
	Command       string     `json:"command"`
	OK            bool       `json:"ok"`
	Data          any        `json:"data,omitempty"`
	Error         *errorBody `json:"error,omitempty"`
}

type errorBody struct {
	Message string `json:"message"`
}

// writeJSON prints doc as a single line. HTML escaping is off so paths and
// URLs read the same as in text output.
func writeJSON(w io.Writer, doc envelope) error {
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	return enc.Encode(doc)
}
```

- [ ] **Step 5: Implement the router**

`internal/cli/cli.go`:

```go
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

// Package cli implements schrodeck's command line. Every command can print a
// JSON document instead of text (--json, after the command name); that
// document is contract E (docs/contracts/cli-json.md) and is what any UI
// consumes. Logs never go to stdout.
package cli

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
)

// Exit codes.
const (
	ExitOK    = 0 // success
	ExitFail  = 1 // the command ran and reports failure
	ExitUsage = 2 // the command line was wrong
)

// Env is everything a command may touch. Tests build one with buffers.
type Env struct {
	Stdout  io.Writer
	Stderr  io.Writer
	Version string
	Logger  *slog.Logger
	Checks  []Check
}

// result is what a command produced.
type result struct {
	data   any    // the envelope's "data" member
	text   string // human-readable output
	failed bool   // ran, but reports failure: "ok": false and exit 1
}

type command struct {
	name    string
	summary string
	run     func(ctx context.Context, env Env, args []string) (result, error)
}

// usageError marks a wrong command line (exit 2) as opposed to a failed run.
type usageError struct{ msg string }

func (e usageError) Error() string { return e.msg }

// Run executes one command line and returns the process exit code.
func Run(ctx context.Context, args []string, env Env) int {
	if len(args) == 0 {
		printUsage(env.Stderr)
		return ExitUsage
	}
	if args[0] == "help" || args[0] == "-h" || args[0] == "--help" {
		printUsage(env.Stderr)
		return ExitOK
	}
	cmd, ok := lookup(args[0])
	if !ok {
		fmt.Fprintf(env.Stderr, "schrodeck: unknown command %q\n\n", args[0])
		printUsage(env.Stderr)
		return ExitUsage
	}

	fs := flag.NewFlagSet("schrodeck "+cmd.name, flag.ContinueOnError)
	fs.SetOutput(env.Stderr)
	asJSON := fs.Bool("json", false, "print one JSON document (contract E) instead of text")
	if err := fs.Parse(args[1:]); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return ExitOK
		}
		return ExitUsage
	}

	logger := env.Logger.With("component", "cli", "command", cmd.name)
	logger.Debug("command started")
	res, err := cmd.run(ctx, env, fs.Args())
	if err != nil {
		var ue usageError
		if errors.As(err, &ue) {
			fmt.Fprintf(env.Stderr, "schrodeck %s: %v\n", cmd.name, err)
			return ExitUsage
		}
		logger.Error("command failed", "error", err)
		if *asJSON {
			if werr := writeJSON(env.Stdout, envelope{SchemaVersion: SchemaVersion, Command: cmd.name, OK: false, Error: &errorBody{Message: err.Error()}}); werr != nil {
				logger.Error("writing JSON output failed", "error", werr)
			}
		} else {
			fmt.Fprintf(env.Stderr, "schrodeck %s: %v\n", cmd.name, err)
		}
		return ExitFail
	}

	if *asJSON {
		if err := writeJSON(env.Stdout, envelope{SchemaVersion: SchemaVersion, Command: cmd.name, OK: !res.failed, Data: res.data}); err != nil {
			logger.Error("writing JSON output failed", "error", err)
			return ExitFail
		}
	} else {
		fmt.Fprint(env.Stdout, res.text)
	}
	logger.Debug("command finished", "ok", !res.failed)
	if res.failed {
		return ExitFail
	}
	return ExitOK
}

func printUsage(w io.Writer) {
	fmt.Fprintln(w, "usage: schrodeck <command> [--json] [arguments]")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "commands:")
	for _, c := range commands() {
		fmt.Fprintf(w, "  %-10s %s\n", c.name, c.summary)
	}
}

func lookup(name string) (command, bool) {
	for _, c := range commands() {
		if c.name == name {
			return c, true
		}
	}
	return command{}, false
}
```

- [ ] **Step 6: Implement the commands**

`internal/cli/commands.go`:

```go
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

package cli

import (
	"context"
	"fmt"
	"strings"
)

// Check statuses reported by doctor.
const (
	StatusPass = "pass"
	StatusFail = "fail"
	StatusSkip = "skip"
)

// Check is one doctor check. M0 ships the wiring and no checks; M1 replaces
// this with the read-only probes of contracts B and C.
type Check struct {
	ID  string
	Run func(ctx context.Context) CheckResult
}

// CheckResult is one line of doctor's report.
type CheckResult struct {
	ID     string `json:"id"`
	Status string `json:"status"`
	Detail string `json:"detail,omitempty"`
}

func commands() []command {
	return []command{
		{name: "version", summary: "print the schrodeck version", run: runVersion},
		{name: "status", summary: "show what schrodeck manages on this computer (read-only)", run: runStatus},
		{name: "doctor", summary: "run read-only checks and report problems", run: runDoctor},
	}
}

type versionData struct {
	SchrodeckVersion string `json:"schrodeck_version"`
}

func noArgs(name string, args []string) error {
	if len(args) > 0 {
		return usageError{fmt.Sprintf("%s takes no arguments, got %q", name, args)}
	}
	return nil
}

func runVersion(_ context.Context, env Env, args []string) (result, error) {
	if err := noArgs("version", args); err != nil {
		return result{}, err
	}
	return result{data: versionData{SchrodeckVersion: env.Version}, text: fmt.Sprintf("schrodeck %s\n", env.Version)}, nil
}

func runStatus(_ context.Context, env Env, args []string) (result, error) {
	if err := noArgs("status", args); err != nil {
		return result{}, err
	}
	text := fmt.Sprintf("schrodeck %s\nNo setups yet: syncing arrives in a later milestone.\n", env.Version)
	return result{data: versionData{SchrodeckVersion: env.Version}, text: text}, nil
}

type doctorData struct {
	Checks []CheckResult `json:"checks"`
}

func runDoctor(ctx context.Context, env Env, args []string) (result, error) {
	if err := noArgs("doctor", args); err != nil {
		return result{}, err
	}
	data := doctorData{Checks: []CheckResult{}}
	failed := false
	var text strings.Builder
	for _, c := range env.Checks {
		r := c.Run(ctx)
		r.ID = c.ID
		data.Checks = append(data.Checks, r)
		if r.Status == StatusFail {
			failed = true
		}
		fmt.Fprintf(&text, "%-5s %s %s\n", r.Status, r.ID, r.Detail)
	}
	if len(env.Checks) == 0 {
		text.WriteString("No checks in this version.\n")
	}
	return result{data: data, text: text.String(), failed: failed}, nil
}
```

- [ ] **Step 7: Write contract E**

`docs/contracts/cli-json.md`:

````markdown
# Contract E: the `--json` CLI (Go core ↔ any UI or script)

Index of all contracts: [README.md](README.md). Decided in ADR [0024](../adr/0024-documented-contracts.md); UIs call the CLI and never reimplement sync logic (ADR [0018](../adr/0018-runtime-and-architecture.md)).

Current `schema_version`: **1**

## Rules

- Every command accepts `--json` **after the command name** (`schrodeck status --json`) and then prints **exactly one JSON object on one line** to stdout. Logs and usage messages go to stderr only.
- The object always starts with `schema_version`. Adding a member anywhere is **not** a breaking change; removing, renaming or changing the type of one **is**, and bumps `schema_version` in the same PR (`internal/cli/json.go`, `SchemaVersion`).
- Golden files in `internal/cli/testdata/golden/` pin every document shape. A PR that changes one updates the golden file and this page together.

## Envelope

| member | type | meaning |
|---|---|---|
| `schema_version` | integer | this contract's version |
| `command` | string | the command that ran |
| `ok` | boolean | `false` when the command reports failure (exit code 1) |
| `data` | object | the command's result; absent when `error` is present |
| `error` | object `{message}` | present only when the command could not run |

## Exit codes

| code | meaning |
|---|---|
| 0 | success |
| 1 | the command ran and reports failure (`ok: false`), or could not run (`error`) |
| 2 | usage error; nothing is printed to stdout |

## Commands

| command | `data` |
|---|---|
| `version` | `{schrodeck_version}` |
| `status` | `{schrodeck_version}`; members are added as features land |
| `doctor` | `{checks: [{id, status, detail?}]}`, `status` ∈ `pass` \| `fail` \| `skip` |
````

Edit `docs/contracts/README.md`, row **E**, last column: replace `currently ADR [0018](../adr/0018-runtime-and-architecture.md). It moves here as \`cli-json.md\` when the code defines it.` with `[cli-json.md](cli-json.md)`.

Edit `docs/adr/0024-documented-contracts.md`, in Consequences: replace `E still lives in ADR 0018 and moves into \`docs/contracts/\` when the code defines it.` with `E moved into [cli-json.md](../contracts/cli-json.md) when M0 defined it (issue #3).`

- [ ] **Step 8: Replace the entry point**

`cmd/schrodeck/main.go`:

```go
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

// Command schrodeck keeps Stream Deck setups identical across computers.
package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/csmarshall/schrodeck/internal/cli"
	"github.com/csmarshall/schrodeck/internal/logging"
	"github.com/csmarshall/schrodeck/internal/version"
)

func main() {
	logger, err := logging.New(os.Stderr, os.Getenv(logging.EnvLevel))
	if err != nil {
		fmt.Fprintln(os.Stderr, "schrodeck:", err)
		os.Exit(cli.ExitUsage)
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	code := cli.Run(ctx, os.Args[1:], cli.Env{
		Stdout:  os.Stdout,
		Stderr:  os.Stderr,
		Version: version.Version,
		Logger:  logger,
	})
	stop()
	os.Exit(code)
}
```

- [ ] **Step 9: Run the tests**

Run: `go test ./internal/cli/ && go run ./cmd/schrodeck status --json`
Expected: `ok  github.com/csmarshall/schrodeck/internal/cli`, then `{"schema_version":1,"command":"status","ok":true,"data":{"schrodeck_version":"dev"}}`.

- [ ] **Step 10: Commit**

```bash
git add internal/cli cmd/schrodeck docs/contracts
git add docs/adr/0024-documented-contracts.md
git commit -m "feat(#3): CLI skeleton with version/status/doctor and contract E (--json) with golden tests"
```

---

### Task 4: Port interfaces and the contract A name check

**Files:**
- Create: `internal/ports/ports.go`, `tools/portcheck/portcheck.go`, `tools/portcheck/portcheck_test.go`, `tools/portcheck/cmd/portcheck/main.go`
- Modify: `docs/contracts/os-connector.md` (§6 `Watcher`: add a `ctx` parameter and define `Event`)

**Interfaces:**
- Produces (package `github.com/csmarshall/schrodeck/internal/ports`): `Paths`, `HostIdentity`, `AppControl`, `DeviceEnumerator`, `AppPrefs`, `Watcher`, `Scheduler`, `Notifier`, `StoreSync` (methods exactly as contract A), plus `Geometry{Columns, Rows, Dials int}`, `Deck{AppDeviceID, ManifestDeviceID string; Geometry Geometry; Model string; Virtual bool; SerialHash string}`, `Event{Paths []string}`, `AgentSpec{Executable string; Args []string; DeviceAttach bool}`, `AgentStatus{Installed, Running bool; Detail string}`, `Severity` (`SeverityInfo|SeverityWarning|SeverityError`), `Notification{Title, Body string; Severity Severity; Persistent bool}`, `Freshness` (`Unknown` = zero value, `Fresh`, `InFlight`, `Conflict`).
- Produces (package `portcheck`): `type Port struct{Name string; Methods []string}`; `FromDoc(md []byte) ([]Port, error)`; `FromSource(dir string) ([]Port, error)`; `Compare(doc, src []Port) []string`.

- [ ] **Step 1: Change contract A's `Watcher` (ours, so code and doc change together)**

The sketch `Watch(paths []string, debounce time.Duration) (<-chan Event, error)` gives no way to stop a watcher, so every caller and test would leak a goroutine. In `docs/contracts/os-connector.md` § 6, replace the code block with:

```go
type Event struct {
    Paths []string // changed paths seen in one debounce window; a hint, never truth
}
type Watcher interface {
    Watch(ctx context.Context, paths []string, debounce time.Duration) (<-chan Event, error) // the channel closes when ctx ends
}
```

- [ ] **Step 2: Write the failing portcheck tests**

`tools/portcheck/portcheck_test.go`:

```go
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

package portcheck

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const docTwoPorts = "# A\n\n```go\ntype Paths interface {\n    Home() string\n    LogDir() string\n}\n```\n\ntext\n\n```go\ntype Notifier interface { Notify(n Notification) error }\ntype Notification struct { Title string }\n```\n"

func TestFromDoc(t *testing.T) {
	ports, err := FromDoc([]byte(docTwoPorts))
	if err != nil {
		t.Fatal(err)
	}
	got := render(ports)
	want := "Notifier: Notify | Paths: Home LogDir"
	if got != want {
		t.Fatalf("FromDoc = %q, want %q", got, want)
	}
}

func writeSource(t *testing.T, src string) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "ports.go"), []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestCompareAgreement(t *testing.T) {
	doc, _ := FromDoc([]byte(docTwoPorts))
	src, err := FromSource(writeSource(t, "package ports\ntype Paths interface { Home() string; LogDir() string }\ntype Notifier interface { Notify(n Notification) error }\ntype Notification struct{}\ntype unexported interface{ x() }\n"))
	if err != nil {
		t.Fatal(err)
	}
	if diffs := Compare(doc, src); len(diffs) != 0 {
		t.Fatalf("expected agreement, got %v", diffs)
	}
}

// Known-bad cases: each must produce at least one difference.
func TestCompareDetectsDrift(t *testing.T) {
	doc, _ := FromDoc([]byte(docTwoPorts))
	cases := map[string]string{
		"missing port":   "package ports\ntype Paths interface { Home() string; LogDir() string }\n",
		"extra port":     "package ports\ntype Paths interface { Home() string; LogDir() string }\ntype Notifier interface { Notify(n Notification) error }\ntype Clock interface { Now() int }\n",
		"missing method": "package ports\ntype Paths interface { Home() string }\ntype Notifier interface { Notify(n Notification) error }\n",
		"renamed method": "package ports\ntype Paths interface { Home() string; Logs() string }\ntype Notifier interface { Notify(n Notification) error }\n",
	}
	for name, code := range cases {
		src, err := FromSource(writeSource(t, code))
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if diffs := Compare(doc, src); len(diffs) == 0 {
			t.Errorf("%s: drift not detected", name)
		}
	}
}

// The real check, also run by CI through main.go.
func TestRepositoryPortsMatchContractA(t *testing.T) {
	md, err := os.ReadFile(filepath.Join("..", "..", "docs", "contracts", "os-connector.md"))
	if err != nil {
		t.Fatal(err)
	}
	doc, err := FromDoc(md)
	if err != nil {
		t.Fatal(err)
	}
	src, err := FromSource(filepath.Join("..", "..", "internal", "ports"))
	if err != nil {
		t.Fatal(err)
	}
	if len(doc) == 0 {
		t.Fatal("no ports found in contract A; the parser is broken, not the contract")
	}
	if diffs := Compare(doc, src); len(diffs) != 0 {
		t.Fatalf("internal/ports differs from docs/contracts/os-connector.md:\n%s", strings.Join(diffs, "\n"))
	}
}

func render(ps []Port) string {
	var parts []string
	for _, p := range ps {
		parts = append(parts, p.Name+": "+strings.Join(p.Methods, " "))
	}
	return strings.Join(parts, " | ")
}
```

- [ ] **Step 3: Run to verify failure**

Run: `go test ./tools/portcheck/`
Expected: FAIL, `undefined: FromDoc`.

- [ ] **Step 4: Implement portcheck**

`tools/portcheck/portcheck.go`:

```go
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

// Package portcheck compares the port interfaces sketched in contract A
// (docs/contracts/os-connector.md) with the Go interfaces in internal/ports.
// Contract A is the single source of truth for the port list (ADR 0024); this
// check is what keeps the code from drifting away from it.
package portcheck

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// Port is one interface: its name and sorted method names.
type Port struct {
	Name    string
	Methods []string
}

var goBlock = regexp.MustCompile("(?s)```go\n(.*?)```")

// FromDoc returns every interface declared in the document's ```go blocks.
func FromDoc(md []byte) ([]Port, error) {
	var ports []Port
	for i, m := range goBlock.FindAllSubmatch(md, -1) {
		src := append([]byte("package doc\n"), m[1]...)
		f, err := parser.ParseFile(token.NewFileSet(), fmt.Sprintf("block%d.go", i), src, 0)
		if err != nil {
			return nil, fmt.Errorf("go block %d does not parse: %w", i, err)
		}
		ports = append(ports, interfaces(f, false)...)
	}
	return sortPorts(ports), nil
}

// FromSource returns every exported interface declared in the non-test Go
// files of dir.
func FromSource(dir string) ([]Port, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	fset := token.NewFileSet()
	var ports []Port
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		f, err := parser.ParseFile(fset, filepath.Join(dir, name), nil, 0)
		if err != nil {
			return nil, err
		}
		ports = append(ports, interfaces(f, true)...)
	}
	return sortPorts(ports), nil
}

func interfaces(f *ast.File, exportedOnly bool) []Port {
	var ports []Port
	ast.Inspect(f, func(n ast.Node) bool {
		ts, ok := n.(*ast.TypeSpec)
		if !ok {
			return true
		}
		it, ok := ts.Type.(*ast.InterfaceType)
		if !ok || (exportedOnly && !ts.Name.IsExported()) {
			return true
		}
		p := Port{Name: ts.Name.Name}
		for _, field := range it.Methods.List {
			for _, name := range field.Names {
				p.Methods = append(p.Methods, name.Name)
			}
		}
		sort.Strings(p.Methods)
		ports = append(ports, p)
		return true
	})
	return ports
}

func sortPorts(ps []Port) []Port {
	sort.Slice(ps, func(i, j int) bool { return ps[i].Name < ps[j].Name })
	return ps
}

// Compare returns one line per difference between the documented and the
// implemented ports, sorted; an empty result means they agree.
func Compare(doc, src []Port) []string {
	byName := func(ps []Port) map[string][]string {
		m := map[string][]string{}
		for _, p := range ps {
			m[p.Name] = p.Methods
		}
		return m
	}
	d, s := byName(doc), byName(src)
	var diffs []string
	for name, dm := range d {
		sm, ok := s[name]
		if !ok {
			diffs = append(diffs, fmt.Sprintf("port %s is in contract A but not in internal/ports", name))
			continue
		}
		if strings.Join(dm, ",") != strings.Join(sm, ",") {
			diffs = append(diffs, fmt.Sprintf("port %s: contract A methods [%s], internal/ports methods [%s]", name, strings.Join(dm, " "), strings.Join(sm, " ")))
		}
	}
	for name := range s {
		if _, ok := d[name]; !ok {
			diffs = append(diffs, fmt.Sprintf("port %s is in internal/ports but not in contract A", name))
		}
	}
	sort.Strings(diffs)
	return diffs
}
```

The command lives in its own `main` package, `tools/portcheck/cmd/portcheck/main.go`:

```go
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

// Command portcheck fails when internal/ports and contract A disagree.
// Usage: go run ./tools/portcheck/cmd/portcheck -doc docs/contracts/os-connector.md -src internal/ports
package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/csmarshall/schrodeck/tools/portcheck"
)

func main() {
	docPath := flag.String("doc", "docs/contracts/os-connector.md", "contract A document")
	srcDir := flag.String("src", "internal/ports", "package directory holding the port interfaces")
	flag.Parse()

	md, err := os.ReadFile(*docPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, "portcheck:", err)
		os.Exit(2)
	}
	doc, err := portcheck.FromDoc(md)
	if err != nil {
		fmt.Fprintln(os.Stderr, "portcheck:", err)
		os.Exit(2)
	}
	src, err := portcheck.FromSource(*srcDir)
	if err != nil {
		fmt.Fprintln(os.Stderr, "portcheck:", err)
		os.Exit(2)
	}
	if len(doc) == 0 {
		fmt.Fprintln(os.Stderr, "portcheck: no ports found in", *docPath)
		os.Exit(2)
	}
	diffs := portcheck.Compare(doc, src)
	for _, d := range diffs {
		fmt.Fprintln(os.Stderr, d)
	}
	if len(diffs) > 0 {
		os.Exit(1)
	}
	fmt.Printf("portcheck: %d ports match contract A\n", len(doc))
}
```

- [ ] **Step 5: Run the unit tests (the repository test must still fail: no ports yet)**

Run: `go test ./tools/portcheck/`
Expected: `TestFromDoc`, `TestCompareAgreement`, `TestCompareDetectsDrift` pass; `TestRepositoryPortsMatchContractA` FAILS with `open ../../internal/ports: no such file or directory`.

- [ ] **Step 6: Write the ports**

`internal/ports/ports.go`:

```go
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

// Package ports declares what an operating system must provide for schrodeck
// to run on it. Contract A (docs/contracts/os-connector.md) is the single
// source of truth: the interfaces here mirror it exactly, and
// tools/portcheck fails CI when they differ. Read the contract for each
// port's invariants; they are not repeated here.
package ports

import (
	"context"
	"time"
)

// Paths says where things live.
type Paths interface {
	AppDataRoot() string
	ProfilesDir() string
	PluginsDir() string
	IconPacksDir() string
	StateDir() string
	LogDir() string
	ConfigPointer() string
	Home() string
	StoreCandidates() []string
}

// HostIdentity identifies this host and user. The raw hardware id is never
// logged or stored.
type HostIdentity interface {
	HardwareID() (string, error)
	UserName() string
	FriendlyName() string
}

// AppControl drives the Stream Deck app process. Quit returning nil
// guarantees no app process can write ProfilesDir afterwards.
type AppControl interface {
	Installed() (bool, error)
	Running() (bool, error)
	Quit(timeout time.Duration) error
	Launch() error
	WaitSettled(quietFor, max time.Duration) error
}

// Geometry is a deck's key grid plus dials.
type Geometry struct {
	Columns int
	Rows    int
	Dials   int
}

// Deck is one deck the app knows on this host.
type Deck struct {
	AppDeviceID      string
	ManifestDeviceID string
	Geometry         Geometry
	Model            string
	Virtual          bool
	SerialHash       string
}

// DeviceEnumerator lists this host's decks from the app's own data.
type DeviceEnumerator interface {
	Decks() ([]Deck, error)
}

// AppPrefs reads the app's preferences. Read-only; the selected profile is
// never written (ADR 0019).
type AppPrefs interface {
	AppVersion() (string, error)
	SelectedProfile(appDeviceID string) (string, error)
	DeviceRecords() ([]map[string]any, error)
}

// Event is a coalesced change notification. Events are hints, never truth.
type Event struct {
	Paths []string
}

// Watcher reports changes below paths, recursively. The channel closes when
// ctx ends.
type Watcher interface {
	Watch(ctx context.Context, paths []string, debounce time.Duration) (<-chan Event, error)
}

// AgentSpec describes the resident per-user agent to install.
type AgentSpec struct {
	Executable   string
	Args         []string
	DeviceAttach bool
}

// AgentStatus is what doctor and install --check report about the agent.
type AgentStatus struct {
	Installed bool
	Running   bool
	Detail    string
}

// Scheduler installs and inspects the resident agent.
type Scheduler interface {
	Install(spec AgentSpec) error
	Uninstall() error
	Status() (AgentStatus, error)
}

// Severity grades a notification.
type Severity int

// Notification severities.
const (
	SeverityInfo Severity = iota
	SeverityWarning
	SeverityError
)

// Notification is one user-visible message. Deduplication is the core's job.
type Notification struct {
	Title      string
	Body       string
	Severity   Severity
	Persistent bool
}

// Notifier delivers notifications.
type Notifier interface {
	Available() bool
	Notify(n Notification) error
}

// Freshness is the sync client's view of a set of store paths. The zero
// value is Unknown, so an unset value fails closed.
type Freshness int

// Freshness values.
const (
	Unknown Freshness = iota
	Fresh
	InFlight
	Conflict
)

// StoreSync asks the cloud sync client whether the shared folder is current.
type StoreSync interface {
	ReadFreshness(paths []string) (Freshness, error)
	PushConfirmed(paths []string) (bool, error)
	EnsureDownloaded(paths []string) error
}
```

- [ ] **Step 7: Run all portcheck tests and the command**

Run: `go test ./tools/portcheck/ && go run ./tools/portcheck/cmd/portcheck`
Expected: `ok`, then `portcheck: 9 ports match contract A` (the number is printed, not asserted; it comes from the document).

- [ ] **Step 8: Prove the command fails on drift**

Temporarily rename `EnsureDownloaded` to `EnsureLocal` in `internal/ports/ports.go` and run `go run ./tools/portcheck/cmd/portcheck; echo "exit $?"`. Expected: a `port StoreSync: contract A methods [EnsureDownloaded PushConfirmed ReadFreshness], internal/ports methods [EnsureLocal PushConfirmed ReadFreshness]` line and `exit 1`. Rename it back by hand (the file is not committed yet) and re-run: exit 0.

- [ ] **Step 9: Commit**

```bash
git add internal/ports tools/portcheck docs/contracts/os-connector.md
git commit -m "feat(#3): port interfaces mirroring contract A, portcheck keeps them in step (Watcher gains ctx)"
```

---

### Task 5: Fakes for every port

**Files:**
- Create: `internal/ports/fake/fake.go`, `internal/ports/fake/app.go`, `internal/ports/fake/watcher.go`, `internal/ports/fake/fake_test.go`

**Interfaces:**
- Consumes: `ports.*`.
- Produces (package `github.com/csmarshall/schrodeck/internal/ports/fake`):
  - `fake.Paths{Root string}` (all paths under `Root`)
  - `fake.HostIdentity{Hardware, User, Friendly string}`
  - `fake.NewApp(profilesDir string) *fake.App` with exported knob `IgnoreQuit bool`
  - `fake.Decks{List []ports.Deck; Err error}`
  - `fake.Prefs{Version string; Selected map[string]string; Records []map[string]any}`
  - `fake.Watcher{Interval time.Duration}` (recursive polling over real directories)
  - `fake.Scheduler`, `fake.Notifier{Avail bool}` with `Sent() []ports.Notification`, `fake.StoreSync{State ports.Freshness; Confirmed bool}`
  - `fake.DirDigest(dir string) (string, error)` (content digest of a directory tree; used by the fake app and the conformance suite)

- [ ] **Step 1: Write the failing tests**

`internal/ports/fake/fake_test.go`:

```go
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

package fake

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/csmarshall/schrodeck/internal/ports"
)

// Compile-time proof that every fake satisfies its port.
var (
	_ ports.Paths            = Paths{}
	_ ports.HostIdentity     = HostIdentity{}
	_ ports.AppControl       = (*App)(nil)
	_ ports.DeviceEnumerator = Decks{}
	_ ports.AppPrefs         = Prefs{}
	_ ports.Watcher          = Watcher{}
	_ ports.Scheduler        = (*Scheduler)(nil)
	_ ports.Notifier         = (*Notifier)(nil)
	_ ports.StoreSync        = (*StoreSync)(nil)
)

func TestAppLifecycle(t *testing.T) {
	dir := t.TempDir()
	app := NewApp(dir)
	if err := app.Launch(); err != nil {
		t.Fatal(err)
	}
	if err := app.WaitSettled(20*time.Millisecond, time.Second); err != nil {
		t.Fatal(err)
	}
	if err := app.Quit(time.Second); err != nil {
		t.Fatal(err)
	}
	if running, _ := app.Running(); running {
		t.Fatal("still running after Quit")
	}
}

func TestAppIgnoringQuitTimesOut(t *testing.T) {
	app := NewApp(t.TempDir())
	app.IgnoreQuit = true
	if err := app.Launch(); err != nil {
		t.Fatal(err)
	}
	start := time.Now()
	err := app.Quit(50 * time.Millisecond)
	if !errors.Is(err, ErrQuitTimeout) {
		t.Fatalf("Quit error = %v, want ErrQuitTimeout", err)
	}
	if time.Since(start) < 50*time.Millisecond {
		t.Fatal("Quit gave up before its timeout")
	}
	if running, _ := app.Running(); !running {
		t.Fatal("an app that ignored Quit must still be running")
	}
}

func TestWatcherSeesDeepWrite(t *testing.T) {
	root := t.TempDir()
	deep := filepath.Join(root, "a", "b", "c")
	if err := os.MkdirAll(deep, 0o755); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	ch, err := Watcher{Interval: 5 * time.Millisecond}.Watch(ctx, []string{root}, 20*time.Millisecond)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(deep, "f"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	select {
	case ev := <-ch:
		if len(ev.Paths) == 0 {
			t.Fatal("event with no paths")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("no event for a write three levels deep")
	}
	cancel()
	for range ch {
	}
}

func TestDirDigestChangesWithContent(t *testing.T) {
	dir := t.TempDir()
	a, err := DirDigest(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "f"), []byte("1"), 0o644); err != nil {
		t.Fatal(err)
	}
	b, _ := DirDigest(dir)
	if err := os.WriteFile(filepath.Join(dir, "f"), []byte("2"), 0o644); err != nil {
		t.Fatal(err)
	}
	c, _ := DirDigest(dir)
	if a == b || b == c {
		t.Fatalf("digest did not change: %s %s %s", a, b, c)
	}
}

func TestNotifierRecords(t *testing.T) {
	n := &Notifier{Avail: true}
	if err := n.Notify(ports.Notification{Title: "t"}); err != nil {
		t.Fatal(err)
	}
	if got := n.Sent(); len(got) != 1 || got[0].Title != "t" {
		t.Fatalf("Sent() = %v", got)
	}
}
```

- [ ] **Step 2: Run to verify failure**

Run: `go test ./internal/ports/fake/`
Expected: FAIL, `undefined: Paths` (and the rest).

- [ ] **Step 3: Implement the simple fakes**

`internal/ports/fake/fake.go`:

```go
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

// Package fake provides test doubles for every port in contract A. Core tests
// run against these on any OS; the conformance suite runs against them too,
// so a fake that drifts from the contract fails the same tests a real
// connector would.
package fake

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"sync"

	"github.com/csmarshall/schrodeck/internal/ports"
)

// Paths puts every location under Root.
type Paths struct{ Root string }

func (p Paths) AppDataRoot() string   { return filepath.Join(p.Root, "app") }
func (p Paths) ProfilesDir() string   { return filepath.Join(p.AppDataRoot(), "ProfilesV3") }
func (p Paths) PluginsDir() string    { return filepath.Join(p.AppDataRoot(), "Plugins") }
func (p Paths) IconPacksDir() string  { return filepath.Join(p.AppDataRoot(), "IconPacks") }
func (p Paths) StateDir() string      { return filepath.Join(p.Root, "state") }
func (p Paths) LogDir() string        { return filepath.Join(p.Root, "logs") }
func (p Paths) ConfigPointer() string { return filepath.Join(p.Root, "config", "config.toml") }
func (p Paths) Home() string          { return filepath.Join(p.Root, "home") }
func (p Paths) StoreCandidates() []string {
	return []string{filepath.Join(p.Root, "store")}
}

// HostIdentity returns fixed values.
type HostIdentity struct{ Hardware, User, Friendly string }

func (h HostIdentity) HardwareID() (string, error) {
	if h.Hardware == "" {
		return "", fmt.Errorf("fake host identity: no hardware id")
	}
	return h.Hardware, nil
}
func (h HostIdentity) UserName() string     { return h.User }
func (h HostIdentity) FriendlyName() string { return h.Friendly }

// Decks returns a fixed deck list.
type Decks struct {
	List []ports.Deck
	Err  error
}

func (d Decks) Decks() ([]ports.Deck, error) { return d.List, d.Err }

// Prefs returns fixed preferences.
type Prefs struct {
	Version  string
	Selected map[string]string
	Records  []map[string]any
}

func (p Prefs) AppVersion() (string, error) {
	if p.Version == "" {
		return "", fmt.Errorf("fake prefs: no app version")
	}
	return p.Version, nil
}

func (p Prefs) SelectedProfile(appDeviceID string) (string, error) {
	id, ok := p.Selected[appDeviceID]
	if !ok {
		return "", fmt.Errorf("fake prefs: no device %q", appDeviceID)
	}
	return id, nil
}

func (p Prefs) DeviceRecords() ([]map[string]any, error) { return p.Records, nil }

// Scheduler records the installed agent in memory.
type Scheduler struct {
	mu   sync.Mutex
	spec *ports.AgentSpec
}

func (s *Scheduler) Install(spec ports.AgentSpec) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.spec = &spec
	return nil
}

func (s *Scheduler) Uninstall() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.spec = nil
	return nil
}

func (s *Scheduler) Status() (ports.AgentStatus, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return ports.AgentStatus{Installed: s.spec != nil, Running: s.spec != nil}, nil
}

// Notifier records what it was asked to deliver.
type Notifier struct {
	Avail bool
	mu    sync.Mutex
	sent  []ports.Notification
}

func (n *Notifier) Available() bool { return n.Avail }

func (n *Notifier) Notify(m ports.Notification) error {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.sent = append(n.sent, m)
	return nil
}

// Sent returns a copy of every notification delivered so far.
func (n *Notifier) Sent() []ports.Notification {
	n.mu.Lock()
	defer n.mu.Unlock()
	return append([]ports.Notification(nil), n.sent...)
}

// StoreSync reports a fixed freshness.
type StoreSync struct {
	State      ports.Freshness
	Confirmed  bool
	mu         sync.Mutex
	downloaded [][]string
}

func (s *StoreSync) ReadFreshness([]string) (ports.Freshness, error) { return s.State, nil }
func (s *StoreSync) PushConfirmed([]string) (bool, error)            { return s.Confirmed, nil }
func (s *StoreSync) EnsureDownloaded(paths []string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.downloaded = append(s.downloaded, append([]string(nil), paths...))
	return nil
}

// DirDigest is a content digest of every regular file below dir (relative
// path and bytes). A missing dir digests like an empty one.
func DirDigest(dir string) (string, error) {
	var lines []string
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			if os.IsNotExist(err) && path == dir {
				return filepath.SkipDir
			}
			return err
		}
		if !d.Type().IsRegular() {
			return nil
		}
		b, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(dir, path)
		sum := sha256.Sum256(b)
		lines = append(lines, filepath.ToSlash(rel)+"\x00"+hex.EncodeToString(sum[:]))
		return nil
	})
	if err != nil {
		return "", err
	}
	sort.Strings(lines)
	h := sha256.New()
	for _, l := range lines {
		h.Write([]byte(l + "\n"))
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}
```

- [ ] **Step 4: Implement the fake app**

`internal/ports/fake/app.go`:

```go
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

package fake

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// ErrQuitTimeout is returned when the app is still running at Quit's timeout.
var ErrQuitTimeout = errors.New("fake app: still running at quit timeout")

// App simulates the Stream Deck app: launching rewrites a state file in
// ProfilesDir (as the real app rewrites manifests on launch), and a graceful
// quit flushes once and then never writes again.
type App struct {
	ProfilesDir string
	// IgnoreQuit simulates an app that does not exit when asked.
	IgnoreQuit bool

	mu       sync.Mutex
	running  bool
	launches int
}

// NewApp returns an installed, stopped app writing into profilesDir.
func NewApp(profilesDir string) *App { return &App{ProfilesDir: profilesDir} }

func (a *App) Installed() (bool, error) { return true, nil }

func (a *App) Running() (bool, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.running, nil
}

func (a *App) Launch() error {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.running = true
	a.launches++
	return a.writeState(fmt.Sprintf("launched %d", a.launches))
}

func (a *App) Quit(timeout time.Duration) error {
	a.mu.Lock()
	ignore := a.IgnoreQuit
	a.mu.Unlock()
	if ignore {
		time.Sleep(timeout)
		return ErrQuitTimeout
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	if err := a.writeState("flushed on quit"); err != nil {
		return err
	}
	a.running = false
	return nil
}

func (a *App) WaitSettled(quietFor, max time.Duration) error {
	deadline := time.Now().Add(max)
	last, err := DirDigest(a.ProfilesDir)
	if err != nil {
		return err
	}
	quietSince := time.Now()
	for {
		running, _ := a.Running()
		if running && time.Since(quietSince) >= quietFor {
			return nil
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("fake app: not settled within %s", max)
		}
		time.Sleep(quietFor / 4)
		cur, err := DirDigest(a.ProfilesDir)
		if err != nil {
			return err
		}
		if cur != last {
			last, quietSince = cur, time.Now()
		}
	}
}

// writeState must be called with a.mu held.
func (a *App) writeState(s string) error {
	if err := os.MkdirAll(a.ProfilesDir, 0o755); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(a.ProfilesDir, ".fake-app-state"), []byte(s), 0o644)
}
```

- [ ] **Step 5: Implement the fake watcher**

`internal/ports/fake/watcher.go`:

```go
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

package fake

import (
	"context"
	"io/fs"
	"path/filepath"
	"sort"
	"time"

	"github.com/csmarshall/schrodeck/internal/ports"
)

// Watcher polls real directories recursively. It honors contract A's
// semantics (recursive, debounced, hints only) without any OS API, so core
// tests and the conformance suite can run it anywhere.
type Watcher struct {
	// Interval between polls; 10ms when zero.
	Interval time.Duration
}

type fileState struct {
	size    int64
	modTime time.Time
	dir     bool
}

func (w Watcher) interval() time.Duration {
	if w.Interval <= 0 {
		return 10 * time.Millisecond
	}
	return w.Interval
}

// Watch emits one Event per quiet period: changes are collected until none
// has been seen for debounce, then delivered together.
func (w Watcher) Watch(ctx context.Context, paths []string, debounce time.Duration) (<-chan ports.Event, error) {
	prev, err := scan(paths)
	if err != nil {
		return nil, err
	}
	ch := make(chan ports.Event, 16)
	go func() {
		defer close(ch)
		ticker := time.NewTicker(w.interval())
		defer ticker.Stop()
		pending := map[string]bool{}
		var lastChange time.Time
		for {
			select {
			case <-ctx.Done():
				return
			case now := <-ticker.C:
				cur, err := scan(paths)
				if err != nil {
					continue // events are hints; the next poll retries
				}
				for _, p := range changed(prev, cur) {
					pending[p] = true
					lastChange = now
				}
				prev = cur
				if len(pending) > 0 && now.Sub(lastChange) >= debounce {
					ev := ports.Event{}
					for p := range pending {
						ev.Paths = append(ev.Paths, p)
					}
					sort.Strings(ev.Paths)
					pending = map[string]bool{}
					select {
					case ch <- ev:
					case <-ctx.Done():
						return
					}
				}
			}
		}
	}()
	return ch, nil
}

func scan(roots []string) (map[string]fileState, error) {
	states := map[string]fileState{}
	for _, root := range roots {
		err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			info, err := d.Info()
			if err != nil {
				return err
			}
			states[path] = fileState{size: info.Size(), modTime: info.ModTime(), dir: d.IsDir()}
			return nil
		})
		if err != nil {
			return nil, err
		}
	}
	return states, nil
}

func changed(a, b map[string]fileState) []string {
	var out []string
	for p, sb := range b {
		sa, ok := a[p]
		if !ok || (!sb.dir && (sa.size != sb.size || !sa.modTime.Equal(sb.modTime))) {
			out = append(out, p)
		}
	}
	for p := range a {
		if _, ok := b[p]; !ok {
			out = append(out, p)
		}
	}
	return out
}
```

Directory modification times are ignored on purpose: a directory's mtime changes when a direct child is added, which would let a non-recursive implementation look recursive for shallow writes.

- [ ] **Step 6: Run the tests**

Run: `go test -race ./internal/ports/fake/`
Expected: `ok`.

- [ ] **Step 7: Commit**

```bash
git add internal/ports/fake
git commit -m "feat(#3): fakes for every contract A port, including a recursive polling watcher"
```

---

### Task 6: Conformance suite skeleton

**Files:**
- Create: `internal/conformance/conformance.go`, `internal/conformance/appcontrol.go`, `internal/conformance/watcher.go`, `internal/conformance/identity.go`, `internal/conformance/conformance_test.go`

**Interfaces:**
- Consumes: `ports.*`, `fake.*`, `fake.DirDigest`.
- Produces (package `github.com/csmarshall/schrodeck/internal/conformance`):
  - `type TB interface{ Helper(); Errorf(format string, args ...any) }` (`*testing.T` satisfies it)
  - `type AppHarness struct{ App ports.AppControl; ProfilesDir string; QuitTimeout, SettleQuiet, SettleMax time.Duration }`
  - `AppControlQuit(tb TB, h AppHarness)`; `AppControlQuitTimeout(tb TB, stuck ports.AppControl, timeout time.Duration)`
  - `WatcherRecursive(tb TB, w ports.Watcher, root string, debounce time.Duration)`; `WatcherBurstIsOneEvent(tb TB, w ports.Watcher, root string, debounce time.Duration)`
  - `HostIdentityStable(tb TB, h ports.HostIdentity)`
  - Real adapters (M1 onward) call these same functions from their own tests on their OS's runner.

- [ ] **Step 1: Write the failing tests, including the known-bad fakes**

`internal/conformance/conformance_test.go`:

```go
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
	"testing"
	"time"

	"github.com/csmarshall/schrodeck/internal/ports"
	"github.com/csmarshall/schrodeck/internal/ports/fake"
)

// recorder is a TB that records failures instead of failing the test, so a
// test can assert that a suite DOES fail on a known-bad implementation.
type recorder struct{ failures []string }

func (r *recorder) Helper() {}
func (r *recorder) Errorf(format string, args ...any) {
	r.failures = append(r.failures, fmt.Sprintf(format, args...))
}

func harness(t *testing.T, app ports.AppControl, dir string) AppHarness {
	return AppHarness{App: app, ProfilesDir: dir, QuitTimeout: time.Second, SettleQuiet: 200 * time.Millisecond, SettleMax: 3 * time.Second}
}

// --- AppControl ------------------------------------------------------------

func TestAppControlQuitOnFake(t *testing.T) {
	dir := t.TempDir()
	AppControlQuit(t, harness(t, fake.NewApp(dir), dir))
}

// writesAfterQuit violates contract A: Quit returns nil but the process keeps
// writing ProfilesDir.
type writesAfterQuit struct{ *fake.App }

func (w writesAfterQuit) Quit(timeout time.Duration) error {
	if err := w.App.Quit(timeout); err != nil {
		return err
	}
	go func() {
		time.Sleep(50 * time.Millisecond) // well inside the suite's 200ms post-quit window
		_ = os.WriteFile(filepath.Join(w.App.ProfilesDir, "late.json"), []byte("{}"), 0o644)
	}()
	return nil
}

func TestAppControlQuitCatchesWriteAfterQuit(t *testing.T) {
	dir := t.TempDir()
	rec := &recorder{}
	AppControlQuit(rec, harness(t, writesAfterQuit{fake.NewApp(dir)}, dir))
	if len(rec.failures) == 0 {
		t.Fatal("suite passed an app that writes ProfilesDir after Quit returned nil")
	}
}

func TestAppControlQuitTimeoutOnFake(t *testing.T) {
	app := fake.NewApp(t.TempDir())
	app.IgnoreQuit = true
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
	if len(rec.failures) == 0 {
		t.Fatal("suite passed an app whose Quit returns nil while it is still running")
	}
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

// topLevelOnly violates contract A: it does not watch recursively.
type topLevelOnly struct{}

func (topLevelOnly) Watch(ctx context.Context, paths []string, debounce time.Duration) (<-chan ports.Event, error) {
	list := func() string {
		var names []string
		for _, p := range paths {
			entries, _ := os.ReadDir(p)
			for _, e := range entries {
				info, _ := e.Info()
				if info != nil && !e.IsDir() {
					names = append(names, fmt.Sprintf("%s:%d:%d", e.Name(), info.Size(), info.ModTime().UnixNano()))
				} else {
					names = append(names, e.Name())
				}
			}
		}
		sort.Strings(names)
		return fmt.Sprint(names)
	}
	ch := make(chan ports.Event, 4)
	go func() {
		defer close(ch)
		prev := list()
		for {
			select {
			case <-ctx.Done():
				return
			case <-time.After(10 * time.Millisecond):
				if cur := list(); cur != prev {
					prev = cur
					ch <- ports.Event{Paths: paths}
				}
			}
		}
	}()
	return ch, nil
}

func TestWatcherRecursiveCatchesTopLevelOnly(t *testing.T) {
	rec := &recorder{}
	WatcherRecursive(rec, topLevelOnly{}, t.TempDir(), debounce)
	if len(rec.failures) == 0 {
		t.Fatal("suite passed a watcher that only sees the top level")
	}
}

// --- HostIdentity ----------------------------------------------------------

func TestHostIdentityOnFake(t *testing.T) {
	HostIdentityStable(t, fake.HostIdentity{Hardware: "HW-TEST-0001", User: "alice", Friendly: "test mac"})
}

type flappingIdentity struct{ n *int }

func (f flappingIdentity) HardwareID() (string, error) {
	*f.n++
	return fmt.Sprintf("HW-%d", *f.n), nil
}
func (f flappingIdentity) UserName() string     { return "alice" }
func (f flappingIdentity) FriendlyName() string { return "test mac" }

func TestHostIdentityCatchesUnstableID(t *testing.T) {
	rec := &recorder{}
	n := 0
	HostIdentityStable(rec, flappingIdentity{&n})
	if len(rec.failures) == 0 {
		t.Fatal("suite passed a hardware id that changes between calls")
	}
}
```

- [ ] **Step 2: Run to verify failure**

Run: `go test ./internal/conformance/`
Expected: FAIL, `undefined: AppHarness` (and the suite functions).

- [ ] **Step 3: Implement the suites**

`internal/conformance/conformance.go`:

```go
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

// Package conformance is contract A's conformance suite: tests written once
// against the port interfaces and run against every implementation (the
// fakes here, each real connector on its own OS's CI runner). Every suite has
// a known-bad implementation in this package's tests that it must fail.
package conformance

// TB is the part of testing.TB the suites use. *testing.T satisfies it; the
// suite's own tests pass a recorder to prove a suite fails on bad input.
// Suites report with Errorf and return early instead of calling Fatalf.
type TB interface {
	Helper()
	Errorf(format string, args ...any)
}
```

`internal/conformance/appcontrol.go`:

```go
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

package conformance

import (
	"time"

	"github.com/csmarshall/schrodeck/internal/ports"
	"github.com/csmarshall/schrodeck/internal/ports/fake"
)

// AppHarness is what the AppControl suites need from an implementation.
type AppHarness struct {
	App         ports.AppControl
	ProfilesDir string
	QuitTimeout time.Duration
	// SettleQuiet is also the window after Quit during which no write may
	// appear; a real connector sets it to the settle window contract C P4
	// measured for its app version.
	SettleQuiet time.Duration
	SettleMax   time.Duration
}

// AppControlQuit checks contract A's Quit guarantee: after Quit returns nil
// the app is not running and nothing writes ProfilesDir.
func AppControlQuit(tb TB, h AppHarness) {
	tb.Helper()
	if err := h.App.Launch(); err != nil {
		tb.Errorf("Launch: %v", err)
		return
	}
	if err := h.App.WaitSettled(h.SettleQuiet, h.SettleMax); err != nil {
		tb.Errorf("WaitSettled after Launch: %v", err)
		return
	}
	if err := h.App.Quit(h.QuitTimeout); err != nil {
		tb.Errorf("Quit: %v", err)
		return
	}
	running, err := h.App.Running()
	if err != nil {
		tb.Errorf("Running after Quit: %v", err)
		return
	}
	if running {
		tb.Errorf("Running() = true after Quit returned nil")
		return
	}
	before, err := fake.DirDigest(h.ProfilesDir)
	if err != nil {
		tb.Errorf("digest ProfilesDir: %v", err)
		return
	}
	time.Sleep(h.SettleQuiet)
	after, err := fake.DirDigest(h.ProfilesDir)
	if err != nil {
		tb.Errorf("digest ProfilesDir: %v", err)
		return
	}
	if before != after {
		tb.Errorf("ProfilesDir changed after Quit returned nil; contract A: no app process may write it afterwards")
	}
}

// AppControlQuitTimeout checks that an app which won't exit makes Quit return
// an error no earlier than the timeout, and is left running.
func AppControlQuitTimeout(tb TB, stuck ports.AppControl, timeout time.Duration) {
	tb.Helper()
	if err := stuck.Launch(); err != nil {
		tb.Errorf("Launch: %v", err)
		return
	}
	start := time.Now()
	err := stuck.Quit(timeout)
	elapsed := time.Since(start)
	if err == nil {
		tb.Errorf("Quit returned nil for an app that did not exit")
		return
	}
	if elapsed < timeout {
		tb.Errorf("Quit gave up after %s, before its %s timeout", elapsed, timeout)
	}
	running, rerr := stuck.Running()
	if rerr != nil {
		tb.Errorf("Running after failed Quit: %v", rerr)
		return
	}
	if !running {
		tb.Errorf("app reported not running after Quit failed")
	}
}
```

`fake.DirDigest` is a pure file-reading helper; importing the fake package from the suite keeps one implementation of "has this directory changed".

`internal/conformance/watcher.go`:

```go
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

package conformance

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/csmarshall/schrodeck/internal/ports"
)

// waitFor is how long a suite waits for an event: generous, derived from the
// debounce so a slow runner doesn't turn a pass into a flake.
func waitFor(debounce time.Duration) time.Duration { return 10*debounce + 2*time.Second }

// WatcherRecursive checks that a write three directory levels below the
// watched root produces an event. The directories exist before watching
// starts, so only the file write can trigger it.
func WatcherRecursive(tb TB, w ports.Watcher, root string, debounce time.Duration) {
	tb.Helper()
	deep := filepath.Join(root, "a", "b", "c")
	if err := os.MkdirAll(deep, 0o755); err != nil {
		tb.Errorf("mkdir: %v", err)
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	ch, err := w.Watch(ctx, []string{root}, debounce)
	if err != nil {
		tb.Errorf("Watch: %v", err)
		return
	}
	if err := os.WriteFile(filepath.Join(deep, "deep.txt"), []byte("x"), 0o644); err != nil {
		tb.Errorf("write: %v", err)
		return
	}
	select {
	case <-ch:
	case <-time.After(waitFor(debounce)):
		tb.Errorf("no event for a write three levels deep within %s; contract A requires recursive watching", waitFor(debounce))
	}
}

// WatcherBurstIsOneEvent checks that several writes inside one debounce
// window are delivered as exactly one event.
func WatcherBurstIsOneEvent(tb TB, w ports.Watcher, root string, debounce time.Duration) {
	tb.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	ch, err := w.Watch(ctx, []string{root}, debounce)
	if err != nil {
		tb.Errorf("Watch: %v", err)
		return
	}
	for i := 0; i < 5; i++ {
		if err := os.WriteFile(filepath.Join(root, fmt.Sprintf("f%d", i)), []byte("x"), 0o644); err != nil {
			tb.Errorf("write: %v", err)
			return
		}
	}
	select {
	case <-ch:
	case <-time.After(waitFor(debounce)):
		tb.Errorf("no event for a burst of writes")
		return
	}
	select {
	case ev := <-ch:
		tb.Errorf("a burst within one debounce window produced a second event %v", ev.Paths)
	case <-time.After(3 * debounce):
	}
}
```

`internal/conformance/identity.go`:

```go
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

package conformance

import "github.com/csmarshall/schrodeck/internal/ports"

// HostIdentityStable checks that the hardware id is present and identical
// across calls, and that the user name is present. Stability across two
// processes is checked by each real connector's own test, which runs a helper
// subprocess (M1).
func HostIdentityStable(tb TB, h ports.HostIdentity) {
	tb.Helper()
	a, err := h.HardwareID()
	if err != nil {
		tb.Errorf("HardwareID: %v", err)
		return
	}
	b, err := h.HardwareID()
	if err != nil {
		tb.Errorf("HardwareID (second call): %v", err)
		return
	}
	if a == "" {
		tb.Errorf("HardwareID is empty")
	}
	if a != b {
		tb.Errorf("HardwareID changed between two calls")
	}
	if h.UserName() == "" {
		tb.Errorf("UserName is empty")
	}
}
```

The suite never prints the hardware id itself (contract A: never logged).

- [ ] **Step 4: Run the tests**

Run: `go test -race ./internal/conformance/`
Expected: `ok` (each known-bad test passes because its suite recorded failures).

- [ ] **Step 5: Commit**

```bash
git add internal/conformance
git commit -m "feat(#3): contract A conformance suite skeleton; known-bad fakes must fail it"
```

---

### Task 7: Architecture checks (OS-free core, deckformat boundary)

**Files:**
- Create: `tools/archcheck/archcheck.go`, `tools/archcheck/archcheck_test.go`, `tools/archcheck/cmd/archcheck/main.go`

**Interfaces:**
- Produces (package `github.com/csmarshall/schrodeck/tools/archcheck`):
  - `type Package struct{ ImportPath string; Module *Module; Imports, TestImports, XTestImports []string }`, `type Module struct{ Path string }`
  - `OSPackages []string` (denylist)
  - `ModulePath(dir string) (string, error)` (reads `dir/go.mod`)
  - `List(dir string, env []string, args ...string) ([]Package, error)` (runs `go list -json`)
  - `CoreViolations(pkgs []Package, module string) []string`
  - `BoundaryViolations(pkgs []Package, forbiddenModule string) []string`
- Command: `go run ./tools/archcheck/cmd/archcheck -mode core -dir .` and `go run ./tools/archcheck/cmd/archcheck -mode boundary -dir deckformat -forbid .`

- [ ] **Step 1: Write the failing tests**

`tools/archcheck/archcheck_test.go`:

```go
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

package archcheck

import (
	"os"
	"path/filepath"
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
```

- [ ] **Step 2: Run to verify failure**

Run: `go test ./tools/archcheck/`
Expected: FAIL, `undefined: Package`.

- [ ] **Step 3: Implement**

`tools/archcheck/archcheck.go`:

```go
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

// Package archcheck enforces two import rules:
//   - the core (every package of the root module except cmd/, tools/ and
//     internal/connector/) imports nothing OS-specific and no connector,
//     command or tool package (ADR 0018);
//   - deckformat depends on no package of the schrodeck module (ADR 0031).
package archcheck

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

// Module is the part of `go list -json`'s Module we need.
type Module struct{ Path string }

// Package is the part of `go list -json` output we need.
type Package struct {
	ImportPath   string
	Module       *Module
	Imports      []string
	TestImports  []string
	XTestImports []string
}

// OSPackages may not be imported by core packages. A path matches an entry
// exactly or as a prefix followed by "/". "C" is cgo.
var OSPackages = []string{"os/exec", "os/signal", "os/user", "syscall", "golang.org/x/sys", "C"}

// exemptDirs are the root-module subtrees allowed to touch the OS.
var exemptDirs = []string{"/cmd/", "/tools/", "/internal/connector/"}

func matches(path, entry string) bool { return path == entry || strings.HasPrefix(path, entry+"/") }

func exempt(module, importPath string) bool {
	for _, d := range exemptDirs {
		if strings.HasPrefix(importPath+"/", module+d) {
			return true
		}
	}
	return false
}

// CoreViolations lists every forbidden import made by a core package of module.
func CoreViolations(pkgs []Package, module string) []string {
	var out []string
	for _, p := range pkgs {
		if p.Module == nil || p.Module.Path != module || exempt(module, p.ImportPath) {
			continue
		}
		all := append(append(append([]string{}, p.Imports...), p.TestImports...), p.XTestImports...)
		for _, imp := range all {
			bad := false
			for _, d := range OSPackages {
				if matches(imp, d) {
					bad = true
				}
			}
			if strings.HasPrefix(imp, module+"/") && exempt(module, imp) {
				bad = true
			}
			if bad {
				out = append(out, fmt.Sprintf("%s imports %s", p.ImportPath, imp))
			}
		}
	}
	sort.Strings(out)
	return dedupe(out)
}

// BoundaryViolations lists every package in a dependency closure that belongs
// to the forbidden module.
func BoundaryViolations(pkgs []Package, forbiddenModule string) []string {
	var out []string
	for _, p := range pkgs {
		if p.Module != nil && p.Module.Path == forbiddenModule {
			out = append(out, fmt.Sprintf("depends on %s (module %s)", p.ImportPath, forbiddenModule))
		}
	}
	sort.Strings(out)
	return out
}

func dedupe(s []string) []string {
	var out []string
	for i, v := range s {
		if i == 0 || v != s[i-1] {
			out = append(out, v)
		}
	}
	return out
}

// ModulePath reads the module path from dir/go.mod.
func ModulePath(dir string) (string, error) {
	b, err := os.ReadFile(filepath.Join(dir, "go.mod"))
	if err != nil {
		return "", err
	}
	sc := bufio.NewScanner(bytes.NewReader(b))
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if rest, ok := strings.CutPrefix(line, "module "); ok {
			return strings.Trim(strings.TrimSpace(rest), `"`), nil
		}
	}
	return "", fmt.Errorf("%s/go.mod has no module line", dir)
}

// List runs `go list -json <args>` in dir with extra environment entries.
func List(dir string, env []string, args ...string) ([]Package, error) {
	cmd := exec.Command("go", append([]string{"list", "-json"}, args...)...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), env...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("go list in %s: %v: %s", dir, err, stderr.String())
	}
	dec := json.NewDecoder(bytes.NewReader(out))
	var pkgs []Package
	for {
		var p Package
		if err := dec.Decode(&p); errors.Is(err, io.EOF) {
			break
		} else if err != nil {
			return nil, err
		}
		pkgs = append(pkgs, p)
	}
	return pkgs, nil
}
```

`tools/archcheck/cmd/archcheck/main.go`:

```go
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

// Command archcheck enforces schrodeck's import rules.
//
//	archcheck -mode core -dir .                          core is OS-free (ADR 0018)
//	archcheck -mode boundary -dir deckformat -forbid .   deckformat imports no schrodeck package (ADR 0031)
package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/csmarshall/schrodeck/tools/archcheck"
)

// goosList: a darwin-only file is invisible to `go list` on Linux, so the core
// is listed once per OS we keep buildable.
var goosList = []string{"darwin", "linux", "windows"}

func main() {
	mode := flag.String("mode", "", "core | boundary")
	dir := flag.String("dir", ".", "module directory to check")
	forbid := flag.String("forbid", ".", "boundary mode: directory of the module that must not be depended on")
	flag.Parse()

	var violations []string
	switch *mode {
	case "core":
		module, err := archcheck.ModulePath(*dir)
		fail(err)
		for _, goos := range goosList {
			pkgs, err := archcheck.List(*dir, []string{"GOOS=" + goos}, "./...")
			fail(err)
			for _, v := range archcheck.CoreViolations(pkgs, module) {
				violations = append(violations, "GOOS="+goos+": "+v)
			}
		}
	case "boundary":
		forbidden, err := archcheck.ModulePath(*forbid)
		fail(err)
		for _, goos := range goosList {
			pkgs, err := archcheck.List(*dir, []string{"GOOS=" + goos}, "-deps", "-test", "./...")
			fail(err)
			for _, v := range archcheck.BoundaryViolations(pkgs, forbidden) {
				violations = append(violations, "GOOS="+goos+": "+v)
			}
		}
	default:
		fmt.Fprintln(os.Stderr, "archcheck: -mode must be core or boundary")
		os.Exit(2)
	}
	for _, v := range violations {
		fmt.Fprintln(os.Stderr, v)
	}
	if len(violations) > 0 {
		os.Exit(1)
	}
	fmt.Printf("archcheck %s: ok (%v)\n", *mode, goosList)
}

func fail(err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, "archcheck:", err)
		os.Exit(2)
	}
}
```

- [ ] **Step 4: Run the tests and both checks on the repository**

Run: `go test ./tools/archcheck/ && go run ./tools/archcheck/cmd/archcheck -mode core -dir . && go run ./tools/archcheck/cmd/archcheck -mode boundary -dir deckformat -forbid .`
Expected: `ok`, `archcheck core: ok ([darwin linux windows])`, `archcheck boundary: ok ([darwin linux windows])`.

- [ ] **Step 5: Prove the core check fails on the real repository**

Add `import _ "os/exec"` to `internal/logging/logging.go`, run the core check, expect exit 1 with `internal/logging imports os/exec` for each GOOS. Remove the import and re-run: exit 0.

- [ ] **Step 6: Commit**

```bash
git add tools/archcheck
git commit -m "feat(#3): archcheck: OS-free core per GOOS and the deckformat import boundary, with known-bad end-to-end tests"
```

---

### Task 8: gofmt and leak-scan detectors with self-tests

**Files:**
- Create: `tools/ci/check-gofmt.sh`, `tools/ci/leak-scan.sh`, `tools/ci/set-leak-scan-secret.sh`
- Modify: `tools/ci/selftest.sh` (add their cases)

**Interfaces:**
- Produces: `tools/ci/check-gofmt.sh [file...]` (0 = formatted, 1 = not); `tools/ci/leak-scan.sh` (run from a repository root; 0 clean, 1 hit, 2 git error; reads optional `LEAK_SCAN_EXTRA`).

The owner's personal identifiers (the same list as the manual pre-push scan in the global rules) must not be published in this public repository, not even inside the scanner. The scanner therefore carries only generic patterns, and CI adds the personal ones from a repository secret, `LEAK_SCAN_EXTRA`. Pull requests from forks get no secrets; the scan then runs the generic patterns and says so on every run.

- [ ] **Step 1: Add the self-test cases (they fail until the scripts exist)**

In `tools/ci/selftest.sh`, insert before the final `if [[ $failures -gt 0 ]]` block:

```bash
# --- check-gofmt.sh ---------------------------------------------------------
{ header_go; printf 'package good\n\nfunc F() {}\n'; } >"$scratch/fmt_good.go"
{ header_go; printf 'package bad\nfunc  F( ){ }\n'; } >"$scratch/fmt_bad.go"
expect 0 "check-gofmt passes a formatted file" "$here/check-gofmt.sh" "$scratch/fmt_good.go"
expect 1 "check-gofmt fails an unformatted file" "$here/check-gofmt.sh" "$scratch/fmt_bad.go"

# --- leak-scan.sh -----------------------------------------------------------
# Each case is its own throwaway git repository, because the scanner reads
# tracked files only. Bad strings are assembled at runtime so this file
# itself never matches the scanner's patterns.
make_repo() { # make_repo <name> <file-content>
  local dir="$scratch/repo-$1"
  mkdir -p "$dir"
  git -C "$dir" init -q
  printf '%s\n' "$2" >"$dir/content.txt"
  git -C "$dir" add content.txt
  echo "$dir"
}
in_repo() { # in_repo <dir> <command...>
  local dir=$1
  shift
  (cd "$dir" && "$@")
}
clean=$(make_repo clean "docs use /Users/<user>/bin and @(1)[4057/143/<deck>]")
home=$(make_repo home "$(printf 'path: /Users/%s/bin/demo.sh' alice)")
device=$(make_repo device "$(printf 'id: @(1)[4057/143/%s]' AB12CD34EF)")
extra=$(make_repo extra "$(printf 'mentions %s here' zebra-marker)")
expect 0 "leak-scan passes placeholders (known-good)" in_repo "$clean" env LEAK_SCAN_EXTRA= "$here/leak-scan.sh"
expect 1 "leak-scan fails a real home path" in_repo "$home" env LEAK_SCAN_EXTRA= "$here/leak-scan.sh"
expect 1 "leak-scan fails a serial-bearing device id" in_repo "$device" env LEAK_SCAN_EXTRA= "$here/leak-scan.sh"
expect 0 "leak-scan without LEAK_SCAN_EXTRA misses a personal word" in_repo "$extra" env LEAK_SCAN_EXTRA= "$here/leak-scan.sh"
expect 1 "leak-scan with LEAK_SCAN_EXTRA catches it" in_repo "$extra" env LEAK_SCAN_EXTRA=zebra-marker "$here/leak-scan.sh"
generic_only=$(in_repo "$clean" env LEAK_SCAN_EXTRA= "$here/leak-scan.sh" 2>&1 | grep -c 'personal identifiers were NOT checked' || true)
if [[ $generic_only -gt 0 ]]; then
  echo "ok    leak-scan reports generic-only mode"
else
  echo "FAIL  leak-scan did not say the personal patterns were skipped"
  failures=$((failures + 1))
fi
```

- [ ] **Step 2: Run to verify the new cases fail**

Run: `tools/ci/selftest.sh`
Expected: the header cases `ok`; the gofmt and leak-scan cases `FAIL` (exit 127); final exit 1.

- [ ] **Step 3: Write `check-gofmt.sh`**

```bash
#!/usr/bin/env bash
# This Source Code Form is subject to the terms of the Mozilla Public
# License, v. 2.0. If a copy of the MPL was not distributed with this
# file, You can obtain one at https://mozilla.org/MPL/2.0/.
#
# Fails if a Go file is not gofmt-formatted.
# Usage: tools/ci/check-gofmt.sh [file...]
#        With no arguments, checks every tracked .go file.
unset TMOUT
set -euo pipefail

files=()
if [[ $# -gt 0 ]]; then
  files=("$@")
else
  while IFS= read -r f; do files+=("$f"); done < <(git ls-files '*.go')
fi
if [[ ${#files[@]} -eq 0 ]]; then
  echo "check-gofmt: no Go files"
  exit 0
fi
unformatted=$(gofmt -l "${files[@]}")
if [[ -n $unformatted ]]; then
  echo "check-gofmt: these files need gofmt:" >&2
  echo "$unformatted" >&2
  exit 1
fi
echo "check-gofmt: ${#files[@]} file(s) formatted"
```

- [ ] **Step 4: Write `leak-scan.sh`**

```bash
#!/usr/bin/env bash
# This Source Code Form is subject to the terms of the Mozilla Public
# License, v. 2.0. If a copy of the MPL was not distributed with this
# file, You can obtain one at https://mozilla.org/MPL/2.0/.
#
# Leak scan (ADR 0020): fails if a tracked file contains a personal identifier.
# Generic patterns are built in. The owner's personal identifiers are NOT in
# this public file: they come from $LEAK_SCAN_EXTRA (a PCRE alternation) that
# CI reads from a repository secret. Without it, only the generic patterns run
# and the scan says so.
# Usage: tools/ci/leak-scan.sh   (from a repository root)
unset TMOUT
set -euo pipefail

# - home directories of real machines (docs use the /Users/<user> placeholder)
# - Stream Deck device ids that still carry a USB serial (fixtures use <deck>)
readonly GENERIC='/Users/(?!Shared/)[a-z]|@\([0-9]+\)\[[0-9]+/[0-9]+/[A-Za-z0-9]{6,}\]'

pattern=$GENERIC
label="generic patterns"
if [[ -n ${LEAK_SCAN_EXTRA:-} ]]; then
  pattern="$GENERIC|$LEAK_SCAN_EXTRA"
  label="generic and personal patterns"
else
  echo "leak-scan: WARNING: LEAK_SCAN_EXTRA is empty, so personal identifiers were NOT checked" >&2
fi

set +e
hits=$(git grep -nIiP -e "$pattern" -- . ':(exclude)LICENSE')
rc=$?
set -e
case $rc in
  0)
    printf '%s\n' "$hits" >&2
    echo "leak-scan: FAILED ($label)" >&2
    exit 1
    ;;
  1)
    echo "leak-scan: clean ($label)"
    ;;
  *)
    echo "leak-scan: git grep failed (exit $rc)" >&2
    exit 2
    ;;
esac
```

- [ ] **Step 5: Write the secret-setup script for the owner**

`tools/ci/set-leak-scan-secret.sh`:

```bash
#!/usr/bin/env bash
# This Source Code Form is subject to the terms of the Mozilla Public
# License, v. 2.0. If a copy of the MPL was not distributed with this
# file, You can obtain one at https://mozilla.org/MPL/2.0/.
#
# Stores the owner's personal leak-scan patterns as the LEAK_SCAN_EXTRA
# repository secret. The value is read from a local file that is never
# committed, so the identifiers never appear in this public repository.
# Usage: tools/ci/set-leak-scan-secret.sh <file-with-one-PCRE-alternation>
unset TMOUT
set -euo pipefail

if [[ $# -ne 1 || ! -f $1 ]]; then
  echo "usage: $0 <file-with-one-PCRE-alternation>" >&2
  exit 2
fi
gh secret set LEAK_SCAN_EXTRA --repo csmarshall/schrodeck <"$1"
echo "LEAK_SCAN_EXTRA set; the next CI run will report 'generic and personal patterns'"
```

```bash
chmod +x tools/ci/check-gofmt.sh tools/ci/leak-scan.sh tools/ci/set-leak-scan-secret.sh
```

- [ ] **Step 6: Run the self-tests and the real checks**

Run: `tools/ci/selftest.sh && git add -A && tools/ci/check-gofmt.sh && tools/ci/leak-scan.sh`
Expected: every self-test line `ok`; `check-gofmt: N file(s) formatted`; `leak-scan: clean (generic patterns)` plus the WARNING line (no secret locally). Then run the manual pre-push scan from the global rules as well; it must print nothing.

- [ ] **Step 7: Commit**

```bash
git add tools/ci
git commit -m "ci(#3): gofmt and leak-scan detectors with self-tests; personal patterns come from a secret"
```

---

### Task 9: CI workflow, pull request, merge

**Files:**
- Create: `.github/workflows/ci.yml`

**Interfaces:**
- Consumes: every script and tool above.
- Produces: required status checks `checks (linux)` and `tests (macOS)`.

- [ ] **Step 1: Write the workflow**

`.github/workflows/ci.yml`:

```yaml
# This Source Code Form is subject to the terms of the Mozilla Public
# License, v. 2.0. If a copy of the MPL was not distributed with this
# file, You can obtain one at https://mozilla.org/MPL/2.0/.
name: ci

on:
  pull_request:
  push:
    branches: [main]

permissions:
  contents: read

concurrency:
  group: ci-${{ github.ref }}
  cancel-in-progress: true

env:
  STATICCHECK_VERSION: v0.8.1

jobs:
  checks:
    name: checks (linux)
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@3d3c42e5aac5ba805825da76410c181273ba90b1 # v7.0.1
      - uses: actions/setup-go@b7ad1dad31e06c5925ef5d2fc7ad053ef454303e # v7.0.0
        with:
          go-version-file: go.mod
      - name: Detector self-tests (each detector seen to fail on known-bad input)
        run: tools/ci/selftest.sh
      - name: MPL-2.0 headers
        run: tools/ci/check-headers.sh
      - name: gofmt
        run: tools/ci/check-gofmt.sh
      - name: Leak scan
        env:
          LEAK_SCAN_EXTRA: ${{ secrets.LEAK_SCAN_EXTRA }}
        run: tools/ci/leak-scan.sh
      - name: go vet
        run: |
          go vet ./...
          (cd deckformat && go vet ./...)
      - name: staticcheck
        run: |
          go install honnef.co/go/tools/cmd/staticcheck@"$STATICCHECK_VERSION"
          staticcheck ./...
          (cd deckformat && staticcheck ./...)
      - name: Core is OS-free (ADR 0018)
        run: go run ./tools/archcheck/cmd/archcheck -mode core -dir .
      - name: deckformat imports no schrodeck package (ADR 0031)
        run: go run ./tools/archcheck/cmd/archcheck -mode boundary -dir deckformat -forbid .
      - name: Ports match contract A (ADR 0024)
        run: go run ./tools/portcheck/cmd/portcheck -doc docs/contracts/os-connector.md -src internal/ports
      - name: Cross-build for every OS we keep possible
        run: |
          for goos in darwin linux windows; do
            GOOS=$goos go build ./...
            (cd deckformat && GOOS=$goos go build ./...)
          done
      - name: Tests (Linux)
        run: |
          go test -race ./...
          (cd deckformat && go test -race ./...)

  macos:
    name: tests (macOS)
    runs-on: macos-latest
    steps:
      - uses: actions/checkout@3d3c42e5aac5ba805825da76410c181273ba90b1 # v7.0.1
      - uses: actions/setup-go@b7ad1dad31e06c5925ef5d2fc7ad053ef454303e # v7.0.0
        with:
          go-version-file: go.mod
      - name: Tests (macOS)
        run: |
          go test -race ./...
          (cd deckformat && go test -race ./...)
```

- [ ] **Step 2: Run everything the workflow runs, locally**

Run:

```bash
tools/ci/selftest.sh && tools/ci/check-headers.sh && tools/ci/check-gofmt.sh && tools/ci/leak-scan.sh \
 && go vet ./... && (cd deckformat && go vet ./...) \
 && go run honnef.co/go/tools/cmd/staticcheck@v0.8.1 ./... && (cd deckformat && go run honnef.co/go/tools/cmd/staticcheck@v0.8.1 ./...) \
 && go run ./tools/archcheck/cmd/archcheck -mode core -dir . \
 && go run ./tools/archcheck/cmd/archcheck -mode boundary -dir deckformat -forbid . \
 && go run ./tools/portcheck/cmd/portcheck \
 && go test -race ./... && (cd deckformat && go test -race ./...)
```

Expected: every step prints its ok line; exit 0. If staticcheck v0.8.1 does not support Go 1.27, it says so explicitly; then pin the newest release that does (`go list -m -versions honnef.co/go/tools`), update `STATICCHECK_VERSION`, and note it in the PR body.

- [ ] **Step 3: Commit and push the branch**

```bash
git add .github/workflows/ci.yml
git commit -m "ci(#3): GitHub Actions: checks on Linux, tests on Linux and macOS"
export $(cat ~/.ssh_agent_socket)
git push -u origin 3-scaffold
```

(Pushing is an outward action: confirm with the owner first if this plan is being executed without standing approval.)

- [ ] **Step 4: Open the PR**

```bash
gh pr create --title "M0: Go scaffold, contract E CLI, ports + fakes + conformance, CI" --body-file - <<'EOF'
Closes #3

M0 foundations per docs/superpowers/plans/2026-10-02-m0-foundations.md.

- Go module + nested `deckformat` module (ADR 0031), CLI skeleton with `version`/`status`/`doctor` and `--json` (contract E, new `docs/contracts/cli-json.md`, golden tests).
- Port interfaces mirroring contract A (Watcher gains a `ctx`), fakes for every port, conformance-suite skeleton; each suite fails on a known-bad fake.
- CI: self-tests for every detector, MPL headers, gofmt, leak scan (generic patterns + `LEAK_SCAN_EXTRA` secret), vet, staticcheck, OS-free core per GOOS, deckformat boundary, contract A port check, cross-build, tests on Linux and macOS.
- Moved from issue #3 to M1: the normalization fixtures (they need the hasher, which M1 builds).
EOF
```

Append the session link line required by the current attribution rules to the PR body if executing in a session that has one.

- [ ] **Step 5: Owner sets the leak-scan secret**

Ask the owner to run (it reads the pattern list from a local, uncommitted file):

```bash
tools/ci/set-leak-scan-secret.sh ~/path/to/leak-patterns.txt 2>&1 | tee set-leak-scan-secret_$(date +"%F-%H%M.%S").log
```

Then re-run CI (`gh run rerun <run-id>`) and confirm the Leak scan step prints `clean (generic and personal patterns)`.

- [ ] **Step 6: Wait for green CI**

Run: `gh pr checks --watch` then `gh pr view --json mergeStateStatus`
Expected: both checks `pass`; `mergeStateStatus` is `CLEAN`. If a job sticks at `queued` while its run reports success, `gh run rerun <run-id>`.

- [ ] **Step 7: Code review, owner review, merge**

Dispatch a code-reviewer subagent on `gh pr diff` (it must not `open` anything and must end with DECISIONS NEEDED). Fix findings in new commits on the branch. After the owner approves: `gh pr merge --squash --delete-branch`, then `git -C ~/work/personal/schrodeck pull` and `git worktree remove ~/work/claude/schrodeck-worktrees/3-scaffold`.

- [ ] **Step 8: Make the checks required**

Ask the owner to mark `checks (linux)` and `tests (macOS)` as required status checks on `main` (repository settings or `gh api` with branch protection). This is a repository-settings change, so it needs the owner's go-ahead.

---

## Self-review

- **Spec / milestone coverage:** M0 row: repo + CI (Tasks 1, 8, 9), Go core with no OS imports (Task 7), port interfaces (Task 4) + fakes (Task 5) + conformance skeleton (Task 6). Issue #3 bullets: CLI skeleton with `status --json` and `doctor` wiring (Task 3); core with no OS imports (Task 7); ports exactly per contract A (Task 4) with fakes and conformance where a fake violating `Quit` fails (Task 6); gofmt/vet/staticcheck/test on Linux and macOS (Task 9); MPL header on every source file (Task 1); normalization fixtures → moved to M1 (stated in the PR body). ADR 0024's "Verified by" for A (conformance + port-name check) and E (golden files) are covered; C and D belong to M1 and M2.
- **Placeholders:** none; every code step has complete code. The staticcheck-version fallback is a concrete procedure, not a TBD.
- **Type consistency:** `cli.Check`/`CheckResult`/`Status*` are used identically in Tasks 3; `ports.Event{Paths}` and `Watch(ctx, …)` match between Task 4, the fake (Task 5) and the suite (Task 6); `fake.DirDigest` is defined in Task 5 and used in Task 6.
- **Review Focus:** each line names its pinning test and owning task.
