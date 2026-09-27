package main

import (
	"bufio"
	"bytes"
	"io"
	"io/fs"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"
)

// Security regression tests. They pin down the properties the site's threat
// model depends on: strict headers everywhere, no inline or third-party code,
// escaped user input, no files outside static/, and bounded resource use.

func do(h http.Handler, method, target string, body io.Reader, hdr map[string]string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, target, body)
	for k, v := range hdr {
		req.Header.Set(k, v)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func TestSecurityHeadersOnEveryResponse(t *testing.T) {
	h := testApp(t).routes()
	form := "application/x-www-form-urlencoded"
	bad := validForm()
	bad.Set("email", "x")
	cases := []struct {
		name   string
		method string
		path   string
		body   string
		hdr    map[string]string
		code   int
	}{
		{"index", "GET", "/", "", nil, 200},
		{"variant", "GET", "/v/varg", "", nil, 200},
		{"static", "GET", "/static/css/base.css", "", nil, 200},
		{"health", "GET", "/healthz", "", nil, 200},
		{"not found", "GET", "/nope", "", nil, 404},
		{"bad method", "PUT", "/", "", nil, 405},
		{"contact ok", "POST", "/contact", validForm().Encode(), map[string]string{"Content-Type": form}, 303},
		{"contact invalid", "POST", "/contact", bad.Encode(), map[string]string{"Content-Type": form}, 422},
		{"contact cross-site", "POST", "/contact", validForm().Encode(), map[string]string{"Content-Type": form, "Origin": "https://evil.example"}, 403},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			rec := do(h, c.method, c.path, strings.NewReader(c.body), c.hdr)
			if rec.Code != c.code {
				t.Fatalf("code = %d, want %d", rec.Code, c.code)
			}
			hd := rec.Header()
			csp := hd.Get("Content-Security-Policy")
			for _, want := range []string{"default-src 'self'", "script-src 'self'", "style-src 'self'", "frame-ancestors 'none'", "base-uri 'none'", "object-src 'none'", "form-action 'self'"} {
				if !strings.Contains(csp, want) {
					t.Errorf("CSP missing %q: %s", want, csp)
				}
			}
			for _, weak := range []string{"unsafe-inline", "unsafe-eval", "*", "http:", "https:"} {
				if strings.Contains(csp, weak) {
					t.Errorf("CSP contains %q: %s", weak, csp)
				}
			}
			want := map[string]string{
				"X-Content-Type-Options":       "nosniff",
				"Referrer-Policy":              "strict-origin-when-cross-origin",
				"Cross-Origin-Opener-Policy":   "same-origin",
				"Cross-Origin-Resource-Policy": "same-origin",
				"Cross-Origin-Embedder-Policy": "require-corp",
				"X-Frame-Options":              "DENY",
			}
			for k, v := range want {
				if hd.Get(k) != v {
					t.Errorf("%s = %q, want %q", k, hd.Get(k), v)
				}
			}
			if hd.Get("Permissions-Policy") == "" {
				t.Error("missing Permissions-Policy")
			}
			for _, leak := range []string{"Server", "X-Powered-By", "Set-Cookie"} {
				if hd.Get(leak) != "" {
					t.Errorf("unexpected %s header: %q", leak, hd.Get(leak))
				}
			}
		})
	}
}

var (
	tagRe        = regexp.MustCompile(`(?is)<([a-z][a-z0-9-]*)\b([^>]*)>`)
	attrRe       = regexp.MustCompile(`(?is)\s([a-z][a-z0-9-:]*)\s*=\s*("[^"]*"|'[^']*'|[^\s>]+)`)
	scriptBodyRe = regexp.MustCompile(`(?is)<script\b[^>]*>(.*?)</script>`)
)

var urlAttrs = map[string]bool{"src": true, "href": true, "srcset": true, "action": true, "poster": true, "data": true, "formaction": true}

// checkHTMLPolicy enforces the CSP's promises in the markup itself, so a
// violation fails here rather than silently breaking in a browser.
func checkHTMLPolicy(t *testing.T, where, html string) {
	t.Helper()
	for _, m := range scriptBodyRe.FindAllStringSubmatch(html, -1) {
		if strings.TrimSpace(m[1]) != "" {
			t.Errorf("%s: inline script: %.80q", where, m[1])
		}
	}
	for _, m := range tagRe.FindAllStringSubmatch(html, -1) {
		tag := strings.ToLower(m[1])
		if tag == "style" {
			t.Errorf("%s: <style> element", where)
		}
		hasSrc := false
		for _, a := range attrRe.FindAllStringSubmatch(m[2], -1) {
			name := strings.ToLower(a[1])
			val := strings.Trim(a[2], `"'`)
			switch {
			case name == "style":
				t.Errorf("%s: style attribute on <%s>", where, tag)
			case strings.HasPrefix(name, "on"):
				t.Errorf("%s: event handler %s on <%s>", where, name, tag)
			case urlAttrs[name] && strings.HasPrefix(strings.ToLower(strings.TrimSpace(val)), "javascript:"):
				t.Errorf("%s: javascript: URL on <%s>", where, tag)
			}
			if name == "src" {
				hasSrc = true
			}
			// Plain links may point anywhere; anything the browser loads must be same-origin.
			if tag != "a" && urlAttrs[name] {
				for _, u := range strings.Split(val, ",") {
					u = strings.TrimSpace(u)
					if strings.Contains(u, "//") || strings.HasPrefix(strings.ToLower(u), "data:") {
						t.Errorf("%s: <%s %s=%q> loads from another origin", where, tag, name, u)
					}
				}
			}
			if tag == "a" && name == "target" && val == "_blank" && !strings.Contains(m[2], "noopener") {
				t.Errorf("%s: target=_blank without rel=noopener", where)
			}
		}
		if tag == "script" && !hasSrc {
			t.Errorf("%s: <script> without src", where)
		}
		switch tag {
		case "iframe", "frame", "object", "embed", "base":
			t.Errorf("%s: <%s> element", where, tag)
		}
	}
}

func TestRenderedHTMLHasNoInlineOrThirdPartyCode(t *testing.T) {
	h := testApp(t).routes()
	pages := []string{"/"}
	for _, v := range variants {
		pages = append(pages, "/v/"+v.Slug, "/v/"+v.Slug+"?sent=1")
	}
	for _, p := range pages {
		rec := do(h, "GET", p, nil, nil)
		checkHTMLPolicy(t, p, rec.Body.String())
	}
	// Error re-render path too.
	for _, v := range variants {
		f := url.Values{"variant": {v.Slug}}
		rec := postContact(h, f, "")
		if rec.Code != http.StatusUnprocessableEntity {
			t.Fatalf("%s: code %d", v.Slug, rec.Code)
		}
		checkHTMLPolicy(t, "422 "+v.Slug, rec.Body.String())
	}
}

func TestStaticAssetsAreSelfContained(t *testing.T) {
	external := regexp.MustCompile(`(?i)(https?:)?//[a-z0-9.-]+\.[a-z]{2,}`)
	sinks := regexp.MustCompile(`\b(eval|Function)\s*\(|\.innerHTML\s*=|\.outerHTML\s*=|document\.write|insertAdjacentHTML|setTimeout\s*\(\s*["'\x60]`)
	err := fs.WalkDir(embedded, "static", func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		if !strings.HasSuffix(p, ".css") && !strings.HasSuffix(p, ".js") {
			return nil
		}
		b, err := fs.ReadFile(embedded, p)
		if err != nil {
			return err
		}
		src := string(b)
		if m := external.FindString(src); m != "" {
			t.Errorf("%s references an external origin: %s", p, m)
		}
		if strings.HasSuffix(p, ".css") && strings.Contains(src, "@import") {
			t.Errorf("%s uses @import", p)
		}
		if strings.HasSuffix(p, ".js") {
			if m := sinks.FindString(src); m != "" {
				t.Errorf("%s uses a dangerous DOM/code sink: %s", p, m)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

var xssPayloads = []string{
	`"><script>alert(1)</script>`,
	`'><img src=x onerror=alert(1)>`,
	`</textarea><script>alert(1)</script>`,
	`" autofocus onfocus="alert(1)`,
	`javascript:alert(1)`,
	`<svg/onload=alert(1)>`,
	"{{.Site}}",
}

func TestContactReRenderEscapesInput(t *testing.T) {
	h := testApp(t).routes()
	for _, field := range []string{"name", "company", "message", "email"} {
		for _, p := range xssPayloads {
			f := validForm()
			f.Set(field, p)
			if field != "email" {
				f.Set("email", "invalid") // force the 422 re-render that echoes input back
			}
			rec := postContact(h, f, "")
			if rec.Code != http.StatusUnprocessableEntity {
				t.Fatalf("%s=%q: code %d, want 422", field, p, rec.Code)
			}
			body := rec.Body.String()
			for _, raw := range []string{"<script>alert", "<img src=x", "<svg/onload", `" autofocus`, "</textarea><"} {
				if strings.Contains(body, raw) {
					t.Errorf("%s=%q: response contains unescaped %q", field, p, raw)
				}
			}
			if strings.Contains(p, "{{") && !strings.Contains(body, "{{.Site}}") {
				t.Errorf("%s: template syntax in input was evaluated", field)
			}
			checkHTMLPolicy(t, field+"="+p, body)
		}
	}
}

func TestContactRedirectStaysLocal(t *testing.T) {
	for _, v := range []string{"//evil.example", "https://evil.example", `/\evil.example`, "varg\r\nSet-Cookie: x=1", "../../etc/passwd", "%2F%2Fevil", ""} {
		f := validForm()
		f.Set("variant", v)
		rec := postContact(testApp(t).routes(), f, "")
		loc := rec.Header().Get("Location")
		if loc != "/v/varg?sent=1#contact" {
			t.Errorf("variant=%q: Location = %q", v, loc)
		}
		if rec.Header().Get("Set-Cookie") != "" {
			t.Errorf("variant=%q: header injection", v)
		}
	}
}

func TestContactOriginCheck(t *testing.T) {
	h := testApp(t).routes()
	// httptest requests have Host "example.com".
	for origin, want := range map[string]int{
		"http://example.com":           303,
		"https://example.com":          303, // behind the TLS proxy the scheme differs from what Go sees
		"null":                         403, // sandboxed iframes, data: URLs
		"http://example.com.evil.test": 403,
		"http://evil.test/example.com": 403,
		"http://example.com:8443":      403,
		"http://user@evil.test":        403,
		"://":                          403,
	} {
		if rec := postContact(h, validForm(), origin); rec.Code != want {
			t.Errorf("Origin %q: code %d, want %d", origin, rec.Code, want)
		}
	}
}

func TestContactBodyLimit(t *testing.T) {
	h := testApp(t).routes()
	big := validForm()
	big.Set("message", strings.Repeat("A", 1<<20))
	rec := postContact(h, big, "")
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("1 MiB body: code %d, want 400", rec.Code)
	}
}

func TestMethodsAreRestricted(t *testing.T) {
	h := testApp(t).routes()
	for _, c := range []struct{ method, path string }{
		{"GET", "/contact"}, {"PUT", "/contact"}, {"DELETE", "/"}, {"PATCH", "/v/varg"},
		{"TRACE", "/"}, {"POST", "/"}, {"POST", "/static/css/base.css"}, {"PROPFIND", "/"},
	} {
		rec := do(h, c.method, c.path, nil, nil)
		if rec.Code != http.StatusMethodNotAllowed {
			t.Errorf("%s %s = %d, want 405", c.method, c.path, rec.Code)
		}
	}
}

func TestStaticCannotEscape(t *testing.T) {
	h := testApp(t).routes()
	paths := []string{
		"/static/../main.go", "/static/..%2fmain.go", "/static/%2e%2e/main.go", "/static/%2e%2e%2fgo.mod",
		"/static/css/../../go.mod", "/static//etc/passwd", "/static/..\\main.go", "/static/%5c..%5cmain.go",
		"/static/images/cocoparisienne-forest-1258845.jpg", // originals are not embedded
		"/templates/partials.html", "/static/../templates/partials.html", "/.git/config", "/go.mod",
		"/static/css", "/static/images/opt", "/static/js/", "/static/css/base.css/",
		"/static/css/base.css%00.js", "/static/css/base.css;.js", "/static/%84", "/static/css//base.css",
	}
	secrets := []string{"package main", "module wintermuteconsulting", "{{define", "[core]", "root:"}
	for _, p := range paths {
		rec := do(h, "GET", p, nil, nil)
		if rec.Code == http.StatusOK {
			t.Errorf("GET %s = 200", p)
		}
		for _, s := range secrets {
			if strings.Contains(rec.Body.String(), s) {
				t.Errorf("GET %s leaked %q", p, s)
			}
		}
		if rec.Code >= 500 {
			t.Errorf("GET %s = %d", p, rec.Code)
		}
	}
}

func TestLogInjection(t *testing.T) {
	var buf bytes.Buffer
	a, err := newApp(embedded, false, false, slog.New(slog.NewTextHandler(&buf, nil)))
	if err != nil {
		t.Fatal(err)
	}
	f := validForm()
	f.Set("name", "Ada\ntime=2020-01-01 level=ERROR msg=\"forged entry\"")
	f.Set("message", "line one\r\nlevel=ERROR msg=forged\n\x1b[31mred\x1b[0m")
	if rec := postContact(a.routes(), f, ""); rec.Code != http.StatusSeeOther {
		t.Fatalf("code %d", rec.Code)
	}
	lines := 0
	sc := bufio.NewScanner(&buf)
	for sc.Scan() {
		lines++
		if strings.HasPrefix(sc.Text(), "level=ERROR") || strings.Contains(sc.Text(), "\x1b") {
			t.Errorf("log line forged or carries raw control chars: %q", sc.Text())
		}
	}
	if lines != 1 {
		t.Errorf("one submission produced %d log lines:\n%s", lines, buf.String())
	}
}

func TestClientIPIgnoresForwardedHeaderUnlessTrusted(t *testing.T) {
	r := httptest.NewRequest("POST", "/contact", nil)
	r.RemoteAddr = "203.0.113.9:5555"
	r.Header.Set("X-Forwarded-For", "198.51.100.1, 192.0.2.7")

	untrusted := &app{}
	if ip := untrusted.clientIP(r); ip != "203.0.113.9" {
		t.Errorf("untrusted: clientIP = %q, want the socket address", ip)
	}
	trusted := &app{trustProxy: true}
	if ip := trusted.clientIP(r); ip != "192.0.2.7" {
		t.Errorf("trusted: clientIP = %q, want the proxy-appended last hop", ip)
	}
}

func TestRateLimitResistsSpoofingAndRotation(t *testing.T) {
	a, err := newApp(embedded, false, true, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatal(err)
	}
	h := a.routes()
	send := func(xff string) int {
		req := httptest.NewRequest(http.MethodPost, "/contact", strings.NewReader(validForm().Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.Header.Set("X-Forwarded-For", xff)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		return rec.Code
	}

	// A client-forged first hop must not buy a fresh bucket: the proxy's hop is what counts.
	for i := 0; i < 5; i++ {
		send("10.0.0." + strconv.Itoa(i) + ", 198.51.100.20")
	}
	if code := send("10.9.9.9, 198.51.100.20"); code != http.StatusTooManyRequests {
		t.Errorf("forged XFF prefix bypassed the limit: %d", code)
	}

	// Rotating addresses inside one IPv6 /64 is one client.
	for i := 0; i < 5; i++ {
		send("2001:db8:1:2::" + strconv.Itoa(i+1))
	}
	if code := send("2001:db8:1:2:ffff::1"); code != http.StatusTooManyRequests {
		t.Errorf("IPv6 /64 rotation bypassed the limit: %d", code)
	}
	if code := send("2001:db8:1:3::1"); code != http.StatusSeeOther {
		t.Errorf("a different /64 should have its own bucket: %d", code)
	}
}

func TestRateKey(t *testing.T) {
	for in, want := range map[string]string{
		"192.0.2.1":            "192.0.2.1",
		"::ffff:192.0.2.1":     "192.0.2.1",
		"2001:db8:1:2:3:4:5:6": "2001:db8:1:2::/64",
		"2001:db8:1:2::1%eth0": "2001:db8:1:2::/64",
		"not an ip":            "not an ip",
		"":                     "",
	} {
		if got := rateKey(in); got != want {
			t.Errorf("rateKey(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestRateLimiterMemoryIsBounded(t *testing.T) {
	now := time.Unix(0, 0)
	l := newRateLimiter(5, time.Minute)
	l.now = func() time.Time { return now }
	for i := 0; i < 20000; i++ {
		l.allow(net.IPv4(10, byte(i>>16), byte(i>>8), byte(i)).String())
	}
	now = now.Add(2 * time.Minute)
	l.allow("trigger-sweep")
	if n := len(l.hits); n > 1 {
		t.Errorf("%d expired keys survived the sweep", n)
	}
}

func TestServerLimits(t *testing.T) {
	s := newServer("127.0.0.1:0", http.NotFoundHandler())
	if s.ReadHeaderTimeout <= 0 || s.ReadHeaderTimeout > 10*time.Second {
		t.Errorf("ReadHeaderTimeout = %v", s.ReadHeaderTimeout)
	}
	if s.ReadTimeout <= 0 || s.WriteTimeout <= 0 || s.IdleTimeout <= 0 {
		t.Errorf("unbounded timeouts: read %v write %v idle %v", s.ReadTimeout, s.WriteTimeout, s.IdleTimeout)
	}
	if s.MaxHeaderBytes <= 0 || s.MaxHeaderBytes > 64<<10 {
		t.Errorf("MaxHeaderBytes = %d", s.MaxHeaderBytes)
	}
}

// startServer runs the production server config on a loopback port.
func startServer(t *testing.T) string {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	s := newServer("", testApp(t).routes())
	go s.Serve(ln)
	t.Cleanup(func() { s.Close() })
	return ln.Addr().String()
}

func TestOversizedHeadersRejected(t *testing.T) {
	addr := startServer(t)
	req, _ := http.NewRequest("GET", "http://"+addr+"/", nil)
	req.Header.Set("X-Big", strings.Repeat("a", 64<<10))
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return // connection closed is also a rejection
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusRequestHeaderFieldsTooLarge {
		t.Errorf("64 KiB header: code %d, want 431", resp.StatusCode)
	}
}

func TestSlowlorisConnectionIsDropped(t *testing.T) {
	if testing.Short() {
		t.Skip("waits for the real header timeout")
	}
	t.Parallel()
	addr := startServer(t)
	c, err := net.Dial("tcp", addr)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	io.WriteString(c, "GET / HTTP/1.1\r\nHost: x\r\n") // never finish the headers
	c.SetReadDeadline(time.Now().Add(8 * time.Second))
	start := time.Now()
	_, err = c.Read(make([]byte, 1))
	for err == nil { // drain whatever error response the server sends before closing
		_, err = c.Read(make([]byte, 512))
	}
	if ne, ok := err.(net.Error); ok && ne.Timeout() {
		t.Fatalf("server kept a half-sent request open for %v", time.Since(start))
	}
}
