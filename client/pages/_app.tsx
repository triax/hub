import "bulma/css/bulma.min.css";

import type { AppProps } from 'next/app'
import { useEffect, useState } from 'react';

function App({ Component, pageProps, router }: AppProps) {
    const [myself, setMyself] = useState({});
    useEffect(() => {
        if (router.pathname == "/login") return;
        const endpoint = process.env.API_BASE_URL + "/api/1/users/current"
        fetch(endpoint).then(res => res.json()).then(res => setMyself(res));
    }, [router.pathname]);
    return <Component {...pageProps} myself={myself} />
}

export default App
