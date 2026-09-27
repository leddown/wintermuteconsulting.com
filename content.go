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

type Site struct {
	Name       string // full name: titles, meta, footers
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
}

var site = Site{
	Name:       "Wintermute Consulting",
	LogoMark:   "Wintermute",
	LogoSub:    "Consulting",
	Descriptor: "Nordic security & risk counsel",
	Tagline:    "The threat is already in the forest.",
	Lede: "A small team of senior advisors providing discreet security counsel to banks, private equity, " +
		"family offices and wealth managers. We lead your security programme, prepare you for the regulator, " +
		"and stay through the long night when something goes wrong.",
	Email:    "contact@example.com",
	Location: "Oslo · Stockholm · Helsinki",
	// Service codes: the two-letter prefix picks the icon in templates/ps.html ("svc-icon").
	Services: []Service{
		{"VC-01", "Virtual CISO", "Senior security leadership, part-time or interim, for a single institution or across a private equity portfolio. We own the security programme, report to your board and sit across the table from your regulator, for as long as you need us."},
		{"RG-02", "Regulatory audits & inspections", "Preparation for supervisory inspections and ICT audits, whether the supervisor is the ECB, a Nordic authority or the FSRA in Abu Dhabi Global Market: evidence in order, people briefed, no surprises in the room."},
		{"RM-03", "ICT risk management frameworks", "Information security and ICT risk management frameworks built on DORA and its technical standards, the EBA guidelines and ISO/IEC 27001, from the policy the board approves to the controls that prove it."},
		{"RE-04", "Remediation & implementation", "When findings land, we turn them into a plan the supervisor accepts, then carry it from regulatory text to technical implementation with your IT teams and third parties until the evidence holds."},
		{"CX-05", "Board crisis exercises", "Tabletop exercises and crisis simulations for boards, investment committees and family councils: realistic scenarios, a clock that keeps running, and a debrief that shows who must decide what."},
		{"TR-06", "Training", "Security and operational resilience training matched to the role, from awareness for all staff to private briefings for board members, principals and their families."},
	},
	Principles: []Principle{
		{"Senior by design", "No junior bench, no hand-offs. The advisor who scopes your engagement is the one in the room with your board and your supervisor."},
		{"From paper to practice", "A policy is only as good as the control behind it. We carry every requirement from the regulation to the system that proves it."},
		{"Discretion", "We never publish client names or discuss our work, we don't sell data, and this site sets no cookies."},
	},
	// Placeholder figures: replace with real ones before launch.
	Stats: []Stat{
		{15, "yrs", "average senior experience"},
		{1, "", "named advisor per client"},
		{3, "", "Nordic capitals"},
	},
	Industries: []Industry{
		{"Private equity", "General partners and their portfolio companies: cyber due diligence before signing, a remediation plan in the first hundred days, and a clean account of security at exit."},
		{"Family offices", "Single- and multi-family offices with the assets of an institution and the staff of a household: protecting principals, their families and their privacy."},
		{"Wealth & asset management", "Boutique wealth managers and funds meeting allocators' operational due diligence and their regulators' expectations with a lean team."},
		{"Banking", "Banks and payment institutions preparing for supervisory inspections and DORA's requirements for ICT risk management."},
		{"Trading & markets", "Trading firms, brokers and venues where availability, latency and third-party infrastructure are regulatory questions."},
		{"Digital assets", "Crypto-asset service providers under MiCA in Europe or the FSRA in ADGM, where custody and key management are the business itself."},
	},
	Insights: []Insight{
		{"Private equity", "Cyber due diligence: the question before the signature", "What an investment committee should know about a target's security before closing, and what belongs in the first hundred days after.", 6},
		{"Family offices", "The assets of an institution, the staff of a household", "Why attackers study family offices for months before they act, and the few controls that protect principals and their privacy.", 5},
		{"Supervision", "After the inspection: turning findings into a plan the supervisor accepts", "What supervisors look for in a remediation plan, and why the first draft is usually rejected.", 7},
	},
}
