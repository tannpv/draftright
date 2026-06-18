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
	"errors"
	"time"
)

// ErrUnknownTemplate is returned by Update/Preview when the path key is not a
// known builtin template. Message mirrors Node's NotFoundException('Unknown
// template') exactly — the handler maps it to 404 not-found.
var ErrUnknownTemplate = errors.New("Unknown template")

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
	Upsert(ctx context.Context, key, subject, html string) error
	Delete(ctx context.Context, key string) error
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

// Update saves a customization for an existing builtin key (UPSERT on
// template_key). An unknown builtin key → ErrUnknownTemplate (Node throws
// NotFoundException before saving).
func (s *AdminTemplatesService) Update(ctx context.Context, key, subject, html string) error {
	if _, ok := builtinTemplates[key]; !ok {
		return ErrUnknownTemplate
	}
	return s.repo.Upsert(ctx, key, subject, html)
}

// Reset deletes any customization for the key, restoring the builtin. Node does
// NOT validate the key — it accepts ANY key and is idempotent (no 404).
func (s *AdminTemplatesService) Reset(ctx context.Context, key string) error {
	return s.repo.Delete(ctx, key)
}

// Preview renders the template for the key with sample vars, DB-override-aware.
// Unknown key → ErrUnknownTemplate. Mirrors Node renderTemplate(key, sample):
// the DB customization (if any) overrides the builtin def, then {{vars}} are
// substituted (subject unescaped, html escaped).
func (s *AdminTemplatesService) Preview(ctx context.Context, key string) (subject, html string, err error) {
	def, ok := builtinTemplates[key]
	if !ok {
		return "", "", ErrUnknownTemplate
	}
	// Sample vars — byte-for-byte parity with Node's previewEmailTemplate.
	sample := map[string]string{
		"name":    "Tan",
		"code":    "123456",
		"plan":    "Pro",
		"amount":  "124.000 ₫",
		"expires": time.Now().Add(30 * 24 * time.Hour).Format("Mon Jan 02 2006"),
	}

	subjTpl := def.subject
	htmlTpl := def.html
	customs, err := s.repo.ListCustomizations(ctx)
	if err != nil {
		return "", "", err
	}
	if c, ok := customs[key]; ok {
		subjTpl = c.Subject
		htmlTpl = c.HTML
	}
	return substitute(subjTpl, sample, false), substitute(htmlTpl, sample, true), nil
}
