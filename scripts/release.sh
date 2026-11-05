#!/bin/sh
set -eu

version=${1:?Usage: ./scripts/release.sh VERSION}
case "$version" in
  *[!a-zA-Z0-9._-]*|'') echo "Invalid version" >&2; exit 1 ;;
esac
mkdir -p dist
stage=$(mktemp -d)
trap 'rm -rf "$stage"' EXIT HUP INT TERM
for target in darwin/arm64 darwin/amd64 linux/arm64 linux/amd64; do
  target_os=${target%/*}
  target_arch=${target#*/}
  CGO_ENABLED=0 GOOS="$target_os" GOARCH="$target_arch" go build -trimpath \
    -ldflags "-s -w -X main.version=${version#v}" -o "$stage/hop" ./cmd/hop
  cp README.md LICENSE THIRD_PARTY_NOTICES "$stage/"
  tar -czf "dist/hop_${version}_${target_os}_${target_arch}.tar.gz" -C "$stage" \
    hop README.md LICENSE THIRD_PARTY_NOTICES
done
(cd dist && shasum -a 256 "hop_${version}_"*.tar.gz > checksums.txt)
