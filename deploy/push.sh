#!/bin/sh
# Build on this workstation and deploy over SSH. The target never compiles.
#
# OpenBSD production (smallest footprint: base system only, nothing installed):
#   deploy/push.sh openbsd user@vps DOMAIN=example.com WWW=1   # first time / config change:
#                                                              #   full setup-openbsd.sh
#   deploy/push.sh openbsd user@vps                            # every update after that:
#                                                              #   new binary + restart only
#
# Linux dev server (usually updated on the box with deploy/update-linux.sh
# instead; this is the push alternative):
#   deploy/push.sh linux user@devbox
#
# Rule: NAME=value settings mean "run the full setup script"; none means
# "replace the binary". A binary-only update keeps the previous binary as
# <binary>.prev and puts it back automatically if the new one fails its health
# check. Needs doas (OpenBSD) or sudo (Linux) on the target; you'll be prompted
# if they ask for a password.
set -eu

os=${1:-}
target=${2:-}
case $os in linux | openbsd) ;; *)
	sed -n '2,19p' "$0" | sed 's/^# \{0,1\}//'
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
if [ "$os" = openbsd ]; then su=doas; else su=sudo; fi

case $(ssh "$target" uname -m) in
x86_64 | amd64) arch=amd64 ;;
aarch64 | arm64) arch=arm64 ;;
*) echo "unsupported remote architecture" >&2 && exit 1 ;;
esac

sh "$root/deploy/build.sh" "$os" "$arch"
bin=$root/dist/wintermuteconsulting-$os-$arch

if [ $# -eq 0 ]; then
	# Binary-only update: one file up, swap, restart, health check, roll back on failure.
	rhome=$(ssh "$target" pwd)
	up=$rhome/wintermuteconsulting.upload
	upd=$rhome/wintermuteconsulting-update.sh
	tmp=$(mktemp)
	cat >"$tmp" <<'EOF'
set -eu
b=/usr/local/bin/wintermuteconsulting
[ -x "$b" ] || { echo "not installed yet: run push.sh with settings (e.g. DOMAIN=...) for the first setup" >&2; exit 1; }
install -m 0555 "$1" "$b.new"
cp -p "$b" "$b.prev"
mv -f "$b.new" "$b"
case $(uname -s) in
OpenBSD)
	restart() { rcctl restart wintermuteconsulting >/dev/null; }
	addr=$(rcctl get wintermuteconsulting flags | sed -n 's/.*-addr \([^ ]*\).*/\1/p')
	probe() { ftp -Vo /dev/null "http://$addr/healthz" >/dev/null 2>&1; }
	;;
*)
	restart() { systemctl restart wintermuteconsulting; }
	addr=$(sed -n 's/^ADDR=//p' /etc/wintermuteconsulting/wintermuteconsulting.env | tail -1)
	probe() { curl -fsS -o /dev/null "http://$addr/healthz"; }
	;;
esac
restart
i=0
until probe; do
	i=$((i + 1))
	if [ $i -ge 10 ]; then
		echo "new binary failed its health check on $addr; restoring the previous one" >&2
		mv -f "$b.prev" "$b"
		restart
		exit 1
	fi
	sleep 1
done
rm -f "$1"
echo "==> updated and healthy on $addr (previous binary kept as $b.prev)"
EOF
	scp -q "$bin" "$target:$up"
	scp -q "$tmp" "$target:$upd"
	rm -f "$tmp"
	# shellcheck disable=SC2029 # paths are meant to expand here
	ssh -t "$target" "$su sh $upd $up; rc=\$?; rm -f $upd; exit \$rc"
	exit
fi

# Full setup: binary, setup script and the host audit script.
dir=wintermuteconsulting-deploy
# shellcheck disable=SC2029 # $dir is meant to expand here
ssh "$target" "mkdir -p $dir"
scp -q "$bin" "$root/deploy/setup-$os.sh" "$root/security/run.sh" "$target:$dir/"

# Quote each setting for the remote shell.
settings=""
for kv in "$@"; do
	settings="$settings '$(printf '%s' "$kv" | sed "s/'/'\\\\''/g")'"
done
# shellcheck disable=SC2029
ssh -t "$target" "cd $dir && $su env$settings sh setup-$os.sh"
