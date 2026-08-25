# emailkit — one transactional email transport for four projects

**Date:** 2026-08-25
**Status:** approved, not yet implemented
**Repos touched:** new `tannpv/emailkit`; `tannpv/draftright`, `tannpv/liseuse`, `tannpv/bacnam`

## Why

bacnam and liseuse both authenticate on `email + password_hash` and have no way to
send mail. bacnam's identity README lists password reset and invitations as
deferred; liseuse has no reset path at all. Both need a transport before either
flow can be built.

draftright already has a complete, working Resend integration — client, Svix-signed
delivery webhook, bounce-driven suppression, template store, send log. The choice is
therefore not *what to build* but *where the one copy lives*.

Copying `internal/email` into two more repos would create three definitions of one
policy. That is the failure mode Rule #1 names, and it has already cost this codebase
once: a copy-pasted `app_releases` upsert drifted between the manual and CI release
paths, `sha256` landed in one copy only, and Windows shipped installers with no
integrity verification for two months (#22).

## Scope

**In:** the transport layer — send, template render, delivery webhook, suppression —
extracted into a shared module and adopted by all four repos.

**Out, deliberately:**

- Password reset and email verification flows. These are new security surface
  (token generation, expiry, single-use, enumeration resistance) and do not belong in
  the same change that introduces a shared module. They land per project afterwards.
- **moncar.** Its identity is phone-primary (Zalo/SMS OTP); `email` is a secondary
  column. Adding email there solves nothing today. Dropping it also means the three
  sending domains — `draftright.info`, `bacnam.co`, `liseuse.info` — fit Resend's
  free tier exactly (3 domains, 3,000/month, 100/day).

## Architecture

`github.com/tannpv/emailkit`, seeded from draftright's code rather than written
fresh. Two consumer-side ports, defined in emailkit, implemented per project:

```go
// Sender posts one email. resendClient (prod) and fakes (test) satisfy it.
type Sender interface {
    Send(ctx context.Context, from, to, subject, html string) (providerID string, err error)
}

// Store is the project's own persistence. emailkit never sees a schema.
type Store interface {
    // send path
    IsSuppressed(ctx context.Context, email string) (bool, error)
    LogSend(ctx context.Context, r SendRecord) error
    Template(ctx context.Context, key string) (subject, html string, ok bool)

    // webhook path — two distinct operations, deliberately not merged:
    // one updates the existing log row by provider id, the other grows the
    // suppression list by address. A delivered event does the first only.
    MarkByProviderID(ctx context.Context, providerID, status string, reason *string) error
    Suppress(ctx context.Context, email, reason string) error
}
```

`Store` is the reason bacnam's `tenant_id` never reaches the shared module: bacnam's
implementation closes over the tenant, and emailkit never learns tenancy exists. Had
emailkit owned the Postgres layer, it would have to model a concept only one of four
consumers has.

`Sender` is an interface for the same reason stated as the third-case test: one more
provider (SES direct, Postmark) is a new implementation, not an edit to the core.

### What moves and what stays

| stays in draftright | moves to emailkit |
|---|---|
| `admin_logs_*`, `admin_templates_*` (~635 lines of admin UI) | `resend.go`, `webhook.go`, the generic half of `templates.go` |
| `repo_pg.go` → becomes its `Store` implementation | the `deliver` / `fire` / `SendRaw` core |
| `SendVerification`, `SendPasswordReset`, `SendRenewalReminder`, `SendSubscriptionActivated`, `SendPaymentFailed`, `SendSubscriptionExpired` | generic `Send(ctx, key, to, vars)` |

The last row matters most. Those six methods are DraftRight *product* vocabulary.
Promoting them would put "renewal reminder" and "payment failed" into a
language-learning app's dependency tree. A shared module that accumulates every
consumer's nouns is not shared — it is a union. They stay in draftright as thin
wrappers over `Send`.

### Couplings removed on the way out

Three ties to draftright internals must be parameterised, because bacnam uses
`httpx` for its error convention and liseuse has its own:

- `shared.WriteError`, `shared.WriteJSON` — the webhook's response writer
- `shared.ISOMillis` — used only by `SendTestEmail`
- `const defaultFrom = "DraftRight <noreply@draftright.info>"` — becomes config
- `SendTestEmail`'s inline HTML carries DraftRight copy — caller supplies the body

## Security fix carried by this work

`verify()` reads `svix-timestamp`, signs over it, and **never validates it**:

```go
id := hdr.Get("svix-id")
ts := hdr.Get("svix-timestamp")     // read...
mac.Write([]byte(id + "." + ts + "." + string(body)))   // ...signed, never checked
```

The timestamp cannot be altered without breaking the signature, but nothing rejects
an old one. A captured webhook POST stays valid indefinitely.

**Impact:** replaying a captured `email.bounced` re-suppresses that address on
demand. A suppressed user stops receiving password resets — denial of delivery
against anyone whose bounce event was ever observed.

The fix is a tolerance window (5 minutes, per Svix's spec), applied once in emailkit.
Copying the current code into three more repos would replicate the hole four times;
this is the shared-module argument made concrete.

## Rule #1 enforcement — the chokepoint and the machine

A convention only reviewers enforce rots across work written months apart, so the
guarantee is structural:

- `deliver()` stays **unexported**. `Send` and `SendRaw` are the only exported paths
  and both funnel through it. Skipping the suppression check is not a discipline
  problem — it is unrepresentable.
- `Sender` is exported so consumers can inject fakes; the concrete `resendClient`
  is **not**. No project can construct one and bypass logging.
- **The machine:** a test in emailkit asserting `deliver` is the sole caller of
  `Sender.Send`, plus a CI import-lint failing any consumer repo that imports a
  Resend SDK directly. Drift fails the build instead of shipping.

### Literals that carry meaning

Four groups become named constants, defined once, with the Resend spec cited:

| group | current form |
|---|---|
| event types | `"email.delivered"`, `"email.bounced"`, `"email.complained"` |
| log statuses | `"delivered"`, `"bounced"`, `"complained"` |
| suppression reasons | `"bounced"`, `"complained"` |
| bounce permanence | `strings.Contains(type+" "+subType, "permanent")` |

The last is a substring match over two concatenated fields. It becomes a
table-driven classifier so a new bounce category is a data row, not another
`Contains`. The permanent/transient distinction is load-bearing and must survive
the move: a transient bounce (full mailbox, greylisting) must never suppress a real
user.

## Configuration

Per project, no shared defaults: `RESEND_API_KEY`, `RESEND_WEBHOOK_SECRET`,
`EMAIL_FROM`.

**Blocking prerequisite.** `bacnam.co` and `liseuse.info` currently have no SPF and
no DKIM. Neither can send until their Resend domain records exist at the DNS
provider — a dashboard action, not something this work can do.

`draftright.info` is already correct and needs nothing:

```
send.draftright.info                SPF   v=spf1 include:dc-fd741b8612._spfm.send.draftright.info ~all
send.draftright.info                MX    feedback-smtp.ap-northeast-1.amazonses.com
resend._domainkey.draftright.info   DKIM  p=MIGfMA0GCSqGSIb3...
_dmarc.draftright.info                    v=DMARC1; p=quarantine; adkim=r; aspf=r
```

The apex `v=spf1 -all` is deliberate: the apex sends nothing, the envelope-from is
the subdomain, and apex DKIM supplies DMARC alignment for `From: noreply@draftright.info`.

Separately and already true in production: draftright's `RESEND_API_KEY` and
`RESEND_WEBHOOK_SECRET` are empty — lost with the destroyed droplet. Email is dead
there today. Restoring them is a prerequisite for step 2 verifying anything.

## Rollout

Each step is its own branch and its own `/epiphanydev:full-review` pass.

1. **emailkit** — extract, fix the replay window, add the enforcement test.
   draftright's `service_test.go` and `webhook_test.go` move and must pass with no
   behavioural change. That is a real discriminator: they were written against the
   current behaviour and will fail if extraction alters it.

   **Corrected during planning:** `templates.go` splits three ways, not two, and
   `templates_test.go` therefore **splits rather than moving**. The generic
   substitution and escaping cases go to emailkit; the five product cases
   (`Verification`, `PasswordReset`, `SubscriptionActivated`, `FormatAmount`,
   `DateString`) stay in draftright alongside `builtinTemplates`, `shell()`,
   `formatAmount`, `groupThousands` and `dateString`. This forces a third injection
   point the spec did not originally name: a caller-supplied template `Registry`.
   Without it, DraftRight's subscription copy would live in the shared module —
   the precise outcome this design exists to prevent.
2. **draftright migrates.** The only consumer with a proven production integration,
   so it validates the module against reality before anything new depends on it.
   Done on `main` in a clean clone — not the host's deployed copy, which is 99
   commits behind and carries five uncommitted local edits.
3. **liseuse**, then **bacnam.** bacnam last: multi-tenancy makes its `Store` the
   most interesting implementation, and by then the interface is settled.

### Risk

Step 2 modifies a production email path that currently works. The regression net is
the inherited test suite passing unchanged. If those tests need edits to pass, the
extraction changed behaviour and must be reworked rather than the tests relaxed.

## Verification

- emailkit unit tests, including the replay-window rejection and the
  sole-caller assertion.
- draftright's three inherited test files, green without modification.
- One smoke-test send per project against its real domain, confirming DKIM
  alignment and a `delivered` webhook landing in that project's own log table.
- Import-lint green across all four repos.

## Follow-up work this unblocks

Password reset and email verification in bacnam and liseuse — separate specs,
separate plans, each with its own token-handling security review.
