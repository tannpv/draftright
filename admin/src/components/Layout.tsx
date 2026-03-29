import { NavLink, Outlet } from 'react-router-dom';
import { logout, getAdminEmail } from '../auth';

const navItems = [
  { path: '/', label: 'Dashboard', icon: '⊞', exact: true },
  { path: '/users', label: 'Users', icon: '👥', exact: false },
  { path: '/plans', label: 'Plans', icon: '📋', exact: false },
  { path: '/providers', label: 'AI Providers', icon: '🤖', exact: false },
];

export default function Layout() {
  return (
    <div className="flex h-screen bg-gray-100">
      {/* Sidebar */}
      <aside className="w-60 bg-white border-r border-gray-200 flex flex-col">
        {/* Logo */}
        <div className="px-6 py-5 border-b border-gray-200">
          <h1 className="text-xl font-bold text-blue-600 tracking-tight">DraftRight</h1>
          <p className="text-xs text-gray-400 mt-0.5">Admin Portal</p>
        </div>

        {/* Nav */}
        <nav className="flex-1 px-3 py-4 space-y-1">
          {navItems.map((item) => (
            <NavLink
              key={item.path}
              to={item.path}
              end={item.exact}
              className={({ isActive }) =>
                `flex items-center gap-3 px-3 py-2.5 rounded-lg text-sm font-medium transition-colors ${
                  isActive
                    ? 'bg-blue-50 text-blue-700'
                    : 'text-gray-600 hover:bg-gray-100 hover:text-gray-900'
                }`
              }
            >
              <span className="text-base">{item.icon}</span>
              {item.label}
            </NavLink>
          ))}
        </nav>

        {/* User + Logout */}
        <div className="px-3 py-4 border-t border-gray-200">
          <p className="px-3 text-xs text-gray-400 truncate mb-2">{getAdminEmail()}</p>
          <button
            onClick={logout}
            className="flex items-center gap-3 w-full px-3 py-2.5 rounded-lg text-sm font-medium text-gray-600 hover:bg-red-50 hover:text-red-600 transition-colors"
          >
            <span className="text-base">🚪</span>
            Logout
          </button>
        </div>
      </aside>

      {/* Main content */}
      <div className="flex-1 flex flex-col overflow-hidden">
        {/* Header */}
        <header className="bg-white border-b border-gray-200 px-8 py-4">
          <h2 className="text-lg font-semibold text-gray-900">DraftRight Admin</h2>
        </header>

        {/* Page content */}
        <main className="flex-1 overflow-y-auto p-8">
          <Outlet />
        </main>
      </div>
    </div>
  );
}
