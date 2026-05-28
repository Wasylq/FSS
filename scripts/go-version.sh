#!/usr/bin/env sh
# Print the Go version declared in go.mod (e.g. "1.26.2").
#
# Used by the Docker workflows to keep the Dockerfile's GO_VERSION build-arg
# in sync with go.mod, and by the CI lint job to guard against drift between
# the Dockerfile default and go.mod. Local builders can run this directly:
#
#     docker build --build-arg GO_VERSION=$(scripts/go-version.sh) .
#
# Or rely on the Dockerfile's fallback default (kept in sync by the guard).

set -eu

cd -- "$(dirname -- "$0")/.."

# go.mod's `go` directive lines look like:
#   go 1.26.2
# Pick the first matching line and emit only the version token.
version=$(awk '/^go [0-9]+\.[0-9]+(\.[0-9]+)?$/ { print $2; exit }' go.mod)

if [ -z "$version" ]; then
	echo "go-version.sh: could not find 'go X.Y[.Z]' directive in go.mod" >&2
	exit 1
fi

printf '%s\n' "$version"
