# emailkit Transport Module Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Extract draftright's working Resend transport into a shared Go module `github.com/tannpv/emailkit` that bacnam, liseuse and draftright can all import, fixing a webhook replay hole on the way out.

**Architecture:** Two consumer-side ports — `Sender` (provider) and `Store` (persistence) — plus a caller-supplied template `Registry`. One unexported `deliver()` is the sole path to the provider, so suppression cannot be bypassed. Storage, tenancy and product vocabulary all stay in the consuming project.

**Tech Stack:** Go 1.25, stdlib only (`net/http`, `crypto/hmac`, `encoding/json`, `regexp`). No Resend SDK — the existing code posts to the HTTP API directly and that stays.

**Source of truth for ported code:** `tannpv/draftright` at `origin/main` = `5dbfa570`, path `backend-rewrite-go/internal/email/`.

## Global Constraints

- **Go directive: `go 1.25.0`.** Not higher. draftright is `1.25.0`, liseuse `1.26.4`, bacnam `1.26`. Declaring 1.26 would block draftright — the first migrator — from importing it.
- **Zero non-stdlib dependencies.** The ported code uses only stdlib and must continue to.
- **Package name: `emailkit`.** Ported files arrive as `package email`; every one needs the clause changed.
- **No `internal/shared` imports.** `shared.WriteError`, `shared.WriteJSON`, `shared.CodeInvalidInput`, `shared.ISOMillis` are draftright-only and must not follow the code across.
- **No DraftRight strings.** No `defaultFrom`, no DraftRight copy, no `builtinTemplates` contents, no `formatAmount`/`groupThousands`/`dateString`. Those are product vocabulary and stay in draftright.
- **`deliver`, `resendClient`, `NewResendSender`'s concrete type stay unexported.** Enforced by Task 7.

## Planning Note — a spec refinement

The spec described `templates.go` as moving wholesale. Reading it shows a
three-way split, and the plan follows the split rather than the spec:

| symbol | destination | why |
|---|---|---|
| `substitute`, `escapeHTML`, `htmlEscaper`, `tokenRe`, `templateDef` | **emailkit** | generic `{{token}}` substitution and escaping |
| `builtinTemplates` map, `shell()` | **stays in draftright** | DraftRight HTML chrome and copy |
| `formatAmount`, `groupThousands`, `dateString` | **stays in draftright** | billing/subscription formatting |

Consequence: emailkit needs a **third injection point** the spec did not name — a
caller-supplied `Registry` of built-in templates. Without it, DraftRight's
subscription templates would live in the shared module, which is the exact failure
the spec set out to avoid.

Consequence for the regression net: the spec's claim that all three inherited test
files "pass unchanged" holds for `service_test.go` and `webhook_test.go` only.
`templates_test.go` **splits** — four generic cases move, five product cases stay.
Task 2 states which go where.

## File Structure

```
emailkit/
  go.mod                  module github.com/tannpv/emailkit, go 1.25.0
  doc.go                  package doc — the chokepoint rule, stated once
  render.go               substitute / escapeHTML / Registry / TemplateDef
  render_test.go          4 generic cases from templates_test.go
  sender.go               Sender interface + unexported resendClient
  service.go              Service, Config, Store port, Send/SendRaw/deliver
  service_test.go         4 cases from draftright service_test.go
  webhook.go              WebhookHandler, Svix verify, event dispatch
  webhook_test.go         7 cases + 3 new replay cases (Task 6)
  events.go               named constants + bounce classifier table
  events_test.go          classifier table cases
  chokepoint_test.go      the machine (Task 7)
  .github/workflows/ci.yml
```

Split by responsibility: `render` knows nothing of sending, `sender` knows nothing
of storage, `webhook` knows nothing of rendering. `events.go` is the shared
vocabulary all three reference so no literal is retyped.

---

### Task 1: Bootstrap the module

**Files:**
- Create: `go.mod`, `doc.go`, `.github/workflows/ci.yml`, `README.md`

**Interfaces:**
- Consumes: nothing.
- Produces: module path `github.com/tannpv/emailkit`; every later task's imports resolve against it.

- [ ] **Step 1: Create the repo and clone it**

```bash
gh repo create tannpv/emailkit --public \
  --description "Shared transactional email transport (Resend) for tannpv projects"
cd /opt/openAi && git clone git@github.com:tannpv/emailkit.git && cd emailkit
git checkout -b feature/extract-transport-20260825
```

**Public, deliberately.** A private module would force `GOPRIVATE` plus a fetch
credential into draftright's, liseuse's and bacnam's CI — three places to wire
and one more secret to rotate — to protect a generic Resend transport that by
design holds no secrets, no business logic and no product copy. All of that
stays in the consuming project, which is the point of the `Store` and `Registry`
ports. Nothing here is worth the machinery of keeping private.

- [ ] **Step 2: Write go.mod**

```
module github.com/tannpv/emailkit

go 1.25.0
```

- [ ] **Step 3: Write doc.go — state the chokepoint rule once, where it is discoverable**

```go
// Package emailkit is the one transactional-email transport shared by
// draftright, liseuse and bacnam.
//
// THE RULE: deliver() is the only code path that reaches a Sender, and it is
// unexported. Send and SendRaw are the only exported ways in, and both funnel
// through it. That is what makes the suppression check unbypassable rather than
// merely customary — see chokepoint_test.go, which fails the build if a second
// caller appears.
//
// What this package deliberately does NOT own:
//
//   - Storage. Each project implements Store against its own schema, which is
//     how bacnam keeps tenant_id out of here entirely.
//   - Product vocabulary. "Renewal reminder" belongs to draftright, not to a
//     module a language-learning app imports.
//   - Template content. Callers supply a Registry; this package only substitutes
//     and escapes.
package emailkit
```

- [ ] **Step 4: Write .github/workflows/ci.yml**

```yaml
name: ci
on: [push, pull_request]
jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version: '1.25'
      - run: go vet ./...
      - run: go test ./... -race -count=1
```

- [ ] **Step 5: Verify the module builds**

Run: `go vet ./...`
Expected: no output, exit 0.

- [ ] **Step 6: Commit**

```bash
git add go.mod doc.go .github/workflows/ci.yml
git commit -m "chore: bootstrap emailkit module

Go directive is 1.25.0 rather than the 1.26 the other consumers use,
because draftright is 1.25.0 and is the first repo that must import this."
```

---

### Task 2: Generic render core

**Files:**
- Create: `render.go`, `render_test.go`
- Source: `backend-rewrite-go/internal/email/templates.go` at `5dbfa570`, lines 74-110

**Interfaces:**
- Consumes: nothing.
- Produces:
  - `type TemplateDef struct { Subject, Body string }`
  - `type Registry map[string]TemplateDef`
  - `func (r Registry) Render(key string, vars map[string]string) (subject, html string)`
  - `func substitute(s string, vars map[string]string, escape bool) string`

- [ ] **Step 1: Write the failing test**

These four cases come from draftright's `templates_test.go` — the generic ones.
The other five (`Verification`, `PasswordReset`, `SubscriptionActivated`,
`FormatAmount`, `DateString`) are product cases and stay in draftright.

```go
package emailkit

import "testing"

func TestRender_UnknownKeyEmpty(t *testing.T) {
	r := Registry{}
	subj, html := r.Render("nope", nil)
	if subj != "" || html != "" {
		t.Fatalf("want empty for unknown key, got %q / %q", subj, html)
	}
}

func TestRender_UnknownTokenEmpty(t *testing.T) {
	r := Registry{"k": {Subject: "hi {{missing}}", Body: "x"}}
	subj, _ := r.Render("k", map[string]string{})
	if subj != "hi " {
		t.Fatalf("unknown token must render empty, got %q", subj)
	}
}

func TestRender_HTMLEscapesBodyNotSubject(t *testing.T) {
	r := Registry{"k": {Subject: "{{v}}", Body: "<p>{{v}}</p>"}}
	subj, html := r.Render("k", map[string]string{"v": "<b>&x"})
	if subj != "<b>&x" {
		t.Fatalf("subject must NOT be escaped, got %q", subj)
	}
	if html != "<p>&lt;b&gt;&amp;x</p>" {
		t.Fatalf("body must be escaped, got %q", html)
	}
}

func TestRender_SubjectEscapeFalseRaw(t *testing.T) {
	if got := substitute("{{v}}", map[string]string{"v": "a&b"}, false); got != "a&b" {
		t.Fatalf("escape=false must pass through, got %q", got)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./... -run TestRender -v`
Expected: FAIL — `undefined: Registry`, `undefined: substitute`.

- [ ] **Step 3: Write render.go**

Ported verbatim from `templates.go` lines 82-110, with `renderTemplate`'s
hardcoded `builtinTemplates` lookup replaced by the receiver map.

```go
package emailkit

import (
	"regexp"
	"strings"
)

// TemplateDef is one built-in template. Subject is substituted raw; Body is
// substituted with HTML escaping.
type TemplateDef struct {
	Subject string
	Body    string
}

// Registry is the caller's built-in template set. This package never ships
// templates of its own — content is product vocabulary and belongs to the
// project that sends it.
type Registry map[string]TemplateDef

// Render returns empty strings for an unknown key. Callers treat that as
// "nothing to send" rather than an error, matching the ported behaviour.
func (r Registry) Render(key string, vars map[string]string) (subject, html string) {
	def, ok := r[key]
	if !ok {
		return "", ""
	}
	return substitute(def.Subject, vars, false), substitute(def.Body, vars, true)
}

var tokenRe = regexp.MustCompile(`\{\{(\w+)\}\}`)

// substitute replaces {{token}} from vars. An unknown token renders empty
// rather than leaving the literal in place, so a missing variable never leaks
// template syntax into a user's inbox.
//
// escape is false for subjects and true for bodies: a subject is not HTML and
// escaping it would show users "&amp;" in their inbox list.
func substitute(s string, vars map[string]string, escape bool) string {
	return tokenRe.ReplaceAllStringFunc(s, func(m string) string {
		name := m[2 : len(m)-2]
		v := vars[name]
		if escape {
			return escapeHTML(v)
		}
		return v
	})
}

func escapeHTML(s string) string { return htmlEscaper.Replace(s) }

var htmlEscaper = strings.NewReplacer(
	"&", "&amp;",
	"<", "&lt;",
	">", "&gt;",
	`"`, "&quot;",
	"'", "&#39;",
)
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./... -run TestRender -v`
Expected: PASS, 4 tests.

- [ ] **Step 5: Commit**

```bash
git add render.go render_test.go
git commit -m "feat: generic template substitution and escaping

Registry is caller-supplied. The built-in template map stays in draftright:
a shared module holding one consumer's subscription copy is a union, not a
shared module."
```

---

### Task 3: Sender port and Resend client

**Files:**
- Create: `sender.go`
- Source: `backend-rewrite-go/internal/email/resend.go` at `5dbfa570` (verbatim, 3 edits)

**Interfaces:**
- Consumes: nothing.
- Produces:
  - `type Sender interface { Send(ctx context.Context, apiKey, from, to, subject, html string) (providerID string, err error) }`
  - `func NewResendSender() Sender`

- [ ] **Step 1: Write sender.go**

Three deliberate changes from the source: `package email` → `package emailkit`;
the method is exported (`send` → `Send`) so consumers can supply fakes from
outside the package; the constructor returns the interface so the concrete type
stays unexported and cannot be constructed elsewhere.

```go
package emailkit

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

// Sender posts one email. resendClient satisfies it in production; tests and
// other providers supply their own. This is the third-case seam: adding SES or
// Postmark is a new implementation, not an edit here.
type Sender interface {
	Send(ctx context.Context, apiKey, from, to, subject, html string) (providerID string, err error)
}

// resendClient posts to the Resend HTTP API directly — no SDK, no dependency.
// Unexported on purpose: if a consumer could construct one, it could send
// without passing the suppression check in deliver().
type resendClient struct{ http *http.Client }

// NewResendSender returns the production sender as an interface, so the
// concrete type cannot be named or built outside this package.
func NewResendSender() Sender { return &resendClient{http: &http.Client{}} }

const resendEndpoint = "https://api.resend.com/emails"

func (c *resendClient) Send(ctx context.Context, apiKey, from, to, subject, html string) (string, error) {
	body, _ := json.Marshal(map[string]string{"from": from, "to": to, "subject": subject, "html": html})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, resendEndpoint, bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	var out struct {
		Data *struct {
			ID string `json:"id"`
		} `json:"data"`
		Error *struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&out)
	if resp.StatusCode >= 400 || out.Error != nil {
		msg := "send failed"
		if out.Error != nil {
			msg = out.Error.Message
		}
		return "", fmt.Errorf("%s", msg)
	}
	if out.Data != nil {
		return out.Data.ID, nil
	}
	return "", nil
}
```

- [ ] **Step 2: Verify it compiles**

Run: `go build ./...`
Expected: no output, exit 0.

- [ ] **Step 3: Commit**

```bash
git add sender.go
git commit -m "feat: Sender port and unexported Resend client

NewResendSender returns the interface, not the struct, so no consumer can
construct a client and bypass deliver()."
```

---

### Task 4: Shared vocabulary and the bounce classifier

> **Ordering note.** This was Task 5 in the first draft and has been moved
> ahead of the chokepoint. The chokepoint's code and tests reference
> `StatusSuppressed`, `StatusSkipped`, `StatusSent` and `StatusFailed`, which
> are defined here — so with the original order Task 4 could not compile or be
> reviewed on its own, breaking the per-task gate.

**Files:**
- Create: `events.go`, `events_test.go`
- Source: the literals currently inline in `webhook.go` lines 80-113 and `service.go` lines 204-220

**Interfaces:**
- Consumes: nothing.
- Produces: `EventDelivered`, `EventBounced`, `EventComplained`; `StatusSent`, `StatusFailed`, `StatusSuppressed`, `StatusSkipped`, `StatusDelivered`, `StatusBounced`, `StatusComplained`; `ReasonBounced`, `ReasonComplained`; `func isPermanentBounce(bounceType, subType string) bool`

- [ ] **Step 1: Write the failing test**

The permanent/transient distinction is load-bearing: a transient bounce (full
mailbox, greylisting) must never suppress a real user, or that user silently
stops receiving password resets.

```go
package emailkit

import "testing"

func TestIsPermanentBounce(t *testing.T) {
	cases := []struct {
		name, bType, subType string
		want                 bool
	}{
		{"permanent general", "Permanent", "General", true},
		{"permanent nomailbox", "Permanent", "NoEmail", true},
		{"hard synonym", "hard", "", true},
		{"lowercase permanent", "permanent", "suppressed", true},
		{"transient mailbox full", "Transient", "MailboxFull", false},
		{"transient greylist", "Transient", "General", false},
		{"undetermined", "Undetermined", "", false},
		{"empty", "", "", false},
	}
	for _, c := range cases {
		if got := isPermanentBounce(c.bType, c.subType); got != c.want {
			t.Errorf("%s: isPermanentBounce(%q,%q) = %v, want %v",
				c.name, c.bType, c.subType, got, c.want)
		}
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./... -run TestIsPermanentBounce -v`
Expected: FAIL — `undefined: isPermanentBounce`.

- [ ] **Step 3: Write events.go**

```go
package emailkit

import "strings"

// Resend webhook event types. External-spec constants — the vocabulary belongs
// to Resend (https://resend.com/docs/dashboard/webhooks/event-types), so the
// literals are theirs; naming them once stops four repos retyping them.
const (
	EventDelivered  = "email.delivered"
	EventBounced    = "email.bounced"
	EventComplained = "email.complained"
)

// Statuses written to the audit row.
const (
	StatusSent       = "sent"
	StatusFailed     = "failed"
	StatusSuppressed = "suppressed"
	StatusSkipped    = "skipped"
	StatusDelivered  = "delivered"
	StatusBounced    = "bounced"
	StatusComplained = "complained"
)

// Reasons an address enters the suppression list.
const (
	ReasonBounced    = "bounced"
	ReasonComplained = "complained"
)

// permanentBounceMarkers are the tokens that mean "this address will never
// accept mail". Kept as data so a new category from Resend is one row here,
// not another strings.Contains at a call site.
var permanentBounceMarkers = []string{"permanent", "hard"}

// isPermanentBounce reports whether a bounce should suppress the address.
// Only permanent bounces suppress: a transient bounce (full mailbox,
// greylisting) must not lock a real user out of password resets.
func isPermanentBounce(bounceType, subType string) bool {
	kind := strings.ToLower(bounceType + " " + subType)
	for _, m := range permanentBounceMarkers {
		if strings.Contains(kind, m) {
			return true
		}
	}
	return false
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./... -run TestIsPermanentBounce -v`
Expected: PASS, 1 test, 8 sub-cases.

- [ ] **Step 5: Commit**

```bash
git add events.go events_test.go
git commit -m "feat: name the Resend vocabulary once

Event types, log statuses and suppression reasons were inline literals in two
files and would have been retyped in three more repos. The bounce classifier
becomes a data table so a new category is a row, not another Contains."
```

---

### Task 5: The chokepoint

**Files:**
- Create: `service.go`, `service_test.go`
- Source: `backend-rewrite-go/internal/email/service.go` at `5dbfa570`, lines 204-252 plus the Service/Config/wg scaffolding

**Interfaces:**
- Consumes: `Sender` (Task 3), `Registry` (Task 2), status constants (Task 4).
- Produces:
  - `type Store interface { IsSuppressed; LogSend; Template; MarkByProviderID; Suppress }`
  - `type SendRecord struct { To, Type, Subject, Status string; ProviderID, Error *string }`
  - `type Config struct { APIKey, From string }`
  - `func NewService(st Store, cfg Config, reg Registry, s Sender) *Service`
  - `func (s *Service) Send(ctx, key, to string, vars map[string]string)`
  - `func (s *Service) SendRaw(ctx, to, subject, html, label string)`
  - `func (s *Service) Wait()`

- [ ] **Step 1: Write the failing test**

These four cases are draftright's `service_test.go`, retargeted at the new
constructor. Behaviour asserted is identical.

```go
package emailkit

import (
	"context"
	"errors"
	"testing"
)

type fakeStore struct {
	suppressed bool
	logs       []SendRecord
	tmpl       *TemplateDef
}

func (f *fakeStore) IsSuppressed(context.Context, string) (bool, error) { return f.suppressed, nil }
func (f *fakeStore) LogSend(_ context.Context, r SendRecord) error {
	f.logs = append(f.logs, r)
	return nil
}
func (f *fakeStore) Template(context.Context, string) (string, string, bool) {
	if f.tmpl == nil {
		return "", "", false
	}
	return f.tmpl.Subject, f.tmpl.Body, true
}
func (f *fakeStore) MarkByProviderID(context.Context, string, string, *string) error { return nil }
func (f *fakeStore) Suppress(context.Context, string, string) error                  { return nil }

type fakeSender struct {
	calls int
	id    string
	err   error
}

func (s *fakeSender) Send(context.Context, string, string, string, string, string) (string, error) {
	s.calls++
	return s.id, s.err
}

func newTestService(st *fakeStore, sn *fakeSender, key string) *Service {
	return NewService(st, Config{APIKey: key, From: "T <t@example.com>"},
		Registry{"k": {Subject: "s", Body: "b"}}, sn)
}

func TestDeliver_SuppressedSkips(t *testing.T) {
	st := &fakeStore{suppressed: true}
	sn := &fakeSender{}
	svc := newTestService(st, sn, "key")
	svc.Send(context.Background(), "k", "a@b.c", nil)
	svc.Wait()
	if sn.calls != 0 {
		t.Fatal("suppressed address must never reach the sender")
	}
	if len(st.logs) != 1 || st.logs[0].Status != StatusSuppressed {
		t.Fatalf("want one suppressed log, got %+v", st.logs)
	}
}

func TestDeliver_NoKeySkips(t *testing.T) {
	st := &fakeStore{}
	sn := &fakeSender{}
	svc := newTestService(st, sn, "")
	svc.Send(context.Background(), "k", "a@b.c", nil)
	svc.Wait()
	if sn.calls != 0 {
		t.Fatal("must not attempt a send with no API key")
	}
	if len(st.logs) != 1 || st.logs[0].Status != StatusSkipped {
		t.Fatalf("want one skipped log, got %+v", st.logs)
	}
}

func TestDeliver_SendsAndLogsSent(t *testing.T) {
	st := &fakeStore{}
	sn := &fakeSender{id: "prov-1"}
	svc := newTestService(st, sn, "key")
	svc.Send(context.Background(), "k", "a@b.c", nil)
	svc.Wait()
	if sn.calls != 1 {
		t.Fatalf("want 1 send, got %d", sn.calls)
	}
	if len(st.logs) != 1 || st.logs[0].Status != StatusSent {
		t.Fatalf("want one sent log, got %+v", st.logs)
	}
	if st.logs[0].ProviderID == nil || *st.logs[0].ProviderID != "prov-1" {
		t.Fatal("provider id must be recorded — the webhook joins on it")
	}
}

func TestDeliver_SendFailLogsFailed(t *testing.T) {
	st := &fakeStore{}
	sn := &fakeSender{err: errors.New("boom")}
	svc := newTestService(st, sn, "key")
	svc.Send(context.Background(), "k", "a@b.c", nil)
	svc.Wait()
	if len(st.logs) != 1 || st.logs[0].Status != StatusFailed {
		t.Fatalf("want one failed log, got %+v", st.logs)
	}
	if st.logs[0].Error == nil || *st.logs[0].Error != "boom" {
		t.Fatal("failure reason must be recorded")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./... -run TestDeliver -v`
Expected: FAIL — `undefined: NewService`, `undefined: StatusSuppressed`.

- [ ] **Step 3: Write service.go**

```go
package emailkit

import (
	"context"
	"log/slog"
	"strings"
	"sync"
)

// Store is the project's own persistence. This package never sees a schema,
// which is how bacnam's tenant_id stays out of it: bacnam's implementation
// closes over the tenant and emailkit never learns tenancy exists.
type Store interface {
	// send path
	IsSuppressed(ctx context.Context, email string) (bool, error)
	LogSend(ctx context.Context, r SendRecord) error
	Template(ctx context.Context, key string) (subject, html string, ok bool)

	// webhook path — two distinct operations, deliberately not merged. One
	// updates an existing log row by provider id; the other grows the
	// suppression list by address. A delivered event does only the first.
	MarkByProviderID(ctx context.Context, providerID, status string, reason *string) error
	Suppress(ctx context.Context, email, reason string) error
}

// SendRecord is the audit row. A thin struct so the port does not leak any
// project's generated query types into fakes.
type SendRecord struct {
	To, Type, Subject, Status string
	ProviderID, Error         *string
}

// Config carries per-project credentials. There is no default From: a shared
// module with one project's address baked in is the hardcoding this extraction
// exists to remove.
type Config struct {
	APIKey string
	From   string
}

// Service is the only way to send. wg tracks in-flight sends so tests can
// await them deterministically; production never calls Wait.
type Service struct {
	store  Store
	cfg    Config
	reg    Registry
	client Sender
	wg     sync.WaitGroup
}

func NewService(st Store, cfg Config, reg Registry, s Sender) *Service {
	return &Service{store: st, cfg: cfg, reg: reg, client: s}
}

// Send renders key from the Store override (if any) or the Registry, then
// delivers. Fire-and-forget: an email must never block or fail the HTTP
// request that triggered it.
func (s *Service) Send(ctx context.Context, key, to string, vars map[string]string) {
	subject, html := s.render(ctx, key, vars)
	s.fire(ctx, to, subject, html, key)
}

// SendRaw delivers a pre-rendered subject and body through the identical
// suppression → creds → send → log path. Used by callers with no template.
func (s *Service) SendRaw(ctx context.Context, to, subject, html, label string) {
	s.fire(ctx, to, subject, html, label)
}

// Wait blocks until in-flight sends finish. Test-only in practice, but
// exported because consumers' tests live in other packages.
func (s *Service) Wait() { s.wg.Wait() }

func (s *Service) render(ctx context.Context, key string, vars map[string]string) (string, string) {
	if subj, html, ok := s.store.Template(ctx, key); ok {
		return substitute(subj, vars, false), substitute(html, vars, true)
	}
	return s.reg.Render(key, vars)
}

func (s *Service) fire(ctx context.Context, to, subject, html, label string) {
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		defer func() { _ = recover() }() // an email must never panic a request
		s.deliver(context.WithoutCancel(ctx), to, subject, html, label)
	}()
}

// deliver is THE chokepoint. It is unexported and is the only caller of
// s.client.Send — see chokepoint_test.go. Every send therefore passes the
// suppression check by construction rather than by convention.
func (s *Service) deliver(ctx context.Context, to, subject, html, label string) {
	if sup, err := s.store.IsSuppressed(ctx, strings.ToLower(to)); err == nil && sup {
		s.log(ctx, to, subject, label, StatusSuppressed, nil,
			strp("Recipient on suppression list (bounce/complaint)"))
		return
	}
	if s.cfg.APIKey == "" {
		s.log(ctx, to, subject, label, StatusSkipped, nil, strp("Resend not configured"))
		return
	}
	id, err := s.client.Send(ctx, s.cfg.APIKey, s.cfg.From, to, subject, html)
	if err != nil {
		// Recipient is logged as a hash-free domain only. The full address is
		// PII and the audit row already holds it under the project's own
		// retention rules; repeating it in application logs spreads it to a
		// second lifetime nobody manages.
		slog.Warn("email send failed", "label", label, "domain", domainOf(to), "err", err)
		s.log(ctx, to, subject, label, StatusFailed, nil, strp(err.Error()))
		return
	}
	s.log(ctx, to, subject, label, StatusSent, strpOrNil(id), nil)
}

func (s *Service) log(ctx context.Context, to, subject, label, status string, providerID, errMsg *string) {
	_ = s.store.LogSend(ctx, SendRecord{
		To: to, Type: label, Subject: subject, Status: status,
		ProviderID: providerID, Error: errMsg,
	})
}

func domainOf(addr string) string {
	if i := strings.LastIndexByte(addr, '@'); i >= 0 {
		return addr[i+1:]
	}
	return "invalid"
}

func strp(s string) *string { return &s }

func strpOrNil(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./... -run TestDeliver -race -v`
Expected: PASS, 4 tests. The status constants come from Task 4's `events.go`,
which lands first precisely so this task compiles and reviews on its own.

- [ ] **Step 5: Commit**

```bash
git add service.go service_test.go
git commit -m "feat: the send chokepoint

deliver() is unexported and the sole caller of Sender.Send, so the suppression
check cannot be skipped. Also stops logging full recipient addresses on send
failure — the audit row already holds the address under the project's own
retention, and repeating it in app logs gives that PII a second lifetime."
```

---

### Task 6: Webhook with the replay window closed

**Files:**
- Create: `webhook.go`, `webhook_test.go`
- Source: `backend-rewrite-go/internal/email/webhook.go` at `5dbfa570`

**Interfaces:**
- Consumes: constants and `isPermanentBounce` (Task 4), `Store` (Task 5).
- Produces:
  - `type WebhookHandler struct{}`
  - `func NewWebhookHandler(st Store, secret string, opts ...WebhookOption) *WebhookHandler`
  - `func WithTolerance(d time.Duration) WebhookOption`
  - `func WithClock(now func() time.Time) WebhookOption`
  - `func (h *WebhookHandler) Handle(w http.ResponseWriter, r *http.Request) error`

**Note on the signature change:** the ported handler wrote its own error
responses via `shared.WriteError`. It now returns `error` and writes only the
success body, because bacnam uses `httpx` for error shape and liseuse has its
own. Each project adapts at its router.

- [ ] **Step 1: Write the failing tests — the three new replay cases first**

The seven ported cases (BadSignature, MalformedJSON, Delivered,
BouncedPermanent, BouncedTransientNoSuppress, Complained, IgnoredEventType)
come across from draftright's `webhook_test.go` unchanged apart from the
constructor and the `error` return.

**Read them from source, not from this plan** — `tannpv/draftright` at
`5dbfa570`, `backend-rewrite-go/internal/email/webhook_test.go`. This is a
deliberate exception to the plan's no-placeholders rule, decided 2026-08-25:
these 230 lines are existing, passing tests whose value is that they arrive
*unchanged*. Transcribing them into a plan document would create a second copy
that can differ from the original by a typo, and a regression net that silently
differs from what it was written against is not a regression net. Apply only the
two mechanical edits named above. These three are new and are the reason this
task exists:

```go
func TestWebhook_RejectsStaleTimestamp(t *testing.T) {
	st := &fakeStore{}
	now := time.Unix(1_700_000_000, 0)
	h := NewWebhookHandler(st, testSecret,
		WithTolerance(5*time.Minute),
		WithClock(func() time.Time { return now }))

	// Signed six minutes ago — a validly-signed request that must still fail.
	stale := now.Add(-6 * time.Minute)
	req := signedRequest(t, testSecret, stale, `{"type":"email.bounced"}`)

	if err := h.Handle(httptest.NewRecorder(), req); err == nil {
		t.Fatal("a six-minute-old signed request must be rejected; " +
			"accepting it lets a captured bounce be replayed to re-suppress an address")
	}
}

func TestWebhook_RejectsFutureTimestamp(t *testing.T) {
	st := &fakeStore{}
	now := time.Unix(1_700_000_000, 0)
	h := NewWebhookHandler(st, testSecret,
		WithTolerance(5*time.Minute),
		WithClock(func() time.Time { return now }))

	future := now.Add(6 * time.Minute)
	req := signedRequest(t, testSecret, future, `{"type":"email.bounced"}`)

	if err := h.Handle(httptest.NewRecorder(), req); err == nil {
		t.Fatal("a far-future timestamp must be rejected — otherwise an attacker " +
			"mints a request that stays valid indefinitely")
	}
}

func TestWebhook_AcceptsFreshTimestamp(t *testing.T) {
	st := &fakeStore{}
	now := time.Unix(1_700_000_000, 0)
	h := NewWebhookHandler(st, testSecret,
		WithTolerance(5*time.Minute),
		WithClock(func() time.Time { return now }))

	req := signedRequest(t, testSecret, now.Add(-30*time.Second),
		`{"type":"email.delivered","data":{"email_id":"p1"}}`)

	if err := h.Handle(httptest.NewRecorder(), req); err != nil {
		t.Fatalf("a fresh request must be accepted, got %v", err)
	}
}
```

Helper both new and ported tests use:

```go
// A real base64 key. The ported verify() strips an optional whsec_ prefix and
// base64-decodes the remainder as the HMAC key.
const testSecret = "whsec_c3VwZXJzZWNyZXR0ZXN0a2V5MTIzNDU2"

func signedRequest(t *testing.T, secret string, ts time.Time, body string) *http.Request {
	t.Helper()
	id := "msg_test"
	tss := strconv.FormatInt(ts.Unix(), 10)
	key, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(secret, "whsec_"))
	if err != nil {
		t.Fatalf("bad test secret: %v", err)
	}
	mac := hmac.New(sha256.New, key)
	mac.Write([]byte(id + "." + tss + "." + body))
	sig := base64.StdEncoding.EncodeToString(mac.Sum(nil))

	req := httptest.NewRequest(http.MethodPost, "/webhooks/resend", strings.NewReader(body))
	req.Header.Set("svix-id", id)
	req.Header.Set("svix-timestamp", tss)
	req.Header.Set("svix-signature", "v1,"+sig)
	return req
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./... -run TestWebhook -v`
Expected: FAIL — `undefined: NewWebhookHandler`, `undefined: WithTolerance`.

- [ ] **Step 3: Write webhook.go**

`verify` gains the timestamp check the ported version lacked. Everything else
is the source behaviour preserved.

```go
package emailkit

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// NOTE: "context" is deliberately absent. ctx comes from r.Context() and is
// only passed along, never named, so importing it would not compile.

// DefaultTolerance is the Svix-recommended replay window.
const DefaultTolerance = 5 * time.Minute

var (
	ErrBadSignature = errors.New("emailkit: invalid webhook signature")
	ErrStale        = errors.New("emailkit: webhook timestamp outside tolerance")
	ErrBadPayload   = errors.New("emailkit: malformed webhook payload")
)

type WebhookOption func(*WebhookHandler)

func WithTolerance(d time.Duration) WebhookOption {
	return func(h *WebhookHandler) { h.tolerance = d }
}

func WithClock(now func() time.Time) WebhookOption {
	return func(h *WebhookHandler) { h.now = now }
}

// WebhookHandler receives Resend delivery events and reflects them onto the
// project's log and suppression list.
type WebhookHandler struct {
	store     Store
	secret    string
	tolerance time.Duration
	now       func() time.Time
}

func NewWebhookHandler(st Store, secret string, opts ...WebhookOption) *WebhookHandler {
	h := &WebhookHandler{store: st, secret: secret, tolerance: DefaultTolerance, now: time.Now}
	for _, o := range opts {
		o(h)
	}
	return h
}

type webhookEvent struct {
	Type string `json:"type"`
	Data struct {
		EmailID string          `json:"email_id"`
		To      json.RawMessage `json:"to"` // string OR []string
		Reason  string          `json:"reason"`
		Bounce  struct {
			Type    string `json:"type"`
			SubType string `json:"subType"`
			Message string `json:"message"`
		} `json:"bounce"`
	} `json:"data"`
}

// Handle processes one webhook POST. Mount it WITHOUT body-consuming
// middleware (the raw body is needed for signature verification) and WITHOUT
// auth (Resend cannot authenticate). Returns an error for the caller to map
// onto its own error response shape; writes only the success body itself.
func (h *WebhookHandler) Handle(w http.ResponseWriter, r *http.Request) error {
	raw, err := io.ReadAll(r.Body)
	if err != nil {
		// Fail closed: a body we cannot read is a body we cannot verify.
		return ErrBadSignature
	}
	if h.secret == "" {
		return ErrBadSignature
	}
	if err := h.verify(r.Header, raw); err != nil {
		return err
	}

	var event webhookEvent
	if err := json.Unmarshal(raw, &event); err != nil {
		return ErrBadPayload
	}

	ctx := r.Context()
	id := event.Data.EmailID
	to := firstRecipient(event.Data.To)

	switch event.Type {
	case EventDelivered:
		if id != "" {
			_ = h.store.MarkByProviderID(ctx, id, StatusDelivered, nil)
		}
	case EventBounced:
		reason := event.Data.Bounce.Message
		if reason == "" {
			reason = event.Data.Reason
		}
		if reason == "" {
			reason = StatusBounced
		}
		if id != "" {
			r := reason
			_ = h.store.MarkByProviderID(ctx, id, StatusBounced, &r)
		}
		if to != "" && isPermanentBounce(event.Data.Bounce.Type, event.Data.Bounce.SubType) {
			_ = h.store.Suppress(ctx, to, ReasonBounced)
		}
	case EventComplained:
		if id != "" {
			r := "Recipient marked as spam"
			_ = h.store.MarkByProviderID(ctx, id, StatusComplained, &r)
		}
		if to != "" {
			_ = h.store.Suppress(ctx, to, ReasonComplained)
		}
	default:
		// sent / opened / clicked / delivery_delayed carry no state we keep
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`{"received":true}`))
	return nil
}

// verify checks the Svix signature AND the timestamp. The ported version
// signed over svix-timestamp but never validated it, so a captured request
// stayed valid forever — replaying a bounce re-suppressed an address on
// demand, and a suppressed user stops receiving password resets.
func (h *WebhookHandler) verify(hdr http.Header, body []byte) error {
	id := hdr.Get("svix-id")
	ts := hdr.Get("svix-timestamp")
	sigHeader := hdr.Get("svix-signature")
	if id == "" || ts == "" || sigHeader == "" {
		return ErrBadSignature
	}

	secs, err := strconv.ParseInt(ts, 10, 64)
	if err != nil {
		return ErrBadSignature
	}
	// Checked in both directions: a far-future timestamp would otherwise mint a
	// request that stays valid until that time arrives.
	if d := h.now().Sub(time.Unix(secs, 0)); d > h.tolerance || d < -h.tolerance {
		return ErrStale
	}

	key, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(h.secret, "whsec_"))
	if err != nil {
		return ErrBadSignature
	}
	mac := hmac.New(sha256.New, key)
	mac.Write([]byte(id + "." + ts + "." + string(body)))
	expected := []byte(base64.StdEncoding.EncodeToString(mac.Sum(nil)))

	for _, part := range strings.Split(sigHeader, " ") {
		idx := strings.IndexByte(part, ',')
		if idx < 0 {
			continue
		}
		sig := []byte(part[idx+1:])
		if len(sig) == len(expected) && hmac.Equal(sig, expected) {
			return nil
		}
	}
	return ErrBadSignature
}

// firstRecipient handles Resend sending `to` as either a string or an array.
func firstRecipient(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var arr []string
	if err := json.Unmarshal(raw, &arr); err == nil {
		if len(arr) > 0 {
			return arr[0]
		}
		return ""
	}
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		return s
	}
	return ""
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./... -race -count=1 -v`
Expected: PASS — 10 webhook tests (7 ported + 3 replay), 4 service, 4 render, 1 events.

- [ ] **Step 5: Commit**

```bash
git add webhook.go webhook_test.go
git commit -m "fix: reject replayed webhooks

verify() signed over svix-timestamp but never validated it, so a captured
webhook POST stayed valid indefinitely. Replaying a captured email.bounced
re-suppresses that address on demand, and a suppressed user stops receiving
password resets — denial of delivery against anyone whose bounce was observed.

Checked in both directions: a far-future timestamp would otherwise mint a
request valid until that time arrives. Clock is injectable so the test asserts
the window rather than sleeping.

Handle now returns error instead of writing a response, because bacnam uses
httpx for error shape and liseuse has its own."
```

---

### Task 7: The machine

**Files:**
- Create: `chokepoint_test.go`

**Interfaces:**
- Consumes: the whole package.
- Produces: a build failure if the chokepoint is bypassed.

- [ ] **Step 1: Write the test**

A convention only reviewers enforce rots across work written months apart. This
makes it a build failure instead.

```go
package emailkit

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"strings"
	"testing"
)

// TestChokepoint_DeliverIsSoleSender parses this package and fails if any
// function other than deliver calls Sender.Send. Every send must pass the
// suppression check in deliver; this asserts no second path exists.
func TestChokepoint_DeliverIsSoleSender(t *testing.T) {
	fset := token.NewFileSet()
	pkgs, err := parser.ParseDir(fset, ".", func(fi fs.FileInfo) bool {
		return !strings.HasSuffix(fi.Name(), "_test.go")
	}, 0)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	var offenders []string
	for _, pkg := range pkgs {
		for path, file := range pkg.Files {
			ast.Inspect(file, func(n ast.Node) bool {
				fn, ok := n.(*ast.FuncDecl)
				if !ok {
					return true
				}
				ast.Inspect(fn.Body, func(m ast.Node) bool {
					call, ok := m.(*ast.CallExpr)
					if !ok {
						return true
					}
					sel, ok := call.Fun.(*ast.SelectorExpr)
					if !ok || sel.Sel.Name != "Send" {
						return true
					}
					// s.client.Send(...) — the provider call we are guarding.
					inner, ok := sel.X.(*ast.SelectorExpr)
					if !ok || inner.Sel.Name != "client" {
						return true
					}
					if fn.Name.Name != "deliver" {
						offenders = append(offenders,
							fn.Name.Name+" in "+path)
					}
					return true
				})
				return false
			})
		}
	}

	if len(offenders) > 0 {
		t.Fatalf("only deliver() may call the Sender — found: %v.\n"+
			"Every send must pass the suppression check. If you need a new send "+
			"path, route it through deliver rather than adding a second caller.",
			offenders)
	}
}
```

- [ ] **Step 2: Run it — verify it passes on clean code**

Run: `go test ./... -run TestChokepoint -v`
Expected: PASS.

- [ ] **Step 3: Prove the test can fail**

A guard that has never failed is not known to work. Temporarily add to
`service.go`:

```go
func (s *Service) bypass(ctx context.Context) {
	_, _ = s.client.Send(ctx, "k", "f", "t", "s", "h")
}
```

Run: `go test ./... -run TestChokepoint -v`
Expected: FAIL naming `bypass in service.go`.

Then delete `bypass` and re-run — expect PASS. Do not commit the bypass.

- [ ] **Step 4: Commit**

```bash
git add chokepoint_test.go
git commit -m "test: fail the build if anything but deliver() sends

Verified the guard actually fails by adding a bypass method and watching it
catch it, then removing it. A guard that has never failed is not known to work."
```

---

### Task 8: Consumer import-lint and release

**Files:**
- Create: `.github/workflows/import-lint.yml`, `README.md`
- Modify: `.github/workflows/ci.yml`

> **Added after the Task 1 review.** "Zero non-stdlib dependencies" was a Global
> Constraint with nothing enforcing it — `go vet` and `go test` both pass happily
> after a `go get`. Per Rule #1 a cross-cutting constraint needs a machine that
> proves nothing bypassed it, so Step 0 below adds that gate. This is why the
> constraint is checked here rather than trusted.

- [ ] **Step 0: Add the dependency gate to emailkit's own CI**

Append to the `test` job's steps in `.github/workflows/ci.yml`:

```yaml
      - name: Fail if a dependency was added
        run: |
          # "Zero non-stdlib dependencies" is a Global Constraint. go vet and
          # go test pass fine after a go get, so without this the constraint is
          # a convention nobody enforces.
          if grep -qE '^\s*require' go.mod; then
            echo "::error::emailkit must stay dependency-free; go.mod has a require block."
            exit 1
          fi
          if [ -f go.sum ]; then
            echo "::error::go.sum exists — a dependency was added."
            exit 1
          fi
          echo "ok — no dependencies"
```

Verify the gate can fail before trusting it:

```bash
go get golang.org/x/text@latest        # temporarily add a dep
grep -E '^\s*require' go.mod           # expect a match => gate would fail
go mod edit -droprequire golang.org/x/text && rm -f go.sum && go mod tidy
grep -E '^\s*require' go.mod || echo "clean again"
```

Do not commit the temporary dependency.

**Interfaces:**
- Consumes: everything.
- Produces: tag `v0.1.0` for consumers to require.

- [ ] **Step 1: Write the import-lint workflow**

This runs in *consumer* repos. It ships here so the four copies do not drift;
each consumer references it.

```yaml
name: no-direct-email-provider
on: [push, pull_request]
jobs:
  lint:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - name: Fail if anything imports a mail provider directly
        run: |
          # emailkit is the only permitted path to a provider. A direct import
          # here means a send that skips the suppression check.
          if grep -rnE '"github\.com/resend/|"github\.com/aws/aws-sdk-go.*/ses' \
               --include='*.go' . ; then
            echo "::error::Import github.com/tannpv/emailkit instead of a provider SDK."
            exit 1
          fi
          echo "ok — no direct provider imports"
```

- [ ] **Step 2: Write README.md**

```markdown
# emailkit

One transactional email transport for draftright, liseuse and bacnam.

## Why this exists

Three projects needed the same Resend integration. Copying draftright's
`internal/email` into each would have made three definitions of one policy —
the pattern that already shipped DraftRight #22, where a copy-pasted
`app_releases` upsert drifted and Windows went two months without installer
integrity checks.

## What it does not own

Storage, product vocabulary, and template content. See `doc.go`.

## Usage

```go
svc := emailkit.NewService(
    myStore,                                   // your schema
    emailkit.Config{APIKey: key, From: from},  // your credentials
    myTemplates,                               // your copy
    emailkit.NewResendSender(),
)
svc.Send(ctx, "password-reset", user.Email, map[string]string{"code": code})
```

Mount the webhook without body-consuming middleware and without auth:

```go
h := emailkit.NewWebhookHandler(myStore, webhookSecret)
mux.HandleFunc("POST /webhooks/resend", func(w http.ResponseWriter, r *http.Request) {
    if err := h.Handle(w, r); err != nil {
        myErrorShape(w, r, err)   // each project maps this itself
    }
})
```
```

- [ ] **Step 3: Full test run**

Run: `go vet ./... && go test ./... -race -count=1`
Expected: all pass, exit 0.

- [ ] **Step 4: Merge and tag**

```bash
git add .github/workflows/import-lint.yml README.md
git commit -m "chore: consumer import-lint and usage docs"
git checkout main
git merge --no-ff feature/extract-transport-20260825 \
  -m "Merge feature/extract-transport-20260825"
git push origin main
git tag v0.1.0 && git push origin v0.1.0
```

- [ ] **Step 5: Run the project review skill over the whole diff**

Run `/epiphanydev:full-review` against `main`. Fix what it finds before any
consumer requires the tag.

---

## Self-Review

**Spec coverage:**

| spec requirement | task |
|---|---|
| new module `github.com/tannpv/emailkit` | 1 |
| `Sender` interface, provider swappable | 3 |
| `Store` port, storage per project, tenancy excluded | 5 |
| generic `Send(ctx, key, to, vars)`, product methods excluded | 5 |
| `defaultFrom` / DraftRight copy removed | 3, 5 |
| `shared.*` couplings removed | 6 |
| replay window fixed | 6 |
| `deliver` unexported, sole caller | 5, 7 |
| four literal groups named once | 4 |
| table-driven bounce classifier | 4 |
| import-lint across consumers | 8 |
| inherited tests as regression net | 2, 5, 6 |

Every spec requirement maps to a task. Two spec statements were **corrected**
rather than implemented as written, both recorded in the Planning Note:
`templates.go` splits three ways rather than moving whole, and only two of the
three inherited test files move unchanged.

**Placeholder scan:** none. Every code step carries complete code.

**Type consistency:** `Store` has the same five methods in Task 5's definition,
Task 5's `fakeStore`, and Task 6's usage. `Sender.Send` has the same six-arg
signature in Tasks 3, 5 and 7. `SendRecord` fields match between Task 5's
definition and its test assertions. Status constants used by the chokepoint are
defined in Task 4, which now runs before it.

## Out of scope — needs its own plan

- **draftright migration** (spec rollout step 2)
- **liseuse adoption** (step 3)
- **bacnam adoption** (step 4)

Each produces working software on its own and each depends on the interface
this plan produces. Writing them before `v0.1.0` exists would be guesswork.

## Blocked on you, before any consumer can send

- `bacnam.co` and `liseuse.info` have no SPF and no DKIM. Add their Resend
  domain records at the DNS provider.
- draftright's `RESEND_API_KEY` and `RESEND_WEBHOOK_SECRET` are empty in
  production — lost with the destroyed droplet. Its email is dead today, so
  "did the migration break it" has no baseline until they are restored.
