/**
 * フロントエンドのエラーをサーバ（/api/1/client-errors）へ報告する。
 * 報告経路は決して throw しない（エラー報告でアプリを二次的に壊さない）。
 */

export interface ClientErrorInput {
  message: string;
  stack?: string;
  url?: string;
  ts?: number;
}

// 同一メッセージの短時間連投を抑制する（unhandledrejection の連鎖等によるノイズ対策）。
let lastMessage = "";
let lastSentAt = 0;
const DEDUP_WINDOW_MS = 10_000;

export function reportClientError(input: ClientErrorInput): void {
  try {
    const now = Date.now();
    if (input.message === lastMessage && now - lastSentAt < DEDUP_WINDOW_MS) return;
    lastMessage = input.message;
    lastSentAt = now;

    const env = import.meta.env as Record<string, string | undefined>;
    const payload = {
      message: input.message,
      stack: input.stack,
      url: input.url ?? (typeof location !== "undefined" ? location.href : undefined),
      userAgent: typeof navigator !== "undefined" ? navigator.userAgent : undefined,
      release: env?.VITE_RELEASE,
      ts: input.ts ?? now,
    };

    // keepalive: ページ遷移・クラッシュ中でも送信を試みる
    void fetch("/api/1/client-errors", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(payload),
      keepalive: true,
    }).catch(() => {
      /* 報告の失敗は握りつぶす */
    });
  } catch {
    /* 報告経路で発生した例外は無視する */
  }
}
