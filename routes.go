package main

import (
	"io/fs"
	"mime"
	"net/http"
	"strings"
)

// Go's built-in MIME table lacks video types, and a minimal VPS may have no
// /etc/mime.types. With nosniff set, a wrong type would stop eye footage playing.
func init() {
	_ = mime.AddExtensionType(".webm", "video/webm")
	_ = mime.AddExtensionType(".mp4", "video/mp4")
}

func (a *app) routes() http.Handler {
	mux := http.NewServeMux()

	static, _ := fs.Sub(a.assets, "static")
	fileServer := http.StripPrefix("/static/", http.FileServerFS(static))
	mux.Handle("GET /static/", a.cacheStatic(staticGuard(fileServer)))

	mux.HandleFunc("GET /{$}", a.handleIndex)
	mux.HandleFunc("GET /v/{slug}", a.handleVariant)
	mux.HandleFunc("GET /v/{slug}/about", a.handleAbout)
	mux.HandleFunc("POST /contact", a.handleContact)
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write([]byte("ok")) })

	return securityHeaders(mux)
}

func (a *app) handleIndex(w http.ResponseWriter, r *http.Request) {
	a.render(w, http.StatusOK, "index", pageData{})
}

func (a *app) handleVariant(w http.ResponseWriter, r *http.Request) {
	v, ok := findVariant(r.PathValue("slug"))
	if !ok {
		http.NotFound(w, r)
		return
	}
	q := r.URL.Query()
	a.renderVariant(w, r, http.StatusOK, pageData{Variant: v, Sent: q.Get("sent") == "1", View: viewOverride(q.Get("view"))})
}

func (a *app) handleAbout(w http.ResponseWriter, r *http.Request) {
	v, ok := findVariant(r.PathValue("slug"))
	if !ok {
		http.NotFound(w, r)
		return
	}
	a.renderVariant(w, r, http.StatusOK, pageData{Variant: v, Page: "about", Portrait: hasPortrait(a.assets), View: viewOverride(r.URL.Query().Get("view"))})
}

// securityHeaders sets a strict policy: every asset is same-origin, so the site
// loads nothing from third parties and needs no cookie banner. HSTS is left to
// the TLS-terminating reverse proxy.
func securityHeaders(next http.Handler) http.Handler {
	const csp = "default-src 'self'; img-src 'self' data:; style-src 'self'; script-src 'self'; " +
		"font-src 'self'; connect-src 'self'; form-action 'self'; frame-ancestors 'none'; base-uri 'none'; object-src 'none'"
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("Content-Security-Policy", csp)
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("Referrer-Policy", "strict-origin-when-cross-origin")
		h.Set("Permissions-Policy", "camera=(), microphone=(), geolocation=(), interest-cohort=()")
		h.Set("Cross-Origin-Opener-Policy", "same-origin")
		h.Set("Cross-Origin-Resource-Policy", "same-origin")
		h.Set("Cross-Origin-Embedder-Policy", "require-corp") // safe: every asset is same-origin
		h.Set("X-Frame-Options", "DENY")                      // frame-ancestors covers modern browsers; this covers old ones
		next.ServeHTTP(w, r)
	})
}

func (a *app) cacheStatic(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if a.dev {
			w.Header().Set("Cache-Control", "no-cache")
		} else {
			w.Header().Set("Cache-Control", "public, max-age=86400")
		}
		next.ServeHTTP(w, r)
	})
}

// staticGuard 404s directory listings and any path fs.FS would reject (invalid
// UTF-8, "..", empty elements), which FileServerFS would otherwise turn into a 500.
func staticGuard(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/") || !fs.ValidPath(strings.TrimPrefix(r.URL.Path, "/static/")) {
			http.NotFound(w, r)
			return
		}
		next.ServeHTTP(w, r)
	})
}
