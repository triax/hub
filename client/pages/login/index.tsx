
// https://api.slack.com/authentication/sign-in-with-slack#assets
// TODO: ここ、CSRFあるから、POSTのほうがいいと思う
export default function Login() {
  return (
    <section className="h-screen w-screen flex items-center justify-center bg-gray-100">
      <form method="GET" action="/auth/start">
        <h1 className="text-center font-bold text-xl py-2">Team Hub</h1>
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