#!/usr/bin/env bash
# This Source Code Form is subject to the terms of the Mozilla Public
# License, v. 2.0. If a copy of the MPL was not distributed with this
# file, You can obtain one at https://mozilla.org/MPL/2.0/.
#
# Self-tests for the CI detectors in tools/ci/. Each detector is run against a known-good input (must pass) and at least one known-bad input (must fail). A detector that has never been seen to fail has not been tested. Every case runs; the script exits non-zero if any verdict was wrong.
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

# A missing detector must not let the want-1 cases pass by accident (bash 3.2 reports a failed exec as exit 1 here, not 127).
if [[ ! -x "$here/check-headers.sh" ]]; then
  echo "selftest: $here/check-headers.sh is missing or not executable" >&2
  exit 1
fi

# in_dir <dir> <command...>: run a command from inside a directory.
in_dir() {
  local dir=$1
  shift
  (cd "$dir" && "$@")
}

header_go() {
  printf '%s\n' \
    '// This Source Code Form is subject to the terms of the Mozilla Public' \
    '// License, v. 2.0. If a copy of the MPL was not distributed with this' \
    '// file, You can obtain one at https://mozilla.org/MPL/2.0/.'
}

header_sh() {
  printf '%s\n' \
    '# This Source Code Form is subject to the terms of the Mozilla Public' \
    '# License, v. 2.0. If a copy of the MPL was not distributed with this' \
    '# file, You can obtain one at https://mozilla.org/MPL/2.0/.'
}

# --- check-headers.sh with explicit files ------------------------------------
{ header_go; printf 'package good\n'; } >"$scratch/good.go"
printf 'package bad\n' >"$scratch/bad.go"
{ printf '#!/usr/bin/env bash\n'; header_sh; } >"$scratch/good.sh"
# Header present but not at the top: must fail now that placement is enforced.
{ printf 'package misplaced\n\n'; header_go; } >"$scratch/misplaced.go"
# Shebang file whose header is on line 3 instead of line 2.
{ printf '#!/usr/bin/env bash\n\n'; header_sh; } >"$scratch/misplaced.sh"
# Header cut short after its first line.
{ header_go | sed -n 1p; printf 'package truncated\n'; } >"$scratch/truncated.go"
# Header whose second line is altered.
{ header_go | sed -n 1p; printf '// License, v. 3.0.\n'; header_go | sed -n 3p; printf 'package altered\n'; } >"$scratch/altered.go"

expect 0 "check-headers passes a Go file with the header" "$here/check-headers.sh" "$scratch/good.go"
expect 0 "check-headers passes a script with shebang + header" "$here/check-headers.sh" "$scratch/good.sh"
expect 1 "check-headers fails a Go file without the header" "$here/check-headers.sh" "$scratch/bad.go"
expect 1 "check-headers fails when one of several files lacks it" "$here/check-headers.sh" "$scratch/good.go" "$scratch/bad.go"
expect 1 "check-headers fails a Go file whose header is not at the top" "$here/check-headers.sh" "$scratch/misplaced.go"
expect 1 "check-headers fails a script whose header is below line 2" "$here/check-headers.sh" "$scratch/misplaced.sh"
expect 1 "check-headers fails a truncated header" "$here/check-headers.sh" "$scratch/truncated.go"
expect 1 "check-headers fails an altered header" "$here/check-headers.sh" "$scratch/altered.go"

# --- check-headers.sh with no arguments (the mode CI runs) --------------------
# Each case is its own throwaway git repository; the checker lists tracked files, so they are staged with git add.
repo_good="$scratch/repo-good"
repo_bad="$scratch/repo-bad"
repo_empty="$scratch/repo-empty"
for r in "$repo_good" "$repo_bad" "$repo_empty"; do
  mkdir -p "$r"
  git -C "$r" init -q
done
cp "$scratch/good.go" "$scratch/good.sh" "$repo_good/"
cp "$scratch/good.go" "$scratch/bad.go" "$repo_bad/"
printf 'not a source file\n' >"$repo_empty/notes.txt"
git -C "$repo_good" add good.go good.sh
git -C "$repo_bad" add good.go bad.go
git -C "$repo_empty" add notes.txt

expect 0 "check-headers (no args) passes a repo of headed files" in_dir "$repo_good" "$here/check-headers.sh"
expect 1 "check-headers (no args) fails a repo with an unheaded file" in_dir "$repo_bad" "$here/check-headers.sh"
expect 1 "check-headers (no args) fails when it finds 0 files to check" in_dir "$repo_empty" "$here/check-headers.sh"

if [[ $failures -gt 0 ]]; then
  echo "selftest: $failures detector verdict(s) wrong" >&2
  exit 1
fi
echo "selftest: all detector verdicts as expected"
