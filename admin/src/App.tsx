import { BrowserRouter, Routes, Route, Navigate, Outlet } from 'react-router-dom';
import { isAuthenticated } from './auth';
import Layout from './components/Layout';
import LoginPage from './pages/LoginPage';
import DashboardPage from './pages/DashboardPage';
import UsersPage from './pages/UsersPage';
import UserDetailPage from './pages/UserDetailPage';
import PlansPage from './pages/PlansPage';
import ProvidersPage from './pages/ProvidersPage';
import ProfilePage from './pages/ProfilePage';
import AnalyticsPage from './pages/AnalyticsPage';
import TransactionsPage from './pages/TransactionsPage';
import PaymentsPage from './pages/PaymentsPage';
import SettingsPage from './pages/SettingsPage';
import EmailLogsPage from './pages/EmailLogsPage';
import EmailTemplatesPage from './pages/EmailTemplatesPage';
import AdminUsersPage from './pages/AdminUsersPage';
import AdminAuditLogPage from './pages/AdminAuditLogPage';
import ErrorsPage from './pages/ErrorsPage';
import BugReportsPage from './pages/BugReportsPage';
import VersionsPage from './pages/VersionsPage';
import InboxPage from './pages/InboxPage';

function ProtectedRoute() {
  if (!isAuthenticated()) {
    return <Navigate to="/login" replace />;
  }
  return <Outlet />;
}

export default function App() {
  return (
    <BrowserRouter>
      <Routes>
        <Route path="/login" element={<LoginPage />} />
        <Route element={<ProtectedRoute />}>
          <Route element={<Layout />}>
            <Route path="/" element={<DashboardPage />} />
            <Route path="/users" element={<UsersPage />} />
            <Route path="/users/:id" element={<UserDetailPage />} />
            <Route path="/plans" element={<PlansPage />} />
            <Route path="/providers" element={<ProvidersPage />} />
            <Route path="/profile" element={<ProfilePage />} />
            <Route path="/analytics" element={<AnalyticsPage />} />
            <Route path="/transactions" element={<TransactionsPage />} />
            <Route path="/payments" element={<PaymentsPage />} />
            <Route path="/settings" element={<SettingsPage />} />
            <Route path="/email-logs" element={<EmailLogsPage />} />
            <Route path="/email-templates" element={<EmailTemplatesPage />} />
            <Route path="/admin-users" element={<AdminUsersPage />} />
            <Route path="/admin-audit" element={<AdminAuditLogPage />} />
            <Route path="/errors" element={<ErrorsPage />} />
            <Route path="/bug-reports" element={<BugReportsPage />} />
            <Route path="/versions" element={<VersionsPage />} />
            <Route path="/inbox" element={<InboxPage />} />
          </Route>
        </Route>
        {/* Fallback */}
        <Route path="*" element={<Navigate to="/" replace />} />
      </Routes>
    </BrowserRouter>
  );
}
