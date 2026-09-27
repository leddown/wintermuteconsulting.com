# Wintermute Consulting style guide (draft)

Direction for a Nordic boutique security consultancy. Keywords from the brief:
bleak, cold, forest, snow, threat, wolf, wolf eyes, dangers, AI, digital glitch, modern, high-quality imagery.

## Research takeaways

- **Positioning: security leadership and ICT risk advisory, not offensive testing.** Virtual CISO (fractional or interim), regulatory audits and ECB inspections, remediation of findings, information security and ICT risk management frameworks (DORA and its technical standards, EBA guidelines, MiCA, ISO/IEC 27001) from regulatory text to technical implementation, training, and management-board crisis exercises. No pentest or red-team language.
- **Audience: private capital and regulated finance**, written as if for a principal of a Gulf family office: private equity (GPs and portfolio companies: diligence, first hundred days, exit), single- and multi-family offices (principals, their families, their privacy), boutique wealth managers and funds (allocator operational due diligence), banks, trading firms, crypto-asset providers (MiCA; FSRA in ADGM). Register: discreet, formal British English. Words: counsel, principal, discretion, in strict confidence, mandate, stewardship, generational. Never: hype, fear, "hackers", exclamation marks. Nordic supervisors: Finanstilsynet (NO), Finansinspektionen (SE), Finanssivalvonta (FI), plus the ECB for significant institutions.
- **Boutique advisors sell seniority and directness.** "Senior advisors, no hand-offs, one named advisor from first briefing to last closed finding." Short sentences, first person plural, no buzzword stacks. Regulatory terms are used precisely (management body, supervisory findings, ICT risk management framework).
- **Dark themes are the norm in security** because they signal control and sophistication, usually dark with one vivid accent. To stand out, we pair the dark with *natural* imagery (forest, fog, snow) rather than the usual circuit-board or padlock stock.
- **Nordic design principles** are function over decoration, sans-serif type, generous spacing, left-aligned text, clear hierarchy, and a restrained palette taken from the landscape. 2026 trends add oversized expressive type and deep teals and blue-greens with a single electric accent.
- **Glitch effects must be accessible.** Keep them under three flashes per second (WCAG 2.3.1), disable them under `prefers-reduced-motion`, and keep the real text in the DOM for screen readers (the animated copies are `aria-hidden`).
- **Self-hosted fonts.** Loading Google Fonts from Google leaks visitor IPs (LG München, 2022). All fonts are OFL-licensed and served from `/static/fonts`.

## Palette

| Token | Varg (default) | Signal | Mørke | Use |
|---|---|---|---|---|
| `--bg` | `#07090a` | `#040607` | `#0f1010` | page background |
| `--ink` | `#e4ebe8` | `#d6ecf2` | `#ecebe5` | body text |
| `--muted` | `#8d9b96` | `#6f8990` | `#8f8e88` | secondary text |
| `--accent` | `#e9a441` wolf amber | `#7fd4e8` ice | `#ecebe5` | CTAs, highlights |
| `--glitch-a/b` | pale ghost / `#6b0f16` | inherited | inherited | glitch split only, never bright |

Amber is reserved for the eyes and primary actions. Nothing else should glow.

## Typography

- **Space Grotesk** (display): geometric with slightly mechanical terminals, reads as both "technical" and "modern Nordic".
- **Inter** (body): neutral and highly legible.
- **JetBrains Mono** (labels, codes, eyebrows): uppercase with wide tracking, for the "machine" voice.
- **Instrument Serif** (Mørke, Rim): editorial contrast.

## Imagery

Fog, spruce, moss, snow, dusk. Desaturate and darken photos. The forest should feel quiet, not horror-movie. Wolf eyes: real eyeshine footage (trail-camera style, eyes only, on black) composited over the forest, with a procedural canvas fallback. No stock-wolf clichés.
**Still needed:** higher-resolution originals. `juliaboldt-woods` is only 640 px wide and looks soft on large screens. Winter/snow and wolf photography would strengthen Spor.

## Motion rules

1. One ambient motion per viewport: eyes *or* terminal.
2. A glitch is an *event*: a short flash every 5 to 11 s, never a constant jitter. It goes *darker*, never brighter: the screen dims, a few 1-3 px black tears, no neon RGB bands and no inverted images.
3. No motion under `prefers-reduced-motion`. Eyes render as a still frame.
4. Anything interactive responds to hover/focus with a small scramble, never a layout shift.
5. Wolf eyes are only seen when the lights dip: hidden until a glitch flash, then they linger 1–3 s and fade, and may be elsewhere or gone by the next flash. They never follow the pointer. Real eyeshine footage (`data-footage`) is preferred; the drawn fallback is dim almond eyeshine with no pupils.

## Voice

Calm, cold, precise. The forest is the regulatory and risk landscape; we are the guides who know where the supervisor looks and where the ground gives way. The wolf knows where to look. The name Wintermute Consulting nods to Wintermute, the AI in Gibson's *Neuromancer*.

## Sources

- [Blend B2B: best cybersecurity website examples](https://www.blendb2b.com/blog/the-7-best-cybersecurity-website-examples)
- [CyberOptik: best cybersecurity website designs 2026](https://www.cyberoptik.net/blog/best-cybersecurity-website-designs/)
- [Figma: web design trends 2026](https://www.figma.com/resource-library/web-design-trends/)
- [Digital Silk: minimalist web design trends 2026](https://www.digitalsilk.com/digital-trends/minimalist-web-design-trends/)
- [Admind: Scandinavian graphic design and modern branding](https://admindagency.com/scandinavian-graphic-design-and-modern-branding/)
- [Nordic Co-operation design manual: colours](https://pub.norden.org/designmanual-en/colours.html)
- [Pope Tech: accessible animation](https://blog.pope.tech/2025/12/08/design-accessible-animation-and-movement/)
- [Blaze Infosec: penetration testing companies buyer's guide](https://www.blazeinfosec.com/post/penetration-testing-companies/)
- [REDOPS](https://redghostops.com/), [primeObjective](https://primeobjective.net/), [Secra](https://secra.es/en), [Atlan Digital](https://www.atlan.digital/)
- [Fontsource](https://fontsource.org/) (self-hosted OFL fonts)
