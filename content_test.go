package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"strings"
	"testing"
	"testing/fstest"
)

// content.json is edited by hand. These tests stand between a slip there and
// a broken site: the file must be valid JSON that the site accepts, and the
// loader must refuse the usual mistakes and say where they are. build.sh
// runs them before it builds, so a bad file never becomes a binary.

func contentBytes(t *testing.T) []byte {
	t.Helper()
	b, err := fs.ReadFile(embedded, contentFile)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func TestContentFileIsValid(t *testing.T) {
	b := contentBytes(t)
	if !json.Valid(b) {
		var v any
		t.Fatalf("%s is not valid JSON: %s", contentFile, explain(b, json.Unmarshal(b, &v)))
	}
	c, err := parseContent(b)
	if err != nil {
		t.Fatal(err)
	}
	// What the pages are built from is this file and nothing else.
	if got := testApp(t).content; got.Site.Name != c.Site.Name || got.Home.HeroTitle != c.Home.HeroTitle {
		t.Error("the app did not load the embedded content.json")
	}
}

func TestContentRejectsMistakes(t *testing.T) {
	good := contentBytes(t)
	swap := func(old, new string) func(*testing.T) []byte {
		return func(t *testing.T) []byte {
			if !bytes.Contains(good, []byte(old)) {
				t.Fatalf("content.json no longer contains %s: update this test", old)
			}
			return bytes.Replace(good, []byte(old), []byte(new), 1)
		}
	}
	change := func(f func(*Content)) func(*testing.T) []byte {
		return func(t *testing.T) []byte {
			var c Content
			if err := json.Unmarshal(good, &c); err != nil {
				t.Fatal(err)
			}
			f(&c)
			b, err := json.Marshal(c)
			if err != nil {
				t.Fatal(err)
			}
			return b
		}
	}
	lineOf := func(s string) string {
		return fmt.Sprintf("line %d,", 1+bytes.Count(good[:bytes.Index(good, []byte(s))], []byte("\n")))
	}
	for _, tc := range []struct {
		name  string
		input func(*testing.T) []byte
		want  []string // each must appear in the error
	}{
		{"comma after the last item", swap("\n  }\n}", ",\n  }\n}"), []string{"line ", "comma after the last item"}},
		{"missing comma", swap(`"logoMark": "Wintermute",`, `"logoMark": "Wintermute"`), []string{"line ", "comma missing"}},
		{"unescaped quote in a text", swap(`"heroTitle": "Crises come at night."`, `"heroTitle": "Crises "come" at night."`), []string{lineOf(`"heroTitle": "Crises come at night."`), "backslash"}},
		{"text broken over two lines", swap(`"heroTitle": "Crises come at night."`, "\"heroTitle\": \"Crises come\nat night.\""), []string{"line ", "one line"}},
		{"misspelled name", swap(`"heroTitle": "Crises come at night."`, `"heroTitel": "Crises come at night."`), []string{lineOf(`"heroTitle": "Crises come at night."`), `"heroTitel" is not a name the site knows`}},
		{"number in quotes", swap(`"value": 15`, `"value": "15"`), []string{lineOf(`"value": 15`), "whole number without quotes"}},
		{"text after the end", swap("\n  }\n}", "\n  }\n}\n}"), []string{"after the closing brace"}},
		{"file cut short", func(*testing.T) []byte { return good[:len(good)/2] }, []string{"ends early"}},
		{"empty file", func(*testing.T) []byte { return nil }, []string{"the file is empty"}},
		{"homepage wording left out", change(func(c *Content) { c.Home = Copy{} }), []string{"missing or empty", "home.heroTitle", "home.contact.body"}},
		{"a file from before there was one design", swap(`"home": {`, `"designs": {`), []string{lineOf(`"home": {`), `"designs" is not a name the site knows`}},
		{"shared text emptied", change(func(c *Content) { c.Site.Brief = " " }), []string{"missing or empty", "site.brief"}},
		{"homepage text emptied", change(func(c *Content) { c.Home.Contact.Title = "" }), []string{"home.contact.title"}},
		{"service code too short for an icon", change(func(c *Content) { c.Site.Services[0].Code = "A" }), []string{"site.services[1].code"}},
		{"policy date written another way", swap(`"updated": "2026-10-18"`, `"updated": "18.10.2026"`), []string{"site.privacy.updated", "2026-10-18"}},
		{"policy section left empty", change(func(c *Content) { c.Site.Privacy.Sections[1] = PrivacySection{Title: "Empty"} }), []string{"site.privacy.sections[2].body"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := parseContent(tc.input(t))
			if err == nil {
				t.Fatal("accepted")
			}
			for _, want := range tc.want {
				if !strings.Contains(err.Error(), want) {
					t.Errorf("error %q does not say %q", err, want)
				}
			}
			if !strings.HasPrefix(err.Error(), contentFile+": ") {
				t.Errorf("error %q does not name the file", err)
			}
		})
	}
}

// In -dev the file is read again for every page: an edit shows on a refresh,
// and a broken file shows its error instead of a page.
func TestDevRereadsContent(t *testing.T) {
	good := contentBytes(t)
	file := fstest.MapFS{contentFile: &fstest.MapFile{Data: good}}
	a, err := newApp(overlayFS{embedded, file}, true, false, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatal(err)
	}
	h := a.routes()
	get := func() (int, string) {
		rec := do(h, "GET", "/", nil, map[string]string{"User-Agent": uaDesktop})
		return rec.Code, rec.Body.String()
	}
	if code, body := get(); code != 200 || !strings.Contains(body, "Crises come at night.") {
		t.Fatalf("first load: %d", code)
	}
	file[contentFile] = &fstest.MapFile{Data: bytes.Replace(good, []byte("Crises come at night."), []byte("Crises come at noon."), 1)}
	if code, body := get(); code != 200 || !strings.Contains(body, "Crises come at noon.") || strings.Contains(body, "Crises come at night.") {
		t.Errorf("edited text not shown on the next request (code %d)", code)
	}
	file[contentFile] = &fstest.MapFile{Data: good[:len(good)/2]}
	if code, body := get(); code != 500 || !strings.Contains(body, contentFile) {
		t.Errorf("broken file: code %d, body %q", code, body)
	}
}

// Outside -dev a broken file stops the program at start-up, before it serves anything.
func TestBrokenContentStopsStartup(t *testing.T) {
	file := fstest.MapFS{contentFile: &fstest.MapFile{Data: []byte(`{"site": {}}`)}}
	if _, err := newApp(overlayFS{embedded, file}, false, false, slog.New(slog.NewTextHandler(io.Discard, nil))); err == nil {
		t.Error("newApp accepted an empty content.json")
	}
}
