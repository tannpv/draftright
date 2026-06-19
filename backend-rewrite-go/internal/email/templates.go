package email

import (
	"regexp"
	"strings"
)

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
	"verification": {
		subject: "Welcome to DraftRight — confirm your email",
		html: shell("Welcome to DraftRight, {{name}} 👋",
			`<p style="color:#444;line-height:1.6;margin:0 0 16px;">Thanks for joining DraftRight — your AI writing companion. Select any text, pick a tone, and get a polished rewrite in a tap, right from the keyboard, the apps, or the web playground.</p>
    <p style="color:#444;line-height:1.6;margin:0 0 8px;">One quick step to activate your account — enter this code in the app:</p>
    <p style="font-size:30px;font-weight:700;letter-spacing:6px;color:#5b3df6;background:#f3f0ff;border-radius:10px;text-align:center;padding:14px 0;margin:0 0 8px;">{{code}}</p>
    <p style="color:#888;font-size:13px;line-height:1.5;margin:0 0 20px;">The code expires in 15 minutes. If you didn't create a DraftRight account, you can safely ignore this email.</p>
    <p style="color:#444;line-height:1.6;margin:0 0 4px;">Once you're in, try the tones — Simple, Polished, Concise, Natural and more.</p>
    <p style="color:#444;line-height:1.6;margin:0;">Questions? Just reply to this email.</p>`),
	},
	"password-reset": {
		subject: "Reset your DraftRight password",
		html: shell("Reset your password, {{name}}",
			`<p style="color:#444;line-height:1.5;margin:0 0 16px;">Your password reset code is:</p>
    <p style="font-size:28px;font-weight:700;letter-spacing:4px;color:#5b3df6;margin:0 0 16px;">{{code}}</p>
    <p style="color:#444;line-height:1.5;margin:0 0 16px;">Enter it on the reset page to choose a new password. It expires in 15 minutes. If you didn't request this, you can ignore this email.</p>`),
	},
	"subscription-activated": {
		subject: "Your DraftRight {{plan}} subscription is active",
		html: shell("You're all set, {{name}} 🎉",
			`<p style="color:#444;line-height:1.5;margin:0 0 16px;">Your payment of <strong>{{amount}}</strong> was received and your DraftRight <strong>{{plan}}</strong> subscription is now active.</p>
    <p style="color:#444;line-height:1.5;margin:0 0 16px;">Active until <strong>{{expires}}</strong>. Enjoy unlimited rewrites across all your devices.</p>`),
	},
	"subscription-expired": {
		subject: "Your DraftRight {{plan}} subscription has expired",
		html: shell("Your subscription has expired",
			`<p style="color:#444;line-height:1.5;margin:0 0 16px;">Hi {{name}} — your {{plan}} plan has ended. You're now on the Free plan with 10 rewrites per day. Restore Pro anytime to go unlimited.</p>
    <p style="color:#444;line-height:1.5;margin:0 0 16px;"><a href="https://draftright.info/account" style="color:#5b3df6;">draftright.info/account</a></p>`),
	},
	"renewal-reminder": {
		subject: "DraftRight {{plan}} renews on {{expires}}",
		html: shell("Heads up, {{name}}",
			`<p style="color:#444;line-height:1.5;margin:0 0 16px;">Your DraftRight {{plan}} subscription renews on <strong>{{expires}}</strong>. We'll charge {{amount}} to your saved payment method.</p>
    <p style="color:#444;line-height:1.5;margin:0 0 16px;">No action needed if everything looks right. To update your card or cancel, visit your account settings.</p>`),
	},
	"payment-failed": {
		subject: "Action needed: renewal payment failed for DraftRight {{plan}}",
		html: shell("Payment didn't go through",
			`<p style="color:#444;line-height:1.5;margin:0 0 16px;">Hi {{name}} — we tried to charge your saved card to renew your DraftRight {{plan}} subscription, but the charge failed.</p>
    <p style="color:#444;line-height:1.5;margin:0 0 16px;">We'll automatically retry over the next few days. You can update your payment method any time to fix this faster.</p>`),
	},
}

// renderTemplate substitutes {{var}} tokens. Unknown key → ("","").
// Subject is rendered with escape=false, html with escape=true — matching
// Node's renderTemplate (HTML-escape only in HTML context).
func renderTemplate(key string, vars map[string]string) (subject, html string) {
	t, ok := builtinTemplates[key]
	if !ok {
		return "", ""
	}
	return substitute(t.subject, vars, false), substitute(t.html, vars, true)
}

var tokenRe = regexp.MustCompile(`\{\{(\w+)\}\}`)

// substitute replaces each {{token}} with vars[token]. A missing key →
// "" (Go zero value), matching Node's `vars[k] ?? ”`. When escape is
// true the value is HTML-escaped (HTML body context). Only \w+ tokens
// are matched; non-matching braces are left intact (mirrors Node's regex).
func substitute(s string, vars map[string]string, escape bool) string {
	return tokenRe.ReplaceAllStringFunc(s, func(m string) string {
		k := m[2 : len(m)-2] // strip {{ }}
		v := vars[k]
		if escape {
			return escapeHTML(v)
		}
		return v
	})
}

// escapeHTML mirrors Node's escapeHtml: &→&amp;, <→&lt;, >→&gt;,
// "→&quot;, '→&#39;. Ampersand FIRST. strings.NewReplacer applies all
// replacements in a single left-to-right pass with no re-scanning, so the
// &amp; it inserts is never re-escaped — matching the sequential JS chain.
func escapeHTML(s string) string {
	return htmlEscaper.Replace(s)
}

var htmlEscaper = strings.NewReplacer(
	"&", "&amp;",
	"<", "&lt;",
	">", "&gt;",
	`"`, "&quot;",
	"'", "&#39;",
)
