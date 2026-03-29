import { useState, useEffect, useCallback } from 'react';
import StatCard from '../components/StatCard';
import { apiFetch } from '../api';

interface Stats {
  total_users: number;
  active_subscriptions: number;
  rewrites_today: number;
  rewrites_this_month: number;
}

interface PlanBreakdown {
  plan_name: string;
  active_count: number;
  price_cents: number;
}

interface Analytics {
  mrr: number;
  total_revenue: number;
  plans_breakdown: PlanBreakdown[];
}

function formatCents(cents: number): string {
  return `$${(cents / 100).toFixed(2)}`;
}

export default function DashboardPage() {
  const [stats, setStats] = useState<Stats | null>(null);
  const [analytics, setAnalytics] = useState<Analytics | null>(null);
  const [error, setError] = useState('');

  const fetchAll = useCallback(async () => {
    try {
      const [statsData, analyticsData] = await Promise.all([
        apiFetch('/admin/stats') as Promise<Stats>,
        apiFetch('/admin/analytics') as Promise<Analytics>,
      ]);
      setStats(statsData);
      setAnalytics(analyticsData);
      setError('');
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to load stats');
    }
  }, []);

  useEffect(() => {
    fetchAll();
    const interval = setInterval(fetchAll, 30000);
    return () => clearInterval(interval);
  }, [fetchAll]);

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

      {/* Usage stat cards */}
      <div
        style={{
          display: 'grid',
          gridTemplateColumns: 'repeat(auto-fill, minmax(220px, 1fr))',
          gap: 20,
          marginBottom: 20,
        }}
      >
        <StatCard
          icon="👥"
          label="Total Users"
          value={stats?.total_users ?? '—'}
          color="blue"
        />
        <StatCard
          icon="✅"
          label="Active Subscriptions"
          value={stats?.active_subscriptions ?? '—'}
          color="green"
        />
        <StatCard
          icon="✏️"
          label="Rewrites Today"
          value={stats?.rewrites_today ?? '—'}
          color="purple"
        />
        <StatCard
          icon="📊"
          label="Rewrites This Month"
          value={stats?.rewrites_this_month ?? '—'}
          color="orange"
        />
      </div>

      {/* Revenue stat cards */}
      <div
        style={{
          display: 'grid',
          gridTemplateColumns: 'repeat(auto-fill, minmax(220px, 1fr))',
          gap: 20,
          marginBottom: 24,
        }}
      >
        <StatCard
          icon="💰"
          label="MRR"
          value={analytics ? formatCents(analytics.mrr) : '—'}
          color="green"
        />
        <StatCard
          icon="💵"
          label="Total Revenue"
          value={analytics ? formatCents(analytics.total_revenue) : '—'}
          color="blue"
        />

        {/* Plans breakdown card */}
        <div
          style={{
            background: '#2a3547',
            borderRadius: 7,
            padding: '18px 22px',
            gridColumn: 'span 2',
          }}
        >
          <p style={{ color: '#7c8fac', fontSize: 13, fontWeight: 500, margin: '0 0 12px' }}>Active Plans Breakdown</p>
          {analytics && analytics.plans_breakdown.length > 0 ? (
            <div style={{ display: 'flex', flexWrap: 'wrap', gap: '8px 24px' }}>
              {analytics.plans_breakdown.map((p) => (
                <div key={p.plan_name} style={{ display: 'flex', alignItems: 'center', gap: 8, minWidth: 140 }}>
                  <span
                    style={{
                      display: 'inline-block',
                      width: 8,
                      height: 8,
                      borderRadius: '50%',
                      background: '#5d87ff',
                      flexShrink: 0,
                    }}
                  />
                  <span style={{ color: '#eaeff4', fontSize: 14, fontWeight: 600 }}>{p.active_count}</span>
                  <span style={{ color: '#7c8fac', fontSize: 13 }}>{p.plan_name}</span>
                </div>
              ))}
            </div>
          ) : (
            <p style={{ color: '#7c8fac', fontSize: 13, margin: 0 }}>No active subscriptions</p>
          )}
        </div>
      </div>

      <p style={{ color: '#333f55', fontSize: 12, margin: 0 }}>
        Auto-refreshes every 30 seconds
      </p>
    </div>
  );
}
