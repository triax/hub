import "../styles/globals.css";
import { StrictMode } from 'react';
import { createRoot } from 'react-dom/client';
import { RouterProvider } from '@tanstack/react-router';
import { router } from './router';
import { reportClientError } from '../repository/observability';

// Error Boundary で捕捉できない大域エラー（イベントハンドラ・非同期等）を報告する
window.addEventListener('error', (e) => {
  reportClientError({
    message: e.message || 'window.onerror',
    stack: e.error?.stack,
    url: e.filename ? `${e.filename}:${e.lineno}:${e.colno}` : undefined,
  });
});
window.addEventListener('unhandledrejection', (e) => {
  const reason = e.reason as { message?: string; stack?: string } | undefined;
  reportClientError({
    message: reason?.message || String(e.reason) || 'unhandledrejection',
    stack: reason?.stack,
  });
});

createRoot(document.getElementById('app')!).render(
  <StrictMode>
    <RouterProvider router={router} />
  </StrictMode>,
);
