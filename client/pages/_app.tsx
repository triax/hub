import "tailwindcss/tailwind.css";
import "../styles/globals.scss";

import type { AppProps } from 'next/app'
import { useEffect, useState } from 'react';

function App({ Component, pageProps, router }: AppProps) {
  const [isLoading, setIsLoading] = useState(false);
  const [myself, setMyself] = useState({openid:{}, slack: {}});
  useEffect(() => {
    switch (router.pathname) {
    case "/login": return;
    case "/_error": return;
    }
    // TODO: Repositoryつくる
    const endpoint = process.env.API_BASE_URL + "/api/1/myself"
    fetch(endpoint).then(res => res.json()).then(res => setMyself(res));
  }, [router.pathname]);
  return <Component
    {...pageProps}
    myself={myself}
    startLoading={() => setIsLoading(true)}
    stopLoading={() => setIsLoading(false)}
    isLoading={isLoading}
  />
}

export default App
