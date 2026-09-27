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
go build -o wintermute .      # single self-contained binary (templates + assets embedded)
```

Flags/env: `-addr` / `ADDR` (default `127.0.0.1:8080`), `-dev` / `DEV=1`, `-trust-proxy` / `TRUST_PROXY=1` (use the last `X-Forwarded-For` hop as the client IP; enable only behind Caddy/nginx).

## Architecture

- Standard library only (`net/http`, `html/template`, `embed`); no third-party Go modules. Keep it that way unless there's a strong reason.
- `content.go` holds all site copy (services, principles, stats) as Go data shared by every page. Copy changes go there, not in the templates.
- The site currently ships **five alternative homepage designs** ("variants") for the user to choose between: `varg`, `vitt`, `signal`, `morke`, `spor`, listed in `variants` in `app.go` and served at `/v/{slug}`. `/` is a chooser page. Each variant = `templates/<slug>.html` + `static/css/<slug>.css`. Adding one means touching all three places.
- Templates: each page is parsed as its own set with `templates/partials.html` (shared head, glitch text, pictures, contact form, footer). Every page file defines a `"page"` template.
- Contact form: `POST /contact` → honeypot (`website` field), Origin check (no cookies, so no CSRF token), validation, in-memory per-IP rate limit, then PRG redirect to `/v/<slug>?sent=1#contact`. On errors it re-renders the variant with 422. **Delivery is not wired yet.** Submissions are only logged (TODO: transactional email provider).
- Security headers in `routes.go`: strict CSP with everything `'self'`. No inline `<script>`/`<style>` and no `style=""` attributes in templates (setting `el.style` from JS is fine). HSTS belongs on the reverse proxy.
- Only `static/images/opt/` is embedded. Originals in `static/images/` are sources: regenerate resized WebP/JPEG with ffmpeg (`-vf scale=W:-2 -c:v libwebp -quality 72`).
- JS is plain ES modules in `static/js/`, each self-initialising on data attributes: `glitch.js` (`data-glitch`, `data-glitch-hover`, `data-flash`), `wolfeyes.js` (`canvas[data-wolf-eyes]`; eyes appear only on the `wintermute:flash` event that `glitch.js` fires; optional `data-footage` = real eyeshine clip on black), `snow.js`, `terminal.js`, `reveal.js` (`data-reveal`, `data-count`), `descent.js` (`--depth` scroll var). All motion must respect `prefers-reduced-motion`, and flashes must stay under 3 per second.

## Design

Design direction, palette, type and motion rules are in `STYLEGUIDE.md`. Fonts are self-hosted OFL files in `static/fonts/`. Never load assets from third-party origins, because the site's "no cookie banner needed" position depends on it.
