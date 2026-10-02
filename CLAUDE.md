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
go run . -dev                 # http://127.0.0.1:8080; templates/static read from disk and re-parsed per request
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
- `content.go` holds all site copy (services, principles, stats, industries, insights) as Go data shared by every page. Positioning: boutique AI, cyber & ICT risk advisory for regulated finance across the EU, AI first: emerging AI threats and AI governance, the complete policy stack through to implemented, auditable controls, custom frameworks and controls sized to each organisation's risk appetite, plus threat intelligence/geopolitical risk, vCISO, audits and remediation, fintech advisory, crisis exercises, training. Never "Nordic" or "counsel" in site copy; big-firm vocabulary (EY/Deloitte), boutique delivery. See STYLEGUIDE.md. No pentest/red-team language; see STYLEGUIDE.md. Copy changes go there, not in the templates.
- The site currently ships **eight alternative homepage designs** ("variants") for the user to choose between: `varg`, `signal`, `spor`, the professional-services design `natt` (advisory-firm layout built on the brickbard night illustration), and the copies `varg2`, `signal2` (Signal with smaller eyes set deep in its picture), `spor2`, `natt2`. Listed in `variants` in `app.go` and served at `/v/{slug}`. `/` is a chooser page. Each variant = `templates/<slug>.html` + `static/css/<slug>.css` + `templates/m/<slug>.html` (its mobile blocks). Adding one means touching all four places. A "2" copy has its own desktop and mobile templates but reuses the original's stylesheet via `variantTheme` in `app.go` (body classes `<orig> <slug>` / `m--<slug> m--<orig>`); give it its own `static/css/<slug>.css` and drop the `variantTheme` entry once it needs to diverge.
- **About page** at `/v/{slug}/about`, one per design from a single set of files: `templates/about-body.html` (the content, shared by both layouts), `templates/about.html` (desktop: picks the design's stylesheet, header and footer by `.Variant.Theme`; a new design needs a branch there), `templates/m/about.html` (overrides the mobile shell's `m-main`) and `static/css/about.css` (container queries, per-design treatments at the end). Copy is `Site.About` in `content.go`; anything in `[square brackets]` is a placeholder for the owner's own biography and the company's registration details. Never invent those. The portrait appears once all four files from `scripts/portrait.sh` exist (`hasPortrait` in `app.go`); until then a placeholder is drawn. No contact form on the page: its buttons go to the homepage's `#contact`.
- **Links between pages:** each design's header is a partial (`varg-header`, `signal-header`, `spor-header` in `partials.html`, `ps-nav` in `ps.html`) used by its homepage(s) and the About page. Section links are written `{{.Home}}#services`: `.Home` is empty on the homepage and the homepage's URL elsewhere. `pageData.Page` says which page is rendering (`""` or `"about"`).
- **Company name:** the legal name "Wintermute Consulting OÜ" (Estonian private limited company) appears on every page via `Site.Name` (titles, meta, footers, ©). The top logo shows "OÜ" only on the Signal designs (`logo-legal` partial; the mobile bar picks it when `.Variant.Theme` is `signal`); every other design uses the plain `logo` lockup.
- **No glitch effects** (removed on request): no glitch.js, flash layer, glitching or scrambling text anywhere. `TestNoGlitchAnywhere` enforces it. Wolf eyes stay: `wolfeyes.js` fades pairs in and out on its own 5–11 s timer.
- **Phones get a dedicated mobile layout at the same URL** (dynamic serving, `mobile.go`): `Sec-CH-UA-Mobile` client hint first, User-Agent fallback, tablets get desktop. Responses send `Vary: User-Agent, Sec-CH-UA-Mobile`. `?view=mobile|desktop` overrides (no cookie; carried through links, the form's hidden `view` field and the PRG redirect). `templates/mobile.html` is the shared shell; its `{{block}}`s (`m-hero`, `m-ai`, `m-scripts`, `m-theme-color`, titles, `m-cta`) are overridden per design in `templates/m/<slug>.html`. Styles in `static/css/mobile.css`, scoped to `.m`; the design's own stylesheet is loaded first for its tokens. Mobile rules: 16px+ inputs (no iOS zoom), 44px+ tap targets, safe-area insets, single column. Desktop templates must stay responsive anyway (tablets, wrong guesses).
- Templates: each page is parsed as its own set with `templates/partials.html` (shared head, logo, pictures, contact form, footer) and `templates/ps.html` (professional-services family: night illustration, service icons keyed by the service code prefix, logo mark, nav, tiles, insights, column footer; styled by `static/css/ps.css` plus each design's tokens). Every page file defines a `"page"` template.
- Contact form: `POST /contact` → honeypot (`website` field), Origin check (no cookies, so no CSRF token), validation, in-memory per-IP rate limit, then PRG redirect to `/v/<slug>?sent=1#contact`. On errors it re-renders the variant with 422. **Delivery is not wired yet.** Submissions are only logged (TODO: transactional email provider).
- Security headers in `routes.go`: strict CSP with everything `'self'`. No inline `<script>`/`<style>` and no `style=""` attributes in templates (setting `el.style` from JS is fine). HSTS belongs on the reverse proxy.
- Only `static/images/opt/` is embedded. Originals in `static/images/` are sources: regenerate resized WebP/JPEG with ffmpeg (`-vf scale=W:-2 -c:v libwebp -quality 72`).
- JS is plain ES modules in `static/js/`, each self-initialising on data attributes: `wolfeyes.js` (`canvas[data-wolf-eyes]`: pairs fade in, linger and fade out every 5–11 s; `data-pairs`, `data-scale` to shrink, `data-size="lg"`, `data-color="ice"`, optional `data-footage` = real eyeshine clip on black), `terminal.js`, `reveal.js` (`data-reveal`, `data-count`), `descent.js` (`--depth` scroll var), `mnav.js` (mobile `<details data-menu>`: close on link tap/Escape/outside tap). All motion must respect `prefers-reduced-motion` (eyes are then drawn once, still). Pages cross-fade into each other through a CSS cross-document view transition in `base.css` (no JS; off under reduced motion).

## Design

Design direction, palette, type and motion rules are in `STYLEGUIDE.md`. Fonts are self-hosted OFL files in `static/fonts/`. Never load assets from third-party origins, because the site's "no cookie banner needed" position depends on it.
