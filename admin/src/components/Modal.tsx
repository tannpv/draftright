import { ReactNode, useEffect } from 'react';

interface ModalProps {
  title: string;
  children: ReactNode;
  onClose: () => void;
  footer?: ReactNode;
}

export default function Modal({ title, children, onClose, footer }: ModalProps) {
  useEffect(() => {
    const handleKey = (e: KeyboardEvent) => {
      if (e.key === 'Escape') onClose();
    };
    document.addEventListener('keydown', handleKey);
    return () => document.removeEventListener('keydown', handleKey);
  }, [onClose]);

  return (
    <div
      style={{
        position: 'fixed',
        inset: 0,
        zIndex: 200,
        display: 'flex',
        alignItems: 'center',
        justifyContent: 'center',
        padding: '16px',
      }}
    >
      {/* Backdrop */}
      <div
        style={{
          position: 'absolute',
          inset: 0,
          background: 'rgba(0,0,0,0.55)',
          backdropFilter: 'blur(2px)',
        }}
        onClick={onClose}
      />

      {/* Dialog */}
      <div
        style={{
          position: 'relative',
          background: '#2a3547',
          border: '1px solid #333f55',
          borderRadius: 7,
          width: '100%',
          maxWidth: 520,
          boxShadow: '0 20px 60px rgba(0,0,0,0.4)',
          zIndex: 10,
        }}
      >
        {/* Header */}
        <div
          style={{
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'space-between',
            padding: '18px 22px',
            borderBottom: '1px solid #333f55',
          }}
        >
          <h2 style={{ color: '#eaeff4', fontSize: 16, fontWeight: 600, margin: 0 }}>{title}</h2>
          <button
            onClick={onClose}
            style={{
              background: 'transparent',
              border: 'none',
              color: '#7c8fac',
              cursor: 'pointer',
              fontSize: 20,
              lineHeight: 1,
              padding: '0 4px',
              display: 'flex',
              alignItems: 'center',
              justifyContent: 'center',
              borderRadius: 5,
              transition: 'color 0.15s',
            }}
            onMouseEnter={(e) => { (e.currentTarget as HTMLButtonElement).style.color = '#eaeff4'; }}
            onMouseLeave={(e) => { (e.currentTarget as HTMLButtonElement).style.color = '#7c8fac'; }}
          >
            &times;
          </button>
        </div>

        {/* Body */}
        <div style={{ padding: '20px 22px' }}>{children}</div>

        {/* Footer */}
        {footer && (
          <div
            style={{
              display: 'flex',
              justifyContent: 'flex-end',
              gap: 10,
              padding: '14px 22px',
              borderTop: '1px solid #333f55',
            }}
          >
            {footer}
          </div>
        )}
      </div>
    </div>
  );
}
