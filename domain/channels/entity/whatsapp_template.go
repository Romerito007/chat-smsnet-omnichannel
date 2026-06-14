package entity

// WhatsAppTemplate is a render-only mirror of a WhatsApp template owned by the
// external integrator. The chat NEVER talks to Meta and never interprets the
// template's Meta semantics — it stores the structure only to (a) draw the
// selector + variable form, (b) resolve a display string for the chat history,
// and (c) validate that the agent filled the declared variables. The ID is opaque
// (defined by the integrator) and echoed back verbatim on send.
type WhatsAppTemplate struct {
	ID       string                  // opaque integrator id
	Name     string                  // label for the selector
	Language string                  // e.g. "pt_BR"
	Category string                  // optional, display only
	Body     WhatsAppTemplateBody    // the body text + its named variables
	Header   *WhatsAppTemplateHeader // optional
	Buttons  []WhatsAppTemplateButton
	Footer   string
}

// WhatsAppTemplateBody is the body component: a text with {{name}} placeholders
// and the ordered named variables the form must collect.
type WhatsAppTemplateBody struct {
	Text      string
	Variables []WhatsAppTemplateVariable
}

// WhatsAppTemplateVariable is one named placeholder the agent fills.
type WhatsAppTemplateVariable struct {
	Key     string // the placeholder name used in Body.Text as {{key}}
	Label   string // optional, for the form field
	Example string // optional
}

// WhatsAppTemplateHeader is the optional header component (display only).
type WhatsAppTemplateHeader struct {
	Type string // text|image|video|document
	Text string
}

// WhatsAppTemplateButton is an optional button (display only).
type WhatsAppTemplateButton struct {
	Type string // quick_reply|url
	Text string
	URL  string
}

// FindTemplate returns the template with the given id, or nil.
func FindTemplate(templates []WhatsAppTemplate, id string) *WhatsAppTemplate {
	for i := range templates {
		if templates[i].ID == id {
			return &templates[i]
		}
	}
	return nil
}
