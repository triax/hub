import { useSearch } from "@tanstack/react-router";

export default function Login() {
  const search: Record<string, string> = useSearch({ strict: false });
  const destination = search.goto || "";
  return (
    <section className="h-screen w-screen flex items-center justify-center bg-gray-100">
      <form method="GET" action={`/auth/start`}>
        <h1 className="text-center font-bold text-xl py-2">Team Hub</h1>
        <input type="hidden" name="goto" value={destination} />
        <button type="submit">
          <img
            src="https://platform.slack-edge.com/img/sign_in_with_slack.png"
            srcSet="https://platform.slack-edge.com/img/sign_in_with_slack.png 1x, https://platform.slack-edge.com/img/sign_in_with_slack@2x.png 2x"
            alt="Sign in with Slack"
          />
        </button>
      </form>
    </section>
  )
}
