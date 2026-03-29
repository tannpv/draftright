# DraftRight Admin Portal

React SPA with Modernize dark theme for managing the DraftRight platform.

## Tech

- React 18 + TypeScript + Vite
- Tailwind CSS + Modernize dark theme (CSS variables in `index.css`)
- React Router v6, no state management library (hooks only)

## Pages

| Page | Route | Purpose |
|---|---|---|
| Login | `/login` | Admin email/password → JWT |
| Dashboard | `/` | Stat cards + MRR + revenue + plans breakdown |
| Users | `/users` | Searchable, paginated user table |
| User Detail | `/users/:id` | Info, subscription, usage, transactions, actions |
| Plans | `/plans` | CRUD for subscription plans |
| AI Providers | `/providers` | CRUD + test connection |
| Analytics | `/analytics` | Revenue charts, subscriber trends, churn |
| Transactions | `/transactions` | All subscriptions with store type, status |
| Profile | `/profile` | Change password |

## API Client

`src/api.ts` — fetch wrapper with JWT from localStorage. Auto-redirects to `/login` on 401.
Backend URL: `VITE_API_URL` env var (default: `http://localhost:3000`).

## Design Tokens (Modernize)

- Background: `#202936`, Cards: `#2a3547`, Borders: `#333f55`
- Primary: `#5d87ff`, Secondary: `#49beff`, Success: `#13deb9`
- Warning: `#ffae1f`, Danger: `#fa896b`
- Text: `#eaeff4` (primary), `#7c8fac` (muted)
- Font: Plus Jakarta Sans
- Sidebar: 270px fixed, collapsible

## Commands

```bash
npm run dev    # Dev server at http://localhost:5173
npm run build  # Production build to dist/
```

## Backend Field Names

Backend uses **snake_case** (`usage_today`, `is_active`, `price_cents`, `created_at`). All frontend interfaces must match.
