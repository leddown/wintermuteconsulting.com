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
```

Deployment (see header comments in each script for settings). Three machines: this **workstation** builds and runs every test and scanner; the **LAN dev server** (~1 GB RAM) only runs the binary; **production** is OpenBSD. Servers never compile anything.

```sh
deploy/build.sh linux|openbsd [amd64|arm64]    # tests, then static binary in dist/
deploy/push.sh linux user@devbox               # build here, scp, run setup-linux.sh there (sudo)
deploy/push.sh openbsd user@vps DOMAIN=example.com WWW=1   # same for production (doas)
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
- `content.go` holds all site copy (services, principles, stats) as Go data shared by every page. Copy changes go there, not in the templates.
- The site currently ships **five alternative homepage designs** ("variants") for the user to choose between: `varg`, `vitt`, `signal`, `morke`, `spor`, listed in `variants` in `app.go` and served at `/v/{slug}`. `/` is a chooser page. Each variant = `templates/<slug>.html` + `static/css/<slug>.css` + `templates/m/<slug>.html` (its mobile blocks). Adding one means touching all four places.
- **Phones get a dedicated mobile layout at the same URL** (dynamic serving, `mobile.go`): `Sec-CH-UA-Mobile` client hint first, User-Agent fallback, tablets get desktop. Responses send `Vary: User-Agent, Sec-CH-UA-Mobile`. `?view=mobile|desktop` overrides (no cookie; carried through links, the form's hidden `view` field and the PRG redirect). `templates/mobile.html` is the shared shell; its `{{block}}`s (`m-hero`, `m-ai`, `m-scripts`, `m-theme-color`, titles, `m-cta`) are overridden per design in `templates/m/<slug>.html`. Styles in `static/css/mobile.css`, scoped to `.m`; the design's own stylesheet is loaded first for its tokens. Mobile rules: 16px+ inputs (no iOS zoom), 44px+ tap targets, safe-area insets, single column. Desktop templates must stay responsive anyway (tablets, wrong guesses).
- Templates: each page is parsed as its own set with `templates/partials.html` (shared head, glitch text, pictures, contact form, footer). Every page file defines a `"page"` template.
- Contact form: `POST /contact` → honeypot (`website` field), Origin check (no cookies, so no CSRF token), validation, in-memory per-IP rate limit, then PRG redirect to `/v/<slug>?sent=1#contact`. On errors it re-renders the variant with 422. **Delivery is not wired yet.** Submissions are only logged (TODO: transactional email provider).
- Security headers in `routes.go`: strict CSP with everything `'self'`. No inline `<script>`/`<style>` and no `style=""` attributes in templates (setting `el.style` from JS is fine). HSTS belongs on the reverse proxy.
- Only `static/images/opt/` is embedded. Originals in `static/images/` are sources: regenerate resized WebP/JPEG with ffmpeg (`-vf scale=W:-2 -c:v libwebp -quality 72`).
- JS is plain ES modules in `static/js/`, each self-initialising on data attributes: `glitch.js` (`data-glitch`, `data-glitch-hover`, `data-flash`), `wolfeyes.js` (`canvas[data-wolf-eyes]`; eyes appear only on the `wintermute-consulting:flash` event that `glitch.js` fires; optional `data-footage` = real eyeshine clip on black), `snow.js`, `terminal.js`, `reveal.js` (`data-reveal`, `data-count`), `descent.js` (`--depth` scroll var), `mnav.js` (mobile `<details data-menu>`: close on link tap/Escape/outside tap). All motion must respect `prefers-reduced-motion`, and flashes must stay under 3 per second.

## Design

Design direction, palette, type and motion rules are in `STYLEGUIDE.md`. Fonts are self-hosted OFL files in `static/fonts/`. Never load assets from third-party origins, because the site's "no cookie banner needed" position depends on it.
