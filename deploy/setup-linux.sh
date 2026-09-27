#!/bin/sh
# Install (or update) the site on the Linux dev deploy server as a systemd
# service. That's all it does: no packages, no nginx or firewall changes. The
# box's own nginx proxies to the port set in the env file.
#
#   deploy/push.sh linux user@devbox     # from the workstation: build, copy, run this
#   sudo sh setup-linux.sh               # or by hand on the server
#
# Safe to re-run: replaces the binary, rewrites the unit if it changed (keeping
# a timestamped .bak) and restarts. The env file is created once and never
# overwritten, so your edits survive redeploys.
#
#   /etc/wintermute/wintermute.env    listen address/port and options (edit this)
#   /etc/systemd/system/wintermute.service
#   /usr/local/bin/wintermute
#
# Settings (environment):
#   BIN   prebuilt binary (default: wintermute-linux-<arch> next to this script,
#         else ../dist/). Never built on this host.
set -eu

ENV_FILE=/etc/wintermute/wintermute.env

say() { printf '==> %s\n' "$*"; }
die() {
	printf 'error: %s\n' "$*" >&2
	exit 1
}

# put DEST MODE: install stdin at DEST if it differs, backing up the old file.
put() {
	tmp=$(mktemp)
	cat >"$tmp"
	if [ -f "$1" ] && cmp -s "$tmp" "$1"; then
		rm -f "$tmp"
		return 0
	fi
	if [ -f "$1" ]; then
		cp -p "$1" "$1.bak.$(date +%Y%m%d%H%M%S)"
		say "backed up $1"
	fi
	install -D -m "$2" "$tmp" "$1"
	rm -f "$tmp"
	say "wrote $1"
}

[ "$(uname -s)" = Linux ] || die "this script is for Linux; use setup-openbsd.sh on OpenBSD"
[ "$(id -u)" -eq 0 ] || die "run as root (sudo sh $0)"
command -v systemctl >/dev/null || die "systemd is required"

case $(uname -m) in
x86_64) arch=amd64 ;;
aarch64 | arm64) arch=arm64 ;;
*) die "unsupported architecture $(uname -m)" ;;
esac

here=$(cd "$(dirname "$0")" && pwd)
repo=$(cd "$here/.." && pwd)

# --- binary -----------------------------------------------------------------

if [ -z "${BIN:-}" ]; then
	BIN=$here/wintermute-linux-$arch
	[ -f "$BIN" ] || BIN=$repo/dist/wintermute-linux-$arch
fi
[ -f "$BIN" ] || die "no binary at $BIN. Build on the workstation (deploy/build.sh linux $arch) or use deploy/push.sh"

say "installing /usr/local/bin/wintermute"
install -m 0755 "$BIN" /usr/local/bin/wintermute.new
mv -f /usr/local/bin/wintermute.new /usr/local/bin/wintermute

# --- env file (created once, then yours) ------------------------------------

if [ ! -f "$ENV_FILE" ]; then
	install -d -m 0755 "$(dirname "$ENV_FILE")"
	cat >"$ENV_FILE" <<'EOF'
# Wintermute site settings. Read by wintermute.service; apply changes with:
#   sudo systemctl restart wintermute
# Redeploys never overwrite this file.

# Where the site listens. Keep it on loopback: nginx is the public side.
# Change the port here and in nginx's proxy_pass.
ADDR=127.0.0.1:8080

# Take the client IP from the last X-Forwarded-For hop (used by the contact
# form's rate limit). Correct behind nginx ONLY if the proxy block has:
#   proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
# Without that line clients can spoof the header; then set this to 0.
TRUST_PROXY=1

# The Go runtime tries to stay under this; keep it below the unit's MemoryMax (256M).
GOMEMLIMIT=180MiB
EOF
	chmod 0644 "$ENV_FILE"
	say "wrote $ENV_FILE (edit it to change the port)"
else
	say "keeping existing $ENV_FILE"
fi

# --- service ----------------------------------------------------------------

# DynamicUser gives the service a throwaway unprivileged account; it needs no
# files on disk because templates and assets are embedded in the binary.
put /etc/systemd/system/wintermute.service 0644 <<EOF
[Unit]
Description=Wintermute website
After=network.target

[Service]
EnvironmentFile=$ENV_FILE
ExecStart=/usr/local/bin/wintermute
Restart=on-failure
RestartSec=2s
MemoryMax=256M

DynamicUser=yes
NoNewPrivileges=yes
CapabilityBoundingSet=
AmbientCapabilities=
ProtectSystem=strict
ProtectHome=yes
PrivateTmp=yes
PrivateDevices=yes
ProtectHostname=yes
ProtectClock=yes
ProtectKernelTunables=yes
ProtectKernelModules=yes
ProtectKernelLogs=yes
ProtectControlGroups=yes
ProtectProc=invisible
RestrictAddressFamilies=AF_INET AF_INET6 AF_UNIX
RestrictNamespaces=yes
RestrictRealtime=yes
RestrictSUIDSGID=yes
LockPersonality=yes
MemoryDenyWriteExecute=yes
SystemCallArchitectures=native
SystemCallFilter=@system-service
SystemCallFilter=~@privileged
UMask=0077

[Install]
WantedBy=multi-user.target
EOF

systemctl daemon-reload
systemctl enable wintermute >/dev/null 2>&1
systemctl restart wintermute

# --- check ------------------------------------------------------------------

sleep 1
systemctl is-active --quiet wintermute || die "service did not start; see: journalctl -u wintermute -n 50"
addr=$(sed -n 's/^ADDR=//p' "$ENV_FILE" | tail -1)
port=${addr##*:}
if command -v curl >/dev/null && ! curl -fsS "http://127.0.0.1:$port/healthz" >/dev/null; then
	die "service is not answering on port $port; see: journalctl -u wintermute -n 50"
fi

say "done: listening on $addr"
cat <<EOF
    settings: $ENV_FILE  (then: sudo systemctl restart wintermute)
    logs:     journalctl -u wintermute -f   (contact submissions are logged here for now)

    nginx location block for the site (both headers matter):
        location / {
            proxy_pass http://127.0.0.1:$port;
            proxy_set_header Host \$host;                                   # contact form's same-site check
            proxy_set_header X-Forwarded-For \$proxy_add_x_forwarded_for;   # real client IP for the rate limit
        }
EOF
