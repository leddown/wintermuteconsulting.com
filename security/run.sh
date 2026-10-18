#!/bin/sh
# Security test runner for the code, on the workstation. Each stage exits
# non-zero if anything FAILs. Scans of the servers (production, the LAN dev
# box, the hosts themselves) are webmaster's: test/security there.
#
#   security/run.sh code     static analysis, vuln scan, tests (-race), fuzzing
#   security/run.sh local    build, start on loopback, attack it with every scanner
#   security/run.sh all      code + local
#
# Settings (environment):
#   FUZZTIME=2m    per fuzz target (use 30m+ before a release; crashers go to testdata/fuzz/)
#   STRICT=1       a missing scanner counts as FAIL instead of SKIP
#
# External scanners are used when installed (otherwise SKIPped, with a hint):
#   docker|podman (OWASP ZAP), nuclei, nikto, curl
# Reports are written to security/reports/<timestamp>/.
set -eu

here=$(cd "$(dirname "$0")" && pwd)
repo=$(cd "$here/.." && pwd)
FUZZTIME=${FUZZTIME:-2m}
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
		if sh "$repo/build.sh" "$os" >/dev/null 2>&1; then
			check "binary $os/amd64 ($(go version "$repo/dist/wintermuteconsulting-$os-amd64" | awk '{print $2}'))" \
				go run "$GOVULNCHECK" -mode=binary "$repo/dist/wintermuteconsulting-$os-amd64"
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

	for p in /.git/config /.env /go.mod /main.go /content.json /templates/partials.html /static/../go.mod /static/..%2fgo.mod \
		/static/%2e%2e/%2e%2e/etc/passwd /static/ /static/css/ /static/images/brickbard-winter-7635363.jpg \
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
	sh build.sh linux "$(go env GOARCH)" >/dev/null
	./dist/wintermuteconsulting-linux-"$(go env GOARCH)" -addr "127.0.0.1:$port" 2>"$here/.local.log" &
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

cmd=${1:-}
[ $# -gt 0 ] && shift
case $cmd in
code) stage_code ;;
local) stage_local ;;
all)
	stage_code
	stage_local
	;;
*)
	sed -n '2,16p' "$0" | sed 's/^# \{0,1\}//'
	exit 2
	;;
esac
finish
