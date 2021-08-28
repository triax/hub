module.exports = (phase, { defaultConfig }) => {
  /**
   * @type {import('next').NextConfig}
   */
  const nextConfig = {
    images: {
      domains: [
        'avatars.slack-edge.com',
        'a.slack-edge.com',
        'secure.gravatar.com',
      ],
    },
    // build-timeに評価されることに注意
    env: {
      API_BASE_URL: process.env.NODE_ENV == "production" ? "" : "http://localhost:8080",
    },
    /** useFileSystemPublicRoutes について
     * - https://nextjs.org/docs/advanced-features/custom-server#disabling-file-system-routing
     * - https://github.com/vercel/next.js/issues/2682#issuecomment-370664352
     * これ、nodejsのnextライブラリをつくってproductionサーバ建てるときの仕様で、
     * Goで認証/非認証判定するサーバ書く限りはあんまり意味ない。
     * @See https://github.com/vercel/next.js/search?q=useFileSystemPublicRoutes
     * なので、コンポーネントで `next/link` の <Link> を使うのではなくて、
     * nativeな <a> を使うことで、pushStateを阻止し、必ずサーバにGETリクエストが飛ぶようにした。
     * lintで、 `next/link`を使えというwarning出るが、それはeslintrcで黙らせました。
     * しゃーないやろ、Node.jsでサーバ建てたくないんやから。
     */
    // useFileSystemPublicRoutes: false,
  }
  return nextConfig
}
