package main

import (
	"html"
	"os"
	"regexp"
	"strings"
	"testing"
	"time"
)

// The privacy policy, served in both layouts and linked from every page.

func TestPrivacyPage(t *testing.T) {
	a := testApp(t)
	h, p := a.routes(), a.content.Site.Privacy
	const path = "/privacy"
	for _, ua := range []string{uaDesktop, uaIPhone} {
		rec := do(h, "GET", path, nil, map[string]string{"User-Agent": ua})
		if rec.Code != 200 {
			t.Fatalf("GET %s = %d", path, rec.Code)
		}
		body := rec.Body.String()
		where := path + " " + ua[13:20]
		checkHTMLPolicy(t, where, body)
		want := []string{
			"<title>Privacy policy: " + a.content.Site.Name + "</title>",
			`content="` + html.EscapeString(p.Summary) + `"`,
			html.EscapeString(p.Title),
			`<time datetime="` + p.Updated + `">` + p.UpdatedText() + `</time>`,
			html.EscapeString(p.Controller.Title),
			`href="mailto:` + a.content.Site.Email + `"`,
			`href="/static/css/about.css?v=`,
			`href="/privacy" aria-current="page"`,
		}
		for _, f := range a.content.Site.About.Company {
			want = append(want, html.EscapeString(f.Value)) // the controller's registration details
		}
		for _, s := range p.Sections {
			want = append(want, html.EscapeString(s.Title))
			for _, f := range s.Facts {
				want = append(want, html.EscapeString(f.Value))
			}
			for _, pt := range s.Points {
				want = append(want, html.EscapeString(pt))
			}
		}
		for _, w := range want {
			if !strings.Contains(body, w) {
				t.Errorf("%s: missing %q", where, w)
			}
		}
		if n := strings.Count(body, `href="#`) - strings.Count(body, `href="#main"`); n != 0 {
			t.Errorf("%s: %d in-page anchors point at sections that only exist on the homepage", where, n)
		}
		if strings.Contains(body, "<form") {
			t.Errorf("%s: the policy page carries a form", where)
		}
	}
	if _, _, body := getAs(t, path+"?view=desktop", uaIPhone); !strings.Contains(body, `href="/?view=desktop#contact"`) || !strings.Contains(body, `href="/privacy?view=mobile"`) {
		t.Error("a forced layout does not follow links off the policy page")
	}
	if got := (Privacy{Updated: "2026-10-18"}).UpdatedText(); got != "18 October 2026" {
		t.Errorf("UpdatedText = %q", got)
	}
}

// Visitors must be able to find the policy from every page, and from the form
// before they send it (GDPR Art. 13: informed when the data is collected).
func TestPrivacyPolicyIsLinkedEverywhere(t *testing.T) {
	for _, path := range []string{"/", "/about", "/privacy"} {
		for _, ua := range []string{uaDesktop, uaIPhone} {
			_, _, body := getAs(t, path, ua)
			if !strings.Contains(body, `href="/privacy"`) {
				t.Errorf("%s (%s): no link to the privacy policy", path, ua[13:20])
			}
		}
	}
	for _, ua := range []string{uaDesktop, uaIPhone} {
		_, _, body := getAs(t, "/", ua)
		note := regexp.MustCompile(`<p class="form-note">[^<]*<a href="/privacy">`)
		if !note.MatchString(body) {
			t.Errorf("/ (%s): the contact form does not link to the policy", ua[13:20])
		}
	}
}

// The policy makes promises about what the code and the server do. These pin
// each one to the thing that keeps it, so a change to either shows up here.
func TestPrivacyPolicyMatchesTheSite(t *testing.T) {
	policy := strings.ToLower(privacyText(testApp(t).content.Site.Privacy))

	// "30 days, then deleted" (the access log) and "after 30 days at most" (a
	// failed enquiry in /var/log/daemon): production's newsyslog.conf keeps
	// them (TestProductionKeepsThePrivacyPromises).
	for _, promise := range []string{"30 days, then deleted", "after 30 days at most"} {
		if !strings.Contains(policy, promise) {
			t.Errorf("the policy no longer says %q: check it against webmaster's server/newsyslog.conf", promise)
		}
	}
	// "in memory for ten minutes": the contact form's rate-limit window.
	if contactWindow != 10*time.Minute || !strings.Contains(policy, "ten minutes") {
		t.Errorf("the policy says the form holds IP addresses for ten minutes; contactWindow is %s", contactWindow)
	}
	// The access log fields it lists, and only those: no request bodies.
	for _, field := range []string{"ip address", "date and time", "page or file", "status and size", "linked to ours", "user-agent"} {
		if !strings.Contains(policy, field) {
			t.Errorf("the policy no longer lists %q among the access log's fields", field)
		}
	}
	// "no cookies, nothing stored on your device".
	for _, f := range []string{"static/js/reveal.js", "static/js/mnav.js", "static/js/contact.js"} {
		b, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		for _, store := range []string{"cookie", "localStorage", "sessionStorage", "indexedDB"} {
			if strings.Contains(string(b), store) {
				t.Errorf("%s uses %s, but the policy says nothing is stored on the visitor's device", f, store)
			}
		}
	}
}

func privacyText(p Privacy) string {
	parts := []string{p.Lede}
	for _, s := range append([]PrivacySection{p.Controller}, p.Sections...) {
		parts = append(parts, s.Body...)
		parts = append(parts, s.Points...)
		for _, f := range s.Facts {
			parts = append(parts, f.Value)
		}
	}
	return strings.Join(parts, "\n")
}
