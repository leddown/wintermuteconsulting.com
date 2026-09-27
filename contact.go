package main

import (
	"net"
	"net/http"
	"net/mail"
	"net/url"
	"strings"
	"sync"
	"time"
	"unicode/utf8"
)

const contactWindow = 10 * time.Minute

type contactForm struct {
	Name    string
	Email   string
	Company string
	Message string
}

func (f contactForm) validate() map[string]string {
	errs := map[string]string{}
	if f.Name == "" {
		errs["name"] = "Tell us who you are."
	} else if utf8.RuneCountInString(f.Name) > 100 {
		errs["name"] = "Keep the name under 100 characters."
	}
	if a, err := mail.ParseAddress(f.Email); err != nil || a.Address != f.Email || utf8.RuneCountInString(f.Email) > 254 {
		errs["email"] = "We need a valid email address to reply."
	}
	if utf8.RuneCountInString(f.Company) > 120 {
		errs["company"] = "Keep the company name under 120 characters."
	}
	if n := utf8.RuneCountInString(f.Message); n < 10 {
		errs["message"] = "Give us a little more to go on (at least 10 characters)."
	} else if n > 5000 {
		errs["message"] = "Keep the message under 5000 characters."
	}
	return errs
}

func (a *app) handleContact(w http.ResponseWriter, r *http.Request) {
	// No cookies means no CSRF token; reject cross-site posts by Origin instead.
	if o := r.Header.Get("Origin"); o != "" {
		if u, err := url.Parse(o); err != nil || u.Host != r.Host {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
	}

	r.Body = http.MaxBytesReader(w, r.Body, 64<<10)
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	v, ok := findVariant(r.PostFormValue("variant"))
	if !ok {
		v = variants[0]
	}
	back := "/v/" + v.Slug

	// Honeypot: humans never see this field. Pretend success so bots move on.
	if r.PostFormValue("website") != "" {
		http.Redirect(w, r, back+"?sent=1#contact", http.StatusSeeOther)
		return
	}

	form := contactForm{
		Name:    strings.TrimSpace(r.PostFormValue("name")),
		Email:   strings.TrimSpace(r.PostFormValue("email")),
		Company: strings.TrimSpace(r.PostFormValue("company")),
		Message: strings.TrimSpace(r.PostFormValue("message")),
	}

	if errs := form.validate(); len(errs) > 0 {
		a.render(w, http.StatusUnprocessableEntity, v.Slug, pageData{Variant: v, Form: form, Errors: errs})
		return
	}

	if !a.limiter.allow(a.clientIP(r)) {
		a.render(w, http.StatusTooManyRequests, v.Slug, pageData{Variant: v, Form: form,
			Errors: map[string]string{"form": "Too many messages from your network. Try again in a few minutes, or email us directly."}})
		return
	}

	// TODO: deliver via a transactional email provider (Postmark/SES/etc.).
	// Until then submissions only reach the server log.
	a.log.Info("contact submission", "name", form.Name, "email", form.Email, "company", form.Company, "message", form.Message)

	http.Redirect(w, r, back+"?sent=1#contact", http.StatusSeeOther)
}

func (a *app) clientIP(r *http.Request) string {
	if a.trustProxy {
		if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
			// The proxy appends the real client address last.
			parts := strings.Split(xff, ",")
			return strings.TrimSpace(parts[len(parts)-1])
		}
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

// rateLimiter is a fixed-window, in-memory limiter keyed by client IP. It is
// enough for a single-instance site; state resets on restart.
type rateLimiter struct {
	mu     sync.Mutex
	limit  int
	window time.Duration
	hits   map[string][]time.Time
	now    func() time.Time
}

func newRateLimiter(limit int, window time.Duration) *rateLimiter {
	return &rateLimiter{limit: limit, window: window, hits: map[string][]time.Time{}, now: time.Now}
}

func (l *rateLimiter) allow(key string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := l.now()
	cutoff := now.Add(-l.window)

	// Opportunistic sweep so the map can't grow without bound.
	if len(l.hits) > 10000 {
		for k, ts := range l.hits {
			if len(ts) == 0 || ts[len(ts)-1].Before(cutoff) {
				delete(l.hits, k)
			}
		}
	}

	recent := l.hits[key][:0]
	for _, t := range l.hits[key] {
		if t.After(cutoff) {
			recent = append(recent, t)
		}
	}
	if len(recent) >= l.limit {
		l.hits[key] = recent
		return false
	}
	l.hits[key] = append(recent, now)
	return true
}
