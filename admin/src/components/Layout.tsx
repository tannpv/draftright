import { useState, useEffect } from 'react';
import { NavLink, Outlet } from 'react-router-dom';
import { verifyBackend } from '../api';
import { logout, getAdminEmail } from '../auth';

/* ── SVG Icons (inline, no external deps) ───────────────── */
function IconDashboard() {
  return (
    <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" strokeLinejoin="round">
      <rect x="3" y="3" width="7" height="7" rx="1" />
      <rect x="14" y="3" width="7" height="7" rx="1" />
      <rect x="3" y="14" width="7" height="7" rx="1" />
      <rect x="14" y="14" width="7" height="7" rx="1" />
    </svg>
  );
}
function IconUsers() {
  return (
    <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" strokeLinejoin="round">
      <path d="M17 21v-2a4 4 0 0 0-4-4H5a4 4 0 0 0-4 4v2" />
      <circle cx="9" cy="7" r="4" />
      <path d="M23 21v-2a4 4 0 0 0-3-3.87" />
      <path d="M16 3.13a4 4 0 0 1 0 7.75" />
    </svg>
  );
}
function IconPlans() {
  return (
    <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" strokeLinejoin="round">
      <path d="M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8z" />
      <polyline points="14 2 14 8 20 8" />
      <line x1="16" y1="13" x2="8" y2="13" />
      <line x1="16" y1="17" x2="8" y2="17" />
      <polyline points="10 9 9 9 8 9" />
    </svg>
  );
}
function IconProviders() {
  return (
    <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" strokeLinejoin="round">
      <rect x="2" y="3" width="20" height="14" rx="2" ry="2" />
      <line x1="8" y1="21" x2="16" y2="21" />
      <line x1="12" y1="17" x2="12" y2="21" />
    </svg>
  );
}
function IconAnalytics() {
  return (
    <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" strokeLinejoin="round">
      <line x1="18" y1="20" x2="18" y2="10" />
      <line x1="12" y1="20" x2="12" y2="4" />
      <line x1="6" y1="20" x2="6" y2="14" />
    </svg>
  );
}
function IconTransactions() {
  return (
    <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" strokeLinejoin="round">
      <path d="M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8z" />
      <polyline points="14 2 14 8 20 8" />
      <line x1="9" y1="13" x2="15" y2="13" />
      <line x1="9" y1="17" x2="13" y2="17" />
      <polyline points="9 9 10 9 11 9" />
    </svg>
  );
}
function IconPayments() {
  return (
    <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" strokeLinejoin="round">
      <rect x="2" y="4" width="20" height="16" rx="2" />
      <path d="M2 10h20" />
      <path d="M6 16h4" />
      <path d="M14 16h4" />
    </svg>
  );
}
function IconSettings() {
  return (
    <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" strokeLinejoin="round">
      <circle cx="12" cy="12" r="3" />
      <path d="M19.4 15a1.65 1.65 0 0 0 .33 1.82l.06.06a2 2 0 0 1 0 2.83 2 2 0 0 1-2.83 0l-.06-.06a1.65 1.65 0 0 0-1.82-.33 1.65 1.65 0 0 0-1 1.51V21a2 2 0 0 1-2 2 2 2 0 0 1-2-2v-.09A1.65 1.65 0 0 0 9 19.4a1.65 1.65 0 0 0-1.82.33l-.06.06a2 2 0 0 1-2.83 0 2 2 0 0 1 0-2.83l.06-.06A1.65 1.65 0 0 0 4.68 15a1.65 1.65 0 0 0-1.51-1H3a2 2 0 0 1-2-2 2 2 0 0 1 2-2h.09A1.65 1.65 0 0 0 4.6 9a1.65 1.65 0 0 0-.33-1.82l-.06-.06a2 2 0 0 1 0-2.83 2 2 0 0 1 2.83 0l.06.06A1.65 1.65 0 0 0 9 4.68a1.65 1.65 0 0 0 1-1.51V3a2 2 0 0 1 2-2 2 2 0 0 1 2 2v.09a1.65 1.65 0 0 0 1 1.51 1.65 1.65 0 0 0 1.82-.33l.06-.06a2 2 0 0 1 2.83 0 2 2 0 0 1 0 2.83l-.06.06A1.65 1.65 0 0 0 19.4 9a1.65 1.65 0 0 0 1.51 1H21a2 2 0 0 1 2 2 2 2 0 0 1-2 2h-.09a1.65 1.65 0 0 0-1.51 1z" />
    </svg>
  );
}
function IconAdminUsers() {
  return (
    <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" strokeLinejoin="round">
      <path d="M12 22s8-4 8-10V5l-8-3-8 3v7c0 6 8 10 8 10z" />
      <circle cx="12" cy="10" r="3" />
      <path d="M7 20.662V19a2 2 0 0 1 2-2h6a2 2 0 0 1 2 2v1.662" />
    </svg>
  );
}
function IconErrors() {
  return (
    <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" strokeLinejoin="round">
      <path d="M10.29 3.86 1.82 18a2 2 0 0 0 1.71 3h16.94a2 2 0 0 0 1.71-3L13.71 3.86a2 2 0 0 0-3.42 0z" />
      <line x1="12" y1="9" x2="12" y2="13" />
      <line x1="12" y1="17" x2="12.01" y2="17" />
    </svg>
  );
}
function IconLogout() {
  return (
    <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" strokeLinejoin="round">
      <path d="M9 21H5a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h4" />
      <polyline points="16 17 21 12 16 7" />
      <line x1="21" y1="12" x2="9" y2="12" />
    </svg>
  );
}

const navItems = [
  { path: '/',              label: 'Dashboard',    icon: <IconDashboard />,     exact: true  },
  { path: '/users',         label: 'Users',         icon: <IconUsers />,         exact: false },
  { path: '/plans',         label: 'Plans',         icon: <IconPlans />,         exact: false },
  { path: '/providers',     label: 'AI Providers',  icon: <IconProviders />,     exact: false },
  { path: '/analytics',     label: 'Analytics',     icon: <IconAnalytics />,     exact: false },
  { path: '/transactions',  label: 'Transactions',  icon: <IconTransactions />,  exact: false },
  { path: '/payments',      label: 'Payments',      icon: <IconPayments />,      exact: false },
  { path: '/settings',      label: 'Settings',      icon: <IconSettings />,      exact: false },
  { path: '/admin-users',   label: 'Admin Users',   icon: <IconAdminUsers />,    exact: false },
  { path: '/errors',        label: 'Error Reports', icon: <IconErrors />,        exact: false },
];

export default function Layout() {
  const email = getAdminEmail() ?? '';
  const [collapsed, setCollapsed] = useState(false);
  const sidebarWidth = collapsed ? 0 : 270;
  const [backendWarning, setBackendWarning] = useState<string | null>(null);

  useEffect(() => {
    verifyBackend().then((result) => {
      if (result === 'wrong_server') {
        const url = import.meta.env.VITE_API_URL || 'http://localhost:3000';
        setBackendWarning(`Connected to wrong backend -- expected DraftRight API on ${url}`);
      } else if (result === 'unreachable') {
        const url = import.meta.env.VITE_API_URL || 'http://localhost:3000';
        setBackendWarning(`Backend unreachable at ${url}`);
      }
    });
  }, []);

  return (
    <div style={{ display: 'flex', minHeight: '100vh', background: '#202936' }}>

      {/* ── Overlay (mobile/collapsed) ──────────────────── */}
      {!collapsed ? null : null}

      {/* ── Sidebar ─────────────────────────────────────── */}
      <aside
        style={{
          position: 'fixed',
          left: 0,
          top: 0,
          width: 270,
          height: '100vh',
          background: '#202936',
          borderRight: '1px solid #333f55',
          display: 'flex',
          flexDirection: 'column',
          zIndex: 100,
          overflowY: 'auto',
          transform: collapsed ? 'translateX(-270px)' : 'translateX(0)',
          transition: 'transform 0.25s ease',
        }}
      >
        {/* Logo */}
        <div style={{ padding: '24px 20px 20px', borderBottom: '1px solid #333f55' }}>
          <div style={{ display: 'flex', alignItems: 'center', gap: 10 }}>
            <div
              style={{
                width: 34,
                height: 34,
                borderRadius: 8,
                background: 'linear-gradient(135deg, #5d87ff, #49beff)',
                display: 'flex',
                alignItems: 'center',
                justifyContent: 'center',
                flexShrink: 0,
              }}
            >
              <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="#fff" strokeWidth="2.2" strokeLinecap="round" strokeLinejoin="round">
                <path d="M12 20h9" />
                <path d="M16.5 3.5a2.121 2.121 0 0 1 3 3L7 19l-4 1 1-4L16.5 3.5z" />
              </svg>
            </div>
            <div>
              <p style={{ color: '#eaeff4', fontWeight: 700, fontSize: 15, margin: 0, lineHeight: 1.2 }}>DraftRight</p>
              <p style={{ color: '#7c8fac', fontSize: 11, margin: 0, letterSpacing: '0.05em' }}>Admin Portal</p>
            </div>
          </div>
        </div>

        {/* Nav section label */}
        <div style={{ padding: '16px 20px 6px' }}>
          <p style={{ color: '#7c8fac', fontSize: 11, fontWeight: 700, letterSpacing: '0.08em', textTransform: 'uppercase', margin: 0 }}>
            Navigation
          </p>
        </div>

        {/* Nav items */}
        <nav style={{ flex: 1, padding: '4px 12px' }}>
          {navItems.map((item) => (
            <NavLink
              key={item.path}
              to={item.path}
              end={item.exact}
              style={({ isActive }) => ({
                display: 'flex',
                alignItems: 'center',
                gap: 13,
                padding: '10px 10px',
                borderRadius: 7,
                marginBottom: 2,
                color: isActive ? '#5d87ff' : '#7c8fac',
                background: isActive ? 'rgba(93,135,255,0.1)' : 'transparent',
                fontWeight: isActive ? 600 : 400,
                fontSize: 14,
                textDecoration: 'none',
                transition: 'all 0.18s',
              })}
              onMouseEnter={(e) => {
                const el = e.currentTarget as HTMLAnchorElement;
                if (!el.classList.contains('active')) {
                  el.style.background = 'rgba(93,135,255,0.06)';
                  el.style.color = '#eaeff4';
                }
              }}
              onMouseLeave={(e) => {
                const el = e.currentTarget as HTMLAnchorElement;
                if (!el.getAttribute('aria-current')) {
                  el.style.background = 'transparent';
                  el.style.color = '#7c8fac';
                }
              }}
            >
              <span style={{ flexShrink: 0, display: 'flex' }}>{item.icon}</span>
              <span>{item.label}</span>
            </NavLink>
          ))}
        </nav>

        {/* User + Logout */}
        <div
          style={{
            margin: '12px',
            padding: '12px',
            borderRadius: 7,
            background: 'rgba(51,63,85,0.4)',
            borderTop: '1px solid #333f55',
          }}
        >
          <div style={{ display: 'flex', alignItems: 'center', gap: 10, marginBottom: 10 }}>
            <div
              style={{
                width: 34,
                height: 34,
                borderRadius: '50%',
                background: 'linear-gradient(135deg, #5d87ff, #49beff)',
                display: 'flex',
                alignItems: 'center',
                justifyContent: 'center',
                flexShrink: 0,
                fontSize: 13,
                fontWeight: 700,
                color: '#fff',
              }}
            >
              {email.charAt(0).toUpperCase()}
            </div>
            <div style={{ minWidth: 0, flex: 1 }}>
              <p style={{ color: '#eaeff4', fontSize: 13, fontWeight: 600, margin: 0, lineHeight: 1.3 }}>Admin</p>
              <p style={{ color: '#7c8fac', fontSize: 11, margin: 0, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>
                {email}
              </p>
            </div>
          </div>
          <button
            onClick={logout}
            style={{
              display: 'flex',
              alignItems: 'center',
              gap: 8,
              width: '100%',
              padding: '7px 10px',
              borderRadius: 7,
              border: 'none',
              background: 'transparent',
              color: '#7c8fac',
              fontSize: 13,
              fontWeight: 500,
              cursor: 'pointer',
              fontFamily: 'inherit',
              transition: 'all 0.15s',
            }}
            onMouseEnter={(e) => {
              (e.currentTarget as HTMLButtonElement).style.background = 'rgba(250,137,107,0.1)';
              (e.currentTarget as HTMLButtonElement).style.color = '#fa896b';
            }}
            onMouseLeave={(e) => {
              (e.currentTarget as HTMLButtonElement).style.background = 'transparent';
              (e.currentTarget as HTMLButtonElement).style.color = '#7c8fac';
            }}
          >
            <IconLogout />
            Sign Out
          </button>
        </div>
      </aside>

      {/* ── Right side ──────────────────────────────────── */}
      <div style={{ marginLeft: sidebarWidth, flex: 1, display: 'flex', flexDirection: 'column', minHeight: '100vh', transition: 'margin-left 0.25s ease' }}>

        {/* Header */}
        <header
          style={{
            position: 'fixed',
            top: 0,
            left: sidebarWidth,
            right: 0,
            height: 70,
            background: '#202936',
            borderBottom: '1px solid #333f55',
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'space-between',
            padding: '0 28px',
            zIndex: 99,
            transition: 'left 0.25s ease',
          }}
        >
          <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
            <button
              onClick={() => setCollapsed(!collapsed)}
              style={{
                background: 'transparent',
                border: 'none',
                cursor: 'pointer',
                padding: 6,
                borderRadius: 6,
                display: 'flex',
                alignItems: 'center',
                justifyContent: 'center',
                transition: 'background 0.15s',
              }}
              onMouseEnter={(e) => { (e.currentTarget as HTMLButtonElement).style.background = 'rgba(93,135,255,0.1)'; }}
              onMouseLeave={(e) => { (e.currentTarget as HTMLButtonElement).style.background = 'transparent'; }}
              title={collapsed ? 'Show sidebar' : 'Hide sidebar'}
            >
              <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="#7c8fac" strokeWidth="1.8" strokeLinecap="round" strokeLinejoin="round">
                <line x1="3" y1="6" x2="21" y2="6" />
                <line x1="3" y1="12" x2="21" y2="12" />
                <line x1="3" y1="18" x2="21" y2="18" />
              </svg>
            </button>
            <span style={{ color: '#7c8fac', fontSize: 13, marginLeft: 4 }}>DraftRight Admin</span>
          </div>
          <NavLink
            to="/profile"
            style={{
              width: 34,
              height: 34,
              borderRadius: '50%',
              background: 'linear-gradient(135deg, #5d87ff, #49beff)',
              display: 'flex',
              alignItems: 'center',
              justifyContent: 'center',
              fontSize: 13,
              fontWeight: 700,
              color: '#fff',
              textDecoration: 'none',
              transition: 'opacity 0.15s',
            }}
            title="My Profile"
            onMouseEnter={(e) => { (e.currentTarget as HTMLAnchorElement).style.opacity = '0.85'; }}
            onMouseLeave={(e) => { (e.currentTarget as HTMLAnchorElement).style.opacity = '1'; }}
          >
            {email.charAt(0).toUpperCase()}
          </NavLink>
        </header>

        {/* Page content */}
        <main style={{ flex: 1, padding: '28px 28px 40px', marginTop: 70, overflowY: 'auto' }}>
          {backendWarning && (
            <div style={{
              padding: '12px 20px',
              marginBottom: 20,
              borderRadius: 8,
              background: 'rgba(250,137,107,0.15)',
              border: '1px solid #fa896b',
              color: '#fa896b',
              fontSize: 14,
              fontWeight: 500,
            }}>
              {backendWarning}
            </div>
          )}
          <Outlet />
        </main>
      </div>
    </div>
  );
}
