import { useState, useRef, useEffect, ChangeEvent, FormEvent } from 'react';
import Toast from './Toast';
import { getAdminEmail } from '../auth';

const API_URL = import.meta.env.VITE_API_URL || 'http://localhost:3000';
const APP_VERSION = (import.meta.env.VITE_APP_VERSION as string | undefined) || 'admin-portal-2.0.0';
const MAX_FILE_BYTES = 5 * 1024 * 1024; // 5 MB
const MIN_DESC_LEN = 10;
const ALLOWED_TYPES = ['image/png', 'image/jpeg'];

type ToastState = { message: string; type: 'success' | 'error' } | null;

export default function ReportBugButton() {
  const [open, setOpen] = useState(false);
  const [description, setDescription] = useState('');
  const [file, setFile] = useState<File | null>(null);
  const [previewUrl, setPreviewUrl] = useState<string | null>(null);
  const [submitting, setSubmitting] = useState(false);
  const [validationError, setValidationError] = useState<string | null>(null);
  const [toast, setToast] = useState<ToastState>(null);
  const [isDragging, setIsDragging] = useState(false);
  const fileInputRef = useRef<HTMLInputElement>(null);

  // Validate + accept a File from any source (input, drop, clipboard).
  function acceptFile(f: File | null | undefined): boolean {
    if (!f) return false;
    if (!ALLOWED_TYPES.includes(f.type)) {
      setValidationError('Screenshot must be a PNG or JPEG image.');
      return false;
    }
    if (f.size > MAX_FILE_BYTES) {
      setValidationError('Screenshot must be under 5 MB.');
      return false;
    }
    setValidationError(null);
    setFile(f);
    return true;
  }

  // Manage object URL lifecycle
  useEffect(() => {
    if (!file) {
      setPreviewUrl(null);
      return;
    }
    const url = URL.createObjectURL(file);
    setPreviewUrl(url);
    return () => URL.revokeObjectURL(url);
  }, [file]);

  // Esc closes modal
  useEffect(() => {
    if (!open) return;
    const handleKey = (e: KeyboardEvent) => {
      if (e.key === 'Escape' && !submitting) closeModal();
    };
    document.addEventListener('keydown', handleKey);
    return () => document.removeEventListener('keydown', handleKey);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [open, submitting]);

  // Paste support: while modal is open, intercept clipboard images.
  useEffect(() => {
    if (!open) return;
    const handlePaste = (e: ClipboardEvent) => {
      if (submitting) return;
      const items = e.clipboardData?.items;
      if (!items) return;
      for (let i = 0; i < items.length; i++) {
        const it = items[i];
        if (it.kind === 'file' && it.type.startsWith('image/')) {
          const blob = it.getAsFile();
          if (!blob) continue;
          // Clipboard images often come as 'image/png' but with no name; synthesize one.
          const ext = blob.type === 'image/jpeg' ? 'jpg' : 'png';
          const named = new File([blob], `pasted-${Date.now()}.${ext}`, { type: blob.type });
          if (acceptFile(named)) {
            e.preventDefault();
            break;
          }
        }
      }
    };
    document.addEventListener('paste', handlePaste);
    return () => document.removeEventListener('paste', handlePaste);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [open, submitting]);

  function resetForm() {
    setDescription('');
    setFile(null);
    setValidationError(null);
    if (fileInputRef.current) fileInputRef.current.value = '';
  }

  function closeModal() {
    setOpen(false);
    resetForm();
  }

  function handleFileChange(e: ChangeEvent<HTMLInputElement>) {
    const f = e.target.files?.[0] ?? null;
    if (!f) {
      setFile(null);
      return;
    }
    if (!acceptFile(f)) {
      e.target.value = '';
      setFile(null);
    }
  }

  function handleDrop(e: React.DragEvent<HTMLDivElement>) {
    e.preventDefault();
    e.stopPropagation();
    setIsDragging(false);
    if (submitting) return;
    const f = e.dataTransfer.files?.[0];
    if (f) acceptFile(f);
  }

  async function handleSubmit(e: FormEvent) {
    e.preventDefault();
    const trimmed = description.trim();
    if (trimmed.length < MIN_DESC_LEN) {
      setValidationError(`Please describe what happened (at least ${MIN_DESC_LEN} characters).`);
      return;
    }
    setValidationError(null);
    setSubmitting(true);

    const adminEmail = getAdminEmail();
    const fd = new FormData();
    fd.append('description', trimmed);
    fd.append('source', 'admin-portal');
    fd.append('app_version', APP_VERSION);
    fd.append('os_info', navigator.userAgent);
    fd.append(
      'context',
      JSON.stringify({
        url: window.location.href,
        route: window.location.pathname,
      }),
    );
    if (adminEmail && adminEmail !== 'Admin') {
      fd.append('user_email', adminEmail);
    }
    if (file) {
      fd.append('screenshot', file);
    }

    const token = localStorage.getItem('token');
    try {
      const res = await fetch(`${API_URL}/bug-reports`, {
        method: 'POST',
        body: fd,
        headers: token ? { Authorization: `Bearer ${token}` } : undefined,
      });
      if (!res.ok) {
        let errMsg = `Submission failed (${res.status})`;
        try {
          const data = await res.json();
          if (data?.error) errMsg = data.error;
          else if (data?.message) errMsg = Array.isArray(data.message) ? data.message.join(', ') : data.message;
        } catch {
          // ignore parse errors
        }
        setToast({ message: errMsg, type: 'error' });
        setSubmitting(false);
        return;
      }
      setToast({ message: "Thanks! We'll look into it.", type: 'success' });
      setSubmitting(false);
      closeModal();
    } catch (err) {
      const msg = err instanceof Error ? err.message : 'Network error';
      setToast({ message: msg, type: 'error' });
      setSubmitting(false);
    }
  }

  return (
    <>
      {/* Floating launcher button */}
      <button
        onClick={() => setOpen(true)}
        title="Report a bug"
        style={{
          position: 'fixed',
          bottom: 24,
          right: 24,
          zIndex: 150,
          display: 'flex',
          alignItems: 'center',
          gap: 8,
          padding: '10px 16px',
          borderRadius: 999,
          border: '1px solid #333f55',
          background: '#2a3547',
          color: '#eaeff4',
          fontSize: 13,
          fontWeight: 600,
          fontFamily: 'inherit',
          cursor: 'pointer',
          boxShadow: '0 8px 24px rgba(0,0,0,0.35)',
          transition: 'all 0.15s',
        }}
        onMouseEnter={(e) => {
          (e.currentTarget as HTMLButtonElement).style.background = '#333f55';
          (e.currentTarget as HTMLButtonElement).style.borderColor = '#5d87ff';
        }}
        onMouseLeave={(e) => {
          (e.currentTarget as HTMLButtonElement).style.background = '#2a3547';
          (e.currentTarget as HTMLButtonElement).style.borderColor = '#333f55';
        }}
      >
        <span style={{ fontSize: 15, lineHeight: 1 }} aria-hidden="true">🐛</span>
        <span>Report bug</span>
      </button>

      {open && (
        <div
          style={{
            position: 'fixed',
            inset: 0,
            zIndex: 200,
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'center',
            padding: 16,
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
            onClick={() => {
              if (!submitting) closeModal();
            }}
          />

          {/* Dialog */}
          <form
            onSubmit={handleSubmit}
            style={{
              position: 'relative',
              background: '#2a3547',
              border: '1px solid #333f55',
              borderRadius: 7,
              width: '100%',
              maxWidth: 540,
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
              <div>
                <h2 style={{ color: '#eaeff4', fontSize: 16, fontWeight: 600, margin: 0 }}>
                  Report a bug
                </h2>
                <p style={{ color: '#7c8fac', fontSize: 12, margin: '4px 0 0' }}>
                  Help us improve DraftRight by sharing what went wrong.
                </p>
              </div>
              <button
                type="button"
                onClick={() => !submitting && closeModal()}
                disabled={submitting}
                style={{
                  background: 'transparent',
                  border: 'none',
                  color: '#7c8fac',
                  cursor: submitting ? 'not-allowed' : 'pointer',
                  fontSize: 22,
                  lineHeight: 1,
                  padding: '0 4px',
                }}
              >
                &times;
              </button>
            </div>

            {/* Body */}
            <div style={{ padding: '20px 22px', display: 'flex', flexDirection: 'column', gap: 16 }}>
              {/* Description */}
              <div>
                <label
                  htmlFor="bug-description"
                  style={{
                    display: 'block',
                    color: '#eaeff4',
                    fontSize: 13,
                    fontWeight: 600,
                    marginBottom: 6,
                  }}
                >
                  What happened? <span style={{ color: '#fa896b' }}>*</span>
                </label>
                <textarea
                  id="bug-description"
                  value={description}
                  onChange={(e) => setDescription(e.target.value)}
                  placeholder="I clicked X and Y broke..."
                  rows={5}
                  required
                  minLength={MIN_DESC_LEN}
                  disabled={submitting}
                  style={{
                    width: '100%',
                    boxSizing: 'border-box',
                    background: '#202936',
                    color: '#eaeff4',
                    border: '1px solid #333f55',
                    borderRadius: 6,
                    padding: '10px 12px',
                    fontSize: 13,
                    fontFamily: 'inherit',
                    resize: 'vertical',
                    minHeight: 100,
                    outline: 'none',
                  }}
                  onFocus={(e) => { e.currentTarget.style.borderColor = '#5d87ff'; }}
                  onBlur={(e) => { e.currentTarget.style.borderColor = '#333f55'; }}
                />
                <p style={{ color: '#7c8fac', fontSize: 11, margin: '6px 0 0' }}>
                  Minimum {MIN_DESC_LEN} characters.
                </p>
              </div>

              {/* Screenshot upload — file picker + drag-drop + clipboard paste */}
              <div>
                <label
                  style={{
                    display: 'block',
                    color: '#eaeff4',
                    fontSize: 13,
                    fontWeight: 600,
                    marginBottom: 6,
                  }}
                >
                  Attach screenshot <span style={{ color: '#7c8fac', fontWeight: 400 }}>(optional)</span>
                </label>

                {!previewUrl ? (
                  <div
                    onDragEnter={(e) => { e.preventDefault(); e.stopPropagation(); if (!submitting) setIsDragging(true); }}
                    onDragOver={(e) => { e.preventDefault(); e.stopPropagation(); if (!submitting) setIsDragging(true); }}
                    onDragLeave={(e) => { e.preventDefault(); e.stopPropagation(); setIsDragging(false); }}
                    onDrop={handleDrop}
                    onClick={() => !submitting && fileInputRef.current?.click()}
                    style={{
                      border: `2px dashed ${isDragging ? '#5d87ff' : '#333f55'}`,
                      background: isDragging ? 'rgba(93,135,255,0.06)' : '#202936',
                      borderRadius: 8,
                      padding: '20px 16px',
                      textAlign: 'center',
                      cursor: submitting ? 'not-allowed' : 'pointer',
                      transition: 'all 0.15s',
                    }}
                  >
                    <div style={{ color: isDragging ? '#5d87ff' : '#eaeff4', fontSize: 13, fontWeight: 500, marginBottom: 4 }}>
                      {isDragging ? 'Drop image here' : '🖼️  Drag & drop, paste (⌘V), or click to browse'}
                    </div>
                    <div style={{ color: '#7c8fac', fontSize: 11 }}>
                      PNG or JPEG, max 5 MB
                    </div>
                  </div>
                ) : (
                  <div style={{ position: 'relative' }}>
                    <img
                      src={previewUrl}
                      alt="Screenshot preview"
                      style={{
                        maxWidth: '100%',
                        maxHeight: 240,
                        borderRadius: 8,
                        border: '1px solid #333f55',
                        display: 'block',
                      }}
                    />
                    <div style={{ display: 'flex', gap: 12, alignItems: 'center', marginTop: 8 }}>
                      <span style={{ color: '#7c8fac', fontSize: 11 }}>
                        {file ? `${file.name} · ${(file.size / 1024).toFixed(1)} KB` : ''}
                      </span>
                      <button
                        type="button"
                        onClick={() => {
                          setFile(null);
                          if (fileInputRef.current) fileInputRef.current.value = '';
                        }}
                        disabled={submitting}
                        style={{
                          marginLeft: 'auto',
                          background: 'transparent',
                          border: 'none',
                          color: '#fa896b',
                          fontSize: 12,
                          cursor: submitting ? 'not-allowed' : 'pointer',
                          padding: 0,
                          fontFamily: 'inherit',
                        }}
                      >
                        Remove
                      </button>
                    </div>
                  </div>
                )}

                {/* Hidden native input — opened by clicking the dropzone */}
                <input
                  ref={fileInputRef}
                  id="bug-screenshot"
                  type="file"
                  accept="image/png,image/jpeg"
                  onChange={handleFileChange}
                  disabled={submitting}
                  style={{ display: 'none' }}
                />
              </div>

              {validationError && (
                <div
                  style={{
                    background: 'rgba(250,137,107,0.12)',
                    border: '1px solid rgba(250,137,107,0.4)',
                    borderRadius: 6,
                    padding: '8px 12px',
                    color: '#fa896b',
                    fontSize: 12,
                  }}
                >
                  {validationError}
                </div>
              )}
            </div>

            {/* Footer */}
            <div
              style={{
                display: 'flex',
                justifyContent: 'flex-end',
                gap: 10,
                padding: '14px 22px',
                borderTop: '1px solid #333f55',
              }}
            >
              <button
                type="button"
                onClick={closeModal}
                disabled={submitting}
                style={{
                  padding: '8px 16px',
                  borderRadius: 6,
                  border: '1px solid #333f55',
                  background: 'transparent',
                  color: '#eaeff4',
                  fontSize: 13,
                  fontWeight: 500,
                  fontFamily: 'inherit',
                  cursor: submitting ? 'not-allowed' : 'pointer',
                }}
              >
                Cancel
              </button>
              <button
                type="submit"
                disabled={submitting || description.trim().length < MIN_DESC_LEN}
                style={{
                  padding: '8px 18px',
                  borderRadius: 6,
                  border: 'none',
                  background: submitting || description.trim().length < MIN_DESC_LEN ? 'rgba(93,135,255,0.5)' : '#5d87ff',
                  color: '#fff',
                  fontSize: 13,
                  fontWeight: 600,
                  fontFamily: 'inherit',
                  cursor: submitting || description.trim().length < MIN_DESC_LEN ? 'not-allowed' : 'pointer',
                  display: 'inline-flex',
                  alignItems: 'center',
                  gap: 8,
                }}
              >
                {submitting ? 'Submitting...' : 'Submit report'}
              </button>
            </div>
          </form>
        </div>
      )}

      {toast && (
        <Toast
          message={toast.message}
          type={toast.type}
          onClose={() => setToast(null)}
        />
      )}
    </>
  );
}
