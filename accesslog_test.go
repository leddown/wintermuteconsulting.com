package main

import (
	"bytes"
	"io"
	"log/slog"
	"net/http"
	"regexp"
	"strings"
	"testing"
)

// clfLine is the Combined Log Format plus the time taken, as accessLog writes it.
var clfLine = regexp.MustCompile(`^(\S+) - - \[\d{2}/[A-Z][a-z]{2}/\d{4}:\d{2}:\d{2}:\d{2} \+0000\] "((?:[^"\\]|\\.)*)" (\d{3}) (\d+|-) "((?:[^"\\]|\\.)*)" "((?:[^"\\]|\\.)*)" \d+\.\d{3}$`)

func accessApp(t *testing.T, trustProxy bool) (*app, *bytes.Buffer) {
	t.Helper()
	a, err := newApp(embedded, false, trustProxy, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	a.access = &buf
	return a, &buf
}

func accessLines(t *testing.T, buf *bytes.Buffer) [][]string {
	t.Helper()
	var out [][]string
	for _, line := range strings.Split(strings.TrimSuffix(buf.String(), "\n"), "\n") {
		if line == "" {
			continue
		}
		m := clfLine.FindStringSubmatch(line)
		if m == nil {
			t.Fatalf("not a Combined Log Format line: %q", line)
		}
		out = append(out, m)
	}
	return out
}

func TestAccessLogLine(t *testing.T) {
	a, buf := accessApp(t, true)
	h := a.routes()
	do(h, "GET", "/about?view=mobile", nil, map[string]string{
		"X-Forwarded-For": "198.51.100.1, 192.0.2.7",
		"Referer":         "https://ref.example/page",
		"User-Agent":      "TestAgent/1.0",
	})
	do(h, "GET", "/nope", nil, nil)

	lines := accessLines(t, buf)
	if len(lines) != 2 {
		t.Fatalf("want 2 lines, got %d:\n%s", len(lines), buf)
	}
	about := lines[0]
	for i, want := range map[int]string{1: "192.0.2.7", 2: "GET /about?view=mobile HTTP/1.1", 3: "200", 5: "https://ref.example/page", 6: "TestAgent/1.0"} {
		if about[i] != want {
			t.Errorf("field %d = %q, want %q", i, about[i], want)
		}
	}
	if about[4] == "-" {
		t.Error("a rendered page logged no bytes")
	}
	if nf := lines[1]; nf[1] != "192.0.2.1" || nf[3] != "404" || nf[5] != "-" || nf[6] != "-" {
		t.Errorf("404 line: %q", nf[0])
	}
}

func TestAccessLogSkipsHealthCheck(t *testing.T) {
	a, buf := accessApp(t, false)
	do(a.routes(), "GET", "/healthz", nil, nil)
	if buf.Len() != 0 {
		t.Errorf("health check was logged: %q", buf)
	}
}

func TestAccessLogOffByDefault(t *testing.T) {
	a, buf := accessApp(t, false)
	a.access = nil
	if rec := do(a.routes(), "GET", "/", nil, nil); rec.Code != http.StatusOK {
		t.Fatalf("code %d", rec.Code)
	}
	if buf.Len() != 0 {
		t.Error("logged with no access log set")
	}
}

// The contact form's text is personal data: it must never reach the access log.
func TestAccessLogKeepsContactBodyOut(t *testing.T) {
	a, buf := accessApp(t, false)
	f := validForm()
	f.Set("message", "secret-enquiry-text, long enough to pass")
	if rec := postContact(a.routes(), f, ""); rec.Code != http.StatusSeeOther {
		t.Fatalf("code %d", rec.Code)
	}
	lines := accessLines(t, buf)
	if len(lines) != 1 || lines[0][2] != "POST /contact HTTP/1.1" || lines[0][3] != "303" {
		t.Fatalf("contact line: %q", buf)
	}
	if strings.Contains(buf.String(), "secret-enquiry") || strings.Contains(buf.String(), "ada@") {
		t.Errorf("form data in access log: %q", buf)
	}
}

func TestAccessLogInjection(t *testing.T) {
	a, buf := accessApp(t, true)
	h := a.routes()
	do(h, "GET", "/", nil, map[string]string{
		"User-Agent":      "x\" 200 1 \"-\" \"forged\nother - - [01/Jan/2020:00:00:00 +0000] \"GET / HTTP/1.1\" 200 1 \"-\" \"-\" 0.000\x1b[31m",
		"Referer":         "\\\"é\x00",
		"X-Forwarded-For": "not-an-ip\n",
	})
	do(h, "GET", "/", nil, map[string]string{"User-Agent": strings.Repeat("\xff", 10000)})

	if strings.Count(buf.String(), "\n") != 2 {
		t.Fatalf("two requests gave other than two lines:\n%s", buf)
	}
	for _, c := range buf.String() {
		if c != '\n' && (c < 0x20 || c >= 0x7f) {
			t.Fatalf("raw byte %q in access log: %q", c, buf)
		}
	}
	lines := accessLines(t, buf)
	if lines[0][1] != "-" {
		t.Errorf("a non-address client IP was logged as %q", lines[0][1])
	}
	if want := `\\\"\xc3\xa9\x00`; lines[0][5] != want {
		t.Errorf("referer escaped as %q, want %q", lines[0][5], want)
	}
	if ua := lines[1][6]; len(ua) > maxLogUserAgent+3 || !strings.HasSuffix(ua, "...") {
		t.Errorf("oversized User-Agent not cut: %d bytes", len(ua))
	}
}

func TestOpenAccessLog(t *testing.T) {
	if w, err := openAccessLog(""); w != nil || err != nil {
		t.Errorf(`"": %v, %v`, w, err)
	}
	if w, err := openAccessLog("stderr"); w == nil || err != nil {
		t.Errorf("stderr: %v, %v", w, err)
	}
	if _, err := openAccessLog("/var/log/access"); err == nil {
		t.Error("a file path was accepted")
	}
}
