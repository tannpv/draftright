# DraftRight Website

Astro 5 marketing site + web playground. Static-output (`output: "static"`), deployed on draftright.info.

## Tech

- Astro 5 + React 18 islands (TSX components with `client:load` / `client:idle`)
- Tailwind CSS (dark theme, `bg-dark-bg` / `border-dark-border` / `brand-400` tokens)
- Backend URL: `PUBLIC_API_URL` env var (default: `https://api.draftright.info`)

## Pages

| Page | File | Purpose |
|---|---|---|
| Home | `index.astro` | Hero, features, playground island |
| Pricing | `pricing.astro` | Plan cards + subscribe flow |
| Download | `download.astro` | Per-platform download links |
| Sign Up | `signup.astro` | Email registration form |
| Verify Email | `verify-email.astro` | Token-based email confirmation |
| Account | `account.astro` | Logged-in user profile + subscription |
| Checkout | `checkout.astro` | Post-signup payment step |
| Delete Account | `delete-account.astro` | Account deletion confirmation |
| Privacy | `privacy.astro` | Privacy policy |
| Feedback | `feedback.astro` | Public feature-request board (see below) |

## Key Components

- `Nav.astro` — fixed top navbar; links: Features, Pricing, Feedback, Download, Sign up, Try It Free.
- `Playground.tsx` — web rewrite playground (client:load island, `dr_access_token` in localStorage).
- `ReportBugDialog.tsx` — global bug-report modal (client:idle island, `POST /bug-reports` multipart).
- `ReportBugWidget.tsx` — inline trigger that opens the dialog.
- `SuggestFeatureWidget.tsx` — inline "+ Suggest a feature" form; `POST /feedback` JSON.
- `FeedbackBoardCard.tsx` — single feature-request card (title, votes, status badge, platform badge, upvote button).
- `FeedbackBoard.tsx` — full board island: filters (status + target_platform), card list, "Load more" pagination, inline `SuggestFeatureWidget`. Accepts `apiUrl`, `initial`, `initialStatus`, `initialPlatform` props.
- `feedback.astro` — public feature-request board (server-fetches `GET /feedback`, hydrates `FeedbackBoard` island). Voting requires the logged-in user's JWT (`dr_access_token`). Submit form posts to `POST /feedback`. Spec C of the feedback feature.

## Auth Pattern

No server-side sessions. JWT stored in `localStorage` as `dr_access_token`. React islands read it directly; Astro pages pass `apiUrl` as a prop to islands.

## Build & Dev

```bash
cd website
npm run dev    # http://localhost:4000
npm run build  # outputs to dist/
```
