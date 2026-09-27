#!/bin/sh
# Build on this workstation and deploy to a server over SSH. The servers never
# compile anything: the LAN dev box has ~1 GB RAM, and production shouldn't
# carry a toolchain.
#
#   deploy/push.sh linux   user@devbox                              # LAN dev server
#   deploy/push.sh openbsd user@vps DOMAIN=wintermuteconsulting.com WWW=1
#
# Any NAME=value arguments are passed to the setup script (see its header).
# Needs SSH access and sudo (Linux) or doas (OpenBSD) on the target; you'll be
# prompted for the password if they require one. Also copies security/run.sh
# so `sh run.sh host` can audit the server afterwards.
set -eu

os=${1:-}
target=${2:-}
case $os in linux | openbsd) ;; *)
	sed -n '2,13p' "$0" | sed 's/^# \{0,1\}//'
	exit 2
	;;
esac
[ -n "$target" ] || {
	echo "usage: $0 $os [user@]host [NAME=value ...]" >&2
	exit 2
}
shift 2
for kv in "$@"; do
	case $kv in [A-Z_]*=*) ;; *)
		echo "not a NAME=value setting: $kv" >&2
		exit 2
		;;
	esac
done

root=$(cd "$(dirname "$0")/.." && pwd)

case $(ssh "$target" uname -m) in
x86_64 | amd64) arch=amd64 ;;
aarch64 | arm64) arch=arm64 ;;
*) echo "unsupported remote architecture" >&2 && exit 1 ;;
esac

sh "$root/deploy/build.sh" "$os" "$arch"

dir=wintermute-deploy
# shellcheck disable=SC2029 # $dir is meant to expand here
ssh "$target" "mkdir -p $dir"
scp -q "$root/dist/wintermute-$os-$arch" "$root/deploy/setup-$os.sh" "$root/security/run.sh" "$target:$dir/"

if [ "$os" = openbsd ]; then su=doas; else su=sudo; fi
# Quote each setting for the remote shell.
settings=""
for kv in "$@"; do
	settings="$settings '$(printf '%s' "$kv" | sed "s/'/'\\\\''/g")'"
done
# shellcheck disable=SC2029
ssh -t "$target" "cd $dir && $su env$settings sh setup-$os.sh"
