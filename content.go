package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"strings"
	"time"
)

// The site's text is data, not code: it lives in content.json, which is
// embedded in the binary and read at start-up (in -dev, on every request, so
// an edit shows on a refresh). The types below are that file's shape. Edit the
// text there, not here and not in the templates.

type Service struct {
	Code  string `json:"code"` // short tag shown in mono type, e.g. "RT-01"; its two-letter prefix picks the icon in templates/ps.html ("svc-icon")
	Title string `json:"title"`
	Body  string `json:"body"`
}

type Principle struct {
	Title string `json:"title"`
	Body  string `json:"body"`
}

type Industry struct {
	Name string `json:"name"`
	Body string `json:"body"`
}

// Insight is a short point-of-view piece. Teasers only for now: there are no
// article pages yet, so templates link them to the contact form.
type Insight struct {
	Tag     string `json:"tag"`
	Title   string `json:"title"`
	Summary string `json:"summary"`
	Minutes int    `json:"minutes"`
}

type Stat struct {
	Value int    `json:"value"`
	Unit  string `json:"unit"`
	Label string `json:"label"`
}

// Fact is one labelled line in a definition list: the principal's background,
// the company's registration details.
type Fact struct {
	Label string `json:"label"`
	Value string `json:"value"`
}

// Principal is the person presented on the About page, beside their portrait.
type Principal struct {
	Name  string   `json:"name"`
	Role  string   `json:"role"`
	Bio   []string `json:"bio"` // paragraphs
	Facts []Fact   `json:"facts"`
}

// About is the copy for the About page (/about). Anything in
// [square brackets] is a placeholder for the owner's own details.
type About struct {
	Title     string    `json:"title"` // the page's headline
	Lede      string    `json:"lede"`
	Summary   string    `json:"summary"` // meta description: keep it under 160 characters
	Principal Principal `json:"principal"`
	NameNote  string    `json:"nameNote"` // why the firm is called Wintermute
	Company   []Fact    `json:"company"`  // registration details; the legal name and email come from Site
}

// Privacy is the privacy policy (/privacy). Its first section, Controller, is
// shown with the company's details (the legal name, Site.About.Company and
// Site.Email). Text in [square brackets] is a placeholder for a detail only the
// owner can supply.
type Privacy struct {
	Kicker     string           `json:"kicker"` // small label above the title
	Title      string           `json:"title"`
	Lede       string           `json:"lede"`
	Summary    string           `json:"summary"` // meta description: keep it under 160 characters
	Updated    string           `json:"updated"` // date of the last change of substance, as 2026-10-18
	Controller PrivacySection   `json:"controller"`
	Sections   []PrivacySection `json:"sections"`
}

// PrivacySection is one heading of the privacy policy with its paragraphs,
// then a list, then labelled facts (what, why, legal basis, how long), each
// left out where empty.
type PrivacySection struct {
	Title  string   `json:"title"`
	Body   []string `json:"body"`
	Points []string `json:"points"`
	Facts  []Fact   `json:"facts"`
}

// UpdatedText is Updated as the page shows it: "18 October 2026".
func (p Privacy) UpdatedText() string {
	d, err := time.Parse(time.DateOnly, p.Updated)
	if err != nil {
		return p.Updated
	}
	return d.Format("2 January 2006")
}

// Site is what the firm says about itself, wherever a page needs it.
type Site struct {
	Name       string      `json:"name"`     // full legal name: titles, meta, footers
	LogoMark   string      `json:"logoMark"` // logo lockup: the word mark...
	LogoSub    string      `json:"logoSub"`  // ...and the lighter qualifier beside or under it
	Descriptor string      `json:"descriptor"`
	Brief      string      `json:"brief"` // the offer in one sentence, for heroes that must fit a laptop screen
	Lede       string      `json:"lede"`
	Email      string      `json:"email"`
	Location   string      `json:"location"`
	Services   []Service   `json:"services"`
	Principles []Principle `json:"principles"`
	Stats      []Stat      `json:"stats"` // placeholder figures until real ones are supplied
	Industries []Industry  `json:"industries"`
	Insights   []Insight   `json:"insights"`
	About      About       `json:"about"`
	Privacy    Privacy     `json:"privacy"`
}

// Heading is one section's heading on the homepage.
type Heading struct {
	Kicker string   `json:"kicker"` // small label above the title; empty for none
	Title  string   `json:"title"`
	Accent string   `json:"accent"` // rest of the title, set in the accent colour; empty for none
	Body   string   `json:"body"`   // the paragraph that goes with it; empty for none
	Points []string `json:"points"` // short list under the paragraph; empty for none
}

// Copy is the homepage's own wording: the hero title, the buttons and each
// section's heading. The desktop page and the phone page both read it (as
// .Copy), so a change is made once. A layout that has no room for a piece
// leaves it out; it never words it differently:
//   - the phone page has no Insights or Industries sections, no section
//     paragraphs and no contact points;
//   - the phone hero shows Site.Brief under the title, the desktop hero
//     Site.Lede (TestNattPhoneHeroCopyStaysShort caps the phone hero).
type Copy struct {
	HeroTitle       string `json:"heroTitle"`
	HeroTitleAccent string `json:"heroTitleAccent"` // second line of the title, in the accent colour; empty for none
	CTA             string `json:"cta"`             // the main button
	CTASecondary    string `json:"ctaSecondary"`    // the quieter button beside it

	AI         Heading `json:"ai"`
	Services   Heading `json:"services"`
	Insights   Heading `json:"insights"`
	Industries Heading `json:"industries"`
	Approach   Heading `json:"approach"`
	Contact    Heading `json:"contact"`
}

// Content is everything in content.json.
type Content struct {
	Readme []string `json:"_readme"` // notes for whoever edits the file; shown nowhere
	Site   Site     `json:"site"`
	Home   Copy     `json:"home"`
}

const contentFile = "content.json"

func loadContent(assets fs.FS) (*Content, error) {
	b, err := fs.ReadFile(assets, contentFile)
	if err != nil {
		return nil, err
	}
	return parseContent(b)
}

// parseContent reads content.json strictly, because it is edited by hand: a
// misspelled name, a missing text or a stray comma is an error that says
// where, never a page that quietly shows the wrong thing.
func parseContent(b []byte) (*Content, error) {
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.DisallowUnknownFields()
	var c Content
	if err := dec.Decode(&c); err != nil {
		return nil, fmt.Errorf("%s: %s", contentFile, explain(b, err))
	}
	if _, err := dec.Token(); err != io.EOF {
		return nil, fmt.Errorf("%s: %s: there is text after the closing brace", contentFile, position(b, dec.InputOffset()))
	}
	if err := c.validate(); err != nil {
		return nil, fmt.Errorf("%s: %w", contentFile, err)
	}
	return &c, nil
}

// explain words a decoding error for someone editing the file by hand: where
// it is and, for the usual slips, what to look for.
func explain(b []byte, err error) string {
	msg := strings.TrimPrefix(err.Error(), "json: ")
	var syn *json.SyntaxError
	var typ *json.UnmarshalTypeError
	switch {
	case errors.As(err, &syn):
		switch {
		case strings.Contains(msg, "looking for beginning of object key string"), strings.Contains(msg, "looking for beginning of value"):
			msg += " (is there a comma after the last item of a block or list?)"
		case strings.Contains(msg, "after object key:value pair"), strings.Contains(msg, "after array element"):
			msg += " (is a comma missing at the end of the line above, or a quote inside a text missing its backslash?)"
		case strings.Contains(msg, "in string literal"):
			msg += " (a text must stay on one line)"
		}
		return position(b, syn.Offset) + ": " + msg
	case errors.As(err, &typ):
		want := "text"
		switch typ.Type.Kind().String() {
		case "int":
			want = "a whole number without quotes"
		case "slice":
			want = "a list in [ ]"
		case "struct", "map":
			want = "a block in { }"
		}
		return fmt.Sprintf("%s: %s must be %s, not a %s", position(b, typ.Offset), typ.Field, want, typ.Value)
	case errors.Is(err, io.EOF):
		return "the file is empty"
	case errors.Is(err, io.ErrUnexpectedEOF):
		return position(b, int64(len(b))) + ": the file ends early (is a closing brace, bracket or quote missing?)"
	}
	if name, ok := strings.CutPrefix(msg, `unknown field "`); ok {
		name = strings.TrimSuffix(name, `"`)
		msg = fmt.Sprintf("%q is not a name the site knows (misspelled, or in the wrong block?)", name)
		if i := bytes.Index(b, []byte(`"`+name+`"`)); i >= 0 {
			return position(b, int64(i)+1) + ": " + msg
		}
	}
	return msg
}

// position turns a byte offset into "line 12, column 5" for an error message.
func position(b []byte, offset int64) string {
	if offset > int64(len(b)) {
		offset = int64(len(b))
	}
	before := b[:offset]
	line := 1 + bytes.Count(before, []byte("\n"))
	col := len(before) - bytes.LastIndexByte(before, '\n')
	return fmt.Sprintf("line %d, column %d", line, col)
}

// validate checks that every text the pages rely on is there.
func (c *Content) validate() error {
	var missing []string
	need := func(path, value string) {
		if strings.TrimSpace(value) == "" {
			missing = append(missing, path)
		}
	}
	s := c.Site
	need("site.name", s.Name)
	need("site.logoMark", s.LogoMark)
	need("site.descriptor", s.Descriptor)
	need("site.brief", s.Brief)
	need("site.lede", s.Lede)
	need("site.email", s.Email)
	need("site.location", s.Location)
	need("site.about.title", s.About.Title)
	need("site.about.lede", s.About.Lede)
	need("site.about.summary", s.About.Summary)
	need("site.about.principal.name", s.About.Principal.Name)
	need("site.privacy.title", s.Privacy.Title)
	need("site.privacy.lede", s.Privacy.Lede)
	need("site.privacy.summary", s.Privacy.Summary)
	need("site.privacy.controller.title", s.Privacy.Controller.Title)
	if len(s.Privacy.Sections) == 0 {
		missing = append(missing, "site.privacy.sections")
	}
	for i, v := range s.Privacy.Sections {
		at := fmt.Sprintf("site.privacy.sections[%d]", i+1)
		need(at+".title", v.Title)
		if len(v.Body)+len(v.Points)+len(v.Facts) == 0 {
			missing = append(missing, at+".body")
		}
	}
	if _, err := time.Parse(time.DateOnly, s.Privacy.Updated); err != nil {
		return fmt.Errorf("site.privacy.updated %q is not a date written as 2026-10-18", s.Privacy.Updated)
	}
	if len(s.Services) == 0 {
		missing = append(missing, "site.services")
	}
	if len(s.Principles) == 0 {
		missing = append(missing, "site.principles")
	}
	for i, v := range s.Services {
		at := fmt.Sprintf("site.services[%d]", i+1)
		need(at+".title", v.Title)
		need(at+".body", v.Body)
		if len(v.Code) < 2 { // the icon is picked by its first two letters
			return fmt.Errorf("%s.code %q is too short: it starts with the two letters that pick the icon", at, v.Code)
		}
	}
	for i, v := range s.Principles {
		at := fmt.Sprintf("site.principles[%d]", i+1)
		need(at+".title", v.Title)
		need(at+".body", v.Body)
	}

	h := c.Home
	need("home.heroTitle", h.HeroTitle)
	need("home.cta", h.CTA)
	need("home.ctaSecondary", h.CTASecondary)
	need("home.ai.title", h.AI.Title)
	need("home.ai.body", h.AI.Body)
	need("home.services.title", h.Services.Title)
	need("home.insights.title", h.Insights.Title)
	need("home.industries.title", h.Industries.Title)
	need("home.approach.title", h.Approach.Title)
	need("home.contact.title", h.Contact.Title)
	need("home.contact.body", h.Contact.Body)
	if len(missing) > 0 {
		return fmt.Errorf("missing or empty: %s", strings.Join(missing, ", "))
	}
	return nil
}
