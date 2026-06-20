import { maskSecret } from '../common/mask-secret.util';

// Returns a shallow copy with api_key masked. NEVER mutate the loaded entity —
// the provider-strategy registry reads the real api_key off the same object to
// call the upstream API. Mirrors Go AiProvider.MarshalJSON (masks on serialize).
export function maskProvider<T extends { api_key?: string }>(p: T): T {
  if (!p) return p;
  return { ...p, api_key: maskSecret(p.api_key ?? '') };
}
