package main

import (
	"io"
	"log/slog"
	"regexp"
	"strings"
	"testing"
	"testing/fstest"
)

// Stylesheets and scripts are linked by content version, so a deploy reaches
// every browser at once: a changed file has a new URL, and a browser holding
// the old one never asks for it again.

func TestStaticURLsCarryTheContentVersion(t *testing.T) {
	h := testApp(t).routes()
	pages := []string{"/"}
	for _, v := range variants {
		pages = append(pages, "/v/"+v.Slug, "/v/"+v.Slug+"/about")
	}
	asset := regexp.MustCompile(`(?:href|src)="(/static/(?:css|js)/[^"]*)"`)
	seen := map[string]bool{}
	for _, p := range pages {
		for _, ua := range []string{uaDesktop, uaIPhone} {
			rec := do(h, "GET", p, nil, map[string]string{"User-Agent": ua})
			// The page itself is never reused from cache, or it could name old assets.
			if cc := rec.Header().Get("Cache-Control"); cc != "no-cache" {
				t.Errorf("%s: Cache-Control = %q, want no-cache", p, cc)
			}
			refs := asset.FindAllStringSubmatch(rec.Body.String(), -1)
			if len(refs) == 0 {
				t.Errorf("%s: no stylesheet or script found", p)
			}
			for _, m := range refs {
				url := strings.ReplaceAll(m[1], "&amp;", "&")
				if !regexp.MustCompile(`\?v=[0-9a-f]{12}$`).MatchString(url) {
					t.Errorf("%s: %s is linked without its version (use {{static}})", p, url)
				}
				seen[url] = true
			}
		}
	}
	for url := range seen {
		rec := do(h, "GET", url, nil, nil)
		if cc := rec.Header().Get("Cache-Control"); rec.Code != 200 || !strings.Contains(cc, "immutable") {
			t.Errorf("GET %s = %d, Cache-Control %q; want 200 and immutable", url, rec.Code, cc)
		}
	}

	// A bare or outdated URL still works, but is only kept for a day.
	for _, url := range []string{"/static/js/wolfeyes.js", "/static/js/wolfeyes.js?v=000000000000", "/static/js/wolfeyes.js?v=", "/static/fonts/inter.woff2"} {
		rec := do(h, "GET", url, nil, nil)
		if cc := rec.Header().Get("Cache-Control"); rec.Code != 200 || cc != "public, max-age=86400" {
			t.Errorf("GET %s = %d, Cache-Control %q; want 200 and a one-day cache", url, rec.Code, cc)
		}
	}
	// A version on a file that doesn't exist earns nothing.
	if rec := do(h, "GET", "/static/js/nope.js?v=abc", nil, nil); rec.Code != 404 || strings.Contains(rec.Header().Get("Cache-Control"), "immutable") {
		t.Errorf("missing file: %d, Cache-Control %q", rec.Code, rec.Header().Get("Cache-Control"))
	}
}

func TestStaticVersionFollowsTheContent(t *testing.T) {
	quiet := slog.New(slog.NewTextHandler(io.Discard, nil))
	a := testApp(t)
	before := a.staticURL("js/wolfeyes.js")
	version := func(url string) string { _, v, _ := strings.Cut(url, "?v="); return v }
	if before != a.staticURL("js/wolfeyes.js") || version(before) == "" || version(before) == version(a.staticURL("js/reveal.js")) {
		t.Fatalf("version is not stable and per file: %s", before)
	}

	changed := overlayFS{embedded, fstest.MapFS{"static/js/wolfeyes.js": &fstest.MapFile{Data: []byte("// a new release")}}}
	b, err := newApp(changed, false, false, quiet)
	if err != nil {
		t.Fatal(err)
	}
	after := b.staticURL("js/wolfeyes.js")
	if after == before || !strings.HasPrefix(after, "/static/js/wolfeyes.js?v=") {
		t.Errorf("changed file kept its URL: before %s, after %s", before, after)
	}
	if b.staticURL("js/reveal.js") != a.staticURL("js/reveal.js") {
		t.Error("an unchanged file got a new URL")
	}
	// The old URL must not be treated as naming the new content.
	if rec := do(b.routes(), "GET", before, nil, nil); strings.Contains(rec.Header().Get("Cache-Control"), "immutable") {
		t.Error("an outdated version was served as immutable")
	}

	// In -dev files change under the running server: no version, nothing cached.
	dev := &app{dev: true, assets: embedded}
	if got := dev.staticURL("js/wolfeyes.js"); got != "/static/js/wolfeyes.js" {
		t.Errorf("dev URL = %s, want it bare", got)
	}
	// Unknown files get a bare URL and are not remembered.
	if got := a.staticURL("js/nope.js"); got != "/static/js/nope.js" {
		t.Errorf("unknown file URL = %s", got)
	}
	if _, ok := a.versions.Load("js/nope.js"); ok {
		t.Error("unknown file was remembered")
	}
}
