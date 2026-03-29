import { useState } from 'react';
import { NavLink, Outlet } from 'react-router-dom';
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
  { path: '/',          label: 'Dashboard',   icon: <IconDashboard />,  exact: true  },
  { path: '/users',     label: 'Users',        icon: <IconUsers />,      exact: false },
  { path: '/plans',     label: 'Plans',        icon: <IconPlans />,      exact: false },
  { path: '/providers', label: 'AI Providers', icon: <IconProviders />,  exact: false },
];

export default function Layout() {
  const email = getAdminEmail() ?? '';
  const [collapsed, setCollapsed] = useState(false);
  const [showProfileMenu, setShowProfileMenu] = useState(false);
  const sidebarWidth = collapsed ? 0 : 270;

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
          <div style={{ position: 'relative' }}>
            <button
              onClick={() => setShowProfileMenu(!showProfileMenu)}
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
                border: 'none',
                cursor: 'pointer',
              }}
            >
              {email.charAt(0).toUpperCase()}
            </button>
            {showProfileMenu && (
              <>
                <div
                  onClick={() => setShowProfileMenu(false)}
                  style={{ position: 'fixed', inset: 0, zIndex: 199 }}
                />
                <div
                  style={{
                    position: 'absolute',
                    top: 44,
                    right: 0,
                    width: 220,
                    background: '#2a3547',
                    border: '1px solid #333f55',
                    borderRadius: 7,
                    padding: 8,
                    zIndex: 200,
                    boxShadow: '0 8px 24px rgba(0,0,0,0.3)',
                  }}
                >
                  <div style={{ padding: '8px 10px', borderBottom: '1px solid #333f55', marginBottom: 4 }}>
                    <p style={{ color: '#eaeff4', fontSize: 13, fontWeight: 600, margin: 0 }}>Admin</p>
                    <p style={{ color: '#7c8fac', fontSize: 11, margin: '2px 0 0', overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>{email}</p>
                  </div>
                  <button
                    onClick={logout}
                    style={{
                      display: 'flex',
                      alignItems: 'center',
                      gap: 8,
                      width: '100%',
                      padding: '8px 10px',
                      borderRadius: 5,
                      border: 'none',
                      background: 'transparent',
                      color: '#fa896b',
                      fontSize: 13,
                      cursor: 'pointer',
                      fontFamily: 'inherit',
                      transition: 'background 0.15s',
                    }}
                    onMouseEnter={(e) => { (e.currentTarget as HTMLButtonElement).style.background = 'rgba(250,137,107,0.1)'; }}
                    onMouseLeave={(e) => { (e.currentTarget as HTMLButtonElement).style.background = 'transparent'; }}
                  >
                    <IconLogout />
                    Sign Out
                  </button>
                </div>
              </>
            )}
          </div>
        </header>

        {/* Page content */}
        <main style={{ flex: 1, padding: '28px 28px 40px', marginTop: 70, overflowY: 'auto' }}>
          <Outlet />
        </main>
      </div>
    </div>
  );
}
