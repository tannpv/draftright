import { useState, useEffect, useCallback } from 'react';
import { apiFetch } from '../api';
import { formatCurrency } from '../lib/format';

interface MonthStat {
  month: string;
  new_subscriptions: number;
  revenue_cents: number;
  churned: number;
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
  monthly_stats: MonthStat[];
}

function formatMonth(m: string): string {
  const [year, month] = m.split('-');
  const d = new Date(parseInt(year), parseInt(month) - 1, 1);
  return d.toLocaleString('default', { month: 'short', year: '2-digit' });
}

/* ── CSS Bar Chart ──────────────────────────────────────── */
interface BarChartProps {
  data: { label: string; value: number; subLabel?: string; subLabelColor?: string }[];
  formatValue?: (v: number) => string;
  color?: string;
  height?: number;
}

function BarChart({ data, formatValue, color = 'var(--primary)', height = 140 }: BarChartProps) {
  const max = Math.max(...data.map(d => d.value), 1);

  return (
    <div style={{ overflowX: 'auto' }}>
      <div
        style={{
          display: 'flex',
          alignItems: 'flex-end',
          gap: 6,
          minWidth: data.length * 52,
          paddingBottom: 4,
        }}
      >
        {data.map((d, i) => {
          const barH = Math.max((d.value / max) * height, d.value > 0 ? 4 : 0);
          return (
            <div
              key={i}
              style={{
                flex: 1,
                display: 'flex',
                flexDirection: 'column',
                alignItems: 'center',
                gap: 4,
                minWidth: 44,
              }}
            >
              {/* Value on top */}
              <span style={{ color: 'var(--text)', fontSize: 11, fontWeight: 600, whiteSpace: 'nowrap' }}>
                {formatValue ? formatValue(d.value) : d.value}
              </span>

              {/* Bar */}
              <div
                style={{
                  width: '75%',
                  height: barH,
                  background: color,
                  borderRadius: '3px 3px 0 0',
                  opacity: 0.85,
                  transition: 'height 0.3s ease',
                  minHeight: d.value > 0 ? 4 : 0,
                }}
              />

              {/* Month label */}
              <span style={{ color: 'var(--muted)', fontSize: 11, whiteSpace: 'nowrap' }}>{d.label}</span>

              {/* Sub-label (churn) */}
              {d.subLabel !== undefined && (
                <span style={{ color: d.subLabelColor || 'var(--danger)', fontSize: 10, whiteSpace: 'nowrap' }}>
                  {d.subLabel}
                </span>
              )}
            </div>
          );
        })}
      </div>
    </div>
  );
}

/* ── Section card wrapper ───────────────────────────────── */
function Card({ title, children }: { title: string; children: React.ReactNode }) {
  return (
    <div style={{ background: 'var(--card)', borderRadius: 7, overflow: 'hidden', marginBottom: 24 }}>
      <div style={{ padding: '16px 22px', borderBottom: '1px solid var(--border)' }}>
        <h3 style={{ color: 'var(--text)', fontSize: 15, fontWeight: 600, margin: 0 }}>{title}</h3>
      </div>
      <div style={{ padding: '20px 22px' }}>{children}</div>
    </div>
  );
}

export default function AnalyticsPage() {
  const [analytics, setAnalytics] = useState<Analytics | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');

  const fetchAnalytics = useCallback(async () => {
    try {
      const data = await apiFetch('/admin/analytics') as Analytics;
      setAnalytics(data);
      setError('');
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to load analytics');
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => { fetchAnalytics(); }, [fetchAnalytics]);

  if (loading) {
    return (
      <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'center', padding: '80px 0', color: 'var(--muted)' }}>
        Loading analytics...
      </div>
    );
  }

  const stats = analytics?.monthly_stats || [];
  const breakdown = analytics?.plans_breakdown || [];
  const totalRevenue = analytics?.total_revenue || 0;

  const revenueData = stats.map(s => ({
    label: formatMonth(s.month),
    value: s.revenue_cents,
    subLabel: undefined,
  }));

  const subData = stats.map(s => ({
    label: formatMonth(s.month),
    value: s.new_subscriptions,
    subLabel: s.churned > 0 ? `-${s.churned}` : '',
    subLabelColor: 'var(--danger)',
  }));

  return (
    <div>
      {/* Page header */}
      <div style={{ marginBottom: 24 }}>
        <h1 style={{ color: 'var(--text)', fontSize: 22, fontWeight: 700, margin: '0 0 4px' }}>Analytics</h1>
        <p style={{ color: 'var(--muted)', fontSize: 13, margin: 0 }}>
          Subscription and revenue trends over the last 12 months
        </p>
      </div>

      {error && <div className="alert-error" style={{ marginBottom: 20 }}>{error}</div>}

      {/* Summary row */}
      <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fill, minmax(200px, 1fr))', gap: 16, marginBottom: 24 }}>
        {[
          { label: 'MRR', value: formatCurrency(analytics?.mrr || 0), color: 'var(--success)' },
          { label: 'Total Revenue (12mo)', value: formatCurrency(totalRevenue), color: 'var(--primary)' },
          { label: 'Active Plans', value: String(breakdown.length), color: 'var(--warning)' },
        ].map(card => (
          <div key={card.label} style={{ background: 'var(--card)', borderRadius: 7, padding: '18px 22px' }}>
            <p style={{ color: 'var(--muted)', fontSize: 12, fontWeight: 600, textTransform: 'uppercase', letterSpacing: '0.06em', margin: '0 0 8px' }}>{card.label}</p>
            <p style={{ color: card.color, fontSize: 26, fontWeight: 700, margin: 0 }}>{card.value}</p>
          </div>
        ))}
      </div>

      {/* Revenue bar chart */}
      <Card title="Monthly Revenue">
        {revenueData.every(d => d.value === 0) ? (
          <p style={{ color: 'var(--muted)', fontSize: 13, margin: 0 }}>No revenue data yet.</p>
        ) : (
          <BarChart
            data={revenueData}
            formatValue={(v) => v === 0 ? '$0' : formatCurrency(v)}
            color="var(--primary)"
            height={160}
          />
        )}
      </Card>

      {/* Subscribers bar chart */}
      <Card title="New Subscriptions per Month">
        {subData.every(d => d.value === 0) ? (
          <p style={{ color: 'var(--muted)', fontSize: 13, margin: 0 }}>No subscription data yet.</p>
        ) : (
          <>
            <BarChart
              data={subData}
              color="var(--success)"
              height={140}
            />
            <p style={{ color: 'var(--muted)', fontSize: 11, margin: '10px 0 0' }}>
              Red numbers below bars indicate churned subscriptions that month.
            </p>
          </>
        )}
      </Card>

      {/* Plans breakdown table */}
      <Card title="Plans Breakdown">
        {breakdown.length === 0 ? (
          <p style={{ color: 'var(--muted)', fontSize: 13, margin: 0 }}>No active subscriptions.</p>
        ) : (
          <div style={{ overflowX: 'auto' }}>
            <table style={{ width: '100%', borderCollapse: 'collapse' }}>
              <thead>
                <tr style={{ borderBottom: '2px solid var(--border)' }}>
                  {['Plan Name', 'Active Users', 'Plan Price', '% of Subs'].map(h => (
                    <th
                      key={h}
                      style={{
                        padding: '10px 16px',
                        textAlign: 'left',
                        color: 'var(--muted)',
                        fontSize: 12,
                        fontWeight: 700,
                        textTransform: 'uppercase',
                        letterSpacing: '0.05em',
                      }}
                    >
                      {h}
                    </th>
                  ))}
                </tr>
              </thead>
              <tbody>
                {(() => {
                  const totalActive = breakdown.reduce((s, b) => s + b.active_count, 0);
                  return breakdown.map((b, i) => {
                    const pct = totalActive > 0 ? ((b.active_count / totalActive) * 100).toFixed(1) : '0.0';
                    return (
                      <tr key={i} style={{ borderBottom: '1px solid var(--border)' }}>
                        <td style={{ padding: '13px 16px', color: 'var(--text)', fontSize: 14, fontWeight: 500 }}>{b.plan_name}</td>
                        <td style={{ padding: '13px 16px', color: 'var(--primary)', fontSize: 14, fontWeight: 700 }}>{b.active_count}</td>
                        <td style={{ padding: '13px 16px', color: 'var(--text)', fontSize: 14 }}>
                          {b.price_cents === 0 ? 'Free' : formatCurrency(b.price_cents)}
                        </td>
                        <td style={{ padding: '13px 16px', fontSize: 14 }}>
                          <div style={{ display: 'flex', alignItems: 'center', gap: 10 }}>
                            <div style={{ flex: 1, maxWidth: 120, height: 6, background: 'var(--border)', borderRadius: 3 }}>
                              <div
                                style={{
                                  width: `${pct}%`,
                                  height: '100%',
                                  background: 'var(--primary)',
                                  borderRadius: 3,
                                }}
                              />
                            </div>
                            <span style={{ color: 'var(--muted)', fontSize: 13 }}>{pct}%</span>
                          </div>
                        </td>
                      </tr>
                    );
                  });
                })()}
              </tbody>
            </table>
          </div>
        )}
      </Card>
    </div>
  );
}
