package email

import "github.com/tannpv/emailkit"

// Template keys. Previously each of these appeared twice — as a map key here
// and as a literal at the call site in service.go — so renaming a template
// updated one copy and left the other pointing at nothing.
const (
	TemplateVerification          = "verification"
	TemplatePasswordReset         = "password-reset"
	TemplateRenewalReminder       = "renewal-reminder"
	TemplateSubscriptionActivated = "subscription-activated"
	TemplatePaymentFailed         = "payment-failed"
	TemplateSubscriptionExpired   = "subscription-expired"
)

// BuiltinRegistry is draftright's template set in the shape emailkit renders.
// The copy lives here, not in emailkit: a shared module carrying one
// consumer's subscription wording is a union, not a shared module.
func BuiltinRegistry() emailkit.Registry {
	reg := make(emailkit.Registry, len(builtinTemplates))
	for k, v := range builtinTemplates {
		reg[k] = emailkit.TemplateDef{Subject: v.subject, Body: v.html}
	}
	return reg
}

type templateDef struct {
	subject string
	html    string
}

func shell(title, body string) string {
	return `<!doctype html>
<html><body style="font-family:-apple-system,system-ui,sans-serif;background:#f5f5f7;padding:32px;margin:0;">
  <div style="max-width:480px;margin:0 auto;background:#fff;border-radius:12px;padding:32px;">
    <h1 style="font-size:20px;margin:0 0 16px;color:#111;">` + title + `</h1>
    ` + body + `
    <p style="color:#888;font-size:13px;margin:24px 0 0;">— DraftRight</p>
  </div>
</body></html>`
}

// builtinTemplates mirrors email-templates.ts. Bodies use {{var}}
// placeholders substituted at send. NOT shadow-gated (email is
// out-of-band) — ported for functional parity.
var builtinTemplates = map[string]templateDef{
	TemplateVerification: {
		subject: "Welcome to DraftRight — confirm your email",
		html: shell("Welcome to DraftRight, {{name}} 👋",
			`<p style="color:#444;line-height:1.6;margin:0 0 16px;">Thanks for joining DraftRight — your AI writing companion. Select any text, pick a tone, and get a polished rewrite in a tap, right from the keyboard, the apps, or the web playground.</p>
    <p style="color:#444;line-height:1.6;margin:0 0 8px;">One quick step to activate your account — enter this code in the app:</p>
    <p style="font-size:30px;font-weight:700;letter-spacing:6px;color:#5b3df6;background:#f3f0ff;border-radius:10px;text-align:center;padding:14px 0;margin:0 0 8px;">{{code}}</p>
    <p style="color:#888;font-size:13px;line-height:1.5;margin:0 0 20px;">The code expires in 15 minutes. If you didn't create a DraftRight account, you can safely ignore this email.</p>
    <p style="color:#444;line-height:1.6;margin:0 0 4px;">Once you're in, try the tones — Simple, Polished, Concise, Natural and more.</p>
    <p style="color:#444;line-height:1.6;margin:0;">Questions? Just reply to this email.</p>`),
	},
	TemplatePasswordReset: {
		subject: "Reset your DraftRight password",
		html: shell("Reset your password, {{name}}",
			`<p style="color:#444;line-height:1.5;margin:0 0 16px;">Your password reset code is:</p>
    <p style="font-size:28px;font-weight:700;letter-spacing:4px;color:#5b3df6;margin:0 0 16px;">{{code}}</p>
    <p style="color:#444;line-height:1.5;margin:0 0 16px;">Enter it on the reset page to choose a new password. It expires in 15 minutes. If you didn't request this, you can ignore this email.</p>`),
	},
	TemplateSubscriptionActivated: {
		subject: "Your DraftRight {{plan}} subscription is active",
		html: shell("You're all set, {{name}} 🎉",
			`<p style="color:#444;line-height:1.5;margin:0 0 16px;">Your payment of <strong>{{amount}}</strong> was received and your DraftRight <strong>{{plan}}</strong> subscription is now active.</p>
    <p style="color:#444;line-height:1.5;margin:0 0 16px;">Active until <strong>{{expires}}</strong>. Enjoy unlimited rewrites across all your devices.</p>`),
	},
	TemplateSubscriptionExpired: {
		subject: "Your DraftRight {{plan}} subscription has expired",
		html: shell("Your subscription has expired",
			`<p style="color:#444;line-height:1.5;margin:0 0 16px;">Hi {{name}} — your {{plan}} plan has ended. You're now on the Free plan with 10 rewrites per day. Restore Pro anytime to go unlimited.</p>
    <p style="color:#444;line-height:1.5;margin:0 0 16px;"><a href="https://draftright.info/account" style="color:#5b3df6;">draftright.info/account</a></p>`),
	},
	TemplateRenewalReminder: {
		subject: "DraftRight {{plan}} renews on {{expires}}",
		html: shell("Heads up, {{name}}",
			`<p style="color:#444;line-height:1.5;margin:0 0 16px;">Your DraftRight {{plan}} subscription renews on <strong>{{expires}}</strong>. We'll charge {{amount}} to your saved payment method.</p>
    <p style="color:#444;line-height:1.5;margin:0 0 16px;">No action needed if everything looks right. To update your card or cancel, visit your account settings.</p>`),
	},
	TemplatePaymentFailed: {
		subject: "Action needed: renewal payment failed for DraftRight {{plan}}",
		html: shell("Payment didn't go through",
			`<p style="color:#444;line-height:1.5;margin:0 0 16px;">Hi {{name}} — we tried to charge your saved card to renew your DraftRight {{plan}} subscription, but the charge failed.</p>
    <p style="color:#444;line-height:1.5;margin:0 0 16px;">We'll automatically retry over the next few days. You can update your payment method any time to fix this faster.</p>`),
	},
}

// adHocKey is the throwaway registry key substitute renders under. It never
// leaves this function — a Registry is keyed by definition, so rendering an
// unregistered pair needs some key, and naming it once beats inventing one at
// the call.
const adHocKey = "ad-hoc"

// substitute renders an ad-hoc template string that is NOT in
// BuiltinRegistry — an admin's saved override, which the admin preview screen
// must render before it is ever sent. Subject context passes values through
// raw (escape false); HTML body context escapes them (escape true).
//
// The token syntax and the escaping rules are emailkit's, reached through the
// same Registry.Render every real send uses. Keeping draftright's own regexp
// and escaper here would be a second definition of the escaping policy, and a
// divergence between them would show up as an XSS hole in exactly one of the
// two paths.
func substitute(s string, vars map[string]string, escape bool) string {
	subject, body := emailkit.Registry{adHocKey: {Subject: s, Body: s}}.Render(adHocKey, vars)
	if escape {
		return body
	}
	return subject
}
