package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"html/template"
	"io/fs"
	"log/slog"
	"net/http"
	"sync"
)

var templateFuncs = template.FuncMap{
	"inc": func(i int) int { return i + 1 },
}

type app struct {
	assets     fs.FS
	dev        bool
	trustProxy bool
	log        *slog.Logger
	limiter    *rateLimiter

	mu      sync.Mutex
	content *Content // content.json: the site's text
	// "home" and "about", and "m/home" and "m/about" for the phone layout.
	pages map[string]*template.Template

	versions sync.Map // static file ("js/reveal.js") -> hash of its content; see staticVersion
}

func newApp(assets fs.FS, dev, trustProxy bool, log *slog.Logger) (*app, error) {
	a := &app{assets: assets, dev: dev, trustProxy: trustProxy, log: log, limiter: newRateLimiter(5, contactWindow)}
	if err := a.reloadContent(); err != nil {
		return nil, err
	}
	if err := a.parseTemplates(); err != nil {
		return nil, err
	}
	return a, nil
}

// reloadContent reads content.json: once at start-up, and in -dev before every
// page so that a text edit shows on a refresh.
func (a *app) reloadContent() error {
	c, err := loadContent(a.assets)
	if err != nil {
		return err
	}
	a.mu.Lock()
	a.content = c
	a.mu.Unlock()
	return nil
}

// staticURL is the template func "static": the URL of a file under static/
// with a short hash of its content as ?v=. A changed file gets a new URL, so no
// browser keeps showing an old stylesheet or script after a deploy, and an
// unchanged one can be cached for good (see cacheStatic). Every stylesheet and
// script must be linked through it. In -dev the files change under us and are
// served uncached, so the URL is left bare.
func (a *app) staticURL(file string) string {
	if v := a.staticVersion(file); v != "" {
		return "/static/" + file + "?v=" + v
	}
	return "/static/" + file
}

// staticVersion hashes a static file once and remembers the result. Files that
// don't exist are not remembered, so made-up paths can't grow the map.
func (a *app) staticVersion(file string) string {
	if a.dev {
		return ""
	}
	if v, ok := a.versions.Load(file); ok {
		return v.(string)
	}
	b, err := fs.ReadFile(a.assets, "static/"+file)
	if err != nil {
		return ""
	}
	sum := sha256.Sum256(b)
	v := hex.EncodeToString(sum[:6])
	a.versions.Store(file, v)
	return v
}

// parseTemplates builds one template set per page and layout: the shared
// partials plus that page's own file, which defines "page". On a phone both
// pages are templates/mobile.html, the shell with the homepage in it;
// templates/m/about.html replaces its "m-main" for the About page.
func (a *app) parseTemplates() error {
	pages := map[string]*template.Template{}
	sets := map[string][]string{
		"home":    {"templates/home.html"},
		"about":   {"templates/about-body.html", "templates/about.html"},
		"m/home":  {"templates/mobile.html"},
		"m/about": {"templates/about-body.html", "templates/mobile.html", "templates/m/about.html"},
	}
	for name, files := range sets {
		t, err := template.New(name).Funcs(templateFuncs).Funcs(template.FuncMap{"static": a.staticURL}).ParseFS(a.assets, append([]string{"templates/partials.html", "templates/ps.html"}, files...)...)
		if err != nil {
			return fmt.Errorf("parse %s: %w", name, err)
		}
		pages[name] = t
	}
	a.mu.Lock()
	a.pages = pages
	a.mu.Unlock()
	return nil
}

type pageData struct {
	Site     Site
	Copy     Copy   // the homepage's own wording ("home" in content.json)
	Page     string // "" for the homepage, "about" for the About page
	Form     contactForm
	Errors   map[string]string
	Sent     bool
	Portrait bool // the principal's photo is in place (see hasPortrait)

	Mobile bool   // serving the mobile layout
	Phone  bool   // the device looks like a phone (offer a layout switch)
	View   string // explicit ?view= override to carry through links and the form
}

// ViewQuery is the query string that keeps an explicit layout choice on internal links.
func (d pageData) ViewQuery() string {
	if d.View == "" {
		return ""
	}
	return "?view=" + d.View
}

// Path is this page's own URL path, without the layout override.
func (d pageData) Path() string {
	return "/" + d.Page
}

// Home goes in front of the homepage's anchors (#services, #contact): empty on
// the homepage itself, so they stay in-page links, and the homepage's URL on
// the About page.
func (d pageData) Home() string {
	if d.Page == "" {
		return ""
	}
	return "/" + d.ViewQuery()
}

// portraitFiles are the renditions of the principal's photo that
// scripts/portrait.sh writes to static/images/opt. Until all of them exist the
// About page shows a placeholder in the photo's place.
var portraitFiles = []string{"portrait-480.webp", "portrait-960.webp", "portrait-480.jpg", "portrait-960.jpg"}

func hasPortrait(assets fs.FS) bool {
	for _, f := range portraitFiles {
		if _, err := fs.Stat(assets, "static/images/opt/"+f); err != nil {
			return false
		}
	}
	return true
}

// renderPage serves a page in the layout that suits the request.
func (a *app) renderPage(w http.ResponseWriter, r *http.Request, status int, data pageData) {
	data.Phone = isPhone(r)
	data.Mobile = wantMobile(r, data.View)
	data.Portrait = hasPortrait(a.assets)
	varyOnDevice(w)
	name := "home"
	if data.Page != "" {
		name = data.Page
	}
	if data.Mobile {
		name = "m/" + name
	}
	a.render(w, status, name, data)
}

func (a *app) render(w http.ResponseWriter, status int, name string, data pageData) {
	if a.dev {
		err := a.reloadContent()
		if err == nil {
			err = a.parseTemplates()
		}
		if err != nil {
			a.log.Error("reload", "err", err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}
	a.mu.Lock()
	t, content := a.pages[name], a.content
	a.mu.Unlock()

	data.Site = content.Site
	data.Copy = content.Home

	// Render into a buffer so a template error doesn't leave a half-written page.
	var buf bytes.Buffer
	if err := t.ExecuteTemplate(&buf, "page", data); err != nil {
		a.log.Error("render", "page", name, "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	// Pages are always fetched afresh, so they always name the current assets.
	w.Header().Set("Cache-Control", "no-cache")
	w.WriteHeader(status)
	_, _ = buf.WriteTo(w) // client gone mid-response; nothing useful to do
}
