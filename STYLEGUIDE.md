# Wintermute Consulting style guide (draft)

Direction for a boutique information security & ICT risk advisory serving the European Union. Keywords from the brief:
bleak, cold, forest, snow, threat, wolf, wolf eyes, dangers, AI, modern, high-quality imagery.

## Research takeaways

- **Positioning: boutique AI, cyber & ICT risk advisory, AI first.** Most work is in emerging AI: offensive AI (deepfakes, AI-written phishing, automated reconnaissance) and defensive AI (securing and governing the AI an institution deploys, EU AI Act and DORA). The core offer is the complete stack, from board-approved policy to standards, procedures and implemented, auditable controls, so financial institutions can meet modern threats in a compliant and auditable way. Custom frameworks and custom controls, never templates, sized to the organisation and its risk appetite (DORA's proportionality principle: not every institution is in the same place). Frameworks drawn on: DORA and its RTS, NIS2, ISO/IEC 27001, NIST CSF 2.0, NIST SP 800-53, plus local regulation. Also: threat intelligence and geopolitical risk drivers, virtual CISO, regulatory audits and remediation (DORA, ECB expectations, NIS2, national rules), fintech advisory, crisis exercises, training. Big-firm vocabulary (EY, Deloitte), boutique delivery. No pentest or red-team language, and never "counsel".
- **Audience: regulated finance and private capital across the EU**: banks (ECB and national supervision), fintech and payments (payment and e-money institutions, startups from licence application onwards), private equity (GPs and portfolio companies: diligence, first hundred days, exit), SME institutional holders (holding companies, foundations, smaller pension and endowment funds: institutional assets, lean teams), wealth and asset managers (allocator operational due diligence), trading firms and crypto-asset providers (MiCA). Register: discreet, formal British English, the tone of a principal's private bank. Words: advisory, principal, discretion, in strict confidence, management body, supervisor, remediation. Never: "counsel" (we are not a legal or risk counsel), "Nordic" in site copy, hype, fear, "hackers", exclamation marks.
- **Boutique advisors sell seniority and directness.** "Senior advisors, no hand-offs, one named advisor from first briefing to last closed finding." Short sentences, first person plural, no buzzword stacks. Regulatory terms are used precisely (management body, supervisory findings, ICT risk management framework).
- **Dark themes are the norm in security** because they signal control and sophistication, usually dark with one vivid accent. To stand out, we pair the dark with *natural* imagery (forest, fog, snow) rather than the usual circuit-board or padlock stock.
- **Nordic design principles** (visual heritage only; site copy targets the whole EU and never says "Nordic") are function over decoration, sans-serif type, generous spacing, left-aligned text, clear hierarchy, and a restrained palette taken from the landscape. 2026 trends add oversized expressive type and deep teals and blue-greens with a single electric accent.
- **Self-hosted fonts.** Loading Google Fonts from Google leaks visitor IPs (LG München, 2022). All fonts are OFL-licensed and served from `/static/fonts`.

## Palette

| Token | Default (Spor, chooser) | Signal 2 | Use |
|---|---|---|---|
| `--bg` | `#07090a` | `#040607` | page background |
| `--ink` | `#e4ebe8` | `#d6ecf2` | body text |
| `--muted` | `#8d9b96` | `#6f8990` | secondary text |
| `--accent` | `#e9a441` wolf amber | `#7fd4e8` ice | CTAs, highlights |

The research-led design Grense uses the same roles: `#eef1f0` / `#0b1210` / `#4b5955` (light page, dark forest rail). It keeps wolf amber `#e9a441` for the eyes and the primary action; small amber text on its light page uses `#875000`, because the bright amber fails contrast on a light ground.

Amber is reserved for the eyes and primary actions. Nothing else should glow.

## Typography

- **Space Grotesk** (display): geometric with slightly mechanical terminals, reads as both "technical" and "modern Nordic".
- **Inter** (body): neutral and highly legible.
- **JetBrains Mono** (labels, codes, eyebrows): uppercase with wide tracking, for the "machine" voice.

## Imagery

Fog, spruce, moss, snow, dusk. Desaturate and darken photos. The forest should feel quiet, not horror-movie. Wolf eyes: real eyeshine footage (trail-camera style, eyes only, on black) composited over the forest, with a procedural canvas fallback. No stock-wolf clichés.
**Still needed:** higher-resolution originals. `juliaboldt-woods` is only 640 px wide and looks soft on large screens. Winter/snow and wolf photography would strengthen Spor.

## Motion rules

1. One ambient motion per viewport: eyes *or* terminal.
2. No glitch effects: no flashes, tears, RGB splits or scrambled text.
3. No motion under `prefers-reduced-motion`. Eyes render as a still frame.
4. Anything interactive responds to hover/focus with a small scramble, never a layout shift.
5. Wolf eyes fade in on their own every 5 to 11 s, linger 1–3 s and fade out; they may be elsewhere or gone next time. They never follow the pointer. Real eyeshine footage (`data-footage`) is preferred; the drawn fallback is dim almond eyeshine with no pupils. Where the eyes sit deep in a picture, draw them smaller (`data-scale`) and keep them where an animal could stand: on the ground (`data-band`), not among the trunks or up in the canopy.

## Research-led design (Grense)

Built around one finding, in the brief's own terms (cold, forest, wolf eyes, AI). Two others were built and removed on request: Snø (a light page, on the finding that dark text on a light ground reads best) and Vakt (a portrait-led page, on the finding that buyers choose a named expert).

- **Grense: say what you do in the first screen.** Referred prospects most often rule a firm out because they cannot tell what it does, and a stranger gives a homepage a few seconds. The first screen states the offer in one sentence and shows the firm's subject, AI used against the client and AI put to work for them, side by side. The forest never leaves: it is the rail that carries the navigation.

Rules it follows, from the review of the earlier designs: the hero fits a 1366×650 laptop screen; copy never sits on a bright part of a picture; section links are reachable at every width; no numbering on things that are not a sequence; no kicker that only repeats its heading.

## About page and portrait

- **Why it exists.** Buyers of advisory work check who they would be dealing with before they make contact: a named person with a photograph, a specific biography and the company's registered details. A firm that sells "one named advisor" has to show one.
- **The photograph.** Head and shoulders, face filling the frame, eyes to the camera, plain background, daylight or soft light. Cropped 4:5. The designs treat it lightly (Spor mutes the colour a little, Signal 2 and Grense show it in monochrome, Natt leaves it natural): do not darken a face the way the forest pictures are darkened.
- **The biography.** Roles, institutions and years, in the first person plural or the third person, in the same register as the rest of the site. No adjectives doing the work of facts, no client names.
- **Company details** (legal name, form, registry code, registered office, VAT number, email) are on the page because EU law expects them on a company's site, not as decoration.

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
- [Hinge: the Visible Expert](https://hingemarketing.com/blog/story/the-visible-expert-how-ordinary-professionals-become-thought-leaders) (buyers choose named experts: the About page)
- [Bain value-proposition survey, via Media Logic](https://www.medialogic.com/blog/b2b-marketing/b2b-value-proposition/) and [Pitch Kitchen: State of B2B Homepage Messaging 2026](https://www.pitchkitchen.com/2026-state-of-b2b-homepage-messaging) (clarity in the first screen: Grense)
- [Stanford Web Credibility Guidelines](https://credibility.stanford.edu/guidelines/index.html) (show the real organisation and the people behind the site)
- [Hinge Research Institute: referral marketing study](https://hingemarketing.com/uploads/hinge-research-referral-marketing.pdf) (referred prospects rule firms out on an unclear site)
- [6sense: B2B Buyer Experience Report 2025](https://6sense.com/science-of-b2b/buyer-experience-report-2025/) and [Edelman–LinkedIn B2B Thought Leadership Impact Report 2025](https://www.edelman.com/expertise/Business-Marketing/2025-b2b-thought-leadership-report)
- [e-Commerce Directive 2000/31/EC, art. 5](https://eur-lex.europa.eu/legal-content/EN/ALL/?uri=CELEX%3A32000L0031) and the Estonian [Information Society Services Act](https://www.riigiteataja.ee/en/eli/504112013008/consolide) (company details a site must state)
