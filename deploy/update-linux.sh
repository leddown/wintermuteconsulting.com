#!/bin/sh
# Pull the latest code and redeploy, run ON the LAN dev server from its git
# checkout. Builds locally with low-memory settings (one compile at a time,
# eager GC): about 230 MB peak on a first build, about 30 MB once cached.
#
#   cd ~/wintermuteconsulting.com && deploy/update-linux.sh
#
# Settings (environment):
#   TEST=1   also run the short test suite before deploying (slower; the full
#            suite, race detector and scanners stay on the workstation)
#
# One-time setup on the dev server:
#   1. Go, official build (distro packages are usually too old; go.mod pins
#      the toolchain and Go downloads it on first build if yours is older):
#        curl -LO https://go.dev/dl/go1.26.8.linux-amd64.tar.gz
#        sha256sum go1.26.8.linux-amd64.tar.gz   # compare with https://go.dev/dl/
#        sudo tar -C /usr/local -xzf go1.26.8.linux-amd64.tar.gz
#        echo 'export PATH=$PATH:/usr/local/go/bin' >> ~/.profile
#   2. Read-only access to the repo: a GitHub deploy key (Settings > Deploy
#      keys, "Allow write access" unticked), then:
#        git clone git@github.com:leddown/wintermuteconsulting.com.git
#   3. First deploy: deploy/update-linux.sh (it runs setup-linux.sh, which
#      creates the service and /etc/wintermuteconsulting/wintermuteconsulting.env)
set -eu

say() { printf '==> %s\n' "$*"; }
die() {
	printf 'error: %s\n' "$*" >&2
	exit 1
}

repo=$(cd "$(dirname "$0")/.." && pwd)
cd "$repo"

[ "$(id -u)" -ne 0 ] || die "run as your normal user; it uses sudo only to install"
command -v git >/dev/null || die "git is not installed"
command -v go >/dev/null || die "Go is not installed or not on PATH; see the header of $0"

case $(uname -m) in
x86_64) arch=amd64 ;;
aarch64 | arm64) arch=arm64 ;;
*) die "unsupported architecture $(uname -m)" ;;
esac

before=$(git rev-parse --short HEAD)
say "pulling"
git pull --ff-only
after=$(git rev-parse --short HEAD)
if [ "$before" = "$after" ]; then
	say "already at $after; rebuilding and redeploying anyway"
else
	say "updated $before -> $after"
	git log --oneline "$before..$after" | sed 's/^/    /'
fi

# Low-memory build: -p 1 compiles one package at a time, GOMAXPROCS=1 keeps
# each compiler single-threaded, GOGC=50 collects garbage twice as eagerly.
export GOMAXPROCS=1 GOGC=50 CGO_ENABLED=0
if [ "${TEST:-0}" = 1 ]; then
	say "testing (short)"
	go test -p 1 -short ./...
fi
out=dist/wintermuteconsulting-linux-$arch
say "building $out"
mkdir -p dist
go build -p 1 -trimpath -ldflags "-s -w" -o "$out" .

say "deploying (sudo)"
sudo env BIN="$repo/$out" sh "$repo/deploy/setup-linux.sh"
