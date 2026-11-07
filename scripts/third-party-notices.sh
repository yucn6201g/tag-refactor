#!/usr/bin/env bash
# Writes the license notices of everything linked into the tag-refactor
# binary: the Go standard library and every module that the binary of any
# released OS/CPU pair imports. The release workflow puts the file into each
# archive next to the binary, as the licenses of the dependencies ask
# (Apache-2.0 4(a)(d), MIT, BSD, and MPL-2.0 3.2 for hashicorp/hcl/v2, which
# is used unmodified as a library).
#
# Usage: scripts/third-party-notices.sh <output file> [GOOS/GOARCH ...]
# The default pairs are the released ones. It fails when a module has no
# license file, so a new dependency without one stops the release.
set -euo pipefail

out=${1:?usage: third-party-notices.sh <output file> [GOOS/GOARCH ...]}
shift
targets=("$@")
if [ ${#targets[@]} -eq 0 ]; then
  targets=(linux/amd64 linux/arm64 darwin/amd64 darwin/arm64 windows/amd64 windows/arm64)
fi

modules=$(mktemp)
trap 'rm -f "$modules"' EXIT
for target in "${targets[@]}"; do
  CGO_ENABLED=0 GOOS="${target%/*}" GOARCH="${target#*/}" go list -mod=readonly -deps \
    -f '{{with .Module}}{{if not .Main}}{{.Path}}	{{.Version}}	{{.Dir}}{{end}}{{end}}' \
    ./cmd/tag-refactor >>"$modules"
done

goroot=$(go env GOROOT)
{
  echo "Third-party notices for tag-refactor"
  echo
  echo "tag-refactor is licensed under the Apache License 2.0 (see LICENSE)."
  echo "The binary includes the Go standard library and the Go modules listed"
  echo "below, unmodified. Each entry gives the module, its version, where its"
  echo "source code can be obtained, and the text of its license and notice"
  echo "files. Source code of a module is also available from the Go module"
  echo "proxy at https://proxy.golang.org/<module>/@v/<version>.zip (upper-case"
  echo "letters in <module> are written as '!' and the lower-case letter)."
  echo
  echo "================================================================"
  echo "Go standard library ($(go env GOVERSION))"
  echo "Source: https://go.dev/dl/"
  echo "================================================================"
  for f in LICENSE PATENTS; do
    if [ -f "$goroot/$f" ]; then
      echo
      echo "--- $f ---"
      tr -d '\r' <"$goroot/$f"
    fi
  done

  # Sort by module path; the same module appears once whatever the target.
  sort -u -t "	" -k1,1 "$modules" | while IFS="	" read -r path version dir; do
    if [ -z "$dir" ] || [ ! -d "$dir" ]; then
      echo "third-party-notices: no source directory for $path $version (run go mod download)" >&2
      exit 1
    fi
    files=$(find "$dir" -maxdepth 1 -type f \( -iname 'licen[cs]e*' -o -iname 'copying*' -o -iname 'notice*' -o -iname 'patents*' \) | sort)
    if ! grep -qi -e '/licen[cs]e[^/]*$' -e '/copying[^/]*$' <<<"$files"; then
      echo "third-party-notices: $path $version has no license file" >&2
      exit 1
    fi
    escaped=$(sed -E 's/([A-Z])/!\L\1/g' <<<"$path")
    echo
    echo "================================================================"
    echo "$path $version"
    echo "Source: https://proxy.golang.org/$escaped/@v/$version.zip"
    echo "================================================================"
    while IFS= read -r f; do
      echo
      echo "--- $(basename "$f") ---"
      tr -d '\r' <"$f"
    done <<<"$files"
  done
} >"$out"
