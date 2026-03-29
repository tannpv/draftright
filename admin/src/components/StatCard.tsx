interface StatCardProps {
  icon: string;
  label: string;
  value: string | number;
  color?: 'blue' | 'green' | 'purple' | 'orange';
}

const colorMap: Record<string, { bg: string; color: string }> = {
  blue:   { bg: 'rgba(93,135,255,0.12)',  color: '#5d87ff' },
  green:  { bg: 'rgba(19,222,185,0.12)',  color: '#13deb9' },
  purple: { bg: 'rgba(73,190,255,0.12)',  color: '#49beff' },
  orange: { bg: 'rgba(255,174,31,0.12)',  color: '#ffae1f' },
};

export default function StatCard({ icon, label, value, color = 'blue' }: StatCardProps) {
  const palette = colorMap[color];

  return (
    <div
      style={{
        background: '#2a3547',
        borderRadius: 7,
        padding: '22px 26px',
        display: 'flex',
        alignItems: 'center',
        gap: 18,
      }}
    >
      {/* Icon circle */}
      <div
        style={{
          width: 52,
          height: 52,
          borderRadius: 10,
          background: palette.bg,
          display: 'flex',
          alignItems: 'center',
          justifyContent: 'center',
          fontSize: 22,
          flexShrink: 0,
          color: palette.color,
        }}
      >
        {icon}
      </div>

      {/* Text */}
      <div>
        <p style={{ color: '#7c8fac', fontSize: 13, margin: 0, fontWeight: 500 }}>{label}</p>
        <p style={{ color: '#eaeff4', fontSize: 26, fontWeight: 700, margin: '2px 0 0', lineHeight: 1.2 }}>
          {value}
        </p>
      </div>
    </div>
  );
}
