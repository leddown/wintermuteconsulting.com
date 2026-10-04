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

// Variant is one of the alternative homepage designs, served at /v/{Slug}.
type Variant struct {
	Slug    string
	Name    string
	Summary string
}

// variantTheme maps a variant to the design whose stylesheet (and mobile
// treatment) it reuses, for variants that are a tweak of another design.
var variantTheme = map[string]string{"grense2": "grense"}

// Theme is the design whose stylesheet and tokens this variant uses.
func (v Variant) Theme() string {
	if t, ok := variantTheme[v.Slug]; ok {
		return t
	}
	return v.Slug
}

var variants = []Variant{
	{"signal", "Signal", "A supervision console: monospace, a live remediation tracker, scanlines over the forest."},
	{"spor", "Spor", "Scroll-driven descent into the forest that ends face to face with the wolf."},
	{"natt", "Natt", "The dark advisory site: the illustrated night as the page, amber as the single signal colour."},
	{"grense", "Grense", "The tree line runs down the page: the forest and the navigation on the left, your side of it on the right."},
	{"grense2", "Grense 2", "Grense on a darkened cream page."},
}

// psThemes are the designs built on the professional-services furniture
// (templates/ps.html, static/css/ps.css): its section heads, lists and footer.
var psThemes = map[string]bool{"natt": true, "grense": true}

// PS reports whether this variant's design uses the professional-services furniture.
func (v Variant) PS() bool { return psThemes[v.Theme()] }

// Signal reports whether this variant uses the console design: full legal
// form ("OÜ") in the logo, and the terminal lede on the About page.
func (v Variant) Signal() bool { return v.Theme() == "signal" }

func findVariant(slug string) (Variant, bool) {
	for _, v := range variants {
		if v.Slug == slug {
			return v, true
		}
	}
	return Variant{}, false
}

var templateFuncs = template.FuncMap{
	"inc": func(i int) int { return i + 1 },
}

type app struct {
	assets     fs.FS
	dev        bool
	trustProxy bool
	log        *slog.Logger
	limiter    *rateLimiter

	mu sync.Mutex
	// "index"; each variant slug and "m/<slug>" for its mobile layout; "about"
	// (one desktop template for every design) and "m/<slug>/about".
	pages map[string]*template.Template

	versions sync.Map // static file ("js/wolfeyes.js") -> hash of its content; see staticVersion
}

func newApp(assets fs.FS, dev, trustProxy bool, log *slog.Logger) (*app, error) {
	a := &app{assets: assets, dev: dev, trustProxy: trustProxy, log: log, limiter: newRateLimiter(5, contactWindow)}
	if err := a.parseTemplates(); err != nil {
		return nil, err
	}
	return a, nil
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

// parseTemplates builds one template set per page: the shared partials plus
// that page's own file, so each page can define its own "page" block. Mobile
// pages share templates/mobile.html, whose {{block}}s each templates/m/<slug>.html
// overrides. The About page has one desktop template for every design; on a
// phone it is the design's mobile shell with templates/m/about.html on top.
func (a *app) parseTemplates() error {
	pages := map[string]*template.Template{}
	sets := map[string][]string{
		"index": {"templates/index.html"},
		"about": {"templates/about-body.html", "templates/about.html"},
	}
	for _, v := range variants {
		sets[v.Slug] = []string{"templates/" + v.Slug + ".html"}
		sets["m/"+v.Slug] = []string{"templates/mobile.html", "templates/m/" + v.Slug + ".html"}
		sets["m/"+v.Slug+"/about"] = []string{"templates/about-body.html", "templates/mobile.html", "templates/m/" + v.Slug + ".html", "templates/m/about.html"}
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
	Variants []Variant
	Variant  Variant
	Page     string // "" for the design's homepage, "about" for its About page
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

// Sub is the path below a design's homepage ("" or "/about"), so the design
// switcher can stay on the same page.
func (d pageData) Sub() string {
	if d.Page == "" {
		return ""
	}
	return "/" + d.Page
}

// Path is this page's own URL path, without the layout override.
func (d pageData) Path() string {
	return "/v/" + d.Variant.Slug + d.Sub()
}

// Home goes in front of the homepage's anchors (#services, #contact): empty on
// the homepage itself, so they stay in-page links, and the homepage's URL on
// any other page.
func (d pageData) Home() string {
	if d.Page == "" {
		return ""
	}
	return "/v/" + d.Variant.Slug + d.ViewQuery()
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

// renderVariant serves a design's page in the layout that suits the request.
func (a *app) renderVariant(w http.ResponseWriter, r *http.Request, status int, data pageData) {
	data.Phone = isPhone(r)
	data.Mobile = wantMobile(r, data.View)
	data.Portrait = hasPortrait(a.assets)
	varyOnDevice(w)
	name := data.Variant.Slug
	switch {
	case data.Mobile && data.Page != "":
		name = "m/" + name + "/" + data.Page
	case data.Mobile:
		name = "m/" + name
	case data.Page != "":
		name = data.Page
	}
	a.render(w, status, name, data)
}

func (a *app) render(w http.ResponseWriter, status int, name string, data pageData) {
	if a.dev {
		if err := a.parseTemplates(); err != nil {
			a.log.Error("reparse", "err", err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}
	a.mu.Lock()
	t := a.pages[name]
	a.mu.Unlock()

	data.Site = site
	data.Variants = variants

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
