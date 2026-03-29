import { useEffect } from 'react';

interface ToastProps {
  message: string;
  type: 'success' | 'error';
  onClose: () => void;
  duration?: number;
}

export default function Toast({ message, type, onClose, duration = 3000 }: ToastProps) {
  useEffect(() => {
    const timer = setTimeout(onClose, duration);
    return () => clearTimeout(timer);
  }, [duration, onClose]);

  return (
    <div className="fixed bottom-4 right-4 z-50 animate-fade-in">
      <div
        className={`flex items-center gap-3 px-4 py-3 rounded-lg shadow-lg text-white text-sm font-medium ${
          type === 'success' ? 'bg-green-600' : 'bg-red-600'
        }`}
      >
        <span>
          {type === 'success' ? '✓' : '✕'}
        </span>
        <span>{message}</span>
        <button
          onClick={onClose}
          className="ml-2 opacity-70 hover:opacity-100 transition-opacity"
        >
          ×
        </button>
      </div>
    </div>
  );
}
