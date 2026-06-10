/**
 * route 描画中に投げられた例外を捕捉するフォールバック UI。
 * TanStack Router の defaultErrorComponent として全 route に適用する。
 * サーバへの報告は router の defaultOnCatch が担う（componentStack を含められるため）。
 * 本コンポーネントは表示のみを担当する。
 */
export default function ErrorFallback() {
  return (
    <div className="min-h-screen flex flex-col items-center justify-center p-8 text-center">
      <div className="text-3xl mb-3">⚠️</div>
      <h1 className="text-lg font-bold text-gray-800 mb-2">エラーが発生しました</h1>
      <p className="text-sm text-gray-500 mb-6 max-w-xs">
        問題が開発チームに自動で報告されました。お手数ですが、ページの再読み込みをお試しください。
      </p>
      <button
        className="bg-blue-600 text-white px-4 py-2 rounded-md text-sm font-medium cursor-pointer"
        onClick={() => location.reload()}
      >
        再読み込み
      </button>
    </div>
  );
}
