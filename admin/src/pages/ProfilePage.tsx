import { useState, useEffect } from 'react';
import Toast from '../components/Toast';
import { apiFetch } from '../api';
import { getAdminEmail } from '../auth';

interface ToastState {
  message: string;
  type: 'success' | 'error';
}

export default function ProfilePage() {
  const [adminName, setAdminName] = useState('Admin');
  const [adminEmail, setAdminEmail] = useState(getAdminEmail());
  const [currentPassword, setCurrentPassword] = useState('');
  const [newPassword, setNewPassword] = useState('');
  const [confirmPassword, setConfirmPassword] = useState('');
  const [saving, setSaving] = useState(false);
  const [toast, setToast] = useState<ToastState | null>(null);

  useEffect(() => {
    apiFetch('/admin/auth/me')
      .then((data: any) => {
        if (data.name) setAdminName(data.name);
        if (data.email) setAdminEmail(data.email);
      })
      .catch(() => {
        // fall back to localStorage values
      });
  }, []);

  async function handleChangePassword() {
    if (newPassword !== confirmPassword) {
      setToast({ message: 'New passwords do not match', type: 'error' });
      return;
    }
    if (newPassword.length < 8) {
      setToast({ message: 'Password must be at least 8 characters', type: 'error' });
      return;
    }
    setSaving(true);
    try {
      await apiFetch('/admin/auth/change-password', {
        method: 'POST',
        body: JSON.stringify({ current_password: currentPassword, new_password: newPassword }),
      });
      setToast({ message: 'Password changed successfully', type: 'success' });
      setCurrentPassword('');
      setNewPassword('');
      setConfirmPassword('');
    } catch (err) {
      setToast({ message: err instanceof Error ? err.message : 'Failed to change password', type: 'error' });
    } finally {
      setSaving(false);
    }
  }

  return (
    <div style={{ maxWidth: 560 }}>
      <div style={{ marginBottom: 24 }}>
        <h1 style={{ color: '#eaeff4', fontSize: 22, fontWeight: 700, margin: '0 0 4px' }}>My Profile</h1>
        <p style={{ color: '#7c8fac', fontSize: 13, margin: 0 }}>Manage your account</p>
      </div>

      {/* Profile Info */}
      <div style={{ background: '#2a3547', borderRadius: 7, padding: '22px', marginBottom: 20 }}>
        <div style={{ display: 'flex', alignItems: 'center', gap: 16, marginBottom: 16 }}>
          <div
            style={{
              width: 56,
              height: 56,
              borderRadius: '50%',
              background: 'linear-gradient(135deg, #5d87ff, #49beff)',
              display: 'flex',
              alignItems: 'center',
              justifyContent: 'center',
              fontSize: 22,
              fontWeight: 700,
              color: '#fff',
              flexShrink: 0,
            }}
          >
            {adminEmail.charAt(0).toUpperCase()}
          </div>
          <div>
            <p style={{ color: '#eaeff4', fontSize: 16, fontWeight: 600, margin: 0 }}>{adminName}</p>
            <p style={{ color: '#7c8fac', fontSize: 13, margin: '2px 0 0' }}>{adminEmail}</p>
            <span className="badge badge-primary" style={{ marginTop: 6, display: 'inline-block' }}>Administrator</span>
          </div>
        </div>
      </div>

      {/* Change Password */}
      <div style={{ background: '#2a3547', borderRadius: 7 }}>
        <div style={{ padding: '16px 22px', borderBottom: '1px solid #333f55' }}>
          <h3 style={{ color: '#eaeff4', fontSize: 15, fontWeight: 600, margin: 0 }}>Change Password</h3>
        </div>
        <div style={{ padding: '18px 22px', display: 'flex', flexDirection: 'column', gap: 14 }}>
          <div>
            <label style={{ display: 'block', color: '#eaeff4', fontSize: 13, fontWeight: 500, marginBottom: 6 }}>Current Password</label>
            <input
              type="password"
              value={currentPassword}
              onChange={(e) => setCurrentPassword(e.target.value)}
              className="dark-input"
              placeholder="Enter current password"
            />
          </div>
          <div>
            <label style={{ display: 'block', color: '#eaeff4', fontSize: 13, fontWeight: 500, marginBottom: 6 }}>New Password</label>
            <input
              type="password"
              value={newPassword}
              onChange={(e) => setNewPassword(e.target.value)}
              className="dark-input"
              placeholder="Minimum 8 characters"
            />
          </div>
          <div>
            <label style={{ display: 'block', color: '#eaeff4', fontSize: 13, fontWeight: 500, marginBottom: 6 }}>Confirm New Password</label>
            <input
              type="password"
              value={confirmPassword}
              onChange={(e) => setConfirmPassword(e.target.value)}
              className="dark-input"
              placeholder="Re-enter new password"
            />
          </div>
          <div style={{ paddingTop: 4 }}>
            <button
              onClick={handleChangePassword}
              disabled={saving || !currentPassword || !newPassword || !confirmPassword}
              className="btn btn-primary"
            >
              {saving ? 'Saving...' : 'Update Password'}
            </button>
          </div>
        </div>
      </div>

      {toast && (
        <Toast message={toast.message} type={toast.type} onClose={() => setToast(null)} />
      )}
    </div>
  );
}
