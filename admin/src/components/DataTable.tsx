import { ReactNode } from 'react';

// eslint-disable-next-line @typescript-eslint/no-explicit-any
type AnyRow = any;

interface Column<T = AnyRow> {
  header: string;
  key: keyof T | string;
  render?: (row: T) => ReactNode;
}

interface DataTableProps<T = AnyRow> {
  columns: Column<T>[];
  rows: T[];
  onRowClick?: (row: T) => void;
  page?: number;
  totalPages?: number;
  onPageChange?: (page: number) => void;
  loading?: boolean;
  emptyMessage?: string;
}

export default function DataTable<T = AnyRow>({
  columns,
  rows,
  onRowClick,
  page = 1,
  totalPages = 1,
  onPageChange,
  loading = false,
  emptyMessage = 'No data found.',
}: DataTableProps<T>) {
  return (
    <div style={{ background: '#2a3547', borderRadius: 7, overflow: 'hidden' }}>
      <div style={{ overflowX: 'auto' }}>
        <table style={{ width: '100%', borderCollapse: 'collapse' }}>

          {/* Head */}
          <thead>
            <tr style={{ borderBottom: '2px solid #333f55' }}>
              {columns.map((col) => (
                <th
                  key={String(col.key)}
                  style={{
                    padding: '13px 20px',
                    textAlign: 'left',
                    color: '#7c8fac',
                    fontSize: 12,
                    fontWeight: 700,
                    textTransform: 'uppercase',
                    letterSpacing: '0.05em',
                    whiteSpace: 'nowrap',
                  }}
                >
                  {col.header}
                </th>
              ))}
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
      {totalPages > 1 && onPageChange && (
        <div
          style={{
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'space-between',
            padding: '12px 20px',
            borderTop: '1px solid #333f55',
          }}
        >
          <p style={{ color: '#7c8fac', fontSize: 13, margin: 0 }}>
            Page {page} of {totalPages}
          </p>
          <div style={{ display: 'flex', gap: 8 }}>
            <button
              onClick={() => onPageChange(page - 1)}
              disabled={page <= 1}
              style={{
                padding: '5px 14px',
                borderRadius: 7,
                fontSize: 13,
                border: '1px solid #333f55',
                background: 'transparent',
                color: page <= 1 ? '#333f55' : '#7c8fac',
                cursor: page <= 1 ? 'not-allowed' : 'pointer',
                fontFamily: 'inherit',
                transition: 'all 0.15s',
              }}
            >
              Previous
            </button>
            <button
              onClick={() => onPageChange(page + 1)}
              disabled={page >= totalPages}
              style={{
                padding: '5px 14px',
                borderRadius: 7,
                fontSize: 13,
                border: '1px solid #333f55',
                background: 'transparent',
                color: page >= totalPages ? '#333f55' : '#7c8fac',
                cursor: page >= totalPages ? 'not-allowed' : 'pointer',
                fontFamily: 'inherit',
                transition: 'all 0.15s',
              }}
            >
              Next
            </button>
          </div>
        </div>
      )}
    </div>
  );
}
