package main

import (
	"io"
	"io/fs"
	"log/slog"
	"strings"
	"testing"
	"testing/fstest"
)

// The About page: one template for every design, served in both layouts.

func TestAboutPage(t *testing.T) {
	a := testApp(t)
	h, site := a.routes(), a.content.Site
	portrait := "Portrait to follow"
	if hasPortrait(embedded) {
		portrait = `src="/static/images/opt/portrait-960.jpg?v=`
	}
	for _, v := range variants {
		path := "/v/" + v.Slug + "/about"
		for _, ua := range []string{uaDesktop, uaIPhone} {
			rec := do(h, "GET", path, nil, map[string]string{"User-Agent": ua})
			if rec.Code != 200 {
				t.Fatalf("GET %s = %d", path, rec.Code)
			}
			body := rec.Body.String()
			where := path + " " + ua[13:20]
			checkHTMLPolicy(t, where, body)
			for _, want := range []string{
				"<title>About " + site.Name + "</title>",
				`content="` + site.About.Summary + `"`,
				site.About.Title,
				site.About.Principal.Role,
				`href="/static/css/` + v.Theme() + `.css?v=`,
				`href="/static/css/about.css?v=`,
				`href="/v/` + v.Slug + `#contact"`, // the form lives on the homepage
				`href="` + path + `" aria-current="page"`,
				portrait,
			} {
				if !strings.Contains(body, want) {
					t.Errorf("%s: missing %q", where, want)
				}
			}
			// The only in-page anchor is the skip link: the sections are on the homepage.
			if n := strings.Count(body, `href="#`) - strings.Count(body, `href="#main"`); n != 0 {
				t.Errorf("%s: %d in-page anchors point at sections that only exist on the homepage", where, n)
			}
		}

		// Layout: phones get the design's mobile shell, with a way to the desktop page and back.
		_, _, body := getAs(t, path, uaIPhone)
		if !strings.Contains(body, `class="m m--`+v.Slug) || !strings.Contains(body, `href="`+path+`?view=desktop"`) {
			t.Errorf("%s: phone did not get the mobile layout with a desktop switch", path)
		}
		_, _, body = getAs(t, path, uaDesktop)
		if !strings.Contains(body, " about-page\">") || strings.Contains(body, "?view=") {
			t.Errorf("%s: desktop did not get the plain desktop layout", path)
		}
		// A forced layout follows every link off the page, and the design switcher stays on About.
		_, _, body = getAs(t, path+"?view=desktop", uaIPhone)
		for _, want := range []string{`href="/v/` + v.Slug + `?view=desktop#contact"`, `href="/v/` + variants[0].Slug + `/about?view=desktop"`, `href="` + path + `?view=mobile"`} {
			if !strings.Contains(body, want) {
				t.Errorf("%s?view=desktop: missing %q", path, want)
			}
		}

		// Every homepage links to it, in both layouts.
		for _, ua := range []string{uaDesktop, uaIPhone} {
			if _, _, body := getAs(t, "/v/"+v.Slug, ua); !strings.Contains(body, `href="`+path+`"`) {
				t.Errorf("/v/%s (%s): no link to the About page", v.Slug, ua[13:20])
			}
		}
	}
}

// overlayFS serves a few extra files on top of the embedded assets.
type overlayFS struct{ base, extra fs.FS }

func (o overlayFS) Open(name string) (fs.File, error) {
	if f, err := o.extra.Open(name); err == nil {
		return f, nil
	}
	return o.base.Open(name)
}

func TestAboutShowsPortraitOnceAllRenditionsExist(t *testing.T) {
	photo := fstest.MapFS{}
	for i, f := range portraitFiles {
		if hasPortrait(overlayFS{embedded, photo}) && !hasPortrait(embedded) {
			t.Fatalf("hasPortrait true with only %d of %d files", i, len(portraitFiles))
		}
		photo["static/images/opt/"+f] = &fstest.MapFile{Data: []byte("x")}
	}
	assets := overlayFS{embedded, photo}
	if !hasPortrait(assets) {
		t.Fatal("hasPortrait false with every rendition present")
	}

	a, err := newApp(assets, false, false, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatal(err)
	}
	h, site := a.routes(), a.content.Site
	for _, ua := range []string{uaDesktop, uaIPhone} {
		body := do(h, "GET", "/v/natt/about", nil, map[string]string{"User-Agent": ua}).Body.String()
		checkHTMLPolicy(t, "about with portrait", body)
		for _, want := range []string{`src="/static/images/opt/portrait-960.jpg?v=`, `/static/images/opt/portrait-480.webp?v=`, ` 480w, `, `alt="Portrait of ` + site.About.Principal.Name + `"`, `width="960" height="1200"`} {
			if !strings.Contains(body, want) {
				t.Errorf("about with portrait (%s): missing %q", ua[13:20], want)
			}
		}
		if strings.Contains(body, "Portrait to follow") {
			t.Errorf("about with portrait (%s): placeholder still shown", ua[13:20])
		}
	}
	if rec := do(h, "GET", "/static/images/opt/portrait-480.webp", nil, nil); rec.Code != 200 {
		t.Errorf("portrait file = %d", rec.Code)
	}
}
