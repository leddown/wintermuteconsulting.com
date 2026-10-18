# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Role

Act as a website creator and web UI expert with deep knowledge of modern web design. Interpret design language ("modern website", "professional design", "clean", "cluttered", etc.) the way current industry practice and research understand it.

## Project

A public-facing company website, served by one Go program.

- **This repo is the code and the binary build, nothing else.** Everything on a server (deploys, rc.d, relayd, httpd, pf, syslog, certificates, the LAN dev box, scans of live hosts) belongs to **webmaster** (`~/code/webmaster`, its own repo and CLAUDE.md). Don't add deploy or server scripts here.
- **Backend scope:** minimal: the pages and a contact form. No database, shopping cart or e-commerce of any kind; don't add dependencies or infrastructure for these.
- **Hosting:** self-hosted. Production is OpenBSD (base system only; it never compiles: webmaster pushes the binary). A Linux LAN dev box (~1 GB RAM) builds from a git checkout, driven by webmaster's `devbox/update`.

## Standing obligation: explain hosting and compliance

Make sure the user understands the operational and legal requirements that come with self-hosting a public site, and raise them when they're relevant instead of assuming. For example:

- TLS certificates, reverse proxy and firewall setup, running the service, and updates (webmaster does these; say when a site change needs one)
- Cookie and consent banners: whether one is needed depends on what the site sets (analytics, embeds, sessions)
- Privacy policy, and how contact-form submissions are handled (spam protection, where the data goes)

## Commands

```sh
go run . -dev                 # http://127.0.0.1:8080; templates, content.json and static read from disk and re-read per request
go test ./...                 # all tests
go test -run TestContact ./...  # single test
./build.sh linux|openbsd [amd64|arm64]   # tests, then a static binary in dist/ (templates + assets embedded)
go run . -dev -mail-config mail.conf -mail-test   # send one test enquiry with those SMTP settings and exit (keep mail.conf out of git)
scripts/portrait.sh photo.jpg           # the About page's photo -> static/images/opt/portrait-{480,960}.webp (FOCUS=0..1 moves the crop)
security/run.sh code                    # gofmt, vet, staticcheck, gosec, govulncheck (source + built binaries), go test -race, fuzzing
FUZZTIME=30m security/run.sh code       # before a release
security/run.sh local                   # start the binary on loopback; curl probes + ZAP full scan, nuclei, nikto
```

**What webmaster relies on here** (keep these as they are, or change webmaster in the same piece of work):

- `build.sh openbsd ARCH`, which runs the tests and writes `dist/wintermuteconsulting-openbsd-ARCH`.
- The binary's flags (below) and `GET /healthz`.
- The privacy policy's words "30 days, then deleted" and "after 30 days at most" in `content.json`: webmaster's `test/site-config` checks them against its newsyslog retention before every push.
- `security/zap-rules.tsv` and `security/nikto-ignore.txt`, the reviewed scanner false positives (each needs a reason; don't add entries just to make a run green). webmaster's `test/security` uses them for the production scans.

Flags/env: `-addr` / `ADDR` (default `127.0.0.1:8080`), `-dev` / `DEV=1`, `-trust-proxy` / `TRUST_PROXY=1` (use the last `X-Forwarded-For` hop as the client IP; only behind a proxy that appends it), `-access-log` / `ACCESS_LOG` (`syslog`, `stderr` or empty for none), `-mail-config` / `MAIL_CONFIG` (see `mail.conf.example`), `-mail-test`.

Testing: `security_test.go` pins the security properties (headers, no inline/third-party code, escaping, traversal, Origin check, log injection, rate-limit spoofing, server timeouts). `fuzz_test.go` holds the fuzz targets; crashers saved in `testdata/fuzz/` run as regression tests on every `go test`. Scanner versions are pinned in `security/run.sh`; `go.mod` pins the Go toolchain: keep it on a supported, patched release (govulncheck in binary mode fails otherwise).

## Architecture

- Standard library only (`net/http`, `html/template`, `embed`); no third-party Go modules. Keep it that way unless there's a strong reason.
- **All site copy is in `content.json`** (embedded), so text is edited without touching code. `site` is what the firm says about itself (services, principles, stats, industries, insights, brief, lede, the About page, the privacy policy); `home` is the homepage's own wording, read as `.Copy` by both the desktop and phone templates so they never word a thing differently (`TestDesktopAndPhoneShareTheCopy`). The phone page leaves pieces out (no Insights, Industries or contact points; its hero uses `site.brief` where the desktop uses `site.lede`). `_readme` at the top holds notes for whoever edits the file: keep it current. `content.go` holds the types and a strict loader: unknown or misspelled names, syntax slips and empty required texts are errors that name the line. A bad file stops the program at start-up; in `-dev` its error is shown instead of the page. A new field needs a struct field with a `json` tag, and a line in `validate` if pages rely on it. Templates hold no copy beyond navigation and form labels.
- **Positioning:** boutique AI, cyber & ICT risk advisory for regulated finance across the EU, AI first. Never "Nordic" or "counsel" in site copy; no pentest/red-team language; big-firm vocabulary, boutique delivery. See STYLEGUIDE.md.
- **One design, Natt:** an advisory-firm layout (utility bar, nav, service tiles, figures, industries, insights, steps, column footer) on the brickbard night illustration, with amber as the single signal colour. Files: `templates/home.html`, `templates/ps.html` (the furniture), `templates/partials.html` (head, logo, contact form, portrait, layout switch); `static/css/base.css` (fonts, reset, tokens, buttons, form, portrait), `natt.css` (tokens, desktop hero), `ps.css`, `mobile.css`, `about.css`. On a phone the picture stays behind the copy, placed in `cqi` so the moon sits high and the white title never crosses it; the phone hero stays short (`TestNattPhoneHeroCopyStaysShort`). **Removed on request, do not bring back:** the design chooser and `/v/{slug}` URLs (404, `TestNotFound`), the other designs (Signal, Grense, Varg, Snø, Vakt, Spor and earlier Natts; in git history), wolf eyes, and any glitch effects (`TestNoGlitchAnywhere`).
- **About page** (`/about`): `templates/about-body.html` (shared content), `templates/about.html` (desktop), `templates/m/about.html` (phone). Copy is `site.about`; anything in `[square brackets]` is a placeholder for the owner's biography and company details: never invent those. The portrait appears once both files from `scripts/portrait.sh` exist (`hasPortrait`); until then a placeholder is drawn. No contact form there: its buttons go to the homepage's `#contact`.
- **Privacy policy** (`/privacy`): `templates/privacy-body.html`, `templates/privacy.html`, `templates/m/privacy.html`, styled with the About page's sections. Copy is `site.privacy`; `updated` is an ISO date. Linked from both footers and the contact form (`TestPrivacyPolicyIsLinkedEverywhere`). **It must stay true to what the code and servers do:** `TestPrivacyPolicyMatchesTheSite` pins the "ten minutes" to `contactWindow`, the listed access-log fields, and "nothing stored on your device" to the JS; webmaster pins the 30-day retention. Any change to logging, retention, cookies or storage, or a new third-party service, means updating the policy and its `updated` date in the same change.
- **Links between pages:** header and footer are partials (`ps-util`, `ps-nav`, `ps-footer`; on a phone, the bar and menu in `mobile.html`). Section links are `{{.Home}}#services`: `.Home` is empty on the homepage and `/` elsewhere. `pageData.Page` is `""`, `"about"` or `"privacy"`.
- **Company name:** the legal name "Wintermute Consulting OÜ" appears on every page via `Site.Name`; the logo lockup is "Wintermute Consulting" without "OÜ".
- **Phones get a dedicated layout at the same URL** (`mobile.go`): `Sec-CH-UA-Mobile` first, User-Agent fallback, tablets get desktop; `Vary: User-Agent, Sec-CH-UA-Mobile`. `?view=mobile|desktop` overrides (no cookie; carried through links, the form's hidden `view` field and the redirect). `templates/mobile.html` is the shell with the phone homepage; its blocks (`m-main`, `m-title`, `m-styles`) are overridden by `templates/m/*.html`. Mobile rules: 16px+ inputs, 44px+ tap targets, safe-area insets, single column. Desktop templates must stay responsive anyway (tablets, wrong guesses).
- **Templates:** each page and layout is its own set (`parseTemplates` in `app.go`: `home`, `about`, `privacy`, `m/home`, `m/about`, `m/privacy`) plus `partials.html` and `ps.html`. Every page file defines `"page"`.
- **Contact form:** `POST /contact` → honeypot (`website`), Origin check (no cookies, so no CSRF token), validation, in-memory per-IP rate limit, email, then a 303 to `/?sent=1#contact`; errors re-render the homepage with 422. `contact.js` sends it with `fetch` and swaps the answer in place. Without JS the redirect lands on `#contact` with smooth scrolling off (`html:has(.form-status)`). **Delivery** is SMTP (`mail.go`: implicit TLS on 465, STARTTLS required elsewhere off loopback; errors name the failed step), configured by `-mail-config` (strict like `content.json`; Google Workspace on smtp.gmail.com:465 with an app password: 587 timed out from the production host). Reply-To is the visitor; form values in headers are MIME-encoded (`TestContactMailHeaderInjection`). If sending fails the visitor gets a 503 with their text kept and the enquiry is logged in full; delivered enquiries are not logged; without a config enquiries are only logged. The server-side `mail.conf` is created by hand, never written by any script.
- **Static URLs are versioned:** link stylesheets, scripts and replaceable images with `{{static "js/reveal.js"}}`, never a literal `/static/...`: it appends `?v=<content hash>`. Versioned URLs are cached `immutable` for a year, other static files a day, pages `no-cache` (`cacheStatic`, `staticURL`; `TestStaticURLsCarryTheContentVersion`). In `-dev` URLs are bare and nothing is cached.
- **Access log** (`accesslog.go`): one Combined Log Format line per request plus the time taken, readable by GoAccess/GoatCounter. Quoted fields are escaped like Apache's and capped, so a line is always one line of printable ASCII (`TestAccessLogInjection`, `FuzzAppendQuoted`). Request bodies are never logged and `/healthz` is skipped. Client IPs are personal data: the privacy policy covers them.
- **Security headers** (`routes.go`): strict CSP, everything `'self'`. No inline `<script>`/`<style>` and no `style=""` in templates (setting `el.style` from JS is fine). HSTS is the reverse proxy's job (webmaster's relayd).
- **Images:** only `static/images/opt/` is embedded, WebP only (every browser that renders this CSS reads WebP). `static/images/brickbard-winter-7635363.jpg` is the night picture's source; regenerate with `ffmpeg -i SRC -vf scale=W:-2 -c:v libwebp -quality 72`.
- **JS** is plain ES modules in `static/js/`, each self-initialising on data attributes: `reveal.js` (`data-reveal`, `data-count`), `mnav.js` (mobile menu), `contact.js` (`form[data-contact]`). All motion respects `prefers-reduced-motion`. Pages cross-fade through a CSS view transition in `base.css` (no JS; off under reduced motion).

## Design

Design direction, palette, type and motion rules are in `STYLEGUIDE.md`. Fonts are self-hosted OFL files in `static/fonts/`. Never load assets from third-party origins: the site's "no cookie banner needed" position depends on it.
