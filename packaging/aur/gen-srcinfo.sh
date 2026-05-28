#!/usr/bin/env bash
# Regenerate packaging/aur/.SRCINFO from PKGBUILD, deterministically and
# without depending on `makepkg --printsrcinfo` (which is Arch-only and not
# available on ubuntu-latest CI runners).
#
# The output format mirrors what `makepkg --printsrcinfo` emits for our
# single-pkgname PKGBUILD. CI runs this script and `git diff --exit-code` on
# .SRCINFO so accidental drift (e.g. license change in PKGBUILD without
# .SRCINFO update) fails the build.
#
# Usage: ./gen-srcinfo.sh > .SRCINFO   (or just `./gen-srcinfo.sh` to update in place)

set -euo pipefail

cd -- "$(dirname -- "$0")"

# shellcheck disable=SC1091
source ./PKGBUILD

emit() {
	printf 'pkgbase = %s\n' "$pkgname"
	printf '\tpkgdesc = %s\n' "$pkgdesc"
	printf '\tpkgver = %s\n' "$pkgver"
	printf '\tpkgrel = %s\n' "$pkgrel"
	printf '\turl = %s\n' "$url"
	for a in "${arch[@]}"; do printf '\tarch = %s\n' "$a"; done
	for l in "${license[@]}"; do printf '\tlicense = %s\n' "$l"; done
	for m in "${makedepends[@]}"; do printf '\tmakedepends = %s\n' "$m"; done
	for d in "${depends[@]}"; do printf '\tdepends = %s\n' "$d"; done
	for s in "${source[@]}"; do printf '\tsource = %s\n' "$s"; done
	for h in "${sha256sums[@]}"; do printf '\tsha256sums = %s\n' "$h"; done
	printf '\n'
	printf 'pkgname = %s\n' "$pkgname"
}

if [[ "${1:-}" == "--check" ]]; then
	# Diff the generator output against the committed file. Used by CI.
	diff -u .SRCINFO <(emit)
else
	emit > .SRCINFO
	echo "Wrote .SRCINFO" >&2
fi
