import { SelectQueryBuilder, ObjectLiteral } from 'typeorm';

/**
 * Standard query shape for paginated/searchable/sortable list endpoints.
 * Sent as URL params from the admin frontend.
 *
 * Conventions:
 *   - `search` — case-insensitive ILIKE across explicitly listed columns.
 *   - `status` — 'all' | 'active' | 'inactive' — applied to entity.is_active.
 *   - `sort_by` + `sort_order` — column name + 'ASC' | 'DESC'.
 *     Caller MUST validate sort_by against an allow-list (SQL-injection guard).
 *   - `page` 1-indexed, `limit` rows per page (default 10, capped at 100).
 */
export interface ListQuery {
  search?: string;
  status?: 'all' | 'active' | 'inactive';
  sort_by?: string;
  sort_order?: 'ASC' | 'DESC';
  page?: number;
  limit?: number;
}

export interface ListResult<T> {
  rows: T[];
  total: number;
}

/**
 * Apply ListQuery to a TypeORM QueryBuilder.
 *
 * @param qb            QueryBuilder pre-configured with joins/relations.
 * @param query         Parsed ListQuery from controller.
 * @param searchColumns Columns to ILIKE-match against `search` (use alias.field syntax,
 *                      e.g. ['plan.name', 'plan.currency']).
 * @param sortAllowList Allowed sort column names mapped to alias.field
 *                      (e.g. { name: 'plan.name', price: 'plan.price_cents' }).
 * @param defaultSort   Used when no sort_by provided. e.g. 'plan.created_at'.
 * @param statusColumn  Where to apply is_active filter (default 'is_active' on root alias).
 *                      Pass null to disable status filter for this entity.
 */
export async function applyListQuery<T extends ObjectLiteral>(
  qb: SelectQueryBuilder<T>,
  query: ListQuery,
  searchColumns: string[],
  sortAllowList: Record<string, string>,
  defaultSort: string,
  statusColumn: string | null = null,
): Promise<ListResult<T>> {
  // ── Search (ILIKE across columns) ────────────────────────────
  if (query.search?.trim() && searchColumns.length > 0) {
    const term = `%${query.search.trim()}%`;
    const orClauses = searchColumns.map((c, i) => `${c} ILIKE :s${i}`).join(' OR ');
    const params: Record<string, string> = {};
    searchColumns.forEach((_, i) => { params[`s${i}`] = term; });
    qb.andWhere(`(${orClauses})`, params);
  }

  // ── Status filter ───────────────────────────────────────────
  if (statusColumn && query.status && query.status !== 'all') {
    qb.andWhere(`${statusColumn} = :is_active`, { is_active: query.status === 'active' });
  }

  // ── Sort ─────────────────────────────────────────────────────
  // Allow-list lookup prevents arbitrary SQL injection via sort_by.
  const sortField = (query.sort_by && sortAllowList[query.sort_by]) || defaultSort;
  const sortOrder = query.sort_order === 'ASC' ? 'ASC' : 'DESC';
  qb.orderBy(sortField, sortOrder);

  // ── Pagination ──────────────────────────────────────────────
  const page = Math.max(1, query.page || 1);
  const limit = Math.min(100, Math.max(1, query.limit || 10));
  qb.skip((page - 1) * limit).take(limit);

  const [rows, total] = await qb.getManyAndCount();
  return { rows, total };
}

/**
 * Coerce raw query-string values to ListQuery shape.
 * Controllers receive `@Query() q: any` (string fields); this normalizes them.
 */
export function parseListQuery(q: Record<string, unknown> | undefined): ListQuery {
  const raw = q || {};
  const status = raw.status as string | undefined;
  const sort_order = (raw.sort_order as string | undefined)?.toUpperCase();
  return {
    search: typeof raw.search === 'string' ? raw.search : undefined,
    status: status === 'active' || status === 'inactive' || status === 'all' ? status : undefined,
    sort_by: typeof raw.sort_by === 'string' ? raw.sort_by : undefined,
    sort_order: sort_order === 'ASC' || sort_order === 'DESC' ? sort_order : undefined,
    page: raw.page ? parseInt(String(raw.page), 10) : undefined,
    limit: raw.limit ? parseInt(String(raw.limit), 10) : undefined,
  };
}
