#!/bin/sh
# Set up (or update) the site on an OpenBSD server using only the base system:
#
#   pf            firewall: SSH, 80 and 443 in, everything else dropped
#   httpd   :80   Let's Encrypt HTTP-01 challenges, redirect everything else to HTTPS
#   relayd  :443  TLS termination + HSTS, proxies to the Go service on loopback
#   acme-client   certificate issuance, renewed daily from root's crontab
#   rc.d          runs the binary as the unprivileged _wintermuteconsulting user
#
# Run as root on the target host. Safe to re-run: it replaces the binary,
# rewrites changed config (keeping a timestamped .bak) and restarts.
#
#   DOMAIN=wintermuteconsulting.com WWW=1 sh setup-openbsd.sh
#
# Build the binary on your dev machine and copy it over alongside this script:
#   deploy/build.sh openbsd && scp dist/wintermuteconsulting-openbsd-amd64 deploy/setup-openbsd.sh host:
#
# Settings (environment):
#   DOMAIN          public hostname (required). DNS must already point here.
#   WWW=1           also get a certificate for www.$DOMAIN (default 0)
#   BIN             binary to install (default: wintermuteconsulting-openbsd-<arch> next
#                   to this script, else ../dist/)
#   PORT            loopback port for the Go service (default 8080)
#   ACME_STAGING=1  use Let's Encrypt's staging CA while testing (default 0;
#                   browsers won't trust it, but it avoids rate limits)
#   PF=0            leave /etc/pf.conf alone (default 1)
#   LISTEN4/LISTEN6 addresses for relayd to listen on (default: egress addresses)
set -eu

DOMAIN=${DOMAIN:-}
WWW=${WWW:-0}
PORT=${PORT:-8080}
ACME_STAGING=${ACME_STAGING:-0}
PF=${PF:-1}

say() { printf '==> %s\n' "$*"; }
die() {
	printf 'error: %s\n' "$*" >&2
	exit 1
}

# put DEST MODE: install stdin at DEST if it differs, backing up the old file.
# Sets changed=1 when it wrote something.
put() {
	tmp=$(mktemp)
	cat >"$tmp"
	changed=0
	if [ -f "$1" ] && cmp -s "$tmp" "$1"; then
		rm -f "$tmp"
		return 0
	fi
	if [ -f "$1" ]; then
		cp -p "$1" "$1.bak.$(date +%Y%m%d%H%M%S)"
		say "backed up $1"
	fi
	install -m "$2" "$tmp" "$1"
	rm -f "$tmp"
	changed=1
	say "wrote $1"
}

[ "$(uname -s)" = OpenBSD ] || die "this script is for OpenBSD; use setup-linux.sh on Linux"
[ "$(id -u)" -eq 0 ] || die "run as root (doas sh $0)"
[ -n "$DOMAIN" ] || die "set DOMAIN, e.g. DOMAIN=example.com sh $0"

arch=$(uname -m)
here=$(cd "$(dirname "$0")" && pwd)

# --- binary -----------------------------------------------------------------

if [ -z "${BIN:-}" ]; then
	BIN=$here/wintermuteconsulting-openbsd-$arch
	[ -f "$BIN" ] || BIN=$here/../dist/wintermuteconsulting-openbsd-$arch
fi
[ -f "$BIN" ] || die "no binary at $BIN; run 'deploy/build.sh openbsd $arch' on your dev machine and copy it here"

id _wintermuteconsulting >/dev/null 2>&1 ||
	useradd -L daemon -g =uid -c "Wintermute Consulting website" -d /var/empty -s /sbin/nologin _wintermuteconsulting

say "installing /usr/local/bin/wintermuteconsulting"
install -o root -g bin -m 0555 "$BIN" /usr/local/bin/wintermuteconsulting.new
mv -f /usr/local/bin/wintermuteconsulting.new /usr/local/bin/wintermuteconsulting

# --- service ----------------------------------------------------------------

# Output (including contact submissions, for now) goes to syslog: /var/log/daemon.
put /etc/rc.d/wintermuteconsulting 0555 <<EOF
#!/bin/ksh

daemon="/usr/local/bin/wintermuteconsulting"
daemon_flags="-addr 127.0.0.1:$PORT -trust-proxy"
daemon_user="_wintermuteconsulting"
daemon_logger="daemon.info"

. /etc/rc.d/rc.subr

rc_bg=YES
rc_reload=NO

rc_cmd \$1
EOF

rcctl enable wintermuteconsulting
rcctl restart wintermuteconsulting

# --- httpd: ACME challenges and HTTP -> HTTPS -------------------------------

names=$DOMAIN
alias=""
altnames=""
if [ "$WWW" = 1 ]; then
	names="$DOMAIN www.$DOMAIN"
	alias="	alias \"www.$DOMAIN\""
	altnames="	alternative names { \"www.$DOMAIN\" }"
fi

put /etc/httpd.conf 0644 <<EOF
# Managed by setup-openbsd.sh. Local edits are backed up and replaced on re-run.
server "$DOMAIN" {
	listen on * port 80
$alias
	location "/.well-known/acme-challenge/*" {
		root "/acme"
		request strip 2
	}
	location * {
		block return 301 "https://$DOMAIN\$REQUEST_URI"
	}
}
EOF
httpd -n >/dev/null || die "httpd.conf failed its syntax check"
rcctl enable httpd
if ! rcctl check httpd >/dev/null; then
	rcctl start httpd
elif [ "$changed" = 1 ]; then
	rcctl reload httpd
fi

# --- acme-client: certificate -----------------------------------------------

if [ "$ACME_STAGING" = 1 ]; then
	authority=letsencrypt-staging
else
	authority=letsencrypt
fi

put /etc/acme-client.conf 0644 <<EOF
# Managed by setup-openbsd.sh. Local edits are backed up and replaced on re-run.
authority letsencrypt {
	api url "https://acme-v02.api.letsencrypt.org/directory"
	account key "/etc/acme/letsencrypt-privkey.pem"
}

authority letsencrypt-staging {
	api url "https://acme-staging-v02.api.letsencrypt.org/directory"
	account key "/etc/acme/letsencrypt-staging-privkey.pem"
}

domain "$DOMAIN" {
$altnames
	domain key "/etc/ssl/private/$DOMAIN.key"
	domain full chain certificate "/etc/ssl/$DOMAIN.crt"
	sign with $authority
}
EOF
acme-client -n >/dev/null || die "acme-client.conf failed its syntax check"

# A changed config (new names, staging -> production) needs a fresh cert.
acme_flags=""
[ "$changed" = 1 ] && acme_flags=-F
if [ ! -f "/etc/ssl/$DOMAIN.crt" ] || [ -n "$acme_flags" ]; then
	say "requesting certificate for $names"
	acme-client -v $acme_flags "$DOMAIN" ||
		die "certificate request failed. Check that DNS for $names points at this host and port 80 is reachable, then re-run"
fi

# --- relayd: TLS on 443 -----------------------------------------------------

LISTEN4=${LISTEN4:-$(ifconfig egress | awk '$1 == "inet" { print $2; exit }')}
LISTEN6=${LISTEN6:-$(ifconfig egress | awk '$1 == "inet6" && $2 !~ /^fe80/ { print $2; exit }')}
[ -n "$LISTEN4$LISTEN6" ] || die "no address found on the egress interface; set LISTEN4 and/or LISTEN6"

relays=""
for addr in $LISTEN4 $LISTEN6; do
	relays="$relays
relay \"https-$addr\" {
	listen on $addr port 443 tls
	protocol \"wintermuteconsulting\"
	session timeout 60
	forward to <wintermuteconsulting> port $PORT check http \"/healthz\" code 200
}
"
done

# The Go service takes the last X-Forwarded-For hop as the client IP, which
# "append" makes the address relayd actually saw.
put /etc/relayd.conf 0600 <<EOF
# Managed by setup-openbsd.sh. Local edits are backed up and replaced on re-run.
table <wintermuteconsulting> { 127.0.0.1 }

http protocol "wintermuteconsulting" {
	tls keypair "$DOMAIN"
	tls { no tlsv1.0, no tlsv1.1 }

	match request header append "X-Forwarded-For" value "\$REMOTE_ADDR"
	match request header set "X-Forwarded-Proto" value "https"
	match response header set "Strict-Transport-Security" value "max-age=31536000"

	tcp { nodelay, sack }
}
$relays
EOF
relayd -n >/dev/null || die "relayd.conf failed its syntax check"
rcctl enable relayd
if rcctl check relayd >/dev/null; then
	rcctl reload relayd
else
	rcctl start relayd
fi

# --- renewal ----------------------------------------------------------------

renew="acme-client $DOMAIN && rcctl reload relayd"
if ! crontab -l 2>/dev/null | grep -qF "$renew"; then
	say "adding daily certificate renewal to root's crontab"
	{
		crontab -l 2>/dev/null || true
		printf '~\t~\t*\t*\t*\t%s\n' "$renew"
	} | crontab -
fi

# --- pf ---------------------------------------------------------------------

if [ "$PF" = 1 ]; then
	sshport=$(sshd -T 2>/dev/null | awk '$1 == "port" { print $2; exit }')
	sshport=${sshport:-22}
	put /etc/pf.conf 0600 <<EOF
# Managed by setup-openbsd.sh. Local edits are backed up and replaced on re-run.
set skip on lo

block return
block in
pass out

table <bruteforce> persist
block quick from <bruteforce>

pass in on egress proto tcp to port $sshport \\
	keep state (max-src-conn 15, max-src-conn-rate 5/3, overload <bruteforce> flush global)
pass in on egress proto tcp to port { 80 443 }
pass in on egress inet proto icmp icmp-type { echoreq unreach timex }
pass in on egress inet6 proto icmp6

# By default, do not permit remote connections to X11
block return in on ! lo0 proto tcp to port 6000:6010
# Port build user does not need network
block return out log proto {tcp udp} user _pbuild
EOF
	# Existing SSH sessions keep their state across the reload.
	pfctl -nf /etc/pf.conf || die "pf.conf failed its syntax check; the old ruleset is still loaded"
	pfctl -f /etc/pf.conf
fi

# --- check ------------------------------------------------------------------

sleep 1
ftp -Vo /dev/null "http://127.0.0.1:$PORT/healthz" >/dev/null 2>&1 ||
	die "service is not answering on 127.0.0.1:$PORT; see /var/log/daemon"

say "done"
echo "    site:   https://$DOMAIN/"
echo "    logs:   tail -f /var/log/daemon     (contact submissions are logged here for now)"
echo "    update: copy the new binary here and re-run this script"
echo "    keep the OS patched: syspatch (and sysupgrade for each new release)"
