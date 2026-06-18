// Admin email-templates merged list. Mirrors Node admin.controller.ts
// listEmailTemplates: load all email_templates rows, key them by template_key,
// then map EMAIL_TEMPLATES (the 6 builtins, in order) overlaying any DB
// customization onto the builtin default. Returns a BARE array.
//
//	const overrides = await emailTemplateRepo.find();
//	const byKey = new Map(overrides.map(o => [o.template_key, o]));
//	return EMAIL_TEMPLATES.map(def => ({
//	  key, label, variables,
//	  subject: o?.subject ?? def.subject,
//	  html:    o?.html    ?? def.html,
//	  customized: !!o,
//	  default_subject: def.subject,
//	  default_html:    def.html,
//	}));
//
// DRY: subject/html defaults come from the existing builtinTemplates map (in
// templates.go) — same package, no string duplication. Only the ordered
// metadata (key + label + variables) is declared here.
package email

import (
	"context"
	"encoding/json"
)

// DBTemplate is one email_templates customization row (the columns the merge
// needs). Exported so the repo + handler ports share it.
type DBTemplate struct {
	Subject string
	HTML    string
}

// builtinTemplateMeta is the ordered metadata for one builtin. subject/html
// are NOT stored here — they live in builtinTemplates (DRY).
type builtinTemplateMeta struct {
	key       string
	label     string
	variables []string
}

// builtinTemplateOrder mirrors EMAIL_TEMPLATES order in email-templates.ts.
// subject/html defaults come from the existing builtinTemplates map (DRY).
var builtinTemplateOrder = []builtinTemplateMeta{
	{"verification", "Welcome + email verification", []string{"name", "code"}},
	{"password-reset", "Password reset", []string{"name", "code"}},
	{"subscription-activated", "Subscription activated (payment success)", []string{"name", "plan", "amount", "expires"}},
	{"subscription-expired", "Subscription expired", []string{"name", "plan"}},
	{"renewal-reminder", "Renewal reminder", []string{"name", "plan", "expires", "amount"}},
	{"payment-failed", "Payment failed", []string{"name", "plan"}},
}

// TemplateRow is one merged template as Node serialises it. MarshalJSON pins
// the key order: key, label, variables, subject, html, customized,
// default_subject, default_html.
type TemplateRow struct {
	Key            string
	Label          string
	Variables      []string
	Subject        string
	HTML           string
	Customized     bool
	DefaultSubject string
	DefaultHTML    string
}

// MarshalJSON emits keys in exactly: key, label, variables, subject, html,
// customized, default_subject, default_html. Variables is forced non-nil so it
// never serialises as null.
func (tr TemplateRow) MarshalJSON() ([]byte, error) {
	vars := tr.Variables
	if vars == nil {
		vars = []string{}
	}
	return json.Marshal(struct {
		Key            string   `json:"key"`
		Label          string   `json:"label"`
		Variables      []string `json:"variables"`
		Subject        string   `json:"subject"`
		HTML           string   `json:"html"`
		Customized     bool     `json:"customized"`
		DefaultSubject string   `json:"default_subject"`
		DefaultHTML    string   `json:"default_html"`
	}{
		Key: tr.Key, Label: tr.Label, Variables: vars,
		Subject: tr.Subject, HTML: tr.HTML, Customized: tr.Customized,
		DefaultSubject: tr.DefaultSubject, DefaultHTML: tr.DefaultHTML,
	})
}

// adminTemplatesRepo is the service's consumer-side port; *AdminTemplatesRepo
// satisfies it. One method returns the customization map keyed by template_key.
type adminTemplatesRepo interface {
	ListCustomizations(ctx context.Context) (map[string]DBTemplate, error)
}

// AdminTemplatesService is the merged-templates list use case.
type AdminTemplatesService struct {
	repo adminTemplatesRepo
}

// NewAdminTemplatesService wires the repo port.
func NewAdminTemplatesService(repo adminTemplatesRepo) *AdminTemplatesService {
	return &AdminTemplatesService{repo: repo}
}

// List iterates builtinTemplateOrder, overlaying the customization map onto the
// builtin defaults, and returns the ordered rows (always non-nil, length 6).
func (s *AdminTemplatesService) List(ctx context.Context) ([]TemplateRow, error) {
	byKey, err := s.repo.ListCustomizations(ctx)
	if err != nil {
		return nil, err
	}
	rows := make([]TemplateRow, 0, len(builtinTemplateOrder))
	for _, def := range builtinTemplateOrder {
		builtin := builtinTemplates[def.key]
		row := TemplateRow{
			Key:            def.key,
			Label:          def.label,
			Variables:      def.variables,
			Subject:        builtin.subject,
			HTML:           builtin.html,
			Customized:     false,
			DefaultSubject: builtin.subject,
			DefaultHTML:    builtin.html,
		}
		if o, ok := byKey[def.key]; ok {
			row.Subject = o.Subject
			row.HTML = o.HTML
			row.Customized = true
		}
		rows = append(rows, row)
	}
	return rows, nil
}
