import { ReactNode, useState } from 'react';

// eslint-disable-next-line @typescript-eslint/no-explicit-any
type AnyRow = any;

export type SortOrder = 'ASC' | 'DESC';

interface Column<T = AnyRow> {
  header: string;
  key: keyof T | string;
  render?: (row: T) => ReactNode;
  /** When set, header becomes clickable and emits onSortChange(sortKey, order). */
  sortKey?: string;
}

interface DataTableProps<T = AnyRow> {
  columns: Column<T>[];
  rows: T[];
  onRowClick?: (row: T) => void;
  page?: number;
  totalPages?: number;
  onPageChange?: (page: number) => void;
  /** Total row count across all pages (optional — used to render "Showing X-Y of Z"). */
  total?: number;
  /** Current rows-per-page; required to show range string + size selector. */
  pageSize?: number;
  onPageSizeChange?: (size: number) => void;
  /** Available page-size options. Default: [10, 25, 50, 100]. */
  pageSizeOptions?: number[];
  /** Currently sorted column's sortKey. */
  sortBy?: string;
  sortOrder?: SortOrder;
  /** Click on sortable header. Toggle: ASC → DESC → ASC … */
  onSortChange?: (sortBy: string, sortOrder: SortOrder) => void;
  loading?: boolean;
  emptyMessage?: string;
}

/**
 * Compute paginated page numbers with ellipsis.
 * Examples (current=4, total=10): [1, …, 3, 4, 5, …, 10]
 * Edge: total ≤ 7 → show all numbers without ellipsis.
 */
function getPageNumbers(current: number, total: number): (number | '…')[] {
  if (total <= 7) return Array.from({ length: total }, (_, i) => i + 1);
  const pages: (number | '…')[] = [1];
  if (current > 3) pages.push('…');
  const start = Math.max(2, current - 1);
  const end = Math.min(total - 1, current + 1);
  for (let i = start; i <= end; i++) pages.push(i);
  if (current < total - 2) pages.push('…');
  pages.push(total);
  return pages;
}

const btnBase: React.CSSProperties = {
  minWidth: 32,
  height: 30,
  padding: '0 10px',
  borderRadius: 6,
  fontSize: 13,
  border: '1px solid #333f55',
  background: 'transparent',
  color: '#7c8fac',
  cursor: 'pointer',
  fontFamily: 'inherit',
  transition: 'all 0.15s',
  display: 'inline-flex',
  alignItems: 'center',
  justifyContent: 'center',
};

export default function DataTable<T = AnyRow>({
  columns,
  rows,
  onRowClick,
  page = 1,
  totalPages = 1,
  onPageChange,
  total,
  pageSize,
  onPageSizeChange,
  pageSizeOptions = [10, 25, 50, 100],
  sortBy,
  sortOrder,
  onSortChange,
  loading = false,
  emptyMessage = 'No data found.',
}: DataTableProps<T>) {
  const [jumpInput, setJumpInput] = useState('');

  const pageNumbers = getPageNumbers(page, totalPages);
  const showRange = total !== undefined && pageSize !== undefined;
  const rangeStart = showRange ? Math.min((page - 1) * (pageSize || 1) + 1, total!) : 0;
  const rangeEnd = showRange ? Math.min(page * (pageSize || 1), total!) : 0;

  function handleJump(e: React.FormEvent) {
    e.preventDefault();
    const n = parseInt(jumpInput, 10);
    if (Number.isFinite(n) && n >= 1 && n <= totalPages && onPageChange) {
      onPageChange(n);
      setJumpInput('');
    }
  }

  // Show footer when caller wired pagination, even with 1 page (so range + size selector are visible).
  const showPagination = !!onPageChange && (totalPages > 0);

  return (
    <div style={{ background: '#2a3547', borderRadius: 7, overflow: 'hidden' }}>
      <div style={{ overflowX: 'auto' }}>
        <table style={{ width: '100%', borderCollapse: 'collapse' }}>

          {/* Head */}
          <thead>
            <tr style={{ borderBottom: '2px solid #333f55' }}>
              {columns.map((col) => {
                const isSortable = !!col.sortKey && !!onSortChange;
                const isActive = isSortable && sortBy === col.sortKey;
                const arrow = isActive ? (sortOrder === 'ASC' ? '▲' : '▼') : '';
                return (
                  <th
                    key={String(col.key)}
                    onClick={() => {
                      if (!isSortable || !col.sortKey) return;
                      const next: SortOrder = isActive && sortOrder === 'DESC' ? 'ASC' : 'DESC';
                      onSortChange!(col.sortKey, next);
                    }}
                    style={{
                      padding: '13px 20px',
                      textAlign: 'left',
                      color: isActive ? '#5d87ff' : '#7c8fac',
                      fontSize: 12,
                      fontWeight: 700,
                      textTransform: 'uppercase',
                      letterSpacing: '0.05em',
                      whiteSpace: 'nowrap',
                      cursor: isSortable ? 'pointer' : 'default',
                      userSelect: 'none',
                    }}
                  >
                    {col.header}
                    {isSortable && (
                      <span style={{ marginLeft: 6, fontSize: 10, opacity: isActive ? 1 : 0.3 }}>
                        {arrow || '⇅'}
                      </span>
                    )}
                  </th>
                );
              })}
            </tr>
          </thead>

          {/* Body */}
          <tbody>
            {loading ? (
              <tr>
                <td
                  colSpan={columns.length}
                  style={{ padding: '40px 20px', textAlign: 'center', color: '#7c8fac', fontSize: 14 }}
                >
                  <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'center', gap: 10 }}>
                    <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="#5d87ff" strokeWidth="2" strokeLinecap="round">
                      <path d="M12 2v4M12 18v4M4.93 4.93l2.83 2.83M16.24 16.24l2.83 2.83M2 12h4M18 12h4M4.93 19.07l2.83-2.83M16.24 7.76l2.83-2.83">
                        <animateTransform attributeName="transform" type="rotate" from="0 12 12" to="360 12 12" dur="1s" repeatCount="indefinite" />
                      </path>
                    </svg>
                    Loading...
                  </div>
                </td>
              </tr>
            ) : rows.length === 0 ? (
              <tr>
                <td
                  colSpan={columns.length}
                  style={{ padding: '40px 20px', textAlign: 'center', color: '#7c8fac', fontSize: 14 }}
                >
                  {emptyMessage}
                </td>
              </tr>
            ) : (
              rows.map((row, i) => (
                <tr
                  key={i}
                  onClick={() => onRowClick?.(row)}
                  style={{
                    borderBottom: '1px solid #333f55',
                    cursor: onRowClick ? 'pointer' : 'default',
                    transition: 'background 0.15s',
                  }}
                  onMouseEnter={(e) => {
                    if (onRowClick) {
                      (e.currentTarget as HTMLTableRowElement).style.background = 'rgba(93,135,255,0.05)';
                    }
                  }}
                  onMouseLeave={(e) => {
                    (e.currentTarget as HTMLTableRowElement).style.background = 'transparent';
                  }}
                >
                  {columns.map((col) => (
                    <td
                      key={String(col.key)}
                      style={{
                        padding: '14px 20px',
                        whiteSpace: 'nowrap',
                        fontSize: 14,
                        color: '#eaeff4',
                      }}
                    >
                      {col.render
                        ? col.render(row)
                        : String(row[col.key as keyof T] ?? '')}
                    </td>
                  ))}
                </tr>
              ))
            )}
          </tbody>
        </table>
      </div>

      {/* Pagination */}
      {showPagination && (
        <div
          style={{
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'space-between',
            padding: '12px 16px',
            borderTop: '1px solid #333f55',
            gap: 16,
            flexWrap: 'wrap',
          }}
        >
          {/* Left — range + page-size selector */}
          <div style={{ display: 'flex', alignItems: 'center', gap: 14, color: '#7c8fac', fontSize: 13 }}>
            {showRange ? (
              <span>
                Showing <b style={{ color: '#eaeff4' }}>{rangeStart}–{rangeEnd}</b> of <b style={{ color: '#eaeff4' }}>{total}</b>
              </span>
            ) : (
              <span>Page <b style={{ color: '#eaeff4' }}>{page}</b> of <b style={{ color: '#eaeff4' }}>{totalPages}</b></span>
            )}

            {pageSize !== undefined && onPageSizeChange && (
              <label style={{ display: 'flex', alignItems: 'center', gap: 6, fontSize: 12 }}>
                Rows
                <select
                  value={pageSize}
                  onChange={(e) => onPageSizeChange(parseInt(e.target.value, 10))}
                  style={{
                    padding: '4px 8px',
                    borderRadius: 5,
                    border: '1px solid #333f55',
                    background: '#202936',
                    color: '#eaeff4',
                    fontSize: 12,
                    fontFamily: 'inherit',
                    cursor: 'pointer',
                  }}
                >
                  {pageSizeOptions.map((n) => (
                    <option key={n} value={n}>{n}</option>
                  ))}
                </select>
              </label>
            )}
          </div>

          {/* Right — nav buttons */}
          <div style={{ display: 'flex', alignItems: 'center', gap: 4 }}>
            {/* First */}
            <button
              onClick={() => onPageChange(1)}
              disabled={page <= 1}
              title="First page"
              style={{
                ...btnBase,
                color: page <= 1 ? '#333f55' : '#7c8fac',
                cursor: page <= 1 ? 'not-allowed' : 'pointer',
              }}
            >
              «
            </button>

            {/* Previous */}
            <button
              onClick={() => onPageChange(page - 1)}
              disabled={page <= 1}
              title="Previous page"
              style={{
                ...btnBase,
                color: page <= 1 ? '#333f55' : '#7c8fac',
                cursor: page <= 1 ? 'not-allowed' : 'pointer',
              }}
            >
              ‹
            </button>

            {/* Numbered pages */}
            {pageNumbers.map((p, idx) => (
              p === '…' ? (
                <span key={`e-${idx}`} style={{ color: '#7c8fac', padding: '0 6px', fontSize: 13 }}>…</span>
              ) : (
                <button
                  key={p}
                  onClick={() => onPageChange(p)}
                  style={{
                    ...btnBase,
                    background: p === page ? 'rgba(93,135,255,0.15)' : 'transparent',
                    color: p === page ? '#5d87ff' : '#7c8fac',
                    borderColor: p === page ? 'rgba(93,135,255,0.4)' : '#333f55',
                    fontWeight: p === page ? 600 : 400,
                  }}
                >
                  {p}
                </button>
              )
            ))}

            {/* Next */}
            <button
              onClick={() => onPageChange(page + 1)}
              disabled={page >= totalPages}
              title="Next page"
              style={{
                ...btnBase,
                color: page >= totalPages ? '#333f55' : '#7c8fac',
                cursor: page >= totalPages ? 'not-allowed' : 'pointer',
              }}
            >
              ›
            </button>

            {/* Last */}
            <button
              onClick={() => onPageChange(totalPages)}
              disabled={page >= totalPages}
              title="Last page"
              style={{
                ...btnBase,
                color: page >= totalPages ? '#333f55' : '#7c8fac',
                cursor: page >= totalPages ? 'not-allowed' : 'pointer',
              }}
            >
              »
            </button>

            {/* Jump-to-page (only when many pages) */}
            {totalPages > 7 && (
              <form onSubmit={handleJump} style={{ display: 'flex', alignItems: 'center', gap: 6, marginLeft: 8 }}>
                <span style={{ color: '#7c8fac', fontSize: 12 }}>Go to</span>
                <input
                  type="number"
                  min={1}
                  max={totalPages}
                  value={jumpInput}
                  onChange={(e) => setJumpInput(e.target.value)}
                  placeholder={String(page)}
                  style={{
                    width: 56,
                    height: 30,
                    padding: '0 8px',
                    borderRadius: 5,
                    border: '1px solid #333f55',
                    background: '#202936',
                    color: '#eaeff4',
                    fontSize: 13,
                    fontFamily: 'inherit',
                    outline: 'none',
                    textAlign: 'center',
                  }}
                />
              </form>
            )}
          </div>
        </div>
      )}
    </div>
  );
}
