#!/bin/sh
# Set up (or update) the site on a Linux server: systemd service behind Caddy.
#
# Run as root on the target host. Safe to re-run: it replaces the binary,
# rewrites changed config (keeping a timestamped .bak) and restarts.
#
#   DOMAIN=dev.example.com sh deploy/setup-linux.sh   # public host, automatic TLS
#   sh deploy/setup-linux.sh                          # no DOMAIN: plain HTTP on :80 (LAN only)
#
# Settings (environment):
#   DOMAIN     public hostname; Caddy fetches a Let's Encrypt cert for it.
#              Its DNS A/AAAA records must already point here, with 80/443 open.
#   WWW=1      also serve www.$DOMAIN as a redirect to $DOMAIN (default 0)
#   BIN        binary to install (default: dist/wintermute-linux-<arch>, built
#              with deploy/build.sh if missing and Go is available)
#   PORT       loopback port the Go service listens on (default 8080)
#   NOINDEX=1  send X-Robots-Tag: noindex so search engines skip a dev/staging
#              host (default 1; set 0 for production)
#   FIREWALL=1 open SSH/80/443 in ufw or firewalld and enable it (default 1)
#
# Tested against Debian 12+/Ubuntu 22.04+ (apt) and Fedora (dnf).
set -eu

DOMAIN=${DOMAIN:-}
WWW=${WWW:-0}
PORT=${PORT:-8080}
NOINDEX=${NOINDEX:-1}
FIREWALL=${FIREWALL:-1}

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

BIN=${BIN:-$repo/dist/wintermute-linux-$arch}
if [ ! -f "$BIN" ]; then
	command -v go >/dev/null || die "no binary at $BIN and Go is not installed; build elsewhere with deploy/build.sh linux $arch and pass BIN=..."
	say "building $BIN"
	# Build as the invoking user so root doesn't own their Go cache.
	if [ -n "${SUDO_USER:-}" ]; then
		su "$SUDO_USER" -c "sh '$here/build.sh' linux $arch"
	else
		sh "$here/build.sh" linux "$arch"
	fi
fi

say "installing /usr/local/bin/wintermute"
install -m 0755 "$BIN" /usr/local/bin/wintermute.new
mv -f /usr/local/bin/wintermute.new /usr/local/bin/wintermute

# --- service ----------------------------------------------------------------

# DynamicUser gives the service a throwaway unprivileged account; it needs no
# files on disk because templates and assets are embedded in the binary.
put /etc/systemd/system/wintermute.service 0644 <<EOF
[Unit]
Description=Wintermute website
After=network.target

[Service]
ExecStart=/usr/local/bin/wintermute
Environment=ADDR=127.0.0.1:$PORT
Environment=TRUST_PROXY=1
Restart=on-failure
RestartSec=2s

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

# --- reverse proxy ----------------------------------------------------------

if ! command -v caddy >/dev/null; then
	say "installing caddy"
	if command -v apt-get >/dev/null; then
		apt-get update -q
		DEBIAN_FRONTEND=noninteractive apt-get install -y -q caddy
	elif command -v dnf >/dev/null; then
		dnf install -y caddy || die "caddy not in your repos; on RHEL-likes enable EPEL or see https://caddyserver.com/docs/install"
	else
		die "no apt-get or dnf; install caddy manually (https://caddyserver.com/docs/install) and re-run"
	fi
fi

robots=""
[ "$NOINDEX" = 1 ] && robots='	header X-Robots-Tag "noindex, nofollow"'

if [ -n "$DOMAIN" ]; then
	www=""
	[ "$WWW" = 1 ] && www="
www.$DOMAIN {
	redir https://$DOMAIN{uri} permanent
}"
	put /etc/caddy/Caddyfile 0644 <<EOF
# Managed by deploy/setup-linux.sh. Local edits are backed up and replaced on re-run.
$DOMAIN {
	encode zstd gzip
	header Strict-Transport-Security "max-age=31536000"
$robots
	reverse_proxy 127.0.0.1:$PORT
}
$www
EOF
else
	put /etc/caddy/Caddyfile 0644 <<EOF
# Managed by deploy/setup-linux.sh. Local edits are backed up and replaced on re-run.
# Plain HTTP: no DOMAIN was given. Do not expose this to the internet.
:80 {
	encode zstd gzip
$robots
	reverse_proxy 127.0.0.1:$PORT
}
EOF
fi

caddy validate --adapter caddyfile --config /etc/caddy/Caddyfile
systemctl enable caddy >/dev/null 2>&1
if systemctl is-active --quiet caddy; then
	systemctl reload caddy
else
	systemctl start caddy
fi

# --- firewall ---------------------------------------------------------------

if [ "$FIREWALL" = 1 ]; then
	sshport=$(sshd -T 2>/dev/null | awk '$1 == "port" { print $2; exit }')
	sshport=${sshport:-22}
	if command -v ufw >/dev/null; then
		say "configuring ufw (ssh $sshport, 80, 443)"
		ufw allow "$sshport/tcp" >/dev/null # SSH first, so enabling can't lock us out
		ufw allow 80/tcp >/dev/null
		ufw allow 443/tcp >/dev/null
		ufw allow 443/udp >/dev/null # HTTP/3
		ufw --force enable >/dev/null
	elif command -v firewall-cmd >/dev/null && firewall-cmd --state >/dev/null 2>&1; then
		say "configuring firewalld (ssh $sshport, http, https)"
		firewall-cmd --permanent --add-port="$sshport/tcp" >/dev/null
		firewall-cmd --permanent --add-service=http --add-service=https --add-service=http3 >/dev/null 2>&1 ||
			firewall-cmd --permanent --add-service=http --add-service=https >/dev/null
		firewall-cmd --reload >/dev/null
	else
		say "WARNING: no ufw or active firewalld found; configure a firewall yourself (allow only SSH, 80, 443)"
	fi
fi

# --- check ------------------------------------------------------------------

sleep 1
if command -v curl >/dev/null && ! curl -fsS "http://127.0.0.1:$PORT/healthz" >/dev/null; then
	die "service is not answering on 127.0.0.1:$PORT; see: journalctl -u wintermute -n 50"
fi

say "done"
if [ -n "$DOMAIN" ]; then
	echo "    site:   https://$DOMAIN/  (first TLS certificate can take a minute)"
else
	echo "    site:   http://$(hostname -I 2>/dev/null | awk '{print $1}')/"
fi
echo "    logs:   journalctl -u wintermute -f     (contact submissions are logged here for now)"
echo "    update: rebuild, then re-run this script"
