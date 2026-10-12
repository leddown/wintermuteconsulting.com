# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Role

Act as a website creator and web UI expert with deep knowledge of modern web design. Interpret design language ("modern website", "professional design", "clean", "cluttered", etc.) the way current industry practice and research understand it.

## Project

A public-facing company website.

- **Language:** Go. The site is served by a Go program.
- **Backend scope:** Minimal. The most that's planned right now is a contact form.
- **Out of scope:** no database, shopping cart, or e-commerce of any kind. Don't add dependencies or infrastructure for these.
- **Hosting:** Self-hosted on a VPS (the user wrote "VPC").

## Standing obligation: explain hosting and compliance

Make sure the user understands the operational and legal requirements that come with self-hosting a public site, and raise them when they're relevant instead of assuming. For example:

- TLS certificates, reverse proxy and firewall setup, running the service, and updates
- Cookie and consent banners: whether one is needed depends on what the site sets (analytics, embeds, sessions)
- Privacy policy, and how contact-form submissions are handled (spam protection, where the data goes)

## Commands

```sh
go run . -dev                 # http://127.0.0.1:8080; templates, content.json and static read from disk and re-read per request
go test ./...                 # all tests
go test -run TestContact ./...  # single test
go build -o wintermuteconsulting .      # single self-contained binary (templates + assets embedded)
scripts/portrait.sh photo.jpg           # the About page's photo -> static/images/opt/portrait-{480,960}.{webp,jpg} (FOCUS=0..1 moves the crop)
```

Deployment (see header comments in each script for settings). Three machines: this **workstation** runs every test and scanner; the **LAN dev server** (~1 GB RAM) keeps a git checkout and builds itself with low-memory flags (`deploy/update-linux.sh`: ~230 MB peak first build, ~30 MB cached; no tests or scanners there); **production** is OpenBSD with the base system only: it never compiles and has nothing installed, and updates are a single binary pushed from the workstation.

```sh
deploy/build.sh linux|openbsd [amd64|arm64]    # tests, then static binary in dist/
deploy/update-linux.sh                         # ON the dev server, in its checkout: git pull, build, redeploy
deploy/push.sh openbsd user@vps DOMAIN=example.com WWW=1   # production first setup / config change (doas)
deploy/push.sh openbsd user@vps                # production update: new binary + restart, auto-rollback on failed health check
```

- `setup-linux.sh` (LAN dev) only installs the binary and a hardened systemd unit (`MemoryMax=256M`). Settings live in `/etc/wintermuteconsulting/wintermuteconsulting.env` (`ADDR=127.0.0.1:8080`, `TRUST_PROXY=1`, `GOMEMLIMIT`), created once and never overwritten. The box's own nginx (set up manually, not by us) proxies to it and must send `proxy_set_header Host $host;` (else the contact form's Origin check 403s) and `proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;` (else TRUST_PROXY is spoofable). No firewall or package changes.
- `setup-openbsd.sh` (production) uses base only: rc.d + httpd (ACME/redirect) + relayd (TLS, HSTS) + acme-client + pf.

Security testing (`security/run.sh` with no args prints all options; exits non-zero on any FAIL):

```sh
security/run.sh code                 # gofmt, vet, staticcheck, gosec, govulncheck (source + built binaries), go test -race, fuzzing
FUZZTIME=30m security/run.sh code    # before a release
security/run.sh local                # start the binary on loopback; curl probes + ZAP full scan, nuclei, nikto
security/run.sh prod example.com     # production from outside: nmap, TLS/testssl.sh, headers, passive ZAP (ACTIVE=1 to attack)
doas sh run.sh host                  # on a server (push.sh copies run.sh): OpenBSD or the LAN dev box. The only stage allowed on <2 GB hosts
```

- `security_test.go` pins the security properties (headers, no inline/third-party code, escaping, traversal, Origin check, log injection, rate-limit spoofing, server timeouts). `fuzz_test.go` holds the fuzz targets; crashers saved in `testdata/fuzz/` run as regression tests on every `go test`.
- Reviewed scanner false positives / accepted risks live in `security/zap-rules.tsv` and `security/nikto-ignore.txt`; each needs a reason. Don't add entries just to make a run green.
- Scanner tool versions are pinned in `security/run.sh`; `go.mod` pins the Go toolchain (`toolchain` line). Keep it on a supported, patched Go release: govulncheck in binary mode fails otherwise.

Flags/env: `-addr` / `ADDR` (default `127.0.0.1:8080`), `-dev` / `DEV=1`, `-trust-proxy` / `TRUST_PROXY=1` (use the last `X-Forwarded-For` hop as the client IP; enable only behind a proxy that appends the client IP: relayd in production, nginx on the LAN dev box).

## Architecture

- Standard library only (`net/http`, `html/template`, `embed`); no third-party Go modules. Keep it that way unless there's a strong reason.
- **All site copy is in `content.json`** (embedded; moved out of Go on request so text can be edited without touching code). `site` is what the firm says about itself (services, principles, stats, industries, insights, the brief and the lede, the About page). `home` is the homepage's own wording (hero title, buttons, section kickers, titles and paragraphs, contact intro and points): the desktop template and the phone template both read it as `.Copy`, so text is changed in one place and the two layouts never word a thing differently (`TestDesktopAndPhoneShareTheCopy`). A layout may leave a piece out (the phone has no Insights or Industries sections and no contact points, and its hero uses `site.brief` where the desktop hero uses `site.lede`); `_readme` at the top of the file holds these notes for whoever edits it, since JSON has no comments: keep it current. A heading's kicker is shown only where one is set. Templates hold no copy of their own beyond navigation and form labels. `content.go` holds the types and the loader, which is strict because the file is edited by hand: unknown, misspelled or repeated names and empty required texts are errors that name the line (`parseContent`, `validate`). A bad file stops the program at start-up; in `-dev` the file is re-read on every request and its error is shown instead of the page. `content_test.go` checks the real file and that each kind of slip is refused, so `go test` and `deploy/build.sh` catch a bad edit before a binary exists. A new field needs a struct field with a `json` tag, and a line in `validate` if pages rely on it. JSON because it is in the standard library: YAML or TOML would break the no-dependencies rule. Positioning: boutique AI, cyber & ICT risk advisory for regulated finance across the EU, AI first: emerging AI threats and AI governance, the complete policy stack through to implemented, auditable controls, custom frameworks and controls sized to each organisation's risk appetite, plus threat intelligence/geopolitical risk, vCISO, audits and remediation, fintech advisory, crisis exercises, training. Never "Nordic" or "counsel" in site copy; big-firm vocabulary (EY/Deloitte), boutique delivery. See STYLEGUIDE.md. No pentest/red-team language; see STYLEGUIDE.md. Copy changes go in `content.json`, not in the templates.
- **The site is one design, Natt**, chosen from the alternatives on request: the homepage at `/` and the About page at `/about`. It is the professional-services design: an advisory-firm layout (utility bar, nav, service tiles, figures, industries, insights, steps, column footer) built on the brickbard night illustration, with amber as the single signal colour. On a phone the full picture stays behind the copy but is placed by the hero's width in `cqi`, so the moon sits high and always in the same spot, and the copy's top padding keeps its first line below the moon at any text size, so the white title never crosses the white moon; the scrim is clear over the moon and darkens below it, the eyebrow is `--ink` so small text stays above 4.5:1 over the bushes, and the phone hero says less: `Site.Brief` in place of the lede, capped by `TestNattPhoneHeroCopyStaysShort` (`.m--natt` in `mobile.css`). Files: `templates/home.html` (desktop homepage), `templates/mobile.html` (phone shell and phone homepage), the About page's three templates (below), `templates/ps.html` and `templates/partials.html` (shared pieces); `static/css/base.css` (fonts, reset, default tokens, buttons, form, portrait), `natt.css` (Natt's tokens and desktop hero), `ps.css` (the furniture), `mobile.css`, `about.css`. "Natt" lives on as the design's name in those file and class names (`natt.css`, `.natt`, `.m--natt`). **Removed on request, do not bring back:** the choice between designs and everything that served it: the chooser page that was at `/`, the `/v/{slug}` URLs (all 404 now, `TestNotFound`), `variants` and `variantTheme` in `app.go`, the design switcher in the footers, the form's `variant` field and `designs` in `content.json` (`TestHomeIsTheSite`). The last two alternatives, Signal (the console: `signal.css`, `terminal.js`, its header and simulated feed) and Grense (the research-led design with the forest rail: `grense.css`), went when Natt became the site, and with them what only they used: `wolfeyes.js` (no page shows wolf eyes now), the forest and woods renditions in `static/images/opt/` (their originals are still in `static/images/`) and `site.legalForm`. Removed earlier: Varg, Varg 2, the first Signal and Signal 2, Snø, Vakt, Spor, Spor 2, the first Grense, the first Natt (its phone hero let the title run over the moon) and both Natt 2s (the second put the picture in a band above the copy on a phone; the picture is wanted behind the text). They are in git history.
- **About page** at `/about`: `templates/about-body.html` (the content, shared by both layouts), `templates/about.html` (desktop: the site's header and footer around it), `templates/m/about.html` (overrides the mobile shell's `m-main`) and `static/css/about.css` (container queries, Natt's treatment at the end). Copy is `site.about` in `content.json`; anything in `[square brackets]` is a placeholder for the owner's own biography and the company's registration details. Never invent those. The portrait (the `portrait` partial in `partials.html`) appears once all four files from `scripts/portrait.sh` exist (`hasPortrait` in `app.go`, set for every page in `renderPage`); until then a placeholder is drawn. No contact form on the page: its buttons go to the homepage's `#contact`.
- **Links between pages:** the header and footer are partials (`ps-util`, `ps-nav` and `ps-footer` in `ps.html`; on a phone, the bar and menu in `mobile.html`) used by the homepage and the About page. Section links are written `{{.Home}}#services`: `.Home` is empty on the homepage and `/` on the About page. `pageData.Page` says which page is rendering (`""` or `"about"`).
- **Company name:** the legal name "Wintermute Consulting OÜ" (Estonian private limited company) appears on every page via `Site.Name` (titles, meta, footers, ©). The logo is the plain lockup "Wintermute Consulting" (`logo` partial), without "OÜ".
- **No glitch effects** (removed on request): no glitch.js, flash layer, glitching or scrambling text anywhere. `TestNoGlitchAnywhere` enforces it.
- **Phones get a dedicated mobile layout at the same URL** (dynamic serving, `mobile.go`): `Sec-CH-UA-Mobile` client hint first, User-Agent fallback, tablets get desktop. Responses send `Vary: User-Agent, Sec-CH-UA-Mobile`. `?view=mobile|desktop` overrides (no cookie; carried through links, the form's hidden `view` field and the PRG redirect). `templates/mobile.html` is the shell (bar, menu, footer) with the phone homepage in it; its `{{block}}`s (`m-main`, `m-title`, `m-styles`) are what `templates/m/about.html` overrides for the About page; section headings come from `.Copy` through the `m-heading` partial. Styles in `static/css/mobile.css`, scoped to `.m`; `natt.css` is loaded first for its tokens. Mobile rules: 16px+ inputs (no iOS zoom), 44px+ tap targets, safe-area insets, single column. Desktop templates must stay responsive anyway (tablets, wrong guesses).
- Templates: each page and layout is parsed as its own set (`parseTemplates` in `app.go`: `home`, `about`, `m/home`, `m/about`) with `templates/partials.html` (shared head, logo, contact form, portrait, layout switch) and `templates/ps.html` (the furniture: night illustration, service icons keyed by the service code prefix, logo mark, utility bar, nav, tiles, figures, industries, insights, steps, column footer; styled by `static/css/ps.css` with the tokens in `natt.css`). Every page file defines a `"page"` template.
- Contact form: `POST /contact` → honeypot (`website` field), Origin check (no cookies, so no CSRF token), validation, in-memory per-IP rate limit, then PRG redirect to `/?sent=1#contact`. On errors it re-renders the homepage with 422. **Delivery is not wired yet.** Submissions are only logged (TODO: transactional email provider).
- **Static URLs are versioned.** Link every stylesheet and script (and any image that may be replaced under the same name, like the portrait) with the template func `{{static "js/reveal.js"}}`, never a literal `/static/...` path: it appends `?v=<hash of the file's content>`, so a changed file reaches every browser on the next page load. Versioned URLs are served `immutable` for a year, anything else under `/static/` for a day, and pages themselves `no-cache` (`cacheStatic` in `routes.go`, `staticURL` in `app.go`; `TestStaticURLsCarryTheContentVersion` fails on a bare link). In `-dev` URLs are bare and nothing is cached.
- Security headers in `routes.go`: strict CSP with everything `'self'`. No inline `<script>`/`<style>` and no `style=""` attributes in templates (setting `el.style` from JS is fine). HSTS belongs on the reverse proxy.
- Of the images, only `static/images/opt/` is embedded (the night illustration, the favicon and, once made, the portrait). Originals in `static/images/` are sources: regenerate resized WebP/JPEG with ffmpeg (`-vf scale=W:-2 -c:v libwebp -quality 72`).
- JS is plain ES modules in `static/js/`, each self-initialising on data attributes: `reveal.js` (`data-reveal`, `data-count`), `mnav.js` (mobile `<details data-menu>`: close on link tap/Escape/outside tap). All motion must respect `prefers-reduced-motion`. Pages cross-fade into each other through a CSS cross-document view transition in `base.css` (no JS; off under reduced motion).

## Design

Design direction, palette, type and motion rules are in `STYLEGUIDE.md`. Fonts are self-hosted OFL files in `static/fonts/`. Never load assets from third-party origins, because the site's "no cookie banner needed" position depends on it.
