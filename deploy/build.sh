#!/bin/sh
# Cross-compile the site into a single static binary for a target host.
#
#   deploy/build.sh linux            # -> dist/wintermuteconsulting-linux-amd64
#   deploy/build.sh openbsd arm64    # -> dist/wintermuteconsulting-openbsd-arm64
#
# Templates and assets are embedded, so the binary is the whole deployment.
# Run from anywhere inside the repo; needs Go on the build machine only.
set -eu

os=${1:-linux}
arch=${2:-amd64}

case $os in
linux | openbsd) ;;
*)
	echo "usage: $0 linux|openbsd [amd64|arm64]" >&2
	exit 2
	;;
esac

root=$(cd "$(dirname "$0")/.." && pwd)
out="$root/dist/wintermuteconsulting-$os-$arch"
mkdir -p "$root/dist"

cd "$root"
go test ./...
CGO_ENABLED=0 GOOS=$os GOARCH=$arch go build -trimpath -ldflags "-s -w" -o "$out" .

echo "built $out"
