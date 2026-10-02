package main

// Site copy shared by every design variant. Placeholder text until the final
// copy is written; edit here rather than in the templates.

type Service struct {
	Code  string // short tag shown in mono type, e.g. "RT-01"
	Title string
	Body  string
}

type Principle struct {
	Title string
	Body  string
}

type Industry struct {
	Name string
	Body string
}

// Insight is a short point-of-view piece. Teasers only for now: there are no
// article pages yet, so templates link them to the contact form.
type Insight struct {
	Tag     string
	Title   string
	Summary string
	Minutes int
}

type Stat struct {
	Value int
	Unit  string
	Label string
}

// Fact is one labelled line in a definition list: the principal's background,
// the company's registration details.
type Fact struct {
	Label string
	Value string
}

// Principal is the person presented on the About page, beside their portrait.
type Principal struct {
	Name  string
	Role  string
	Bio   []string // paragraphs
	Facts []Fact
}

// About is the copy for the About page (/v/{slug}/about).
type About struct {
	Title     string // the page's headline
	Lede      string
	Summary   string // meta description: keep it under 160 characters
	Principal Principal
	NameNote  string // why the firm is called Wintermute
	Company   []Fact // registration details; the legal name and email come from Site
}

type Site struct {
	Name       string // full legal name: titles, meta, footers
	LegalForm  string // company form, shown in the top logo only where a design asks for it
	LogoMark   string // logo lockup: the word mark...
	LogoSub    string // ...and the lighter qualifier beside or under it
	Descriptor string
	Tagline    string
	Lede       string
	Email      string
	Location   string
	Services   []Service
	Principles []Principle
	Stats      []Stat
	Industries []Industry
	Insights   []Insight
	About      About
}

var site = Site{
	Name:       "Wintermute Consulting OÜ", // Estonian private limited company
	LegalForm:  "OÜ",
	LogoMark:   "Wintermute",
	LogoSub:    "Consulting",
	Descriptor: "AI, cyber & ICT risk advisory",
	Tagline:    "The threat is already in the forest.",
	Lede: "A boutique team of senior advisors helping financial institutions across the European Union meet " +
		"emerging AI and cyber threats. We build the complete stack, from board-approved policy to implemented, " +
		"auditable controls, sized to your organisation and your risk appetite, and stay through the long night " +
		"when something goes wrong.",
	Email:    "info@wintermuteconsulting.com",
	Location: "Serving clients across the European Union",
	// Service codes: the two-letter prefix picks the icon in templates/ps.html ("svc-icon").
	Services: []Service{
		{"AI-01", "Emerging AI: threats, security & governance", "Offensive and defensive AI. How attackers now use it, from voice and video deepfakes to AI-written spear phishing and automated reconnaissance, and how to defend with it. We build the AI policy stack and controls, and govern the AI your institution deploys under the EU AI Act and DORA."},
		{"RM-02", "Custom frameworks & controls", "Your own information security and ICT risk management framework, not a template: the complete stack from board-approved policy to standards, procedures and implemented controls, built on DORA and its regulatory technical standards, NIS2, ISO/IEC 27001, NIST CSF 2.0 and NIST SP 800-53, mapped to local regulation, and sized to your organisation and risk appetite."},
		{"TI-03", "Threat intelligence & geopolitical risk", "Emerging cyber threats and the geopolitical drivers behind them: state-aligned intrusion, hacktivism, sanctions, conflict and supply-chain exposure, translated into scenarios, control changes and briefings your management body can act on."},
		{"VC-04", "Virtual CISO", "Fractional or interim security leadership for a single institution or across a private equity portfolio. We own the security programme, report to your management body and represent it before your supervisor, for as long as you need us."},
		{"RG-05", "Regulatory audits & remediation", "Audits and inspection readiness under DORA, the ECB's supervisory expectations and NIS2, integrated with the national rules of your member state: every requirement traced to a control and its evidence, and remediation programmes that close the findings."},
		{"FT-06", "Fintech advisory", "For founders and early teams: a policy stack and controls sized to your stage, built in from the licence application onwards, and ready for the supervisor, your banking partners and investor due diligence."},
		{"CX-07", "Crisis exercises", "Tabletop exercises and crisis simulations built on today's threats (a deepfaked CFO, a state-aligned outage, a critical provider gone dark) for management bodies, investment committees and crisis teams, with a debrief that shows who must decide what."},
		{"TR-08", "Training", "Security and digital operational resilience training matched to the role: awareness for all staff, including recognising AI-enabled social engineering, specialist sessions for control functions, and briefings for members of the management body."},
	},
	Principles: []Principle{
		{"Sized to you", "Not every institution is in the same place. Frameworks and controls are fitted to your size, complexity and risk appetite, as DORA's proportionality principle intends, never copied from a template."},
		{"Auditable end to end", "Every requirement traced from regulation to policy, control and evidence, so your auditor and your supervisor can follow the thread."},
		{"Boutique by design", "Senior advisors only, with no leverage pyramid and no hand-offs. The advisor who scopes your engagement is the one in the room with your management body."},
		{"Discretion", "We never publish client names or discuss our work, we don't sell data, and this site sets no cookies."},
	},
	// Placeholder figures: replace with real ones before launch.
	Stats: []Stat{
		{15, "yrs", "average senior experience"},
		{1, "", "named advisor per client"},
		{27, "", "EU member states, one DORA rulebook"},
	},
	Industries: []Industry{
		{"Banking", "Credit institutions under ECB and national supervision, preparing for inspections and meeting DORA's requirements for ICT risk management."},
		{"Fintech & payments", "Payment and e-money institutions and fintech startups, from the first licence application to scale, with security sized to the stage."},
		{"Private equity", "General partners and their portfolio companies: cyber due diligence before signing, a remediation plan in the first hundred days, and a clean account of security at exit."},
		{"SME institutional holders", "Small and mid-sized institutional holders, from holding companies and foundations to smaller pension and endowment funds: institutional assets and obligations, run by a lean team."},
		{"Wealth & asset management", "Boutique wealth managers and funds meeting allocators' operational due diligence and their supervisors' expectations with a lean team."},
		{"Trading & digital assets", "Investment firms, trading venues and crypto-asset service providers under MiCA, where availability, custody and third-party infrastructure are regulatory questions."},
	},
	Insights: []Insight{
		{"AI threats", "The CFO's voice is not the CFO: deepfakes in the treasury", "How AI-generated voice and video now drive payment fraud, and the call-back and approval controls that still stop it.", 5},
		{"AI governance", "Your AI policy stack: from risk appetite to controls an auditor can test", "What the management body should decide about AI, how that becomes policy, and the controls that prove it is being followed.", 7},
		{"Geopolitical risk", "Geopolitics is now an ICT risk driver", "Why supervisors expect sanctions, conflict and state-aligned activity to appear in your ICT risk assessment, and the questions your management body should ask.", 7},
	},
	About: About{
		Title: "The advisor you brief is the one who does the work.",
		Lede: "Wintermute Consulting is a boutique AI, cyber and ICT risk advisory for financial institutions across " +
			"the European Union. It is small by design: senior advisors only, no hand-offs, and one named advisor " +
			"from the first briefing to the last closed finding.",
		Summary: "Who is behind Wintermute Consulting OÜ, a boutique AI, cyber and ICT risk advisory for regulated finance across the European Union.",
		// Placeholders: everything in [square brackets] is yours to replace before
		// launch. These are statements about a real person and a registered
		// company, so nothing here is invented. The portrait is added with
		// scripts/portrait.sh.
		Principal: Principal{
			Name: "[Your name]",
			Role: "Founder and principal advisor",
			Bio: []string{
				"[Your background in two or three sentences: the institutions you have worked in, the roles you held and for how long. Boards and supervisors read this for seniority, so name roles and responsibilities rather than adjectives.]",
				"[What you do for clients today: the engagements you lead and the regulation you work with most, such as DORA, NIS2 or the EU AI Act.]",
				"[One human detail to close on: where you are based, or what you do away from work.]",
			},
			Facts: []Fact{
				{"Experience", "[n] years in [field]"},
				{"Qualifications", "[degrees and certifications]"},
				{"Languages", "[languages you work in]"},
				{"Based in", "[city, country]"},
			},
		},
		NameNote: "The name nods to Wintermute, the artificial intelligence in William Gibson's Neuromancer. It marks " +
			"where the firm starts: AI is already part of the threat, and already at work inside the institutions we " +
			"advise. Both call for controls that a supervisor and an auditor can verify.",
		// What EU law expects a company's website to state (e-Commerce Directive
		// art. 5, in Estonia the Information Society Services Act).
		Company: []Fact{
			{"Legal form", "Private limited company (osaühing), registered in Estonia"},
			{"Registry code", "[Estonian commercial register code]"},
			{"Registered office", "[street, postcode, city, Estonia]"},
			{"VAT number", "[EE number, if VAT-registered]"},
		},
	},
}
