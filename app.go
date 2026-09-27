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

var variants = []Variant{
	{"varg", "Varg", "Night forest, glowing wolf eyes that track the cursor, RGB-split glitch on the headline."},
	{"vitt", "Vitt", "Whiteout. Pale, cold and quiet, with drifting snow and inverted glitch flashes."},
	{"signal", "Signal", "An AI threat console: monospace, a live typed feed, scanlines over the forest."},
	{"morke", "Mørke", "Nordic editorial. Big serif type, a strict grid, glitch only on interaction."},
	{"spor", "Spor", "Scroll-driven descent into the forest that ends face to face with the wolf."},
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
	pages map[string]*template.Template // "index" and one entry per variant slug
}

func newApp(assets fs.FS, dev, trustProxy bool, log *slog.Logger) (*app, error) {
	a := &app{assets: assets, dev: dev, trustProxy: trustProxy, log: log, limiter: newRateLimiter(5, contactWindow)}
	if err := a.parseTemplates(); err != nil {
		return nil, err
	}
	return a, nil
}

// parseTemplates builds one template set per page: the shared partials plus
// that page's own file, so each page can define its own "page" block.
func (a *app) parseTemplates() error {
	pages := map[string]*template.Template{}
	names := []string{"index"}
	for _, v := range variants {
		names = append(names, v.Slug)
	}
	for _, name := range names {
		t, err := template.New(name).Funcs(templateFuncs).ParseFS(a.assets, "templates/partials.html", "templates/"+name+".html")
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
	buf.WriteTo(w)
}
