/**
 * fetch のラッパー。HTTPステータスを検証し、エラー時に例外を投げる。
 */
export async function fetchJSON<T>(endpoint: string, init?: RequestInit): Promise<T> {
  if (init?.body && !init.headers) {
    init = {
      ...init,
      headers: { "Content-Type": "application/json" },
    };
  }
  const res = await fetch(endpoint, init);
  if (!res.ok) {
    throw new Error(`HTTP ${res.status}: ${res.statusText}`);
  }
  return res.json();
}
