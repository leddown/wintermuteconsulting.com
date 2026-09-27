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
}

var site = Site{
	Name:       "Wintermute Consulting",
	LogoMark:   "Wintermute",
	LogoSub:    "Consulting",
	Descriptor: "Nordic security consultancy",
	Tagline:    "The threat is already in the forest.",
	Lede: "We are a small pack of senior operators who hunt the way attackers do: " +
		"patiently, quietly, with machine intelligence at our side. " +
		"We find the path through your defences before anyone else walks it.",
	Email:    "contact@example.com",
	Location: "Oslo · Stockholm · Helsinki",
	Services: []Service{
		{"RT-01", "Adversary simulation", "Full-scope red team operations modelled on the actors that actually target Nordic industry. Physical, social and digital, run end to end by the people who scoped it."},
		{"AI-02", "AI & LLM security", "Prompt injection, agent hijacking, data exfiltration through retrieval pipelines, model supply chain. We attack your AI systems before they are turned against you."},
		{"PT-03", "Penetration testing", "Web, cloud, internal network and OT. Manual, senior-led testing with findings your engineers can act on the same week."},
		{"TI-04", "Threat hunting", "Machine-assisted hunting across your telemetry. Our models surface the anomalies; our analysts decide what is prey and what is predator."},
		{"IR-05", "Incident response", "When something is already inside: containment, forensics and a calm voice at three in the morning."},
		{"GV-06", "NIS2 & DORA readiness", "Pragmatic advisory for the regulations now binding Nordic and EU operators. Evidence, not paperwork."},
	},
	Principles: []Principle{
		{"Small by design", "No junior bench, no hand-offs. The operator who scopes your engagement runs it."},
		{"Human-led, AI-sharpened", "We build and train our own tooling. Machines widen the search; people make the judgement."},
		{"Quiet", "We don't publish client names, we don't sell your data, and this site sets no cookies."},
	},
	Stats: []Stat{
		{15, "yrs", "average operator experience"},
		{97, "%", "of engagements reach objective"},
		{3, "", "Nordic capitals"},
	},
}
