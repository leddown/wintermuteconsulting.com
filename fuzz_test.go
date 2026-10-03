package main

import (
	"io"
	"io/fs"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

// Fuzz targets. `go test` runs only the seed corpus; security/run.sh fuzzes
// each one for real (go test -fuzz=FuzzX -fuzztime=...). Crashers land in
// testdata/fuzz/ and then run as regression tests forever.

func fuzzApp(f *testing.F) http.Handler {
	a, err := newApp(embedded, false, true, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		f.Fatal(err)
	}
	a.limiter = newRateLimiter(1<<30, time.Minute) // fuzz the handler, not the limiter
	return a.routes()
}

func FuzzContact(f *testing.F) {
	for _, p := range xssPayloads {
		f.Add("spor", p, "a@b.co", p, p, "", "", p)
	}
	f.Add("signal2", "Ada", "ada@example.com", "", "Hello there, long enough.", "", "", "desktop")
	f.Add("../x", "\x00", "\"a\"@b", "\xff\xfe", strings.Repeat("é", 5001), "bot", "null", "mobile")
	h := fuzzApp(f)

	post := func(form url.Values, phone bool, origin string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodPost, "/contact", strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		if phone {
			req.Header.Set("User-Agent", uaIPhone)
		}
		if origin != "" {
			req.Header.Set("Origin", origin)
		}
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		return rec
	}

	f.Fuzz(func(t *testing.T, variant, name, email, company, message, website, origin, view string) {
		form := url.Values{"variant": {variant}, "name": {name}, "email": {email}, "company": {company}, "message": {message}, "website": {website}, "view": {view}}
		phone := len(name)%2 == 1 // exercise both layouts
		rec := post(form, phone, origin)

		switch rec.Code {
		case http.StatusSeeOther:
			loc := rec.Header().Get("Location")
			if !strings.HasPrefix(loc, "/v/") || strings.ContainsAny(loc, "\r\n\\") || strings.HasPrefix(loc, "//") {
				t.Fatalf("unsafe redirect %q", loc)
			}
			u, err := url.Parse(loc)
			if err != nil || u.Host != "" || u.Scheme != "" {
				t.Fatalf("unsafe redirect %q", loc)
			}
			if _, ok := findVariant(strings.TrimPrefix(u.Path, "/v/")); !ok {
				t.Fatalf("redirect to unknown page %q", loc)
			}
		case http.StatusUnprocessableEntity:
			// For each field, compare with the same request where only that field is
			// replaced, so markup that merely resembles the input (maxlength="5000"
			// vs `000"`, or another field's escaped value="00000000" vs `"000`) doesn't
			// count. A bare & is ignored: its escape (&amp;) contains it, and it
			// cannot break out of markup.
			body := rec.Body.String()
			for _, field := range []string{"name", "email", "company", "message"} {
				in := form.Get(field)
				if len(in) <= 3 || !strings.ContainsAny(in, "<>\"'") {
					continue
				}
				other := url.Values{}
				for k, v := range form {
					other[k] = v
				}
				other.Set(field, "x")
				baseline := post(other, phone, origin).Body.String()
				if strings.Count(body, in) > strings.Count(baseline, in) {
					t.Fatalf("%s echoed unescaped: %q", field, in)
				}
			}
		case http.StatusForbidden, http.StatusBadRequest:
		default:
			t.Fatalf("unexpected status %d", rec.Code)
		}
		if rec.Header().Get("Content-Security-Policy") == "" {
			t.Fatal("missing CSP")
		}
	})
}

func FuzzStaticPath(f *testing.F) {
	for _, s := range []string{"css/base.css", "../main.go", "..%2fgo.mod", "%2e%2e/%2e%2e/etc/passwd", "images/opt/", "js/../../templates/index.html", "css/base.css\x00", "\\..\\main.go"} {
		f.Add(s)
	}
	h := fuzzApp(f)
	served := map[string]bool{}
	fs.WalkDir(embedded, "static", func(p string, d fs.DirEntry, err error) error {
		if err == nil && !d.IsDir() {
			served["/"+p] = true
		}
		return nil
	})

	f.Fuzz(func(t *testing.T, p string) {
		u, err := url.Parse("/static/" + p)
		if err != nil || strings.ContainsAny(p, " \t\r\n") {
			return
		}
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.URL = u
		req.RequestURI = u.RequestURI()
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)

		if rec.Code >= 500 {
			t.Fatalf("GET %q = %d", u, rec.Code)
		}
		if rec.Code == http.StatusOK && !served[u.Path] {
			t.Fatalf("GET %q served %q, which is not an embedded static file", u, u.Path)
		}
		body := rec.Body.String()
		if strings.Contains(body, "package main") || strings.Contains(body, "{{define") || strings.Contains(body, "module wintermuteconsulting") {
			t.Fatalf("GET %q leaked source", u)
		}
	})
}

func FuzzClientIP(f *testing.F) {
	for _, s := range []string{"", "1.2.3.4", "a, b, 2001:db8::1", ",,,", "::ffff:10.0.0.1", "fe80::1%eth0", strings.Repeat(",", 1000)} {
		f.Add(s, "203.0.113.1:443")
	}
	f.Fuzz(func(t *testing.T, xff, remote string) {
		r := httptest.NewRequest(http.MethodPost, "/contact", nil)
		r.RemoteAddr = remote
		bare := r.Clone(r.Context())
		r.Header["X-Forwarded-For"] = []string{xff}

		untrusted := &app{}
		if got, want := untrusted.clientIP(r), untrusted.clientIP(bare); got != want {
			t.Fatalf("untrusted request used X-Forwarded-For: %q, want %q", got, want)
		}
		for _, a := range []*app{untrusted, {trustProxy: true}} {
			k := rateKey(a.clientIP(r))
			if rateKey(k) != k {
				t.Fatalf("rateKey not stable: %q -> %q", k, rateKey(k))
			}
		}
	})
}
