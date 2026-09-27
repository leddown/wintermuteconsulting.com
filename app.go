package main

import (
	"bytes"
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
var variantTheme = map[string]string{"signal2": "signal", "varg2": "varg", "spor2": "spor", "natt2": "natt"}

// Theme is the design whose stylesheet and tokens this variant uses.
func (v Variant) Theme() string {
	if t, ok := variantTheme[v.Slug]; ok {
		return t
	}
	return v.Slug
}

var variants = []Variant{
	{"varg", "Varg", "Night forest, amber on near-black, a status light that keeps watch."},
	{"varg2", "Varg 2", "A copy of Varg to develop separately."},
	{"signal", "Signal", "A supervision console: monospace, a live remediation tracker, scanlines over the forest."},
	{"signal2", "Signal 2", "A copy of Signal to develop separately."},
	{"spor", "Spor", "Scroll-driven descent into the forest that ends face to face with the wolf."},
	{"spor2", "Spor 2", "A copy of Spor to develop separately."},
	{"natt", "Natt", "The dark advisory site: the illustrated night as the page, amber as the single signal colour."},
	{"natt2", "Natt 2", "A copy of Natt to develop separately."},
}

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

	mu    sync.Mutex
	pages map[string]*template.Template // "index", each variant slug, and "m/<slug>" for its mobile layout
}

func newApp(assets fs.FS, dev, trustProxy bool, log *slog.Logger) (*app, error) {
	a := &app{assets: assets, dev: dev, trustProxy: trustProxy, log: log, limiter: newRateLimiter(5, contactWindow)}
	if err := a.parseTemplates(); err != nil {
		return nil, err
	}
	return a, nil
}

// parseTemplates builds one template set per page: the shared partials plus
// that page's own file, so each page can define its own "page" block. Mobile
// pages share templates/mobile.html, whose {{block}}s each templates/m/<slug>.html
// overrides.
func (a *app) parseTemplates() error {
	pages := map[string]*template.Template{}
	sets := map[string][]string{"index": {"templates/index.html"}}
	for _, v := range variants {
		sets[v.Slug] = []string{"templates/" + v.Slug + ".html"}
		sets["m/"+v.Slug] = []string{"templates/mobile.html", "templates/m/" + v.Slug + ".html"}
	}
	for name, files := range sets {
		t, err := template.New(name).Funcs(templateFuncs).ParseFS(a.assets, append([]string{"templates/partials.html", "templates/ps.html"}, files...)...)
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
	Form     contactForm
	Errors   map[string]string
	Sent     bool

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

// renderVariant serves a design in the layout that suits the request.
func (a *app) renderVariant(w http.ResponseWriter, r *http.Request, status int, data pageData) {
	data.Phone = isPhone(r)
	data.Mobile = wantMobile(r, data.View)
	varyOnDevice(w)
	name := data.Variant.Slug
	if data.Mobile {
		name = "m/" + name
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
	w.WriteHeader(status)
	_, _ = buf.WriteTo(w) // client gone mid-response; nothing useful to do
}
