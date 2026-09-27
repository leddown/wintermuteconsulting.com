#!/bin/sh
# Security test runner. Each stage exits non-zero if anything FAILs.
#
#   security/run.sh code                  static analysis, vuln scan, tests (-race), fuzzing
#   security/run.sh local                 build, start on loopback, attack it with every scanner
#   security/run.sh dev http://10.0.0.5   same active attack against the (LAN) dev server
#   security/run.sh prod example.com      external posture of production: ports, TLS, headers,
#                                         passive web scan. ACTIVE=1 adds the attack scans.
#   security/run.sh host                  run ON a server: audit OS, services, firewall, keys
#   security/run.sh all                   code + local
#
# Settings (environment):
#   FUZZTIME=2m    per fuzz target (use 30m+ before a release; crashers go to testdata/fuzz/)
#   ACTIVE=1       prod only: also run ZAP full/active, nuclei and nikto against production.
#                  These submit the contact form and probe for exploits; check your VPS
#                  provider's policy on scanning first.
#   STRICT=1       a missing scanner counts as FAIL instead of SKIP
#   SSHPORT=22     prod: the SSH port expected to be open
#   DOMAIN         host stage: the site's domain (default: guessed from acme-client.conf)
#
# External scanners are used when installed (otherwise SKIPped, with a hint):
#   docker|podman (OWASP ZAP), nuclei, nikto, nmap, testssl.sh, curl, openssl
# Reports are written to security/reports/<timestamp>/.
set -eu

here=$(cd "$(dirname "$0")" && pwd)
repo=$(cd "$here/.." && pwd)
FUZZTIME=${FUZZTIME:-2m}
ACTIVE=${ACTIVE:-0}
STRICT=${STRICT:-0}

# Pinned tool versions: bump deliberately, not implicitly via @latest.
GOVULNCHECK=golang.org/x/vuln/cmd/govulncheck@v1.8.0
STATICCHECK=honnef.co/go/tools/cmd/staticcheck@v0.8.1
GOSEC=github.com/securego/gosec/v2/cmd/gosec@v2.29.0
ZAP_IMAGE=ghcr.io/zaproxy/zaproxy:stable

pass=0 fail=0 warn=0 skip=0
ok() {
	pass=$((pass + 1))
	printf '  \033[32mPASS\033[0m %s\n' "$*"
}
bad() {
	fail=$((fail + 1))
	printf '  \033[31mFAIL\033[0m %s\n' "$*"
}
meh() {
	warn=$((warn + 1))
	printf '  \033[33mWARN\033[0m %s\n' "$*"
}
nope() {
	if [ "$STRICT" = 1 ]; then
		bad "$* (STRICT=1)"
	else
		skip=$((skip + 1))
		printf '  \033[2mSKIP\033[0m %s\n' "$*"
	fi
}
section() { printf '\n\033[1m== %s\033[0m\n' "$*"; }
have() { command -v "$1" >/dev/null 2>&1; }
# check DESC CMD...: PASS if the command succeeds, FAIL (with its output) if not.
check() {
	desc=$1
	shift
	if outp=$("$@" 2>&1); then
		ok "$desc"
	else
		bad "$desc"
		printf '%s\n' "$outp" | tail -30 | sed 's/^/       /'
	fi
}

reports() {
	[ -n "${out:-}" ] && return
	out=$here/reports/$(date +%Y%m%d-%H%M%S)
	mkdir -p "$out"
	chmod 0777 "$out" # the ZAP container writes as its own uid
}

finish() {
	printf '\n%d passed, %d failed, %d warnings, %d skipped\n' "$pass" "$fail" "$warn" "$skip"
	[ -n "${out:-}" ] && printf 'reports: %s\n' "$out"
	[ "$fail" -eq 0 ]
}

# --- code -------------------------------------------------------------------

stage_code() {
	cd "$repo"
	section "Supply chain"
	if [ "$(go list -m all | wc -l)" -eq 1 ]; then ok "no third-party Go modules"; else bad "go.mod pulls in modules: $(go list -m all | tail -n +2 | tr '\n' ' ')"; fi
	if git grep -qIE 'BEGIN ([A-Z]+ )?PRIVATE KEY|AKIA[0-9A-Z]{16}|xox[baprs]-[0-9A-Za-z-]+' -- . ':!security/run.sh' 2>/dev/null; then
		bad "possible secret committed: $(git grep -lIE 'BEGIN ([A-Z]+ )?PRIVATE KEY|AKIA[0-9A-Z]{16}' -- . ':!security/run.sh' | tr '\n' ' ')"
	else
		ok "no private keys or cloud tokens in tracked files"
	fi

	section "Static analysis"
	# shellcheck disable=SC2016 # expanded by the inner shell
	check "gofmt" sh -c 'test -z "$(gofmt -l .)" || { gofmt -l .; exit 1; }'
	check "go vet" go vet ./...
	check "staticcheck (all checks)" go run "$STATICCHECK" -checks all ./...
	check "gosec" go run "$GOSEC" -quiet ./...

	section "Known vulnerabilities (govulncheck)"
	check "source: reachable vulns in stdlib/deps" go run "$GOVULNCHECK" ./...
	# The binary check matters most: it sees the Go version that actually built it.
	for os in linux openbsd; do
		if sh "$repo/deploy/build.sh" "$os" >/dev/null 2>&1; then
			check "binary $os/amd64 ($(go version "$repo/dist/wintermute-$os-amd64" | awk '{print $2}'))" \
				go run "$GOVULNCHECK" -mode=binary "$repo/dist/wintermute-$os-amd64"
		else
			bad "build $os"
		fi
	done

	section "Tests (race detector, incl. slowloris and header-size checks)"
	check "go test -race" go test -race -count=1 ./...

	section "Fuzzing ($FUZZTIME per target)"
	for target in $(go test -list '^Fuzz' . | grep '^Fuzz'); do
		check "$target" go test -run '^$' -fuzz "^$target\$" -fuzztime "$FUZZTIME" .
	done
}

# --- live probes (curl) -----------------------------------------------------

code_of() { curl -s -o /dev/null -w '%{http_code}' --path-as-is --max-time 15 "$@"; }
headers_of() { curl -s -o /dev/null -D - --max-time 15 "$@" | tr -d '\r'; }

# probe BASE_URL: black-box checks that must hold at the edge, whatever sits in front.
probe() {
	base=$1
	section "Live probes: $base"
	have curl || {
		nope "curl not installed"
		return
	}
	h=$(headers_of "$base/")
	if printf '%s' "$h" | head -1 | grep -q ' 200'; then ok "GET / is 200"; else bad "GET / : $(printf '%s' "$h" | head -1)"; fi
	for want in "content-security-policy: default-src 'self'" "x-content-type-options: nosniff" "referrer-policy:" "permissions-policy:" "cross-origin-opener-policy: same-origin"; do
		if printf '%s' "$h" | grep -qi "^$want"; then ok "header $want"; else bad "missing header $want"; fi
	done
	if printf '%s' "$h" | grep -qi '^set-cookie:'; then bad "sets a cookie (breaks the no-consent-banner position)"; else ok "no cookies"; fi
	if printf '%s' "$h" | grep -qiE '^(server|x-powered-by):'; then meh "discloses software: $(printf '%s' "$h" | grep -iE '^(server|x-powered-by):' | tr '\n' ' ')"; else ok "no Server/X-Powered-By"; fi
	case $base in https://*)
		hsts=$(printf '%s' "$h" | grep -i '^strict-transport-security:' | grep -oE 'max-age=[0-9]+' | cut -d= -f2)
		if [ "${hsts:-0}" -ge 31536000 ]; then ok "HSTS max-age >= 1 year"; else bad "HSTS missing or short (${hsts:-none})"; fi
		;;
	esac

	for p in /.git/config /.env /go.mod /main.go /templates/partials.html /static/../go.mod /static/..%2fgo.mod \
		/static/%2e%2e/%2e%2e/etc/passwd /static/ /static/css/ /static/images/cocoparisienne-forest-1258845.jpg \
		/server-status /admin /wp-login.php /debug/pprof/ /static/%84; do
		c=$(code_of "$base$p")
		case $c in 2??) bad "GET $p = $c" ;; 5??) bad "GET $p = $c (server error)" ;; *) ok "GET $p = $c" ;; esac
	done
	for m in TRACE PUT DELETE PATCH OPTIONS; do
		c=$(code_of -X "$m" "$base/")
		case $c in 2??) bad "$m / = $c" ;; *) ok "$m / = $c" ;; esac
	done
	c=$(code_of -X POST -H 'Origin: https://evil.example' --data 'name=a&email=a@b.co&message=0123456789' "$base/contact")
	[ "$c" = 403 ] && ok "cross-site POST /contact = 403" || bad "cross-site POST /contact = $c, want 403"
	if headers_of -H 'Origin: https://evil.example' "$base/" | grep -qi '^access-control-allow-origin:'; then bad "CORS header returned to a foreign origin"; else ok "no CORS for foreign origins"; fi
	loc=$(headers_of -X POST -H 'Host: evil.example' --data 'website=x' "$base/contact" | grep -i '^location:' || true)
	case $loc in *evil.example*) bad "Host header reflected into redirect: $loc" ;; *) ok "Host header not reflected into redirects" ;; esac
	c=$(code_of -H "X-Big: $(head -c 70000 /dev/zero | tr '\0' a)" "$base/")
	case $c in 431 | 400 | 413 | 000) ok "70 KB header rejected ($c)" ;; *) bad "70 KB header accepted ($c)" ;; esac
	c=$(head -c 2000000 /dev/zero | tr '\0' a | sed 's/^/message=/' | code_of -X POST --data-binary @- "$base/contact")
	case $c in 400 | 413 | 403) ok "2 MB POST rejected ($c)" ;; *) bad "2 MB POST = $c" ;; esac
}

# tls HOST: certificate and protocol checks with openssl, full audit with testssl.sh.
tls() {
	host=$1
	section "TLS: $host"
	if have openssl; then
		chain=$(echo | openssl s_client -connect "$host:443" -servername "$host" -verify_return_error 2>&1 || true)
		if printf '%s' "$chain" | grep -q 'Verify return code: 0'; then ok "certificate chain verifies"; else bad "certificate chain does not verify"; fi
		if echo | openssl s_client -connect "$host:443" -servername "$host" 2>/dev/null | openssl x509 -noout -checkend 1209600 >/dev/null 2>&1; then
			ok "certificate valid for 14+ days"
		else
			bad "certificate expires within 14 days (renewal broken?)"
		fi
		for v in tls1_2 tls1_3; do
			if echo | openssl s_client -connect "$host:443" -servername "$host" "-$v" >/dev/null 2>&1; then ok "$v offered"; else meh "$v not offered"; fi
		done
	else
		nope "openssl not installed"
	fi
	if have testssl.sh || have testssl; then
		reports
		t=$(command -v testssl.sh || command -v testssl)
		"$t" --quiet --color 0 --warnings off --jsonfile "$out/testssl.json" --htmlfile "$out/testssl.html" "https://$host" >/dev/null 2>&1 || true
		n=$(grep -cE '"severity" *: *"(HIGH|CRITICAL)"' "$out/testssl.json" 2>/dev/null || true)
		m=$(grep -cE '"severity" *: *"MEDIUM"' "$out/testssl.json" 2>/dev/null || true)
		if [ "${n:-0}" -gt 0 ]; then bad "testssl.sh: $n HIGH/CRITICAL findings (testssl.html)"; else ok "testssl.sh: no HIGH/CRITICAL"; fi
		[ "${m:-0}" -gt 0 ] && meh "testssl.sh: $m MEDIUM findings (testssl.html)"
	else
		nope "testssl.sh not installed (https://testssl.sh, or: apt install testssl.sh)"
	fi
}

# ports HOST EXPECTED...: only the expected TCP ports may answer.
ports() {
	host=$1
	shift
	section "Open ports: $host"
	have nmap || {
		nope "nmap not installed"
		return
	}
	reports
	scan=-sT
	[ "$(id -u)" -eq 0 ] && scan=-sS
	open=$(nmap -Pn "$scan" -p- -T4 --open -oG - "$host" 2>/dev/null | grep -oE '[0-9]+/open/tcp' | cut -d/ -f1 | sort -n | tr '\n' ' ')
	printf '%s\n' "$open" >"$out/nmap-tcp.txt"
	extra=""
	for p in $open; do
		case " $* " in *" $p "*) ;; *) extra="$extra $p" ;; esac
	done
	if [ -z "$extra" ]; then ok "TCP open: ${open:-none} (expected: $*)"; else bad "unexpected open TCP ports:$extra"; fi
	if [ "$(id -u)" -eq 0 ]; then
		udp=$(nmap -Pn -sU --top-ports 100 --open -oG - "$host" 2>/dev/null | grep -oE '[0-9]+/open/udp' | cut -d/ -f1 | tr '\n' ' ')
		if [ -z "$udp" ]; then ok "no open UDP in top 100"; else meh "UDP open (check these are intended): $udp"; fi
	else
		nope "UDP scan needs root"
	fi
}

# scanners URL MODE: MODE is "active" (attacks) or "passive" (spider + passive rules only).
scanners() {
	url=$1
	mode=$2
	section "Web scanners ($mode): $url"
	reports
	rt=""
	have docker && rt=docker
	[ -z "$rt" ] && have podman && rt=podman
	if [ -n "$rt" ]; then
		if [ "$mode" = active ]; then
			script=zap-full-scan.py
			name=zap-full
		else
			script=zap-baseline.py
			name=zap-baseline
		fi
		printf '       running OWASP ZAP %s (this can take a long time)...\n' "$script"
		rc=0
		cp "$here/zap-rules.tsv" "$out/"
		"$rt" run --rm --network host -v "$out:/zap/wrk:rw" "$ZAP_IMAGE" \
			"$script" -t "$url" -c zap-rules.tsv -r "$name.html" -J "$name.json" -m 5 >"$out/$name.log" 2>&1 || rc=$?
		case $rc in
		0) ok "ZAP $name: no alerts above threshold" ;;
		2) meh "ZAP $name: warnings ($name.html)" ;;
		1) bad "ZAP $name: failures ($name.html)" ;;
		*) bad "ZAP $name did not complete (rc $rc, $name.log)" ;;
		esac
	else
		nope "OWASP ZAP needs docker or podman"
	fi

	[ "$mode" = active ] || return 0
	if have nuclei; then
		nuclei -u "$url" -severity low,medium,high,critical -silent -o "$out/nuclei.txt" >/dev/null 2>&1 || true
		if grep -qE '\[(medium|high|critical)\]' "$out/nuclei.txt" 2>/dev/null; then
			bad "nuclei: medium+ findings (nuclei.txt)"
		elif [ -s "$out/nuclei.txt" ]; then
			meh "nuclei: low findings (nuclei.txt)"
		else
			ok "nuclei: nothing low or above"
		fi
	else
		nope "nuclei not installed (https://github.com/projectdiscovery/nuclei)"
	fi
	if have nikto; then
		nikto -h "$url" -nointeractive -Format txt -output "$out/nikto.txt" >/dev/null 2>&1 || true
		# Drop banner lines and reviewed false positives (security/nikto-ignore.txt).
		grep '^+ ' "$out/nikto.txt" 2>/dev/null | grep -v '^+ Target ' | grep -vEf "$here/nikto-ignore.txt" >"$out/nikto-findings.txt" || true
		n=$(wc -l <"$out/nikto-findings.txt" | tr -d ' ')
		if [ "$n" -gt 0 ]; then meh "nikto: $n items to review (nikto-findings.txt)"; else ok "nikto: nothing beyond reviewed false positives"; fi
	else
		nope "nikto not installed"
	fi
}

# --- stages -------------------------------------------------------------------

stage_local() {
	cd "$repo"
	port=${PORT:-18089}
	sh deploy/build.sh linux "$(go env GOARCH)" >/dev/null
	./dist/wintermute-linux-"$(go env GOARCH)" -addr "127.0.0.1:$port" 2>"$here/.local.log" &
	pid=$!
	trap 'kill $pid 2>/dev/null || true' EXIT INT TERM
	i=0
	until curl -fs "http://127.0.0.1:$port/healthz" >/dev/null 2>&1; do
		i=$((i + 1))
		[ $i -gt 50 ] && {
			bad "server did not start"
			return
		}
		sleep 0.1
	done
	probe "http://127.0.0.1:$port"
	scanners "http://127.0.0.1:$port" active
	if grep -q 'level=ERROR' "$here/.local.log"; then bad "server logged errors during the scan (see security/.local.log)"; else ok "no server errors logged during the scan"; fi
}

stage_dev() {
	url=${1:?usage: run.sh dev http://host[:port]}
	url=${url%/}
	probe "$url"
	scanners "$url" active
}

stage_prod() {
	domain=${1:?usage: run.sh prod example.com}
	sshport=${SSHPORT:-22}
	ports "$domain" "$sshport" 80 443
	tls "$domain"
	probe "https://$domain"

	section "HTTP -> HTTPS"
	loc=$(headers_of "http://$domain/v/varg?x=1" | grep -i '^location:' | awk '{print $2}')
	[ "$loc" = "https://$domain/v/varg?x=1" ] && ok "port 80 redirects to HTTPS, keeping the path" || bad "port 80 redirect: '${loc:-none}'"
	c=$(code_of "http://$domain/.well-known/acme-challenge/../../etc/passwd")
	case $c in 2??) bad "ACME challenge path traversal = $c" ;; *) ok "ACME challenge path traversal = $c" ;; esac

	if [ "$ACTIVE" = 1 ]; then
		scanners "https://$domain" active
	else
		scanners "https://$domain" passive
		printf '       (ACTIVE=1 adds attack scans against production)\n'
	fi
}

stage_host() {
	case $(uname -s) in
	OpenBSD) host_openbsd ;;
	Linux) host_linux ;;
	*) bad "unsupported OS $(uname -s)" ;;
	esac
}

host_common() {
	section "SSH"
	if have sshd; then
		cfg=$(sshd -T 2>/dev/null || true)
		for kv in "permitrootlogin no" "passwordauthentication no" "kbdinteractiveauthentication no"; do
			if printf '%s\n' "$cfg" | grep -qi "^$kv\$"; then ok "sshd: $kv"; else meh "sshd: want '$kv' (have '$(printf '%s\n' "$cfg" | grep -i "^${kv% *} " | head -1)')"; fi
		done
	fi
}

host_openbsd() {
	[ "$(id -u)" -eq 0 ] || {
		bad "run as root (doas)"
		return
	}
	domain=${DOMAIN:-$(awk '/^domain /{gsub(/"/,"",$2); print $2; exit}' /etc/acme-client.conf 2>/dev/null)}

	section "Patches"
	p=$(syspatch -c 2>/dev/null || true)
	if [ -z "$p" ]; then ok "syspatch: up to date"; else bad "syspatch: pending: $(printf "%s " $p)"; fi

	section "Services"
	for s in wintermute httpd relayd; do check "rcctl check $s" rcctl check $s; done
	u=$(ps -axo user,command | awk '$2 == "/usr/local/bin/wintermute" {print $1; exit}')
	[ "$u" = _wintermute ] && ok "site runs as _wintermute" || bad "site runs as '${u:-not running}'"
	check "httpd.conf syntax" httpd -n
	check "relayd.conf syntax" relayd -n
	check "acme-client.conf syntax" acme-client -n
	grep -q 'session timeout' /etc/relayd.conf && ok "relayd session timeout set" || meh "relayd uses the 600 s default session timeout"

	section "Firewall"
	pfctl -si 2>/dev/null | grep -q 'Status: Enabled' && ok "pf enabled" || bad "pf disabled"
	check "pf.conf syntax" pfctl -nf /etc/pf.conf
	pfctl -sr 2>/dev/null | grep -q '^block drop in all\|^block in all\|^block drop in' && ok "pf default-denies inbound" || bad "pf has no inbound default block"

	section "Listening sockets"
	l=$(netstat -an | awk '$NF == "LISTEN" {print $4}' | sort -u)
	for a in $l; do
		p=${a##*.}
		h=${a%.*}
		case "$h:$p" in
		127.0.0.1:* | ::1:*) ok "loopback only: $a" ;;
		*:22 | *:80 | *:443) ok "public: $a" ;;
		*) bad "unexpected public listener: $a" ;;
		esac
	done

	section "Keys and certificates"
	if [ -n "$domain" ]; then
		k=/etc/ssl/private/$domain.key
		m=$(stat -f '%Lp %Su' "$k" 2>/dev/null || true)
		[ "$m" = "600 root" ] || [ "$m" = "400 root" ] && ok "$k is $m" || bad "$k permissions: '${m:-missing}'"
		openssl x509 -noout -checkend 1209600 -in "/etc/ssl/$domain.crt" >/dev/null 2>&1 && ok "certificate valid 14+ days" || bad "certificate expires within 14 days"
		crontab -l 2>/dev/null | grep -q "acme-client $domain" && ok "renewal cron present" || bad "no acme-client renewal in root's crontab"
	else
		bad "cannot determine DOMAIN (set DOMAIN=...)"
	fi
	m=$(stat -f '%Lp' /etc/relayd.conf 2>/dev/null || true)
	[ "$m" = 600 ] && ok "/etc/relayd.conf is 600" || meh "/etc/relayd.conf is $m"
	host_common
}

host_linux() {
	section "Service sandbox"
	if have systemd-analyze; then
		score=$(systemd-analyze security wintermute 2>/dev/null | awk '/Overall exposure level/ {print $(NF-1)}')
		case $score in
		'') bad "wintermute.service not installed" ;;
		0.* | 1.* | 2.*) ok "systemd exposure score $score" ;;
		*) meh "systemd exposure score $score (want < 3)" ;;
		esac
	fi
	systemctl is-active --quiet wintermute && ok "wintermute active" || bad "wintermute not running"

	section "Listening sockets"
	if have ss; then
		for a in $(ss -Htln | awk '{print $4}' | sort -u); do
			case $a in
			127.0.0.* | \[::1\]:*) ok "loopback only: $a" ;;
			*:22 | *:80 | *:443) ok "public: $a" ;;
			*) meh "other listener (fine on a LAN dev box if intended): $a" ;;
			esac
		done
	fi

	section "Updates"
	if have apt; then
		n=$(apt list --upgradable 2>/dev/null | grep -c security || true)
		[ "${n:-0}" -eq 0 ] && ok "no pending security updates" || bad "$n pending security updates"
		dpkg -s unattended-upgrades >/dev/null 2>&1 && ok "unattended-upgrades installed" || meh "unattended-upgrades not installed"
	fi
	host_common
}

cmd=${1:-}
[ $# -gt 0 ] && shift
case $cmd in
code) stage_code ;;
local) stage_local ;;
dev) stage_dev "$@" ;;
prod) stage_prod "$@" ;;
host) stage_host ;;
all)
	stage_code
	stage_local
	;;
*)
	sed -n '2,24p' "$0" | sed 's/^# \{0,1\}//'
	exit 2
	;;
esac
finish
