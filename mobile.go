package main

import (
	"net/http"
	"regexp"
)

// Phones get a dedicated mobile layout at the same URL ("dynamic serving").
// Detection prefers the Sec-CH-UA-Mobile client hint, which Chromium browsers
// send by default, and falls back to the User-Agent for Safari and Firefox.
// Tablets get the desktop layout, which is responsive down to phone width, so
// a wrong guess degrades to a still-usable page rather than a broken one.

var (
	phoneUA  = regexp.MustCompile(`(?i)iphone|ipod|android.*mobile|windows phone|iemobile|blackberry|bb10|opera mini|mobile safari|\bmobile\b`)
	tabletUA = regexp.MustCompile(`(?i)ipad|tablet|kindle|silk/|playbook`)
)

func isPhone(r *http.Request) bool {
	switch r.Header.Get("Sec-CH-UA-Mobile") {
	case "?1":
		return true
	case "?0":
		return false
	}
	ua := r.Header.Get("User-Agent")
	return phoneUA.MatchString(ua) && !tabletUA.MatchString(ua)
}

// viewOverride reads ?view=mobile|desktop (or the form's hidden "view" field),
// letting a visitor or tester pick a layout without a cookie.
func viewOverride(v string) string {
	if v == "mobile" || v == "desktop" {
		return v
	}
	return ""
}

// wantMobile decides which layout to serve: an explicit override wins,
// otherwise the device decides.
func wantMobile(r *http.Request, view string) bool {
	switch view {
	case "mobile":
		return true
	case "desktop":
		return false
	}
	return isPhone(r)
}

// varyOnDevice tells caches (and search engines) that the HTML depends on the device.
func varyOnDevice(w http.ResponseWriter) {
	w.Header().Add("Vary", "User-Agent")
	w.Header().Add("Vary", "Sec-CH-UA-Mobile")
}
