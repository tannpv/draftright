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
    <div className="bg-white rounded-xl shadow-sm border border-gray-200 overflow-hidden">
      <div className="overflow-x-auto">
        <table className="min-w-full divide-y divide-gray-200">
          <thead className="bg-gray-50">
            <tr>
              {columns.map((col) => (
                <th
                  key={String(col.key)}
                  className="px-6 py-3 text-left text-xs font-semibold text-gray-500 uppercase tracking-wider"
                >
                  {col.header}
                </th>
              ))}
            </tr>
          </thead>
          <tbody className="bg-white divide-y divide-gray-200">
            {loading ? (
              <tr>
                <td
                  colSpan={columns.length}
                  className="px-6 py-10 text-center text-gray-400 text-sm"
                >
                  Loading...
                </td>
              </tr>
            ) : rows.length === 0 ? (
              <tr>
                <td
                  colSpan={columns.length}
                  className="px-6 py-10 text-center text-gray-400 text-sm"
                >
                  {emptyMessage}
                </td>
              </tr>
            ) : (
              rows.map((row, i) => (
                <tr
                  key={i}
                  onClick={() => onRowClick?.(row)}
                  className={onRowClick ? 'cursor-pointer hover:bg-blue-50 transition-colors' : ''}
                >
                  {columns.map((col) => (
                    <td
                      key={String(col.key)}
                      className="px-6 py-4 whitespace-nowrap text-sm text-gray-700"
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
        <div className="flex items-center justify-between px-6 py-3 border-t border-gray-200 bg-gray-50">
          <p className="text-sm text-gray-500">
            Page {page} of {totalPages}
          </p>
          <div className="flex gap-2">
            <button
              onClick={() => onPageChange(page - 1)}
              disabled={page <= 1}
              className="px-3 py-1 rounded text-sm border border-gray-300 bg-white hover:bg-gray-50 disabled:opacity-40 disabled:cursor-not-allowed"
            >
              Previous
            </button>
            <button
              onClick={() => onPageChange(page + 1)}
              disabled={page >= totalPages}
              className="px-3 py-1 rounded text-sm border border-gray-300 bg-white hover:bg-gray-50 disabled:opacity-40 disabled:cursor-not-allowed"
            >
              Next
            </button>
          </div>
        </div>
      )}
    </div>
  );
}
