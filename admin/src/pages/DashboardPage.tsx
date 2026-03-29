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
      <div className="mb-6">
        <h1 className="text-2xl font-bold text-gray-900">Dashboard</h1>
        <p className="text-gray-500 text-sm mt-1">Overview of your DraftRight platform</p>
      </div>

      {error && (
        <div className="bg-red-50 border border-red-200 text-red-700 text-sm rounded-lg px-4 py-3 mb-6">
          {error}
        </div>
      )}

      <div className="grid grid-cols-1 sm:grid-cols-2 xl:grid-cols-4 gap-6">
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

      <p className="text-xs text-gray-400 mt-6">Auto-refreshes every 30 seconds</p>
    </div>
  );
}
