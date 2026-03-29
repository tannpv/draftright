# DraftRight Admin Portal — Design Spec

**Date:** 2026-03-29
**Status:** Approved
**Sub-project:** 2 of 4 (Admin Portal)

## Overview

React SPA admin portal for managing DraftRight backend. Provides dashboard stats, user management, plan configuration, and AI provider management. Communicates with the backend API via admin JWT auth.

## Tech Stack

- **Framework:** React 18 + Vite
- **Styling:** Tailwind CSS
- **HTTP:** fetch wrapper with JWT auth headers
- **Routing:** React Router v6
- **State:** React hooks (useState, useEffect) — no Redux needed
- **Deployment:** Static files served by nginx in Docker Compose

## Pages

### 1. Login (`/login`)

- Email + password form
- Calls `POST /auth/login`, stores JWT in localStorage
- Redirects to Dashboard on success
- Shows error on invalid credentials

### 2. Dashboard (`/`)

- 4 stat cards: Total Users, Active Subscriptions, Rewrites Today, Rewrites This Month
- Calls `GET /admin/stats`
- Auto-refresh every 30 seconds

### 3. Users (`/users`)

- Search bar (filters by email/name)
- Paginated table: Email, Name, Plan, Usage Today, Status, Joined
- Click row → User Detail page
- Calls `GET /admin/users?search=&page=&limit=`

### 4. User Detail (`/users/:id`)

- User info (email, name, role, active status)
- Current subscription + plan
- Today's usage count
- Recent usage log table (last 20 rewrites)
- Actions: Toggle active, Change role, Grant subscription
- Calls `GET /admin/users/:id`, `PATCH /admin/users/:id`, `POST /admin/subscriptions/grant`

### 5. Plans (`/plans`)

- Table: Name, Daily Limit, Price, Billing Period, Active
- Create button → modal form
- Edit/Delete buttons per row
- Calls CRUD `GET/POST/PATCH/DELETE /admin/plans`

### 6. AI Providers (`/providers`)

- Table: Name, Type, Endpoint, Model, Default, Active
- Create button → modal form
- Edit/Delete buttons per row
- Test Connection button → calls `POST /admin/ai-providers/:id/test`, shows success/error toast
- Calls CRUD `GET/POST/PATCH/DELETE /admin/ai-providers`

## File Structure

```
admin/
├── index.html
├── package.json
├── vite.config.ts
├── tailwind.config.js
├── postcss.config.js
├── tsconfig.json
├── Dockerfile
├── nginx.conf
└── src/
    ├── main.tsx
    ├── App.tsx
    ├── api.ts                  # fetch wrapper with JWT
    ├── auth.ts                 # login/logout, token storage
    ├── components/
    │   ├── Layout.tsx          # sidebar + header + content area
    │   ├── StatCard.tsx        # dashboard stat card
    │   ├── DataTable.tsx       # reusable paginated table
    │   ├── Modal.tsx           # reusable modal dialog
    │   └── Toast.tsx           # success/error notification
    ├── pages/
    │   ├── LoginPage.tsx
    │   ├── DashboardPage.tsx
    │   ├── UsersPage.tsx
    │   ├── UserDetailPage.tsx
    │   ├── PlansPage.tsx
    │   └── ProvidersPage.tsx
    └── index.css               # Tailwind imports
```

## API Client (`api.ts`)

```typescript
const API_URL = import.meta.env.VITE_API_URL || 'http://localhost:3000';

async function apiFetch(path: string, options?: RequestInit) {
  const token = localStorage.getItem('token');
  const res = await fetch(`${API_URL}${path}`, {
    ...options,
    headers: {
      'Content-Type': 'application/json',
      ...(token ? { Authorization: `Bearer ${token}` } : {}),
      ...options?.headers,
    },
  });
  if (res.status === 401) {
    localStorage.removeItem('token');
    window.location.href = '/login';
  }
  if (!res.ok) throw new Error(await res.text());
  return res.json();
}
```

## Docker Compose Addition

```yaml
  admin:
    build: ./admin
    ports:
      - "3001:80"
    depends_on:
      - backend
```

## Layout

Sidebar navigation with:
- Dashboard (home icon)
- Users
- Plans
- AI Providers
- Logout

Header shows "DraftRight Admin" and current admin email.

## Visual Style

- Clean, minimal Tailwind design
- White background, gray sidebar
- Blue accent color for buttons and active nav
- Responsive but desktop-focused (admin portal used on desktop)
