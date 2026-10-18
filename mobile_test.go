package main

import (
	"html"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"testing"
	"unicode/utf8"
)

const (
	uaIPhone        = "Mozilla/5.0 (iPhone; CPU iPhone OS 18_5 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/18.5 Mobile/15E148 Safari/604.1"
	uaAndroidPhone  = "Mozilla/5.0 (Linux; Android 15; Pixel 9) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/140.0.0.0 Mobile Safari/537.36"
	uaAndroidTablet = "Mozilla/5.0 (Linux; Android 14; SM-X710) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/140.0.0.0 Safari/537.36"
	uaIPad          = "Mozilla/5.0 (iPad; CPU OS 17_5 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.5 Mobile/15E148 Safari/604.1"
	uaFirefoxPhone  = "Mozilla/5.0 (Android 15; Mobile; rv:140.0) Gecko/140.0 Firefox/140.0"
	uaFirefoxTablet = "Mozilla/5.0 (Android 15; Tablet; rv:140.0) Gecko/140.0 Firefox/140.0"
	uaGooglebotSP   = "Mozilla/5.0 (Linux; Android 6.0.1; Nexus 5X Build/MMB29P) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/140.0.0.0 Mobile Safari/537.36 (compatible; Googlebot/2.1; +http://www.google.com/bot.html)"
	uaDesktop       = "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/140.0.0.0 Safari/537.36"
	uaMacSafari     = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/18.5 Safari/605.1.15"
)

func TestIsPhone(t *testing.T) {
	for _, c := range []struct {
		name, ua, hint string
		want           bool
	}{
		{"iPhone Safari", uaIPhone, "", true},
		{"Android phone", uaAndroidPhone, "", true},
		{"Firefox phone", uaFirefoxPhone, "", true},
		{"Googlebot smartphone", uaGooglebotSP, "", true},
		{"iPad", uaIPad, "", false},
		{"Android tablet", uaAndroidTablet, "", false},
		{"Firefox tablet", uaFirefoxTablet, "", false},
		{"desktop Chrome", uaDesktop, "", false},
		{"desktop Safari", uaMacSafari, "", false},
		{"no UA", "", "", false},
		{"client hint says mobile", uaDesktop, "?1", true},
		{"client hint says not mobile", uaAndroidPhone, "?0", false},
		{"garbage hint falls back to UA", uaIPhone, "yes", true},
	} {
		r, _ := http.NewRequest("GET", "/", nil)
		r.Header.Set("User-Agent", c.ua)
		if c.hint != "" {
			r.Header.Set("Sec-CH-UA-Mobile", c.hint)
		}
		if got := isPhone(r); got != c.want {
			t.Errorf("%s: isPhone = %v, want %v", c.name, got, c.want)
		}
	}
}

func getAs(t *testing.T, path, ua string) (int, http.Header, string) {
	t.Helper()
	rec := do(testApp(t).routes(), "GET", path, nil, map[string]string{"User-Agent": ua})
	return rec.Code, rec.Header(), rec.Body.String()
}

func TestServesLayoutForDevice(t *testing.T) {
	const mobileBody, desktopBody = `<body class="m m--natt">`, `<body class="ps natt">`

	code, hdr, body := getAs(t, "/", uaIPhone)
	if code != 200 || !strings.Contains(body, mobileBody) {
		t.Errorf("phone did not get the mobile layout (code %d)", code)
	}
	vary := strings.Join(hdr.Values("Vary"), ",")
	if !strings.Contains(vary, "User-Agent") || !strings.Contains(vary, "Sec-CH-UA-Mobile") {
		t.Errorf("Vary = %q", vary)
	}
	if !strings.Contains(body, `href="/?view=desktop"`) {
		t.Error("mobile page has no link to the desktop layout")
	}

	_, _, body = getAs(t, "/", uaDesktop)
	if !strings.Contains(body, desktopBody) {
		t.Error("desktop did not get the desktop layout")
	}
	if strings.Contains(body, "?view=") {
		t.Error("desktop visitors should not see a layout switch")
	}

	_, _, body = getAs(t, "/?view=desktop", uaIPhone)
	if !strings.Contains(body, desktopBody) || !strings.Contains(body, `href="/?view=mobile"`) {
		t.Error("?view=desktop on a phone should serve desktop with a way back")
	}
	if !strings.Contains(body, `href="/about?view=desktop"`) {
		t.Error("forced layout not carried through the link to the About page")
	}

	_, _, body = getAs(t, "/?view=mobile", uaDesktop)
	if !strings.Contains(body, mobileBody) {
		t.Error("?view=mobile should serve the mobile layout")
	}

	_, _, body = getAs(t, "/?view=%22%3E%3Cscript%3E", uaIPhone)
	if !strings.Contains(body, mobileBody) || strings.Contains(body, "<script>") {
		t.Error("bogus view value not ignored")
	}
}

func TestMobilePagesFollowPolicy(t *testing.T) {
	h := testApp(t).routes()
	for _, p := range []string{"/", "/?sent=1"} {
		rec := do(h, "GET", p, nil, map[string]string{"User-Agent": uaIPhone})
		if rec.Code != 200 {
			t.Fatalf("%s: %d", p, rec.Code)
		}
		body := rec.Body.String()
		checkHTMLPolicy(t, "mobile "+p, body)
		for _, want := range []string{`name="viewport"`, "viewport-fit=cover", `name="theme-color"`, `id="contact"`, `id="services"`, `id="ai"`, `id="approach"`} {
			if !strings.Contains(body, want) {
				t.Errorf("mobile %s missing %s", p, want)
			}
		}
	}
}

func TestContactKeepsLayout(t *testing.T) {
	post := func(f url.Values, ua string) (int, string, string) {
		req := strings.NewReader(f.Encode())
		rec := do(testApp(t).routes(), "POST", "/contact", req, map[string]string{
			"Content-Type": "application/x-www-form-urlencoded", "User-Agent": ua,
		})
		return rec.Code, rec.Header().Get("Location"), rec.Body.String()
	}

	bad := validForm()
	bad.Set("email", "nope")
	if code, _, body := post(bad, uaIPhone); code != 422 || !strings.Contains(body, `class="m m--natt"`) {
		t.Errorf("phone error re-render should stay mobile (code %d)", code)
	}

	f := validForm()
	f.Set("view", "desktop")
	if _, loc, _ := post(f, uaIPhone); loc != "/?sent=1&view=desktop#contact" {
		t.Errorf("forced layout lost on redirect: %q", loc)
	}
	f.Set("view", "javascript:alert(1)")
	if _, loc, _ := post(f, uaIPhone); loc != "/?sent=1#contact" {
		t.Errorf("bogus view leaked into redirect: %q", loc)
	}
}

// No glitch effects: nothing that flashes, scrambles or glitches may reach any
// page in either layout, including the contact form's 422 re-render.
func TestNoGlitchAnywhere(t *testing.T) {
	h := testApp(t).routes()
	glitch := []string{"glitch.js", "data-glitch", "data-flash", "flash-layer", "is-glitching"}
	check := func(name, body string) {
		t.Helper()
		for _, g := range glitch {
			if strings.Contains(body, g) {
				t.Errorf("%s: contains %q", name, g)
			}
		}
	}
	bad := validForm()
	bad.Set("email", "nope")
	for _, ua := range []string{uaDesktop, uaIPhone} {
		for _, p := range []string{"/", "/about", "/privacy"} {
			check("GET "+p+" "+ua[13:20], do(h, "GET", p, nil, map[string]string{"User-Agent": ua}).Body.String())
		}
		rec := do(h, "POST", "/contact", strings.NewReader(bad.Encode()), map[string]string{
			"Content-Type": "application/x-www-form-urlencoded", "User-Agent": ua,
		})
		if rec.Code != http.StatusUnprocessableEntity {
			t.Fatalf("422 re-render: code %d", rec.Code)
		}
		check("422 "+ua[13:20], rec.Body.String())
	}
}

// The phone hero (Natt's night picture) is kept short on purpose: the title and one sentence
// under it, so the copy stays well below the moon and the buttons are on the
// first screen. Longer copy belongs further down the page, not in the hero.
func TestNattPhoneHeroCopyStaysShort(t *testing.T) {
	_, _, body := getAs(t, "/", uaIPhone)
	tags := regexp.MustCompile(`<[^>]+>`)
	for _, c := range []struct {
		name string
		re   *regexp.Regexp
		max  int
	}{
		{"title", regexp.MustCompile(`(?s)<h1 class="m-hero__title">(.*?)</h1>`), 60},
		{"lede", regexp.MustCompile(`(?s)<p class="m-hero__lede">(.*?)</p>`), 180},
	} {
		m := c.re.FindStringSubmatch(body)
		if m == nil {
			t.Fatalf("natt: no hero %s on the mobile page", c.name)
		}
		text := strings.Join(strings.Fields(html.UnescapeString(tags.ReplaceAllString(m[1], ""))), " ")
		if n := utf8.RuneCountInString(text); n == 0 || n > c.max {
			t.Errorf("natt: hero %s is %d characters, want 1 to %d: %q", c.name, n, c.max, text)
		}
	}
}

// The homepage's wording lives once, under "home" in content.json: the desktop
// page and the phone page must both show it, and neither may word it differently.
func TestDesktopAndPhoneShareTheCopy(t *testing.T) {
	c := testApp(t).content.Home
	for _, ua := range []string{uaDesktop, uaIPhone} {
		_, _, body := getAs(t, "/", ua)
		for _, text := range []string{c.HeroTitle, c.HeroTitleAccent, c.CTA, c.CTASecondary, c.Services.Title, c.AI.Title, c.AI.Body, c.Approach.Title, c.Contact.Title, c.Contact.Body} {
			if text == "" {
				t.Error("a piece of copy both layouts show is empty")
			} else if !strings.Contains(body, html.EscapeString(text)) {
				t.Errorf("%s: page does not show %q", ua[13:20], text)
			}
		}
	}
}
