import { useState, useEffect, useCallback } from 'react';
import StatCard from '../components/StatCard';
import { apiFetch } from '../api';

interface Stats {
  totalUsers: number;
  activeSubscriptions: number;
  rewritesToday: number;
  rewritesThisMonth: number;
}

export default function DashboardPage() {
  const [stats, setStats] = useState<Stats | null>(null);
  const [error, setError] = useState('');

  const fetchStats = useCallback(async () => {
    try {
      const data = await apiFetch('/admin/stats') as Stats;
      setStats(data);
      setError('');
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to load stats');
    }
  }, []);

  useEffect(() => {
    fetchStats();
    const interval = setInterval(fetchStats, 30000);
    return () => clearInterval(interval);
  }, [fetchStats]);

  return (
    <div>
      {/* Page header */}
      <div style={{ marginBottom: 24 }}>
        <h1 style={{ color: '#eaeff4', fontSize: 22, fontWeight: 700, margin: '0 0 4px' }}>Dashboard</h1>
        <p style={{ color: '#7c8fac', fontSize: 13, margin: 0 }}>
          Overview of your DraftRight platform
        </p>
      </div>

      {error && (
        <div className="alert-error" style={{ marginBottom: 20 }}>
          {error}
        </div>
      )}

      {/* Stat cards */}
      <div
        style={{
          display: 'grid',
          gridTemplateColumns: 'repeat(auto-fill, minmax(220px, 1fr))',
          gap: 20,
          marginBottom: 24,
        }}
      >
        <StatCard
          icon="👥"
          label="Total Users"
          value={stats?.totalUsers ?? '—'}
          color="blue"
        />
        <StatCard
          icon="✅"
          label="Active Subscriptions"
          value={stats?.activeSubscriptions ?? '—'}
          color="green"
        />
        <StatCard
          icon="✏️"
          label="Rewrites Today"
          value={stats?.rewritesToday ?? '—'}
          color="purple"
        />
        <StatCard
          icon="📊"
          label="Rewrites This Month"
          value={stats?.rewritesThisMonth ?? '—'}
          color="orange"
        />
      </div>

      <p style={{ color: '#333f55', fontSize: 12, margin: 0 }}>
        Auto-refreshes every 30 seconds
      </p>
    </div>
  );
}
