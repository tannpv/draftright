/**
 * The DraftRight backend base URL. Single source of truth for every island
 * and page — set `PUBLIC_API_URL` at build time to override; otherwise it
 * defaults to production so a build that forgets the env var still works.
 */
export const API_URL =
  (import.meta.env.PUBLIC_API_URL as string | undefined) || 'https://api.draftright.info';
