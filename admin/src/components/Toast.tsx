import { useEffect } from 'react';

interface ToastProps {
  message: string;
  type: 'success' | 'error';
  onClose: () => void;
  duration?: number;
}

export default function Toast({ message, type, onClose, duration = 3500 }: ToastProps) {
  useEffect(() => {
    const timer = setTimeout(onClose, duration);
    return () => clearTimeout(timer);
  }, [duration, onClose]);

  const isSuccess = type === 'success';

  return (
    <div
      style={{
        position: 'fixed',
        bottom: 24,
        right: 24,
        zIndex: 999,
      }}
    >
      <div
        style={{
          display: 'flex',
          alignItems: 'center',
          gap: 12,
          padding: '12px 18px',
          borderRadius: 7,
          background: 'var(--card)',
          border: `1px solid ${isSuccess ? 'rgba(19,222,185,0.3)' : 'rgba(250,137,107,0.3)'}`,
          boxShadow: '0 8px 30px rgba(0,0,0,0.3)',
          minWidth: 260,
          maxWidth: 380,
        }}
      >
        {/* Icon */}
        <div
          style={{
            width: 30,
            height: 30,
            borderRadius: '50%',
            background: isSuccess ? 'rgba(19,222,185,0.15)' : 'rgba(250,137,107,0.15)',
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'center',
            flexShrink: 0,
            color: isSuccess ? 'var(--success)' : 'var(--danger)',
            fontSize: 14,
            fontWeight: 700,
          }}
        >
          {isSuccess ? '✓' : '✕'}
        </div>

        <span style={{ color: 'var(--text)', fontSize: 13, fontWeight: 500, flex: 1 }}>{message}</span>

        <button
          onClick={onClose}
          style={{
            background: 'transparent',
            border: 'none',
            color: 'var(--muted)',
            cursor: 'pointer',
            fontSize: 16,
            lineHeight: 1,
            padding: 2,
            flexShrink: 0,
          }}
        >
          ×
        </button>
      </div>
    </div>
  );
}
