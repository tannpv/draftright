package email

import "strings"

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
}

// renderTemplate substitutes {{var}} tokens. Unknown key → ("","").
func renderTemplate(key string, vars map[string]string) (subject, html string) {
	t, ok := builtinTemplates[key]
	if !ok {
		return "", ""
	}
	return substitute(t.subject, vars), substitute(t.html, vars)
}

func substitute(s string, vars map[string]string) string {
	for k, v := range vars {
		s = strings.ReplaceAll(s, "{{"+k+"}}", v)
	}
	return s
}
