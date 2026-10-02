package main

import (
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"testing"
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
	for _, v := range variants {
		mobileBody := `class="m m--` + v.Slug
		desktopBody := regexp.MustCompile(`<body class="([a-z]+ )*` + v.Slug + `">`)

		code, hdr, body := getAs(t, "/v/"+v.Slug, uaIPhone)
		if code != 200 || !strings.Contains(body, mobileBody) {
			t.Errorf("%s: phone did not get the mobile layout (code %d)", v.Slug, code)
		}
		vary := strings.Join(hdr.Values("Vary"), ",")
		if !strings.Contains(vary, "User-Agent") || !strings.Contains(vary, "Sec-CH-UA-Mobile") {
			t.Errorf("%s: Vary = %q", v.Slug, vary)
		}
		if !strings.Contains(body, `href="/v/`+v.Slug+`?view=desktop"`) {
			t.Errorf("%s: mobile page has no link to the desktop layout", v.Slug)
		}

		_, _, body = getAs(t, "/v/"+v.Slug, uaDesktop)
		if !desktopBody.MatchString(body) {
			t.Errorf("%s: desktop did not get the desktop layout", v.Slug)
		}
		if strings.Contains(body, "?view=") {
			t.Errorf("%s: desktop visitors should not see a layout switch", v.Slug)
		}

		_, _, body = getAs(t, "/v/"+v.Slug+"?view=desktop", uaIPhone)
		if !desktopBody.MatchString(body) || !strings.Contains(body, `?view=mobile"`) {
			t.Errorf("%s: ?view=desktop on a phone should serve desktop with a way back", v.Slug)
		}
		if !strings.Contains(body, `href="/v/varg?view=desktop"`) {
			t.Errorf("%s: forced layout not carried through the design switcher", v.Slug)
		}

		_, _, body = getAs(t, "/v/"+v.Slug+"?view=mobile", uaDesktop)
		if !strings.Contains(body, mobileBody) {
			t.Errorf("%s: ?view=mobile should serve the mobile layout", v.Slug)
		}

		_, _, body = getAs(t, "/v/"+v.Slug+"?view=%22%3E%3Cscript%3E", uaIPhone)
		if !strings.Contains(body, mobileBody) || strings.Contains(body, "<script>") {
			t.Errorf("%s: bogus view value not ignored", v.Slug)
		}
	}
}

func TestMobilePagesFollowPolicy(t *testing.T) {
	h := testApp(t).routes()
	for _, v := range variants {
		for _, p := range []string{"/v/" + v.Slug, "/v/" + v.Slug + "?sent=1"} {
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
	if code, _, body := post(bad, uaIPhone); code != 422 || !strings.Contains(body, `class="m m--signal"`) {
		t.Errorf("phone error re-render should stay mobile (code %d)", code)
	}

	f := validForm()
	f.Set("view", "desktop")
	if _, loc, _ := post(f, uaIPhone); loc != "/v/signal?sent=1&view=desktop#contact" {
		t.Errorf("forced layout lost on redirect: %q", loc)
	}
	f.Set("view", "javascript:alert(1)")
	if _, loc, _ := post(f, uaIPhone); loc != "/v/signal?sent=1#contact" {
		t.Errorf("bogus view leaked into redirect: %q", loc)
	}
}

// The glitch is gone from every design and the chooser: nothing that flashes,
// scrambles or glitches may reach any page in either layout, including the
// contact form's 422 re-render. (Wolf eyes stay: they fade in on their own timer.)
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
	check("chooser", do(h, "GET", "/", nil, nil).Body.String())
	for _, v := range variants {
		bad := validForm()
		bad.Set("variant", v.Slug)
		bad.Set("email", "nope")
		for _, ua := range []string{uaDesktop, uaIPhone} {
			check("GET "+v.Slug+" "+ua[13:20], do(h, "GET", "/v/"+v.Slug, nil, map[string]string{"User-Agent": ua}).Body.String())
			check("GET "+v.Slug+"/about "+ua[13:20], do(h, "GET", "/v/"+v.Slug+"/about", nil, map[string]string{"User-Agent": ua}).Body.String())
			rec := do(h, "POST", "/contact", strings.NewReader(bad.Encode()), map[string]string{
				"Content-Type": "application/x-www-form-urlencoded", "User-Agent": ua,
			})
			if rec.Code != http.StatusUnprocessableEntity {
				t.Fatalf("%s 422 re-render: code %d", v.Slug, rec.Code)
			}
			check("422 "+v.Slug+" "+ua[13:20], rec.Body.String())
		}
	}

	// Both Signal designs keep their eyes on the picture's forest floor, on both
	// layouts: in the bottom half and below the trunk bases, which end at about
	// 58% of its height. Signal 2's are the smaller ones.
	band := regexp.MustCompile(`data-band="([0-9.]+) ([0-9.]+)"`)
	for _, ua := range []string{uaDesktop, uaIPhone} {
		for _, slug := range []string{"signal", "signal2"} {
			body := do(h, "GET", "/v/"+slug, nil, map[string]string{"User-Agent": ua}).Body.String()
			m := band.FindStringSubmatch(body)
			if m == nil {
				t.Errorf("%s (%s): eyes have no band", slug, ua[13:20])
				continue
			}
			top, _ := strconv.ParseFloat(m[1], 64)
			bottom, _ := strconv.ParseFloat(m[2], 64)
			if top < 0.58 || bottom > 1 || top >= bottom {
				t.Errorf("%s (%s): band %s–%s is not on the forest floor", slug, ua[13:20], m[1], m[2])
			}
			if slug == "signal2" && !strings.Contains(body, `data-scale="0.`) {
				t.Errorf("signal2 (%s): eyes not scaled down", ua[13:20])
			}
		}
	}
}
