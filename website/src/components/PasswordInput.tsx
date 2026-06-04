import { useState } from 'react';

interface Props {
  value: string;
  onChange: (value: string) => void;
  placeholder?: string;
  /** 'current-password' for login, 'new-password' for signup. */
  autoComplete?: string;
  minLength?: number;
  required?: boolean;
}

/**
 * Password field with a mask/reveal (eye) toggle — masked by default.
 * Shared by login + signup so the show/hide behaviour stays consistent
 * and there's no duplicated markup. We deliberately do NOT use a
 * confirm-password field; the reveal toggle covers the same need with
 * less friction (web.dev sign-up best practice).
 */
export default function PasswordInput({
  value,
  onChange,
  placeholder = 'Password',
  autoComplete = 'current-password',
  minLength,
  required,
}: Props) {
  const [show, setShow] = useState(false);

  return (
    <div className="relative">
      <input
        className="w-full rounded-lg bg-dark-card border border-dark-border text-white placeholder-gray-500 p-3 pr-11 focus:outline-none focus:ring-2 focus:ring-brand-400"
        type={show ? 'text' : 'password'}
        placeholder={placeholder}
        value={value}
        onChange={(e) => onChange(e.target.value)}
        required={required}
        minLength={minLength}
        autoComplete={autoComplete}
      />
      <button
        type="button"
        onClick={() => setShow((s) => !s)}
        aria-label={show ? 'Hide password' : 'Show password'}
        aria-pressed={show}
        tabIndex={-1}
        className="absolute inset-y-0 right-0 flex items-center pr-3 text-gray-400 hover:text-white"
      >
        {show ? (
          // eye-off
          <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
            <path d="M17.94 17.94A10.07 10.07 0 0 1 12 20c-7 0-11-8-11-8a18.45 18.45 0 0 1 5.06-5.94M9.9 4.24A9.12 9.12 0 0 1 12 4c7 0 11 8 11 8a18.5 18.5 0 0 1-2.16 3.19m-6.72-1.07a3 3 0 1 1-4.24-4.24" />
            <line x1="1" y1="1" x2="23" y2="23" />
          </svg>
        ) : (
          // eye
          <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
            <path d="M1 12s4-8 11-8 11 8 11 8-4 8-11 8-11-8-11-8z" />
            <circle cx="12" cy="12" r="3" />
          </svg>
        )}
      </button>
    </div>
  );
}
