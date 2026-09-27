package main

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

func testApp(t *testing.T) *app {
	t.Helper()
	a, err := newApp(embedded, false, false, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatal(err)
	}
	return a
}

func TestPagesRender(t *testing.T) {
	h := testApp(t).routes()
	paths := []string{"/", "/static/css/base.css", "/static/js/wolfeyes.js", "/static/images/opt/forest-1600.webp"}
	for _, v := range variants {
		paths = append(paths, "/v/"+v.Slug)
	}
	for _, p := range paths {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, p, nil))
		if rec.Code != http.StatusOK {
			t.Errorf("GET %s = %d", p, rec.Code)
		}
		if rec.Header().Get("Content-Security-Policy") == "" {
			t.Errorf("GET %s missing CSP", p)
		}
	}
}

func TestNotFound(t *testing.T) {
	h := testApp(t).routes()
	for _, p := range []string{"/v/nope", "/nope", "/static/", "/static/css/"} {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, p, nil))
		if rec.Code != http.StatusNotFound {
			t.Errorf("GET %s = %d, want 404", p, rec.Code)
		}
	}
}

func postContact(h http.Handler, form url.Values, origin string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, "/contact", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	if origin != "" {
		req.Header.Set("Origin", origin)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func validForm() url.Values {
	return url.Values{
		"variant": {"signal"},
		"name":    {"Ada"},
		"email":   {"ada@example.com"},
		"message": {"We think someone is inside our network."},
	}
}

func TestContact(t *testing.T) {
	tests := []struct {
		name     string
		mutate   func(url.Values)
		origin   string
		wantCode int
		wantLoc  string
		wantBody string
	}{
		{name: "valid", wantCode: http.StatusSeeOther, wantLoc: "/v/signal?sent=1#contact"},
		{name: "honeypot", mutate: func(f url.Values) { f.Set("website", "http://spam") }, wantCode: http.StatusSeeOther, wantLoc: "/v/signal?sent=1#contact"},
		{name: "bad email", mutate: func(f url.Values) { f.Set("email", "nope") }, wantCode: http.StatusUnprocessableEntity, wantBody: "valid email"},
		{name: "short message", mutate: func(f url.Values) { f.Set("message", "hi") }, wantCode: http.StatusUnprocessableEntity, wantBody: "at least 10"},
		{name: "unknown variant falls back", mutate: func(f url.Values) { f.Set("variant", "../x") }, wantCode: http.StatusSeeOther, wantLoc: "/v/varg?sent=1#contact"},
		{name: "cross-origin", origin: "https://evil.example", wantCode: http.StatusForbidden},
		{name: "same origin", origin: "http://example.com", wantCode: http.StatusSeeOther},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := testApp(t).routes()
			f := validForm()
			if tt.mutate != nil {
				tt.mutate(f)
			}
			rec := postContact(h, f, tt.origin)
			if rec.Code != tt.wantCode {
				t.Fatalf("code = %d, want %d", rec.Code, tt.wantCode)
			}
			if tt.wantLoc != "" && rec.Header().Get("Location") != tt.wantLoc {
				t.Errorf("Location = %q, want %q", rec.Header().Get("Location"), tt.wantLoc)
			}
			if tt.wantBody != "" && !strings.Contains(rec.Body.String(), tt.wantBody) {
				t.Errorf("body missing %q", tt.wantBody)
			}
		})
	}
}

func TestContactRateLimit(t *testing.T) {
	h := testApp(t).routes()
	for i := 0; i < 5; i++ {
		if rec := postContact(h, validForm(), ""); rec.Code != http.StatusSeeOther {
			t.Fatalf("request %d = %d", i, rec.Code)
		}
	}
	if rec := postContact(h, validForm(), ""); rec.Code != http.StatusTooManyRequests {
		t.Fatalf("6th request = %d, want 429", rec.Code)
	}
}

func TestRateLimiterWindow(t *testing.T) {
	now := time.Unix(0, 0)
	l := newRateLimiter(2, time.Minute)
	l.now = func() time.Time { return now }
	//lint:ignore SA4000 allow() has side effects: this is three successive calls
	if !l.allow("a") || !l.allow("a") || l.allow("a") {
		t.Fatal("limit not enforced")
	}
	if !l.allow("b") {
		t.Fatal("keys should be independent")
	}
	now = now.Add(time.Minute + time.Second)
	if !l.allow("a") {
		t.Fatal("window did not expire")
	}
}
