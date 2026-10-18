package main

import (
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// Production is run by webmaster (~/code/webmaster), which owns the OpenBSD
// server's /etc. Some of what this site promises, above all in its privacy
// policy, is kept by that config rather than by this code. These tests read
// webmaster's checkout and fail when the two drift apart. deploy/build.sh runs
// them before every build, and webmaster's bin/push-site sets WEBMASTER_DIR, so
// a production build can't go out against a server config that breaks them.
// Without a webmaster checkout (the LAN dev box) they are skipped.

// webmasterFile reads a file from the webmaster checkout: $WEBMASTER_DIR, or
// ../webmaster next to this repo.
func webmasterFile(t *testing.T, path string) string {
	t.Helper()
	dir, set := os.LookupEnv("WEBMASTER_DIR")
	if !set {
		dir = filepath.Join("..", "webmaster")
	}
	b, err := os.ReadFile(filepath.Join(dir, path))
	if err != nil {
		if !set && os.IsNotExist(err) {
			t.Skipf("no webmaster checkout at %s: production config not checked", dir)
		}
		t.Fatalf("webmaster: %v", err)
	}
	return string(b)
}

// configLines returns a config file's lines with comments and blanks left out,
// each split into fields.
func configLines(s string) [][]string {
	var out [][]string
	for _, line := range strings.Split(s, "\n") {
		f := strings.Fields(line)
		if len(f) == 0 || strings.HasPrefix(f[0], "#") {
			continue
		}
		out = append(out, f)
	}
	return out
}

const productionAccessLog = "/var/log/wintermuteconsulting-access"

// The policy: the access log and any failed enquiry in /var/log/daemon are
// gone after 30 days at most.
func TestProductionKeepsThePrivacyPromises(t *testing.T) {
	rc := webmasterFile(t, "server/rc.d/wintermuteconsulting")
	if !regexp.MustCompile(`(?m)^daemon_flags="[^"]*-access-log syslog`).MatchString(rc) {
		t.Error("server/rc.d/wintermuteconsulting: daemon_flags has no -access-log syslog, so production keeps no access log")
	}

	// syslog.conf sends facility local0 (openAccessLog) to the access log,
	// ahead of any "!program" block that would narrow it to one program.
	routed := false
	for _, f := range configLines(webmasterFile(t, "server/syslog.conf")) {
		if strings.HasPrefix(f[0], "!") || strings.HasPrefix(f[0], "+") || strings.HasPrefix(f[0], "-") {
			break
		}
		if len(f) == 2 && f[0] == "local0.info" && f[1] == productionAccessLog {
			routed = true
		}
	}
	if !routed {
		t.Errorf("server/syslog.conf: no \"local0.info %s\" line before the program blocks", productionAccessLog)
	}
	// bin/site-logs mirrors this file, so its local copies keep the same 30 days.
	if !strings.Contains(webmasterFile(t, "config/apps/wintermuteconsulting.conf"), "ACCESS_LOG="+productionAccessLog+"\n") {
		t.Errorf("config/apps/wintermuteconsulting.conf: ACCESS_LOG is not %s", productionAccessLog)
	}

	// newsyslog.conf: rotated at midnight, 29 archives besides today's file,
	// so no line is older than 30 days.
	rotation := map[string][]string{} // log -> count, size, when
	for _, f := range configLines(webmasterFile(t, "server/newsyslog.conf")) {
		rest := f[1:]
		if len(rest) > 0 && strings.Contains(rest[0], ":") {
			rest = rest[1:] // owner:group
		}
		if len(rest) >= 4 {
			rotation[f[0]] = rest[1:4]
		}
	}
	for _, log := range []string{productionAccessLog, "/var/log/daemon"} {
		if got := strings.Join(rotation[log], " "); got != "29 * $D0" {
			t.Errorf("server/newsyslog.conf rotates %s as %q (count size when), want \"29 * $D0\": the privacy policy promises 30 days at most", log, got)
		}
	}
}

// The rest of what the site expects of its server.
func TestProductionConfigFitsTheSite(t *testing.T) {
	rc := webmasterFile(t, "server/rc.d/wintermuteconsulting")
	flags := regexp.MustCompile(`(?m)^daemon_flags="([^"]*)"`).FindStringSubmatch(rc)
	if flags == nil {
		t.Fatal("server/rc.d/wintermuteconsulting: no daemon_flags line")
	}
	// The rate limit and the access log need the visitor's address, which
	// relayd appends to X-Forwarded-For in every protocol.
	if !strings.Contains(flags[1], "-trust-proxy") {
		t.Error("rc.d: no -trust-proxy, so every visitor looks like 127.0.0.1")
	}
	relayd := webmasterFile(t, "server/relayd.conf")
	protocols := strings.Count(relayd, "\nhttp protocol ")
	appends := strings.Count(relayd, `match request header append "X-Forwarded-For" value "$REMOTE_ADDR"`)
	if protocols == 0 || appends != protocols {
		t.Errorf("relayd.conf: %d http protocols but %d append the client to X-Forwarded-For; -trust-proxy is spoofable without it", protocols, appends)
	}
	// HSTS on 443, for at least a year (security/run.sh prod checks the same
	// from outside): the protocol that holds the TLS keypair sends it.
	hsts := regexp.MustCompile(`match response header set "Strict-Transport-Security" value "max-age=(\d+)`)
	tlsProtocols := 0
	for _, block := range strings.Split(relayd, "\nhttp protocol ")[1:] {
		if !strings.Contains(block, "tls keypair") {
			continue
		}
		tlsProtocols++
		m := hsts.FindStringSubmatch(block)
		if m == nil {
			t.Error("relayd.conf: the TLS protocol sends no Strict-Transport-Security header")
		} else if age, _ := strconv.Atoi(m[1]); age < 31536000 {
			t.Errorf("relayd.conf: Strict-Transport-Security max-age=%s is under a year", m[1])
		}
	}
	if tlsProtocols == 0 {
		t.Error("relayd.conf: no http protocol with a tls keypair")
	}
	// Port 80 never serves the site: relayd sends its plain-HTTP side to httpd,
	// which answers with a 301 to the same URL on https:// (the privacy policy
	// says the site is served only over HTTPS).
	for _, block := range strings.Split(relayd, "\nhttp protocol ")[1:] {
		if !strings.Contains(block, "tls keypair") && strings.Contains(block, "forward to <wintermuteconsulting>") {
			t.Error("relayd.conf: the plain-HTTP protocol forwards to the site; it must go to httpd's redirect")
		}
	}
	for _, block := range strings.Split(relayd, "\nrelay ")[1:] {
		if !regexp.MustCompile(`listen on \S+ port \d+ tls`).MatchString(block) && strings.Contains(block, "forward to <wintermuteconsulting>") {
			t.Error("relayd.conf: a relay without tls forwards to the site")
		}
	}
	httpd := webmasterFile(t, "server/httpd.conf")
	if !strings.Contains(httpd, `block return 301 "https://$HTTP_HOST$REQUEST_URI"`) {
		t.Error(`httpd.conf: no 'block return 301 "https://$HTTP_HOST$REQUEST_URI"', so port 80 doesn't redirect to HTTPS`)
	}
	// Mail goes out once mail.conf exists (deploy/mail.conf.example).
	if !strings.Contains(rc, "-mail-config /etc/wintermuteconsulting/mail.conf") {
		t.Error("rc.d: never passes -mail-config, so enquiries would only be logged, never emailed")
	}
	// The address relayd checks and push.sh probes is the one the service listens on.
	addr := regexp.MustCompile(`-addr (\S+)`).FindStringSubmatch(flags[1])
	app := webmasterFile(t, "config/apps/wintermuteconsulting.conf")
	if addr == nil || !strings.Contains(app, "HEALTH_URL=http://"+addr[1]+"/healthz") {
		t.Errorf("rc.d listens on %v but config/apps/wintermuteconsulting.conf checks another HEALTH_URL", addr)
	}
	if addr != nil && !strings.Contains(relayd, "port "+addr[1][strings.LastIndex(addr[1], ":")+1:]+" check http \"/healthz\"") {
		t.Errorf("relayd.conf does not forward to the service's port (%s)", addr[1])
	}
	// webmaster builds with deploy/build.sh and swaps the binary with push.sh.
	for _, want := range []string{"BUILD_SCRIPT=deploy/build.sh", "BUILD_OUT=dist/wintermuteconsulting-openbsd-$GOARCH", "PUSH_SCRIPT=deploy/push.sh"} {
		if !strings.Contains(app, want) {
			t.Errorf("config/apps/wintermuteconsulting.conf: no %q; webmaster no longer uses this repo's scripts as expected", want)
		}
	}
}
