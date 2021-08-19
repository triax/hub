import "bulma/css/bulma.min.css";

import type { AppProps } from 'next/app'
import { useEffect, useState } from 'react';

function App({ Component, pageProps, router }: AppProps) {
    const [myself, setMyself] = useState({});
    useEffect(() => {
        if (router.pathname == "/login") return;
        fetch("http://localhost:8080/api/1/users/current")
            .then(res => res.json())
            .then(res => setMyself(res));
    }, [router.pathname])
    return <Component {...pageProps} myself={myself} />
}

export default App
