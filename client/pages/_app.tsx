import "tailwindcss/tailwind.css";
import "../styles/globals.scss";

import type { AppProps } from 'next/app'
import { useEffect, useMemo, useState } from 'react';
import MemberRepo from "../repository/MemberRepo";
import Member from "../models/Member";

function App({ Component, pageProps, router }: AppProps) {
  const repo = useMemo(() => new MemberRepo(), []);
  const [isLoading, setIsLoading] = useState(false);
  const [myself, setMyself] = useState<Member>(Member.placeholder());
  useEffect(() => {
    switch (router.pathname) {
    case "/login": case "/_error": return;
    }
    repo.myself().then(setMyself);
  }, [router.pathname, repo]);
  return <Component
    {...pageProps}
    myself={myself}
    startLoading={() => setIsLoading(true)}
    stopLoading={() => setIsLoading(false)}
    isLoading={isLoading}
  />
}

export default App
